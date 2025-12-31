package partial

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

	"github.com/tormodhaugland/co/internal/template"
)

// PartialHookType represents the type of partial lifecycle hook.
type PartialHookType string

const (
	// HookTypePreApply runs before any files are written.
	HookTypePreApply PartialHookType = "pre_apply"
	// HookTypePostApply runs after all files are written.
	HookTypePostApply PartialHookType = "post_apply"
)

// BuildPartialHookEnv builds environment variables for partial hook execution.
func BuildPartialHookEnv(env PartialHookEnv) []string {
	vars := []string{
		"CO_PARTIAL_NAME=" + env.PartialName,
		"CO_PARTIAL_PATH=" + env.PartialPath,
		"CO_TARGET_PATH=" + env.TargetPath,
		"CO_TARGET_DIRNAME=" + env.TargetDirname,
		"CO_DRY_RUN=" + strconv.FormatBool(env.DryRun),
		"CO_VERBOSE=" + strconv.FormatBool(env.Verbose),
		"CO_IS_GIT_REPO=" + strconv.FormatBool(env.IsGitRepo),
		"CO_GIT_REMOTE_URL=" + env.GitRemoteURL,
		"CO_GIT_BRANCH=" + env.GitBranch,
	}

	for k, v := range env.Variables {
		vars = append(vars, "CO_VAR_"+k+"="+v)
	}

	if env.Result != nil {
		vars = append(vars, "CO_FILES_CREATED="+strings.Join(env.Result.FilesCreated, "\n"))
		vars = append(vars, "CO_FILES_SKIPPED="+strings.Join(env.Result.FilesSkipped, "\n"))
		vars = append(vars, "CO_FILES_OVERWRITTEN="+strings.Join(env.Result.FilesOverwritten, "\n"))
		vars = append(vars, "CO_FILES_MERGED="+strings.Join(env.Result.FilesMerged, "\n"))
		vars = append(vars, "CO_FILES_BACKED_UP="+strings.Join(env.Result.FilesBackedUp, "\n"))
	}

	return append(os.Environ(), vars...)
}

// RunPartialHook executes a partial hook script.
//
// Parameters:
//   - hookType: The type of hook being run (pre_apply or post_apply)
//   - spec: The hook specification from the partial manifest
//   - partialPath: Absolute path to the partial directory
//   - env: The hook environment containing context and variables
//   - output: Writer for hook output (can be nil for quiet mode)
//
// Returns:
//   - template.HookResult containing execution details
//   - error if the hook failed (non-zero exit or execution error)
//
// Hook execution behavior:
//   - pre_apply: If fails (exit != 0), abort apply, no files written
//   - post_apply: If fails, log warning, apply is still considered successful
func RunPartialHook(hookType string, spec template.HookSpec, partialPath string, env PartialHookEnv, output io.Writer) (*template.HookResult, error) {
	result := &template.HookResult{
		HookType: template.HookType(hookType),
		Script:   spec.Script,
	}

	if spec.IsEmpty() {
		result.Skipped = true
		return result, nil
	}

	// Resolve script path relative to partial directory
	scriptPath := resolvePartialHookPath(partialPath, spec.Script)

	// Validate script exists and is executable
	if err := validatePartialHookScript(scriptPath); err != nil {
		result.Error = err
		return result, err
	}

	// Parse timeout
	timeout := parseHookTimeout(spec.Timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/bash", scriptPath)

	// Set working directory to target path
	cmd.Dir = env.TargetPath

	// Build environment
	cmd.Env = BuildPartialHookEnv(env)

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
			HookType: hookType,
			Script:   spec.Script,
			Timeout:  spec.Timeout,
		}
		return result, result.Error
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = &HookExecutionError{
			HookType: hookType,
			Script:   spec.Script,
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Err:      err,
		}
		return result, result.Error
	}

	return result, nil
}

// resolvePartialHookPath resolves a hook script path relative to the partial.
// It checks the hooks/ subdirectory first, then the partial root.
func resolvePartialHookPath(partialPath, script string) string {
	// Try hooks/ subdirectory first
	hooksPath := filepath.Join(partialPath, PartialHooksDir, script)
	if _, err := os.Stat(hooksPath); err == nil {
		return hooksPath
	}

	// Try relative to partial root
	return filepath.Join(partialPath, script)
}

// validatePartialHookScript checks if a hook script exists and is executable.
func validatePartialHookScript(scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if os.IsNotExist(err) {
		return &HookNotFoundError{Script: scriptPath}
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

// parseHookTimeout parses a timeout string and returns a duration.
// Supports formats like "30s", "5m", "1h". Defaults to DefaultHookTimeout.
func parseHookTimeout(timeout string) time.Duration {
	if timeout == "" {
		timeout = DefaultHookTimeout
	}

	// Try parsing as Go duration
	if d, err := time.ParseDuration(timeout); err == nil {
		return d
	}

	// Default to 5 minutes
	return 5 * time.Minute
}

// GetPartialHookSpec returns the hook spec for a given hook type from a partial.
func GetPartialHookSpec(p *Partial, hookType PartialHookType) template.HookSpec {
	switch hookType {
	case HookTypePreApply:
		return p.Hooks.PreApply
	case HookTypePostApply:
		return p.Hooks.PostApply
	default:
		return template.HookSpec{}
	}
}

// HasPartialHook checks if a partial has a specific hook defined.
func HasPartialHook(p *Partial, hookType PartialHookType) bool {
	return !GetPartialHookSpec(p, hookType).IsEmpty()
}

// ListPartialHooks returns a list of all defined hooks in a partial.
func ListPartialHooks(p *Partial) []PartialHookType {
	var hooks []PartialHookType

	if HasPartialHook(p, HookTypePreApply) {
		hooks = append(hooks, HookTypePreApply)
	}
	if HasPartialHook(p, HookTypePostApply) {
		hooks = append(hooks, HookTypePostApply)
	}

	return hooks
}

// BuildPartialHookEnvFromApply creates a PartialHookEnv from apply context.
// This is a convenience function for the Apply function.
func BuildPartialHookEnvFromApply(p *Partial, partialPath, targetPath string, vars map[string]string, dryRun, verbose bool, result *ApplyResult) PartialHookEnv {
	env := PartialHookEnv{
		PartialName:   p.Name,
		PartialPath:   partialPath,
		TargetPath:    targetPath,
		TargetDirname: filepath.Base(targetPath),
		DryRun:        dryRun,
		Verbose:       verbose,
		Variables:     vars,
		Result:        result,
	}

	// Check git status of target
	if isGitRepo(targetPath) {
		env.IsGitRepo = true
		env.GitBranch = getGitBranch(targetPath)
		env.GitRemoteURL = getGitRemoteURL(targetPath)
	}

	return env
}
