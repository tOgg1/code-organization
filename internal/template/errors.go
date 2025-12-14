// Package template provides templating and scripting functionality for workspace creation.
package template

import (
	"fmt"
	"strings"
)

// TemplateNotFoundError indicates a template does not exist.
type TemplateNotFoundError struct {
	Name string
}

func (e *TemplateNotFoundError) Error() string {
	return fmt.Sprintf("template not found: %s", e.Name)
}

// InvalidManifestError indicates a template.json file is invalid.
type InvalidManifestError struct {
	Path string
	Err  error
}

func (e *InvalidManifestError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("invalid template manifest at %s: %v", e.Path, e.Err)
	}
	return fmt.Sprintf("invalid template manifest at %s", e.Path)
}

func (e *InvalidManifestError) Unwrap() error {
	return e.Err
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

// CyclicVariableError indicates circular references in variable definitions.
type CyclicVariableError struct {
	Cycle []string
}

func (e *CyclicVariableError) Error() string {
	return fmt.Sprintf("circular variable reference: %s", strings.Join(e.Cycle, " → "))
}

// HookError indicates a hook script failed.
type HookError struct {
	HookType string
	Script   string
	ExitCode int
	Output   string
	Err      error
}

func (e *HookError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("hook %s (%s) failed: %v", e.HookType, e.Script, e.Err)
	}
	if e.ExitCode != 0 {
		return fmt.Sprintf("hook %s (%s) failed with exit code %d", e.HookType, e.Script, e.ExitCode)
	}
	return fmt.Sprintf("hook %s (%s) failed", e.HookType, e.Script)
}

func (e *HookError) Unwrap() error {
	return e.Err
}

// HookTimeoutError indicates a hook exceeded its timeout.
type HookTimeoutError struct {
	HookType string
	Script   string
	Timeout  string
}

func (e *HookTimeoutError) Error() string {
	return fmt.Sprintf("hook %s (%s) timed out after %s", e.HookType, e.Script, e.Timeout)
}

// HookNotFoundError indicates a hook script does not exist.
type HookNotFoundError struct {
	HookType string
	Script   string
}

func (e *HookNotFoundError) Error() string {
	return fmt.Sprintf("hook script not found for %s: %s", e.HookType, e.Script)
}

// HookNotExecutableError indicates a hook script is not executable.
type HookNotExecutableError struct {
	Script string
}

func (e *HookNotExecutableError) Error() string {
	return fmt.Sprintf("hook script is not executable: %s", e.Script)
}

// FileProcessingError indicates an error during file processing.
type FileProcessingError struct {
	SrcPath  string
	DestPath string
	Err      error
}

func (e *FileProcessingError) Error() string {
	return fmt.Sprintf("failed to process file %s: %v", e.SrcPath, e.Err)
}

func (e *FileProcessingError) Unwrap() error {
	return e.Err
}

// PathTraversalError indicates an attempt to write outside the workspace.
type PathTraversalError struct {
	Path          string
	WorkspacePath string
}

func (e *PathTraversalError) Error() string {
	return fmt.Sprintf("path traversal detected: %s is outside workspace %s", e.Path, e.WorkspacePath)
}

// SubstitutionError indicates a variable substitution failed.
type SubstitutionError struct {
	VarName string
	Context string
	Err     error
}

func (e *SubstitutionError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("failed to substitute variable %q in %s: %v", e.VarName, e.Context, e.Err)
	}
	return fmt.Sprintf("failed to substitute variable %q: %v", e.VarName, e.Err)
}

func (e *SubstitutionError) Unwrap() error {
	return e.Err
}

// ValidationError indicates template validation failed.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("template validation failed: %s - %s", e.Field, e.Reason)
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
