package partial

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPrerequisites_AllSatisfied(t *testing.T) {
	// Use commands that exist on all systems
	p := &Partial{
		Requires: Requirements{
			Commands: []string{"ls", "pwd"},
		},
	}

	result, err := CheckPrerequisites(p, t.TempDir())
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if !result.Satisfied {
		t.Error("Expected Satisfied=true for existing commands")
	}
	if len(result.MissingCommands) != 0 {
		t.Errorf("MissingCommands = %v, want empty", result.MissingCommands)
	}
}

func TestCheckPrerequisites_MissingCommands(t *testing.T) {
	p := &Partial{
		Requires: Requirements{
			Commands: []string{"ls", "nonexistent-cmd-xyz123", "another-fake-cmd"},
		},
	}

	result, err := CheckPrerequisites(p, t.TempDir())
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if result.Satisfied {
		t.Error("Expected Satisfied=false for missing commands")
	}
	if len(result.MissingCommands) != 2 {
		t.Errorf("MissingCommands = %v, want 2 items", result.MissingCommands)
	}
}

func TestCheckPrerequisites_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one file but not the other
	os.WriteFile(filepath.Join(tmpDir, "exists.txt"), []byte("content"), 0644)

	p := &Partial{
		Requires: Requirements{
			Files: []string{"exists.txt", "missing.txt"},
		},
	}

	result, err := CheckPrerequisites(p, tmpDir)
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if result.Satisfied {
		t.Error("Expected Satisfied=false for missing files")
	}
	if len(result.MissingFiles) != 1 {
		t.Errorf("MissingFiles = %v, want 1 item", result.MissingFiles)
	}
	if len(result.MissingFiles) > 0 && result.MissingFiles[0] != "missing.txt" {
		t.Errorf("MissingFiles[0] = %q, want \"missing.txt\"", result.MissingFiles[0])
	}
}

func TestCheckPrerequisites_FileGlobPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some .go files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main"), 0644)

	p := &Partial{
		Requires: Requirements{
			Files: []string{"*.go", "*.py"},
		},
	}

	result, err := CheckPrerequisites(p, tmpDir)
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	// *.go should be satisfied, *.py should not
	if result.Satisfied {
		t.Error("Expected Satisfied=false because *.py pattern has no matches")
	}
	if len(result.MissingFiles) != 1 {
		t.Errorf("MissingFiles = %v, want 1 item", result.MissingFiles)
	}
}

func TestCheckPrerequisites_EmptyRequires(t *testing.T) {
	p := &Partial{
		Requires: Requirements{},
	}

	result, err := CheckPrerequisites(p, t.TempDir())
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if !result.Satisfied {
		t.Error("Expected Satisfied=true for empty requires")
	}
}

func TestCheckPrerequisites_ReturnsAllMissing(t *testing.T) {
	// Make sure we get all missing items, not just the first one
	p := &Partial{
		Requires: Requirements{
			Commands: []string{"fake-cmd-1", "fake-cmd-2", "fake-cmd-3"},
			Files:    []string{"missing1.txt", "missing2.txt"},
		},
	}

	result, err := CheckPrerequisites(p, t.TempDir())
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if len(result.MissingCommands) != 3 {
		t.Errorf("MissingCommands = %v, want 3 items", result.MissingCommands)
	}
	if len(result.MissingFiles) != 2 {
		t.Errorf("MissingFiles = %v, want 2 items", result.MissingFiles)
	}
}

func TestCheckPrerequisites_SkipsEmptyStrings(t *testing.T) {
	p := &Partial{
		Requires: Requirements{
			Commands: []string{"", "  ", "ls"},
			Files:    []string{"", "  "},
		},
	}

	result, err := CheckPrerequisites(p, t.TempDir())
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if !result.Satisfied {
		t.Error("Expected Satisfied=true (empty strings should be skipped)")
	}
}

func TestCommandExists_ExistingCommand(t *testing.T) {
	// ls exists on all Unix systems
	if !commandExists("ls") {
		t.Error("commandExists(\"ls\") = false, want true")
	}
}

func TestCommandExists_MissingCommand(t *testing.T) {
	if commandExists("nonexistent-command-xyz123") {
		t.Error("commandExists(\"nonexistent-command-xyz123\") = true, want false")
	}
}

func TestFileExists_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	if !fileExists(tmpDir, "test.txt") {
		t.Error("fileExists should return true for existing file")
	}
}

func TestFileExists_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	if fileExists(tmpDir, "missing.txt") {
		t.Error("fileExists should return false for missing file")
	}
}

func TestFileExists_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	// Directories should not count as files
	if fileExists(tmpDir, "subdir") {
		t.Error("fileExists should return false for directories")
	}
}

func TestFileExists_GlobPattern(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)

	if !fileExists(tmpDir, "*.go") {
		t.Error("fileExists with glob should return true when matches exist")
	}

	if fileExists(tmpDir, "*.py") {
		t.Error("fileExists with glob should return false when no matches")
	}
}

func TestHasGlob(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"*.go", true},
		{"file?.txt", true},
		{"[abc].txt", true},
		{"src/**/*.ts", true},
		{"package.json", false},
		{"path/to/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if result := hasGlob(tt.path); result != tt.expected {
				t.Errorf("hasGlob(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCheckPrerequisites_NestedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)

	p := &Partial{
		Requires: Requirements{
			Files: []string{"src/main.go"},
		},
	}

	result, err := CheckPrerequisites(p, tmpDir)
	if err != nil {
		t.Fatalf("CheckPrerequisites failed: %v", err)
	}

	if !result.Satisfied {
		t.Error("Expected Satisfied=true for nested file path")
	}
}
