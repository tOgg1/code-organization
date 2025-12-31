package partial

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tormodhaugland/co/internal/template"
)

func TestBuildPartialHookEnv_StandardVars(t *testing.T) {
	env := PartialHookEnv{
		PartialName:   "test-partial",
		PartialPath:   "/path/to/partial",
		TargetPath:    "/path/to/target",
		TargetDirname: "target",
		DryRun:        false,
		Verbose:       true,
		IsGitRepo:     true,
		GitRemoteURL:  "https://github.com/test/repo.git",
		GitBranch:     "main",
		Variables:     map[string]string{},
	}

	vars := BuildPartialHookEnv(env)

	// Check that standard vars are present
	findVar := func(key string) string {
		prefix := key + "="
		for _, v := range vars {
			if strings.HasPrefix(v, prefix) {
				return strings.TrimPrefix(v, prefix)
			}
		}
		return ""
	}

	if v := findVar("CO_PARTIAL_NAME"); v != "test-partial" {
		t.Errorf("CO_PARTIAL_NAME = %q, want \"test-partial\"", v)
	}
	if v := findVar("CO_PARTIAL_PATH"); v != "/path/to/partial" {
		t.Errorf("CO_PARTIAL_PATH = %q, want \"/path/to/partial\"", v)
	}
	if v := findVar("CO_TARGET_PATH"); v != "/path/to/target" {
		t.Errorf("CO_TARGET_PATH = %q, want \"/path/to/target\"", v)
	}
	if v := findVar("CO_TARGET_DIRNAME"); v != "target" {
		t.Errorf("CO_TARGET_DIRNAME = %q, want \"target\"", v)
	}
	if v := findVar("CO_DRY_RUN"); v != "false" {
		t.Errorf("CO_DRY_RUN = %q, want \"false\"", v)
	}
	if v := findVar("CO_VERBOSE"); v != "true" {
		t.Errorf("CO_VERBOSE = %q, want \"true\"", v)
	}
	if v := findVar("CO_IS_GIT_REPO"); v != "true" {
		t.Errorf("CO_IS_GIT_REPO = %q, want \"true\"", v)
	}
	if v := findVar("CO_GIT_REMOTE_URL"); v != "https://github.com/test/repo.git" {
		t.Errorf("CO_GIT_REMOTE_URL = %q, want \"https://github.com/test/repo.git\"", v)
	}
	if v := findVar("CO_GIT_BRANCH"); v != "main" {
		t.Errorf("CO_GIT_BRANCH = %q, want \"main\"", v)
	}
}

func TestBuildPartialHookEnv_UserVars(t *testing.T) {
	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   "/path",
		TargetPath:    "/target",
		TargetDirname: "target",
		Variables: map[string]string{
			"APP_NAME": "myapp",
			"PORT":     "8080",
		},
	}

	vars := BuildPartialHookEnv(env)

	findVar := func(key string) string {
		prefix := key + "="
		for _, v := range vars {
			if strings.HasPrefix(v, prefix) {
				return strings.TrimPrefix(v, prefix)
			}
		}
		return ""
	}

	// User vars should be prefixed with CO_VAR_
	if v := findVar("CO_VAR_APP_NAME"); v != "myapp" {
		t.Errorf("CO_VAR_APP_NAME = %q, want \"myapp\"", v)
	}
	if v := findVar("CO_VAR_PORT"); v != "8080" {
		t.Errorf("CO_VAR_PORT = %q, want \"8080\"", v)
	}
}

func TestBuildPartialHookEnv_FileListsForPostApply(t *testing.T) {
	result := &ApplyResult{
		FilesCreated:     []string{"file1.txt", "file2.txt"},
		FilesSkipped:     []string{"skipped.txt"},
		FilesOverwritten: []string{"overwritten.txt"},
		FilesMerged:      []string{},
		FilesBackedUp:    []string{"backup.txt.bak"},
	}

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   "/path",
		TargetPath:    "/target",
		TargetDirname: "target",
		Result:        result,
	}

	vars := BuildPartialHookEnv(env)

	findVar := func(key string) string {
		prefix := key + "="
		for _, v := range vars {
			if strings.HasPrefix(v, prefix) {
				return strings.TrimPrefix(v, prefix)
			}
		}
		return ""
	}

	// File lists should be newline-separated
	if v := findVar("CO_FILES_CREATED"); v != "file1.txt\nfile2.txt" {
		t.Errorf("CO_FILES_CREATED = %q, want \"file1.txt\\nfile2.txt\"", v)
	}
	if v := findVar("CO_FILES_SKIPPED"); v != "skipped.txt" {
		t.Errorf("CO_FILES_SKIPPED = %q, want \"skipped.txt\"", v)
	}
	if v := findVar("CO_FILES_OVERWRITTEN"); v != "overwritten.txt" {
		t.Errorf("CO_FILES_OVERWRITTEN = %q, want \"overwritten.txt\"", v)
	}
	if v := findVar("CO_FILES_MERGED"); v != "" {
		t.Errorf("CO_FILES_MERGED = %q, want empty", v)
	}
	if v := findVar("CO_FILES_BACKED_UP"); v != "backup.txt.bak" {
		t.Errorf("CO_FILES_BACKED_UP = %q, want \"backup.txt.bak\"", v)
	}
}

func TestBuildPartialHookEnv_IncludesSystemEnv(t *testing.T) {
	// Set a test env var
	os.Setenv("TEST_PARTIAL_VAR", "test_value")
	defer os.Unsetenv("TEST_PARTIAL_VAR")

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   "/path",
		TargetPath:    "/target",
		TargetDirname: "target",
	}

	vars := BuildPartialHookEnv(env)

	// System env vars should be included
	found := false
	for _, v := range vars {
		if v == "TEST_PARTIAL_VAR=test_value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("System environment variable TEST_PARTIAL_VAR not found in hook env")
	}
}

func TestRunPartialHook_EmptySpec(t *testing.T) {
	spec := template.HookSpec{} // Empty script

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   "/path",
		TargetPath:    "/target",
		TargetDirname: "target",
	}

	result, err := RunPartialHook("pre_apply", spec, "/path", env, nil)
	if err != nil {
		t.Fatalf("RunPartialHook failed: %v", err)
	}

	if !result.Skipped {
		t.Error("Expected hook to be skipped for empty spec")
	}
}

func TestRunPartialHook_ScriptNotFound(t *testing.T) {
	spec := template.HookSpec{
		Script: "nonexistent-script.sh",
	}

	tmpDir := t.TempDir()
	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   tmpDir,
		TargetPath:    tmpDir,
		TargetDirname: filepath.Base(tmpDir),
	}

	_, err := RunPartialHook("pre_apply", spec, tmpDir, env, nil)
	if err == nil {
		t.Fatal("Expected error for missing script, got nil")
	}

	var notFoundErr *HookNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("Expected HookNotFoundError, got %T: %v", err, err)
	}
}

func TestRunPartialHook_ScriptNotExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a non-executable script
	scriptPath := filepath.Join(tmpDir, "hooks", "test.sh")
	os.MkdirAll(filepath.Dir(scriptPath), 0755)
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test"), 0644) // Not executable

	spec := template.HookSpec{
		Script: "test.sh",
	}

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   tmpDir,
		TargetPath:    tmpDir,
		TargetDirname: filepath.Base(tmpDir),
	}

	_, err := RunPartialHook("pre_apply", spec, tmpDir, env, nil)
	if err == nil {
		t.Fatal("Expected error for non-executable script, got nil")
	}

	var notExecErr *HookNotExecutableError
	if !errors.As(err, &notExecErr) {
		t.Errorf("Expected HookNotExecutableError, got %T: %v", err, err)
	}
}

func TestRunPartialHook_SuccessfulExecution(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an executable script
	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)
	scriptPath := filepath.Join(hooksDir, "test.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'Hello from hook'"), 0755)

	spec := template.HookSpec{
		Script: "test.sh",
	}

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   tmpDir,
		TargetPath:    tmpDir,
		TargetDirname: filepath.Base(tmpDir),
	}

	var output bytes.Buffer
	result, err := RunPartialHook("pre_apply", spec, tmpDir, env, &output)
	if err != nil {
		t.Fatalf("RunPartialHook failed: %v", err)
	}

	if result.Skipped {
		t.Error("Hook should not be skipped")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Output, "Hello from hook") {
		t.Errorf("Output = %q, expected to contain 'Hello from hook'", result.Output)
	}
}

func TestRunPartialHook_NonZeroExit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a script that fails
	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)
	scriptPath := filepath.Join(hooksDir, "fail.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 1"), 0755)

	spec := template.HookSpec{
		Script: "fail.sh",
	}

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   tmpDir,
		TargetPath:    tmpDir,
		TargetDirname: filepath.Base(tmpDir),
	}

	result, err := RunPartialHook("pre_apply", spec, tmpDir, env, nil)
	if err == nil {
		t.Fatal("Expected error for non-zero exit, got nil")
	}

	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}

	var execErr *HookExecutionError
	if !errors.As(err, &execErr) {
		t.Errorf("Expected HookExecutionError, got %T: %v", err, err)
	}
}

func TestRunPartialHook_ScriptInRootDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a script directly in root (not hooks/)
	scriptPath := filepath.Join(tmpDir, "setup.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'root script'"), 0755)

	spec := template.HookSpec{
		Script: "setup.sh",
	}

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   tmpDir,
		TargetPath:    tmpDir,
		TargetDirname: filepath.Base(tmpDir),
	}

	result, err := RunPartialHook("pre_apply", spec, tmpDir, env, nil)
	if err != nil {
		t.Fatalf("RunPartialHook failed: %v", err)
	}

	if !strings.Contains(result.Output, "root script") {
		t.Errorf("Output = %q, expected to contain 'root script'", result.Output)
	}
}

func TestRunPartialHook_HooksDirTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create scripts in both locations
	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Script in hooks/
	os.WriteFile(filepath.Join(hooksDir, "test.sh"), []byte("#!/bin/bash\necho 'from hooks dir'"), 0755)
	// Script in root
	os.WriteFile(filepath.Join(tmpDir, "test.sh"), []byte("#!/bin/bash\necho 'from root'"), 0755)

	spec := template.HookSpec{
		Script: "test.sh",
	}

	env := PartialHookEnv{
		PartialName:   "test",
		PartialPath:   tmpDir,
		TargetPath:    tmpDir,
		TargetDirname: filepath.Base(tmpDir),
	}

	result, err := RunPartialHook("pre_apply", spec, tmpDir, env, nil)
	if err != nil {
		t.Fatalf("RunPartialHook failed: %v", err)
	}

	// Should run the one from hooks/ dir
	if !strings.Contains(result.Output, "from hooks dir") {
		t.Errorf("Output = %q, expected script from hooks/ to take precedence", result.Output)
	}
}

func TestGetPartialHookSpec(t *testing.T) {
	p := &Partial{
		Hooks: PartialHooks{
			PreApply: template.HookSpec{
				Script:  "pre.sh",
				Timeout: "30s",
			},
			PostApply: template.HookSpec{
				Script: "post.sh",
			},
		},
	}

	preSpec := GetPartialHookSpec(p, HookTypePreApply)
	if preSpec.Script != "pre.sh" {
		t.Errorf("PreApply Script = %q, want \"pre.sh\"", preSpec.Script)
	}
	if preSpec.Timeout != "30s" {
		t.Errorf("PreApply Timeout = %q, want \"30s\"", preSpec.Timeout)
	}

	postSpec := GetPartialHookSpec(p, HookTypePostApply)
	if postSpec.Script != "post.sh" {
		t.Errorf("PostApply Script = %q, want \"post.sh\"", postSpec.Script)
	}
}

func TestHasPartialHook(t *testing.T) {
	p := &Partial{
		Hooks: PartialHooks{
			PreApply: template.HookSpec{Script: "pre.sh"},
		},
	}

	if !HasPartialHook(p, HookTypePreApply) {
		t.Error("HasPartialHook(PreApply) = false, want true")
	}
	if HasPartialHook(p, HookTypePostApply) {
		t.Error("HasPartialHook(PostApply) = true, want false")
	}
}

func TestListPartialHooks(t *testing.T) {
	tests := []struct {
		name     string
		partial  *Partial
		expected []PartialHookType
	}{
		{
			name:     "no hooks",
			partial:  &Partial{},
			expected: nil,
		},
		{
			name: "pre_apply only",
			partial: &Partial{
				Hooks: PartialHooks{
					PreApply: template.HookSpec{Script: "pre.sh"},
				},
			},
			expected: []PartialHookType{HookTypePreApply},
		},
		{
			name: "both hooks",
			partial: &Partial{
				Hooks: PartialHooks{
					PreApply:  template.HookSpec{Script: "pre.sh"},
					PostApply: template.HookSpec{Script: "post.sh"},
				},
			},
			expected: []PartialHookType{HookTypePreApply, HookTypePostApply},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ListPartialHooks(tt.partial)
			if len(result) != len(tt.expected) {
				t.Errorf("ListPartialHooks() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildPartialHookEnvFromApply(t *testing.T) {
	tmpDir := t.TempDir()

	p := &Partial{Name: "test-partial"}
	vars := map[string]string{"APP_NAME": "myapp"}
	result := &ApplyResult{FilesCreated: []string{"file.txt"}}

	env := BuildPartialHookEnvFromApply(p, "/partial/path", tmpDir, vars, true, false, result)

	if env.PartialName != "test-partial" {
		t.Errorf("PartialName = %q, want \"test-partial\"", env.PartialName)
	}
	if env.PartialPath != "/partial/path" {
		t.Errorf("PartialPath = %q, want \"/partial/path\"", env.PartialPath)
	}
	if env.TargetPath != tmpDir {
		t.Errorf("TargetPath = %q, want %q", env.TargetPath, tmpDir)
	}
	if env.TargetDirname != filepath.Base(tmpDir) {
		t.Errorf("TargetDirname = %q, want %q", env.TargetDirname, filepath.Base(tmpDir))
	}
	if !env.DryRun {
		t.Error("DryRun = false, want true")
	}
	if env.Verbose {
		t.Error("Verbose = true, want false")
	}
	if env.Variables["APP_NAME"] != "myapp" {
		t.Errorf("Variables[APP_NAME] = %q, want \"myapp\"", env.Variables["APP_NAME"])
	}
	if env.Result != result {
		t.Error("Result not set correctly")
	}
}
