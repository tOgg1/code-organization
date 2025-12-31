package partial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestPartialWithFiles creates a temporary partial with files for apply testing.
func setupTestPartialWithFiles(t *testing.T, name string, manifest *Partial, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	partialDir := filepath.Join(dir, name)
	filesDir := filepath.Join(partialDir, "files")

	os.MkdirAll(filesDir, 0755)

	// Write manifest
	if manifest == nil {
		manifest = &Partial{
			Schema:      1,
			Name:        name,
			Description: "Test partial",
		}
	}
	manifestBytes, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(partialDir, "partial.json"), manifestBytes, 0644)

	// Write files
	for path, content := range files {
		fullPath := filepath.Join(filesDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	return dir
}

func TestApply_HappyPath(t *testing.T) {
	// Create a test partial
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"file1.txt":         "Hello World",
		"subdir/file2.txt":  "Nested file",
		"template.txt.tmpl": "Hello, {{NAME}}!",
	})

	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName:      "test-partial",
		TargetPath:       targetDir,
		Variables:        map[string]string{"NAME": "Test"},
		ConflictStrategy: "skip",
		Yes:              true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Check result
	if result.PartialName != "test-partial" {
		t.Errorf("PartialName = %q, want 'test-partial'", result.PartialName)
	}

	if len(result.FilesCreated) != 3 {
		t.Errorf("FilesCreated = %d, want 3", len(result.FilesCreated))
	}

	// Check files exist
	if _, err := os.Stat(filepath.Join(targetDir, "file1.txt")); err != nil {
		t.Errorf("file1.txt not created: %v", err)
	}

	// Check nested file
	if _, err := os.Stat(filepath.Join(targetDir, "subdir", "file2.txt")); err != nil {
		t.Errorf("subdir/file2.txt not created: %v", err)
	}

	// Check template was processed (extension stripped)
	if _, err := os.Stat(filepath.Join(targetDir, "template.txt")); err != nil {
		t.Errorf("template.txt not created: %v", err)
	}

	// Check template content
	content, _ := os.ReadFile(filepath.Join(targetDir, "template.txt"))
	if string(content) != "Hello, Test!" {
		t.Errorf("template content = %q, want 'Hello, Test!'", content)
	}
}

func TestApply_SkipStrategy(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"existing.txt": "new content",
		"new.txt":      "new file",
	})

	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("original content"), 0644)

	opts := ApplyOptions{
		PartialName:      "test-partial",
		TargetPath:       targetDir,
		ConflictStrategy: "skip",
		Yes:              true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(result.FilesCreated) != 1 {
		t.Errorf("FilesCreated = %d, want 1", len(result.FilesCreated))
	}

	if len(result.FilesSkipped) != 1 {
		t.Errorf("FilesSkipped = %d, want 1", len(result.FilesSkipped))
	}

	// Check existing file was not overwritten
	content, _ := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
	if string(content) != "original content" {
		t.Errorf("existing file was overwritten: got %q", content)
	}
}

func TestApply_OverwriteStrategy(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"existing.txt": "new content",
	})

	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("original content"), 0644)

	opts := ApplyOptions{
		PartialName:      "test-partial",
		TargetPath:       targetDir,
		ConflictStrategy: "overwrite",
		Yes:              true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(result.FilesOverwritten) != 1 {
		t.Errorf("FilesOverwritten = %d, want 1", len(result.FilesOverwritten))
	}

	// Check file was overwritten
	content, _ := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
	if string(content) != "new content" {
		t.Errorf("file was not overwritten: got %q", content)
	}
}

func TestApply_BackupStrategy(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"existing.txt": "new content",
	})

	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("original content"), 0644)

	opts := ApplyOptions{
		PartialName:      "test-partial",
		TargetPath:       targetDir,
		ConflictStrategy: "backup",
		Yes:              true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(result.FilesBackedUp) != 1 {
		t.Errorf("FilesBackedUp = %d, want 1", len(result.FilesBackedUp))
	}

	// Check backup file exists
	if _, err := os.Stat(filepath.Join(targetDir, "existing.txt.bak")); err != nil {
		t.Errorf("backup file not created: %v", err)
	}

	// Check backup has original content
	backupContent, _ := os.ReadFile(filepath.Join(targetDir, "existing.txt.bak"))
	if string(backupContent) != "original content" {
		t.Errorf("backup content = %q, want 'original content'", backupContent)
	}

	// Check original was replaced
	newContent, _ := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
	if string(newContent) != "new content" {
		t.Errorf("new content = %q, want 'new content'", newContent)
	}
}

func TestApply_PreservePatterns(t *testing.T) {
	manifest := &Partial{
		Schema:      1,
		Name:        "test-partial",
		Description: "Test partial",
		Conflicts: ConflictConfig{
			Strategy: "overwrite",
			Preserve: []string{".gitignore"},
		},
	}

	partialsDir := setupTestPartialWithFiles(t, "test-partial", manifest, map[string]string{
		".gitignore": "new content",
		"file.txt":   "file content",
	})

	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte("original"), 0644)

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		Yes:         true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Check .gitignore was skipped
	if len(result.FilesSkipped) != 1 || result.FilesSkipped[0] != ".gitignore" {
		t.Errorf("FilesSkipped = %v, want ['.gitignore']", result.FilesSkipped)
	}

	// Check .gitignore was not modified
	content, _ := os.ReadFile(filepath.Join(targetDir, ".gitignore"))
	if string(content) != "original" {
		t.Errorf(".gitignore was modified: got %q", content)
	}
}

func TestApply_DryRun(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"file.txt": "content",
	})

	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		DryRun:      true,
		Yes:         true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Check files would be created
	if len(result.FilesCreated) != 1 {
		t.Errorf("FilesCreated = %d, want 1", len(result.FilesCreated))
	}

	// Check file was NOT actually created
	if _, err := os.Stat(filepath.Join(targetDir, "file.txt")); !os.IsNotExist(err) {
		t.Error("file was created during dry run")
	}
}

func TestApply_MissingPartial(t *testing.T) {
	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName: "nonexistent",
		TargetPath:  targetDir,
		Yes:         true,
	}

	_, err := Apply(opts, []string{t.TempDir()})
	if err == nil {
		t.Error("Apply() expected error for missing partial")
	}

	if _, ok := err.(*PartialNotFoundError); !ok {
		t.Errorf("Apply() error = %T, want *PartialNotFoundError", err)
	}
}

func TestApply_MissingTarget(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"file.txt": "content",
	})

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  "/nonexistent/path/12345",
		Yes:         true,
	}

	_, err := Apply(opts, []string{partialsDir})
	if err == nil {
		t.Error("Apply() expected error for missing target")
	}

	if _, ok := err.(*TargetNotFoundError); !ok {
		t.Errorf("Apply() error = %T, want *TargetNotFoundError", err)
	}
}

func TestApply_MissingRequiredVariable(t *testing.T) {
	manifest := &Partial{
		Schema:      1,
		Name:        "test-partial",
		Description: "Test partial",
		Variables: []PartialVar{
			{Name: "REQUIRED_VAR", Required: true, Description: "A required variable"},
		},
	}

	partialsDir := setupTestPartialWithFiles(t, "test-partial", manifest, map[string]string{
		"file.txt": "{{REQUIRED_VAR}}",
	})

	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		Yes:         true,
	}

	_, err := Apply(opts, []string{partialsDir})
	if err == nil {
		t.Error("Apply() expected error for missing required variable")
	}

	if !strings.Contains(err.Error(), "REQUIRED_VAR") {
		t.Errorf("error should mention REQUIRED_VAR: %v", err)
	}
}

func TestApply_InvalidVariableValue(t *testing.T) {
	manifest := &Partial{
		Schema:      1,
		Name:        "test-partial",
		Description: "Test partial",
		Variables: []PartialVar{
			{Name: "CHOICE_VAR", Type: "choice", Required: true, Choices: []string{"a", "b", "c"}},
		},
	}

	partialsDir := setupTestPartialWithFiles(t, "test-partial", manifest, map[string]string{
		"file.txt": "{{CHOICE_VAR}}",
	})

	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		Variables:   map[string]string{"CHOICE_VAR": "invalid"},
		Yes:         true,
	}

	_, err := Apply(opts, []string{partialsDir})
	if err == nil {
		t.Error("Apply() expected error for invalid variable value")
	}
}

func TestApply_InvalidConflictStrategy(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"file.txt": "content",
	})

	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName:      "test-partial",
		TargetPath:       targetDir,
		ConflictStrategy: "invalid_strategy",
		Yes:              true,
	}

	_, err := Apply(opts, []string{partialsDir})
	if err == nil {
		t.Error("Apply() expected error for invalid conflict strategy")
	}
}

func TestApply_VariableDefaults(t *testing.T) {
	manifest := &Partial{
		Schema:      1,
		Name:        "test-partial",
		Description: "Test partial",
		Variables: []PartialVar{
			{Name: "VAR_WITH_DEFAULT", Default: "default_value"},
		},
	}

	partialsDir := setupTestPartialWithFiles(t, "test-partial", manifest, map[string]string{
		"file.txt.tmpl": "Value: {{VAR_WITH_DEFAULT}}",
	})

	targetDir := t.TempDir()

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		Yes:         true,
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(result.FilesCreated) != 1 {
		t.Errorf("FilesCreated = %d, want 1", len(result.FilesCreated))
	}

	content, _ := os.ReadFile(filepath.Join(targetDir, "file.txt"))
	if string(content) != "Value: default_value" {
		t.Errorf("content = %q, want 'Value: default_value'", content)
	}
}

func TestApply_BuiltinVariables(t *testing.T) {
	partialsDir := setupTestPartialWithFiles(t, "test-partial", nil, map[string]string{
		"file.txt.tmpl": "Dir: {{DIRNAME}}",
	})

	targetDir := t.TempDir()
	expectedDirname := filepath.Base(targetDir)

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		Yes:         true,
	}

	_, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(targetDir, "file.txt"))
	expected := "Dir: " + expectedDirname
	if string(content) != expected {
		t.Errorf("content = %q, want %q", content, expected)
	}
}

func TestApply_YesFlagDefaultsToSkip(t *testing.T) {
	// When --yes is set and strategy is prompt, should default to skip
	manifest := &Partial{
		Schema:      1,
		Name:        "test-partial",
		Description: "Test partial",
		Conflicts: ConflictConfig{
			Strategy: "prompt", // Would normally require interactive input
		},
	}

	partialsDir := setupTestPartialWithFiles(t, "test-partial", manifest, map[string]string{
		"existing.txt": "new content",
	})

	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("original"), 0644)

	opts := ApplyOptions{
		PartialName: "test-partial",
		TargetPath:  targetDir,
		Yes:         true, // This should cause prompt to be treated as skip
	}

	result, err := Apply(opts, []string{partialsDir})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Should have skipped the file instead of erroring or hanging
	if len(result.FilesSkipped) != 1 {
		t.Errorf("FilesSkipped = %d, want 1", len(result.FilesSkipped))
	}

	// Original should be unchanged
	content, _ := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
	if string(content) != "original" {
		t.Errorf("file was modified: got %q", content)
	}
}

func TestGetPartialBuiltins(t *testing.T) {
	dir := t.TempDir()

	builtins, err := GetPartialBuiltins(dir)
	if err != nil {
		t.Fatalf("GetPartialBuiltins failed: %v", err)
	}

	// Check required builtins exist
	required := []string{"DATE", "YEAR", "TIMESTAMP", "DIRNAME", "DIRPATH", "PARENT_DIRNAME", "IS_GIT_REPO"}
	for _, name := range required {
		if _, ok := builtins[name]; !ok {
			t.Errorf("missing builtin: %s", name)
		}
	}

	// Check DIRNAME is correct
	if builtins["DIRNAME"] != filepath.Base(dir) {
		t.Errorf("DIRNAME = %q, want %q", builtins["DIRNAME"], filepath.Base(dir))
	}

	// Check DIRPATH is absolute
	if builtins["DIRPATH"] != dir {
		t.Errorf("DIRPATH = %q, want %q", builtins["DIRPATH"], dir)
	}

	// Non-git directory should have IS_GIT_REPO=false
	if builtins["IS_GIT_REPO"] != "false" {
		t.Errorf("IS_GIT_REPO = %q, want 'false'", builtins["IS_GIT_REPO"])
	}
}
