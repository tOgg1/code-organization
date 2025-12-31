package partial

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tormodhaugland/co/internal/template"
)

// Apply applies a partial to a target directory.
// This is the main entry point for partial application.
func Apply(opts ApplyOptions, partialsDirs []string) (*ApplyResult, error) {
	// 1. Load partial
	p, partialPath, err := LoadPartialByName(opts.PartialName, partialsDirs)
	if err != nil {
		return nil, err
	}

	// 2. Validate target directory
	absTargetPath, err := filepath.Abs(opts.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("resolving target path: %w", err)
	}

	if err := ValidateTargetPath(absTargetPath); err != nil {
		return nil, err
	}

	// 3. Resolve variables (user-provided + defaults + builtins)
	resolvedVars, err := resolveVariables(p, opts.Variables, absTargetPath)
	if err != nil {
		return nil, err
	}

	// 4. Validate all required variables are provided
	if err := validateVariables(p, resolvedVars); err != nil {
		return nil, err
	}

	// 5. Scan partial files
	files, err := ScanPartialFiles(partialPath, p.Files)
	if err != nil {
		return nil, fmt.Errorf("scanning partial files: %w", err)
	}

	// 6. Determine conflict strategy
	conflictStrategy := p.GetConflictStrategy()
	if opts.ConflictStrategy != "" {
		if !IsValidConflictStrategy(opts.ConflictStrategy) {
			return nil, &ValidationError{
				Field:  "conflict_strategy",
				Reason: fmt.Sprintf("invalid strategy: %s", opts.ConflictStrategy),
			}
		}
		conflictStrategy = opts.ConflictStrategy
	}

	// For non-interactive mode (Yes flag), convert "prompt" to "skip"
	if opts.Yes && conflictStrategy == string(StrategyPrompt) {
		conflictStrategy = string(StrategySkip)
	}

	// 7. Detect conflicts
	extensions := p.GetTemplateExtensions()
	conflictConfig := ConflictConfig{
		Strategy: conflictStrategy,
		Preserve: p.Conflicts.Preserve,
	}

	plan, err := DetectConflicts(files, partialPath, absTargetPath, conflictConfig, extensions)
	if err != nil {
		return nil, fmt.Errorf("detecting conflicts: %w", err)
	}

	// Initialize result
	result := &ApplyResult{
		PartialName:      opts.PartialName,
		TargetPath:       absTargetPath,
		FilesCreated:     []string{},
		FilesSkipped:     []string{},
		FilesOverwritten: []string{},
		FilesMerged:      []string{},
		FilesBackedUp:    []string{},
		HooksRun:         []string{},
		HooksSkipped:     []string{},
		Warnings:         []string{},
	}

	// 8. If DryRun, populate result with planned actions and return
	if opts.DryRun {
		populateDryRunResult(result, plan)
		return result, nil
	}

	// Resolve interactive prompts when configured.
	if conflictStrategy == string(StrategyPrompt) && !opts.Yes {
		if err := resolvePromptActions(plan, resolvedVars); err != nil {
			return result, err
		}
	}

	// 9. Execute pre_apply hook (if not NoHooks) - STUB for Phase 3
	if !opts.NoHooks && !p.Hooks.PreApply.IsEmpty() {
		// Phase 3: Hook execution will be implemented here
		result.HooksSkipped = append(result.HooksSkipped, "pre_apply (not implemented)")
	}

	// 10. Process each file according to its action
	for _, file := range plan.Files {
		switch file.Action {
		case ActionCreate:
			if err := ProcessFile(file.AbsSourcePath, file.AbsDestPath, file.IsTemplate, resolvedVars, extensions); err != nil {
				return result, err
			}
			result.FilesCreated = append(result.FilesCreated, file.RelPath)

		case ActionSkip:
			result.FilesSkipped = append(result.FilesSkipped, file.RelPath)

		case ActionOverwrite:
			if err := ProcessFile(file.AbsSourcePath, file.AbsDestPath, file.IsTemplate, resolvedVars, extensions); err != nil {
				return result, err
			}
			result.FilesOverwritten = append(result.FilesOverwritten, file.RelPath)

		case ActionBackup:
			backupPath, err := ExecuteBackup(file.AbsDestPath)
			if err != nil {
				return result, fmt.Errorf("backup failed for %s: %w", file.RelPath, err)
			}
			result.FilesBackedUp = append(result.FilesBackedUp, filepath.Base(backupPath))

			if err := ProcessFile(file.AbsSourcePath, file.AbsDestPath, file.IsTemplate, resolvedVars, extensions); err != nil {
				return result, err
			}
			result.FilesOverwritten = append(result.FilesOverwritten, file.RelPath)

		case ActionMerge:
			// Phase 4: Merge strategy will be implemented here
			// For now, fall back to skip with a warning
			result.FilesSkipped = append(result.FilesSkipped, file.RelPath)
			result.Warnings = append(result.Warnings, fmt.Sprintf("merge strategy not implemented, skipped: %s", file.RelPath))

		case ActionPrompt:
			// Phase 3: Interactive prompts will be implemented here
			// For now, fall back to skip with a warning
			result.FilesSkipped = append(result.FilesSkipped, file.RelPath)
			result.Warnings = append(result.Warnings, fmt.Sprintf("interactive prompts not implemented, skipped: %s", file.RelPath))
		}
	}

	// 11. Execute post_apply hook (if not NoHooks) - STUB for Phase 3
	if !opts.NoHooks && !p.Hooks.PostApply.IsEmpty() {
		// Phase 3: Hook execution will be implemented here
		result.HooksSkipped = append(result.HooksSkipped, "post_apply (not implemented)")
	}

	return result, nil
}

func resolvePromptActions(plan *FilePlan, vars map[string]string) error {
	var applyAll bool
	var applyAllAction FileAction

	for i := range plan.Files {
		file := &plan.Files[i]
		if file.Action != ActionPrompt {
			continue
		}

		if applyAll {
			file.Action = applyAllAction
			continue
		}

		action, applyToAll, err := promptForConflict(*file, vars)
		if err != nil {
			return err
		}
		if applyToAll {
			applyAll = true
			applyAllAction = action
		}
		file.Action = action
	}

	plan.CountActions()
	return nil
}

func promptForConflict(file FileInfo, vars map[string]string) (FileAction, bool, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		prompt := fmt.Sprintf("? %s (exists)\n  [s]kip  [o]verwrite  [b]ackup  [d]iff  [a]ll-skip  [A]ll-overwrite  [B]all-backup  [q]uit", file.RelPath)
		if CanMerge(file.RelPath) {
			prompt += "  [m]erge"
		}
		prompt += ": "
		fmt.Fprint(os.Stderr, prompt)

		input, err := reader.ReadString('\n')
		if err != nil {
			return ActionSkip, false, fmt.Errorf("reading conflict choice: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch input {
		case "s":
			return ActionSkip, false, nil
		case "o":
			return ActionOverwrite, false, nil
		case "b":
			return ActionBackup, false, nil
		case "m":
			if CanMerge(file.RelPath) {
				return ActionMerge, false, nil
			}
			fmt.Fprintln(os.Stderr, "merge not supported for this file type")
		case "d":
			if err := showDiff(file, vars); err != nil {
				return ActionSkip, false, err
			}
		case "a":
			return ActionSkip, true, nil
		case "A":
			return ActionOverwrite, true, nil
		case "B":
			return ActionBackup, true, nil
		case "q":
			return ActionSkip, false, &ConflictAbortedError{FilePath: file.RelPath}
		default:
			fmt.Fprintln(os.Stderr, "invalid choice")
		}
	}
}

func showDiff(file FileInfo, vars map[string]string) error {
	existing, err := os.ReadFile(file.AbsDestPath)
	if err != nil {
		return fmt.Errorf("reading existing file: %w", err)
	}

	updated, err := renderPartialContent(file, vars)
	if err != nil {
		return err
	}

	diff := formatDiff(existing, updated, 50)
	fmt.Fprintln(os.Stderr, diff)
	return nil
}

func renderPartialContent(file FileInfo, vars map[string]string) ([]byte, error) {
	content, err := os.ReadFile(file.AbsSourcePath)
	if err != nil {
		return nil, fmt.Errorf("reading partial file: %w", err)
	}
	if file.IsTemplate {
		processed, err := template.ProcessTemplateContent(string(content), vars)
		if err != nil {
			return nil, fmt.Errorf("processing template: %w", err)
		}
		return []byte(processed), nil
	}
	return content, nil
}

func formatDiff(existing, updated []byte, maxLines int) string {
	if isBinary(existing) || isBinary(updated) {
		return "--- existing\n+++ partial\n(binary diff not shown)"
	}

	existingLines := splitLines(existing)
	updatedLines := splitLines(updated)

	lines := []string{"--- existing", "+++ partial"}
	max := len(existingLines)
	if len(updatedLines) > max {
		max = len(updatedLines)
	}

	added := 0
	for i := 0; i < max && added < maxLines; i++ {
		var oldLine, newLine string
		if i < len(existingLines) {
			oldLine = existingLines[i]
		}
		if i < len(updatedLines) {
			newLine = updatedLines[i]
		}

		if i < len(existingLines) && i < len(updatedLines) {
			if oldLine == newLine {
				continue
			}
			lines = append(lines, "-"+oldLine)
			lines = append(lines, "+"+newLine)
			added += 2
			continue
		}

		if i < len(existingLines) {
			lines = append(lines, "-"+oldLine)
			added++
		} else if i < len(updatedLines) {
			lines = append(lines, "+"+newLine)
			added++
		}
	}

	if added >= maxLines {
		lines = append(lines, "... (diff truncated)")
	}

	return strings.Join(lines, "\n")
}

func splitLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) != -1
}

// resolveVariables builds a complete variable map from user-provided values, defaults, and builtins.
func resolveVariables(p *Partial, provided map[string]string, targetPath string) (map[string]string, error) {
	resolved := make(map[string]string)

	// Start with partial-specific builtins
	builtins := getPartialBuiltins(targetPath)
	for k, v := range builtins {
		resolved[k] = v
	}

	// Add defaults from partial variables
	for _, v := range p.Variables {
		if v.Default != nil {
			// Convert default value to string
			defaultStr := fmt.Sprintf("%v", v.Default)

			// Substitute builtins in default values
			processed, err := template.ProcessTemplateContent(defaultStr, resolved)
			if err != nil {
				return nil, fmt.Errorf("processing default for %s: %w", v.Name, err)
			}
			resolved[v.Name] = processed
		}
	}

	// Override with user-provided values
	for k, v := range provided {
		resolved[k] = v
	}

	return resolved, nil
}

// validateVariables ensures all required variables are present and values are valid.
func validateVariables(p *Partial, resolved map[string]string) error {
	errs := &MultiError{}

	for _, v := range p.Variables {
		value, exists := resolved[v.Name]

		// Check required
		if v.Required && (!exists || value == "") {
			errs.Add(&MissingRequiredVarError{
				VarName:     v.Name,
				Description: v.Description,
			})
			continue
		}

		// Skip further validation if no value
		if !exists || value == "" {
			continue
		}

		// Validate type-specific constraints
		switch v.Type {
		case template.VarTypeBoolean:
			if value != "true" && value != "false" {
				errs.Add(&InvalidVarValueError{
					VarName: v.Name,
					Value:   value,
					Reason:  "must be 'true' or 'false'",
				})
			}

		case template.VarTypeChoice:
			if len(v.Choices) > 0 {
				valid := false
				for _, choice := range v.Choices {
					if value == choice {
						valid = true
						break
					}
				}
				if !valid {
					errs.Add(&InvalidVarValueError{
						VarName: v.Name,
						Value:   value,
						Reason:  fmt.Sprintf("must be one of: %v", v.Choices),
					})
				}
			}
		}

		// Validate against regex pattern if provided
		if v.Validation != "" {
			re, err := regexp.Compile(v.Validation)
			if err != nil {
				// This should have been caught during partial validation
				errs.Add(&InvalidVarValueError{
					VarName: v.Name,
					Value:   value,
					Reason:  fmt.Sprintf("invalid validation pattern: %v", err),
				})
				continue
			}

			if !re.MatchString(value) {
				errs.Add(&InvalidVarValueError{
					VarName:    v.Name,
					Value:      value,
					Validation: v.Validation,
				})
			}
		}
	}

	return errs.ErrorOrNil()
}

// populateDryRunResult fills the result with what would happen without actually doing it.
func populateDryRunResult(result *ApplyResult, plan *FilePlan) {
	for _, file := range plan.Files {
		switch file.Action {
		case ActionCreate:
			result.FilesCreated = append(result.FilesCreated, file.RelPath)
		case ActionSkip:
			result.FilesSkipped = append(result.FilesSkipped, file.RelPath)
		case ActionOverwrite:
			result.FilesOverwritten = append(result.FilesOverwritten, file.RelPath)
		case ActionBackup:
			result.FilesBackedUp = append(result.FilesBackedUp, file.RelPath+".bak")
			result.FilesOverwritten = append(result.FilesOverwritten, file.RelPath)
		case ActionMerge:
			result.FilesMerged = append(result.FilesMerged, file.RelPath)
		case ActionPrompt:
			// In dry run, show as pending prompt
			result.Warnings = append(result.Warnings, fmt.Sprintf("would prompt for: %s", file.RelPath))
		}
	}
}

// getPartialBuiltins returns built-in variables available for partial templates.
// These are partial-specific and don't require workspace context.
func getPartialBuiltins(targetPath string) map[string]string {
	now := time.Now()

	vars := map[string]string{
		// Time/date builtins
		"DATE":      now.Format("2006-01-02"),
		"DATETIME":  now.Format(time.RFC3339),
		"YEAR":      now.Format("2006"),
		"TIMESTAMP": fmt.Sprintf("%d", now.Unix()),

		// Target directory info
		"DIRNAME":        filepath.Base(targetPath),
		"DIRPATH":        targetPath,
		"PARENT_DIRNAME": filepath.Base(filepath.Dir(targetPath)),
	}

	// Get home directory
	if home, err := os.UserHomeDir(); err == nil {
		vars["HOME"] = home
	}

	// Get git user info
	if name := getGitConfig("user.name"); name != "" {
		vars["GIT_USER_NAME"] = name
	}
	if email := getGitConfig("user.email"); email != "" {
		vars["GIT_USER_EMAIL"] = email
	}

	// Check if target is a git repo
	if isGitRepo(targetPath) {
		vars["IS_GIT_REPO"] = "true"
		if branch := getGitBranch(targetPath); branch != "" {
			vars["GIT_BRANCH"] = branch
		}
		if remoteURL := getGitRemoteURL(targetPath); remoteURL != "" {
			vars["GIT_REMOTE_URL"] = remoteURL
		}
	} else {
		vars["IS_GIT_REPO"] = "false"
	}

	return vars
}

// getGitConfig retrieves a git config value.
func getGitConfig(key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// isGitRepo checks if a directory is inside a git repository.
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// getGitBranch returns the current git branch for a directory.
func getGitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getGitRemoteURL returns the git remote origin URL for a directory.
func getGitRemoteURL(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
