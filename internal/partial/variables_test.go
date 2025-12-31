package partial

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tormodhaugland/co/internal/template"
)

func TestGetPartialBuiltins_BasicVariables(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	vars, err := GetPartialBuiltins(tmpDir)
	if err != nil {
		t.Fatalf("GetPartialBuiltins failed: %v", err)
	}

	// Check DIRNAME is set correctly
	if vars["DIRNAME"] != filepath.Base(tmpDir) {
		t.Errorf("DIRNAME = %q, want %q", vars["DIRNAME"], filepath.Base(tmpDir))
	}

	// Check DIRPATH is an absolute path
	if !filepath.IsAbs(vars["DIRPATH"]) {
		t.Errorf("DIRPATH should be absolute, got %q", vars["DIRPATH"])
	}

	// Check PARENT_DIRNAME is set
	if vars["PARENT_DIRNAME"] == "" {
		t.Error("PARENT_DIRNAME should not be empty")
	}

	// Check date/time variables are set
	if vars["DATE"] == "" {
		t.Error("DATE should not be empty")
	}
	if vars["YEAR"] == "" {
		t.Error("YEAR should not be empty")
	}
	if vars["TIMESTAMP"] == "" {
		t.Error("TIMESTAMP should not be empty")
	}
}

func TestGetPartialBuiltins_NonGitDirectory(t *testing.T) {
	// Create a temp directory that is NOT a git repo
	tmpDir := t.TempDir()

	vars, err := GetPartialBuiltins(tmpDir)
	if err != nil {
		t.Fatalf("GetPartialBuiltins failed: %v", err)
	}

	// IS_GIT_REPO should be "false"
	if vars["IS_GIT_REPO"] != "false" {
		t.Errorf("IS_GIT_REPO = %q, want \"false\"", vars["IS_GIT_REPO"])
	}

	// GIT_BRANCH should not be set (or empty)
	if vars["GIT_BRANCH"] != "" {
		t.Errorf("GIT_BRANCH should be empty for non-git dir, got %q", vars["GIT_BRANCH"])
	}

	// GIT_REMOTE_URL should not be set (or empty)
	if vars["GIT_REMOTE_URL"] != "" {
		t.Errorf("GIT_REMOTE_URL should be empty for non-git dir, got %q", vars["GIT_REMOTE_URL"])
	}
}

func TestGetPartialBuiltins_GitDirectory(t *testing.T) {
	// Create a temp directory and initialize a git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not initialize git repo: %v", err)
	}

	// Configure git user (required for commit)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create a file and commit (required for git.GetInfo to work)
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Skipf("Could not create test file: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not make initial commit: %v", err)
	}

	// Set a local branch name
	cmd = exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		// Try alternative command for older git
		cmd = exec.Command("git", "branch", "-M", "test-branch")
		cmd.Dir = tmpDir
		cmd.Run()
	}

	vars, err := GetPartialBuiltins(tmpDir)
	if err != nil {
		t.Fatalf("GetPartialBuiltins failed: %v", err)
	}

	// IS_GIT_REPO should be "true"
	if vars["IS_GIT_REPO"] != "true" {
		t.Errorf("IS_GIT_REPO = %q, want \"true\"", vars["IS_GIT_REPO"])
	}

	// GIT_BRANCH should be set (we made a commit)
	if vars["GIT_BRANCH"] == "" {
		t.Error("GIT_BRANCH should not be empty after commit")
	}
}

func TestGetPartialBuiltins_GitRemote(t *testing.T) {
	// Create a temp directory and initialize a git repo with remote
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not initialize git repo: %v", err)
	}

	// Configure git user (required for commit)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create a file and commit (required for GetInfo to work)
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Skipf("Could not create test file: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not make initial commit: %v", err)
	}

	// Add a remote
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/test/repo.git")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not add git remote: %v", err)
	}

	vars, err := GetPartialBuiltins(tmpDir)
	if err != nil {
		t.Fatalf("GetPartialBuiltins failed: %v", err)
	}

	// GIT_REMOTE_URL should be set
	if vars["GIT_REMOTE_URL"] != "https://github.com/test/repo.git" {
		t.Errorf("GIT_REMOTE_URL = %q, want %q", vars["GIT_REMOTE_URL"], "https://github.com/test/repo.git")
	}
}

func TestGetPartialBuiltins_RelativePath(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to parent and use relative path
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	vars, err := GetPartialBuiltins("subdir")
	if err != nil {
		t.Fatalf("GetPartialBuiltins failed: %v", err)
	}

	// DIRPATH should still be absolute
	if !filepath.IsAbs(vars["DIRPATH"]) {
		t.Errorf("DIRPATH should be absolute even with relative input, got %q", vars["DIRPATH"])
	}

	// DIRNAME should be "subdir"
	if vars["DIRNAME"] != "subdir" {
		t.Errorf("DIRNAME = %q, want \"subdir\"", vars["DIRNAME"])
	}

	// PARENT_DIRNAME should be the temp dir name
	if vars["PARENT_DIRNAME"] != filepath.Base(tmpDir) {
		t.Errorf("PARENT_DIRNAME = %q, want %q", vars["PARENT_DIRNAME"], filepath.Base(tmpDir))
	}
}

func TestResolvePartialVariables_MergesBuiltinsWithProvided(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "custom_var", Type: template.VarTypeString, Required: false},
		},
	}

	builtins := map[string]string{
		"DIRNAME": "myproject",
		"YEAR":    "2025",
	}
	provided := map[string]string{
		"custom_var": "custom_value",
	}

	resolved, err := ResolvePartialVariables(p, provided, builtins)
	if err != nil {
		t.Fatalf("ResolvePartialVariables failed: %v", err)
	}

	// Check builtins are preserved
	if resolved["DIRNAME"] != "myproject" {
		t.Errorf("DIRNAME = %q, want \"myproject\"", resolved["DIRNAME"])
	}
	if resolved["YEAR"] != "2025" {
		t.Errorf("YEAR = %q, want \"2025\"", resolved["YEAR"])
	}

	// Check provided value is set
	if resolved["custom_var"] != "custom_value" {
		t.Errorf("custom_var = %q, want \"custom_value\"", resolved["custom_var"])
	}
}

func TestResolvePartialVariables_AppliesDefaults(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "port", Type: template.VarTypeString, Default: "3000"},
			{Name: "host", Type: template.VarTypeString, Default: "localhost"},
		},
	}

	resolved, err := ResolvePartialVariables(p, nil, nil)
	if err != nil {
		t.Fatalf("ResolvePartialVariables failed: %v", err)
	}

	if resolved["port"] != "3000" {
		t.Errorf("port = %q, want \"3000\"", resolved["port"])
	}
	if resolved["host"] != "localhost" {
		t.Errorf("host = %q, want \"localhost\"", resolved["host"])
	}
}

func TestResolvePartialVariables_SubstitutesBuiltinsInDefaults(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "app_name", Type: template.VarTypeString, Default: "{{DIRNAME}}"},
		},
	}

	builtins := map[string]string{
		"DIRNAME": "my-app",
	}

	resolved, err := ResolvePartialVariables(p, nil, builtins)
	if err != nil {
		t.Fatalf("ResolvePartialVariables failed: %v", err)
	}

	// Default value should have DIRNAME substituted
	if resolved["app_name"] != "my-app" {
		t.Errorf("app_name = %q, want \"my-app\"", resolved["app_name"])
	}
}

func TestResolvePartialVariables_MissingRequired(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "api_key", Type: template.VarTypeString, Required: true, Description: "Your API key"},
		},
	}

	_, err := ResolvePartialVariables(p, nil, nil)
	if err == nil {
		t.Fatal("Expected error for missing required variable, got nil")
	}

	if !strings.Contains(err.Error(), "api_key") {
		t.Errorf("Error should mention variable name, got: %v", err)
	}
}

func TestResolvePartialVariables_ValidatesType(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "enabled", Type: template.VarTypeBoolean, Required: false},
		},
	}

	// Provide an invalid boolean value
	provided := map[string]string{
		"enabled": "maybe", // Invalid - should be true/false/yes/no/1/0
	}

	_, err := ResolvePartialVariables(p, provided, nil)
	if err == nil {
		t.Fatal("Expected error for invalid boolean value, got nil")
	}
}

func TestResolvePartialVariables_ValidatesChoices(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{
				Name:     "env",
				Type:     template.VarTypeChoice,
				Choices:  []string{"dev", "staging", "prod"},
				Required: true,
			},
		},
	}

	// Provide an invalid choice
	provided := map[string]string{
		"env": "invalid",
	}

	_, err := ResolvePartialVariables(p, provided, nil)
	if err == nil {
		t.Fatal("Expected error for invalid choice, got nil")
	}
}

func TestResolvePartialVariables_ValidatesPattern(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{
				Name:       "port",
				Type:       template.VarTypeString,
				Validation: "^[0-9]+$",
				Required:   true,
			},
		},
	}

	// Provide a non-numeric port
	provided := map[string]string{
		"port": "abc",
	}

	_, err := ResolvePartialVariables(p, provided, nil)
	if err == nil {
		t.Fatal("Expected error for pattern validation failure, got nil")
	}
}

func TestResolvePartialVariables_ProvidedOverridesDefault(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "port", Type: template.VarTypeString, Default: "3000"},
		},
	}

	provided := map[string]string{
		"port": "8080",
	}

	resolved, err := ResolvePartialVariables(p, provided, nil)
	if err != nil {
		t.Fatalf("ResolvePartialVariables failed: %v", err)
	}

	// Provided value should override default
	if resolved["port"] != "8080" {
		t.Errorf("port = %q, want \"8080\"", resolved["port"])
	}
}

func TestResolvePartialVariables_EmptyPartial(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{}, // No variables defined
	}

	builtins := map[string]string{
		"DIRNAME": "test",
	}

	resolved, err := ResolvePartialVariables(p, nil, builtins)
	if err != nil {
		t.Fatalf("ResolvePartialVariables failed: %v", err)
	}

	// Builtins should still be present
	if resolved["DIRNAME"] != "test" {
		t.Errorf("DIRNAME = %q, want \"test\"", resolved["DIRNAME"])
	}
}

func TestResolvePartialVariables_NilInputs(t *testing.T) {
	p := &Partial{
		Variables: []PartialVar{
			{Name: "opt", Type: template.VarTypeString, Required: false},
		},
	}

	// Both provided and builtins are nil
	resolved, err := ResolvePartialVariables(p, nil, nil)
	if err != nil {
		t.Fatalf("ResolvePartialVariables failed: %v", err)
	}

	// Should have empty string for optional var
	if resolved["opt"] != "" {
		t.Errorf("opt = %q, want empty string", resolved["opt"])
	}
}
