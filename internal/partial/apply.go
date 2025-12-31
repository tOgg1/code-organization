package partial

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	// 3. Validate prerequisites
	prereqResult, err := CheckPrerequisites(p, absTargetPath)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	if !prereqResult.Satisfied {
		if !opts.Force {
			return nil, &PrerequisiteFailedError{
				PartialName:     p.Name,
				MissingCommands: prereqResult.MissingCommands,
				MissingFiles:    prereqResult.MissingFiles,
			}
		}
		if len(prereqResult.MissingCommands) > 0 || len(prereqResult.MissingFiles) > 0 {
			warn := "prerequisites missing, applied with --force"
			if len(prereqResult.MissingCommands) > 0 {
				warn += fmt.Sprintf("; commands: %s", strings.Join(prereqResult.MissingCommands, ", "))
			}
			if len(prereqResult.MissingFiles) > 0 {
				warn += fmt.Sprintf("; files: %s", strings.Join(prereqResult.MissingFiles, ", "))
			}
			warnings = append(warnings, warn)
		}
	}

	// 4. Resolve variables (user-provided + defaults + builtins)
	builtins, err := GetPartialBuiltins(absTargetPath)
	if err != nil {
		return nil, err
	}
	resolvedVars, err := ResolvePartialVariables(p, opts.Variables, builtins)
	if err != nil {
		return nil, err
	}

	// 6. Scan partial files
	files, err := ScanPartialFiles(partialPath, p.Files)
	if err != nil {
		return nil, fmt.Errorf("scanning partial files: %w", err)
	}

	// 7. Determine conflict strategy
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

	// 8. Detect conflicts
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
		Warnings:         warnings,
	}

	// 9. If DryRun, populate result with planned actions and return
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

	// 10. Execute pre_apply hook (if not NoHooks)
	if !opts.NoHooks && !p.Hooks.PreApply.IsEmpty() {
		env := BuildPartialHookEnvFromApply(p, partialPath, absTargetPath, resolvedVars, opts.DryRun, false, nil)
		hookResult, err := RunPartialHook(string(HookTypePreApply), p.Hooks.PreApply, partialPath, env, os.Stdout)
		if err != nil {
			return result, err
		}
		if hookResult != nil && !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, "pre_apply")
		} else {
			result.HooksSkipped = append(result.HooksSkipped, "pre_apply")
		}
	}

	// 11. Process each file according to its action
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
			if err := MergeFile(file.AbsDestPath, file.AbsSourcePath, file.AbsDestPath); err != nil {
				return result, fmt.Errorf("merge failed for %s: %w", file.RelPath, err)
			}
			result.FilesMerged = append(result.FilesMerged, file.RelPath)

		case ActionPrompt:
			// Phase 3: Interactive prompts will be implemented here
			// For now, fall back to skip with a warning
			result.FilesSkipped = append(result.FilesSkipped, file.RelPath)
			result.Warnings = append(result.Warnings, fmt.Sprintf("interactive prompts not implemented, skipped: %s", file.RelPath))
		}
	}

	// 12. Execute post_apply hook (if not NoHooks)
	if !opts.NoHooks && !p.Hooks.PostApply.IsEmpty() {
		env := BuildPartialHookEnvFromApply(p, partialPath, absTargetPath, resolvedVars, opts.DryRun, false, result)
		hookResult, err := RunPartialHook(string(HookTypePostApply), p.Hooks.PostApply, partialPath, env, os.Stdout)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("post_apply hook failed: %v", err))
		}
		if hookResult != nil && !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, "post_apply")
		} else {
			result.HooksSkipped = append(result.HooksSkipped, "post_apply")
		}
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
