package partial

import (
	"fmt"
	"strings"
)

// PartialNotFoundError indicates a partial does not exist.
type PartialNotFoundError struct {
	Name string
}

func (e *PartialNotFoundError) Error() string {
	return fmt.Sprintf("partial not found: %s", e.Name)
}

// InvalidManifestError indicates a partial.json file is invalid.
type InvalidManifestError struct {
	Path string
	Err  error
}

func (e *InvalidManifestError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("invalid partial manifest at %s: %v", e.Path, e.Err)
	}
	return fmt.Sprintf("invalid partial manifest at %s", e.Path)
}

func (e *InvalidManifestError) Unwrap() error {
	return e.Err
}

// TargetNotFoundError indicates the target directory doesn't exist.
type TargetNotFoundError struct {
	Path string
}

func (e *TargetNotFoundError) Error() string {
	return fmt.Sprintf("target directory not found: %s", e.Path)
}

// PrerequisiteFailedError indicates required prerequisites are not met.
type PrerequisiteFailedError struct {
	PartialName     string
	MissingCommands []string
	MissingFiles    []string
}

func (e *PrerequisiteFailedError) Error() string {
	var parts []string
	if len(e.MissingCommands) > 0 {
		parts = append(parts, fmt.Sprintf("missing commands: %s", strings.Join(e.MissingCommands, ", ")))
	}
	if len(e.MissingFiles) > 0 {
		parts = append(parts, fmt.Sprintf("missing files: %s", strings.Join(e.MissingFiles, ", ")))
	}
	return fmt.Sprintf("prerequisites not met for partial %q: %s", e.PartialName, strings.Join(parts, "; "))
}

// HasMissing returns true if any prerequisites are missing.
func (e *PrerequisiteFailedError) HasMissing() bool {
	return len(e.MissingCommands) > 0 || len(e.MissingFiles) > 0
}

// HookFailedError indicates a hook script failed during partial application.
type HookFailedError struct {
	HookType string
	Script   string
	ExitCode int
	Output   string
	Err      error
}

func (e *HookFailedError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("hook %s (%s) failed: %v", e.HookType, e.Script, e.Err)
	}
	if e.ExitCode != 0 {
		return fmt.Sprintf("hook %s (%s) failed with exit code %d", e.HookType, e.Script, e.ExitCode)
	}
	return fmt.Sprintf("hook %s (%s) failed", e.HookType, e.Script)
}

func (e *HookFailedError) Unwrap() error {
	return e.Err
}

// HookNotFoundError indicates a hook script does not exist.
type HookNotFoundError struct {
	Script string
}

func (e *HookNotFoundError) Error() string {
	return fmt.Sprintf("hook script not found: %s", e.Script)
}

// HookNotExecutableError indicates a hook script is not executable.
type HookNotExecutableError struct {
	Script string
}

func (e *HookNotExecutableError) Error() string {
	return fmt.Sprintf("hook script is not executable: %s", e.Script)
}

// HookTimeoutError indicates a hook script exceeded its timeout.
type HookTimeoutError struct {
	HookType string
	Script   string
	Timeout  string
}

func (e *HookTimeoutError) Error() string {
	if e.Timeout != "" {
		return fmt.Sprintf("hook %s (%s) timed out after %s", e.HookType, e.Script, e.Timeout)
	}
	return fmt.Sprintf("hook %s (%s) timed out", e.HookType, e.Script)
}

// HookExecutionError indicates a hook script failed during execution.
type HookExecutionError struct {
	HookType string
	Script   string
	ExitCode int
	Output   string
	Err      error
}

func (e *HookExecutionError) Error() string {
	if e.Err != nil && e.ExitCode == 0 {
		return fmt.Sprintf("hook %s (%s) execution failed: %v", e.HookType, e.Script, e.Err)
	}
	if e.ExitCode != 0 {
		return fmt.Sprintf("hook %s (%s) failed with exit code %d", e.HookType, e.Script, e.ExitCode)
	}
	return fmt.Sprintf("hook %s (%s) failed", e.HookType, e.Script)
}

func (e *HookExecutionError) Unwrap() error {
	return e.Err
}

// ConflictAbortedError indicates the user cancelled during conflict resolution.
type ConflictAbortedError struct {
	FilePath string
}

func (e *ConflictAbortedError) Error() string {
	if e.FilePath != "" {
		return fmt.Sprintf("partial application aborted: user cancelled at conflict for %s", e.FilePath)
	}
	return "partial application aborted: user cancelled during conflict resolution"
}

// PathTraversalError indicates an attempt to write outside the target directory.
type PathTraversalError struct {
	Path       string
	TargetPath string
}

func (e *PathTraversalError) Error() string {
	return fmt.Sprintf("path traversal detected: %s is outside target directory %s", e.Path, e.TargetPath)
}

// MissingRequiredVarError indicates a required variable was not provided.
type MissingRequiredVarError struct {
	VarName     string
	Description string
}

func (e *MissingRequiredVarError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("missing required variable %q: %s", e.VarName, e.Description)
	}
	return fmt.Sprintf("missing required variable: %s", e.VarName)
}

// InvalidVarValueError indicates a variable value failed validation.
type InvalidVarValueError struct {
	VarName    string
	Value      string
	Validation string
	Reason     string
}

func (e *InvalidVarValueError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid value for variable %q: %s", e.VarName, e.Reason)
	}
	if e.Validation != "" {
		return fmt.Sprintf("invalid value %q for variable %q: does not match pattern %s", e.Value, e.VarName, e.Validation)
	}
	return fmt.Sprintf("invalid value %q for variable %q", e.Value, e.VarName)
}

// FileProcessingError indicates an error during file processing.
type FileProcessingError struct {
	SrcPath  string
	DestPath string
	Err      error
}

func (e *FileProcessingError) Error() string {
	return fmt.Sprintf("failed to process file %s → %s: %v", e.SrcPath, e.DestPath, e.Err)
}

func (e *FileProcessingError) Unwrap() error {
	return e.Err
}

// MergeError indicates a merge operation failed.
type MergeError struct {
	FilePath string
	Format   string
	Err      error
}

func (e *MergeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to merge %s file %s: %v", e.Format, e.FilePath, e.Err)
	}
	return fmt.Sprintf("failed to merge %s file %s", e.Format, e.FilePath)
}

func (e *MergeError) Unwrap() error {
	return e.Err
}

// ValidationError indicates partial validation failed.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("partial validation failed: %s - %s", e.Field, e.Reason)
}

// MultiError collects multiple errors.
type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("%d errors:\n  - %s", len(e.Errors), strings.Join(msgs, "\n  - "))
}

// Add appends an error to the collection.
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors returns true if any errors were collected.
func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

// ErrorOrNil returns nil if no errors, otherwise returns the MultiError.
func (e *MultiError) ErrorOrNil() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}
