package template

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HookType represents the type of lifecycle hook.
type HookType string

const (
	HookPreCreate    HookType = "pre_create"
	HookPostCreate   HookType = "post_create"
	HookPostClone    HookType = "post_clone"
	HookPostComplete HookType = "post_complete"
	HookPostMigrate  HookType = "post_migrate"
)

// HookResult contains the result of hook execution.
type HookResult struct {
	HookType HookType
	Script   string
	ExitCode int
	Output   string
	Duration time.Duration
	Skipped  bool
	Error    error
}

// BuildHookEnv creates environment variables for hook execution.
func BuildHookEnv(env HookEnv) []string {
	vars := []string{
		"CO_WORKSPACE_PATH=" + env.WorkspacePath,
		"CO_WORKSPACE_SLUG=" + env.WorkspaceSlug,
		"CO_OWNER=" + env.Owner,
		"CO_PROJECT=" + env.Project,
		"CO_CODE_ROOT=" + env.CodeRoot,
		"CO_TEMPLATE_NAME=" + env.TemplateName,
		"CO_TEMPLATE_PATH=" + env.TemplatePath,
		"CO_REPOS_PATH=" + env.ReposPath,
		"CO_DRY_RUN=" + strconv.FormatBool(env.DryRun),
		"CO_VERBOSE=" + strconv.FormatBool(env.Verbose),
	}

	// Add hook output file path
	hookOutputFile := filepath.Join(env.WorkspacePath, ".co-hook-output")
	vars = append(vars, "CO_HOOK_OUTPUT_FILE="+hookOutputFile)

	// Add previous hook output
	vars = append(vars, "CO_PREV_HOOK_OUTPUT="+env.PrevHookOutput)

	// Add user-defined variables with CO_VAR_ prefix
	for k, v := range env.Variables {
		vars = append(vars, "CO_VAR_"+k+"="+v)
	}

	// Include existing environment
	return append(os.Environ(), vars...)
}

// ValidateHookScript checks if a hook script exists and is executable.
func ValidateHookScript(templatePath string, spec HookSpec) error {
	if spec.Script == "" {
		return nil
	}

	scriptPath := ResolveHookPath(templatePath, spec.Script)

	info, err := os.Stat(scriptPath)
	if os.IsNotExist(err) {
		return &HookNotFoundError{Script: spec.Script}
	}
	if err != nil {
		return fmt.Errorf("checking script: %w", err)
	}

	// Check if file is executable
	if info.Mode()&0111 == 0 {
		return &HookNotExecutableError{Script: scriptPath}
	}

	return nil
}

// ResolveHookPath resolves a hook script path relative to the template.
func ResolveHookPath(templatePath, script string) string {
	// Try hooks/ subdirectory first
	hooksPath := filepath.Join(templatePath, TemplateHooksDir, script)
	if _, err := os.Stat(hooksPath); err == nil {
		return hooksPath
	}

	// Try relative to template root
	return filepath.Join(templatePath, script)
}

// ParseTimeout parses a timeout string and returns a duration.
func ParseTimeout(timeout string) time.Duration {
	if timeout == "" {
		timeout = DefaultHookTimeout
	}

	seconds, err := parseTimeoutString(timeout)
	if err != nil {
		return 5 * time.Minute // Default
	}

	return time.Duration(seconds) * time.Second
}

// RunHook executes a hook script.
func RunHook(hookType HookType, spec HookSpec, templatePath string, env HookEnv, output io.Writer) (*HookResult, error) {
	result := &HookResult{
		HookType: hookType,
		Script:   spec.Script,
	}

	if spec.IsEmpty() {
		result.Skipped = true
		return result, nil
	}

	// Validate script
	if err := ValidateHookScript(templatePath, spec); err != nil {
		result.Error = err
		return result, err
	}

	scriptPath := ResolveHookPath(templatePath, spec.Script)
	timeout := ParseTimeout(spec.Timeout)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(ctx, "/bin/bash", scriptPath)
	cmd.Dir = env.WorkspacePath
	cmd.Env = BuildHookEnv(env)

	// Capture output
	var outputBuf bytes.Buffer
	if output != nil {
		cmd.Stdout = io.MultiWriter(&outputBuf, output)
		cmd.Stderr = io.MultiWriter(&outputBuf, output)
	} else {
		cmd.Stdout = &outputBuf
		cmd.Stderr = &outputBuf
	}

	start := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Output = outputBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = &HookTimeoutError{
			HookType: string(hookType),
			Script:   spec.Script,
			Timeout:  spec.Timeout,
		}
		return result, result.Error
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = &HookError{
				HookType: string(hookType),
				Script:   spec.Script,
				ExitCode: result.ExitCode,
				Output:   result.Output,
			}
		} else {
			result.Error = &HookError{
				HookType: string(hookType),
				Script:   spec.Script,
				Err:      err,
			}
		}
		return result, result.Error
	}

	return result, nil
}

// RunAllHooks runs hooks in sequence, passing output between them.
func RunAllHooks(tmpl *Template, templatePath string, env HookEnv, hookTypes []HookType, output io.Writer, noHooks bool) ([]HookResult, error) {
	var results []HookResult
	prevOutput := ""

	for _, hookType := range hookTypes {
		spec := GetHookSpec(tmpl, hookType)

		if noHooks && !spec.IsEmpty() {
			results = append(results, HookResult{
				HookType: hookType,
				Script:   spec.Script,
				Skipped:  true,
			})
			continue
		}

		// Update env with previous hook output
		env.PrevHookOutput = prevOutput

		result, err := RunHook(hookType, spec, templatePath, env, output)
		results = append(results, *result)

		if err != nil {
			return results, err
		}

		// Read hook output file if it exists
		hookOutputFile := filepath.Join(env.WorkspacePath, ".co-hook-output")
		if data, err := os.ReadFile(hookOutputFile); err == nil {
			prevOutput = strings.TrimSpace(string(data))
			// Clean up output file
			os.Remove(hookOutputFile)
		}
	}

	return results, nil
}

// GetHookSpec returns the hook spec for a given hook type.
func GetHookSpec(tmpl *Template, hookType HookType) HookSpec {
	switch hookType {
	case HookPreCreate:
		return tmpl.Hooks.PreCreate
	case HookPostCreate:
		return tmpl.Hooks.PostCreate
	case HookPostClone:
		return tmpl.Hooks.PostClone
	case HookPostComplete:
		return tmpl.Hooks.PostComplete
	case HookPostMigrate:
		return tmpl.Hooks.PostMigrate
	default:
		return HookSpec{}
	}
}

// HasHook checks if a template has a specific hook defined.
func HasHook(tmpl *Template, hookType HookType) bool {
	return !GetHookSpec(tmpl, hookType).IsEmpty()
}

// ListHooks returns a list of all defined hooks in a template.
func ListHooks(tmpl *Template) []HookType {
	var hooks []HookType

	if HasHook(tmpl, HookPreCreate) {
		hooks = append(hooks, HookPreCreate)
	}
	if HasHook(tmpl, HookPostCreate) {
		hooks = append(hooks, HookPostCreate)
	}
	if HasHook(tmpl, HookPostClone) {
		hooks = append(hooks, HookPostClone)
	}
	if HasHook(tmpl, HookPostComplete) {
		hooks = append(hooks, HookPostComplete)
	}
	if HasHook(tmpl, HookPostMigrate) {
		hooks = append(hooks, HookPostMigrate)
	}

	return hooks
}

// MakeScriptExecutable adds execute permissions to a script.
func MakeScriptExecutable(scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if err != nil {
		return err
	}

	// Add execute permission for owner, group, and others
	newMode := info.Mode() | 0111
	return os.Chmod(scriptPath, newMode)
}

// CreateHookOutputFile creates an empty hook output file.
func CreateHookOutputFile(workspacePath string) (string, error) {
	outputPath := filepath.Join(workspacePath, ".co-hook-output")
	f, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	f.Close()
	return outputPath, nil
}

// CleanupHookOutputFile removes the hook output file.
func CleanupHookOutputFile(workspacePath string) error {
	outputPath := filepath.Join(workspacePath, ".co-hook-output")
	err := os.Remove(outputPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
