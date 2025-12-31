package partial

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanPartialFiles(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		filesConfig PartialFiles
		wantFiles   []string
		wantErr     bool
	}{
		{
			name: "finds all files in files/ subdirectory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filesDir := filepath.Join(dir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file1.txt"), []byte("content1"), 0644)
				os.WriteFile(filepath.Join(filesDir, "file2.txt"), []byte("content2"), 0644)
				return dir
			},
			filesConfig: PartialFiles{},
			wantFiles:   []string{"file1.txt", "file2.txt"},
		},
		{
			name: "respects include patterns",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filesDir := filepath.Join(dir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file.go"), []byte(""), 0644)
				os.WriteFile(filepath.Join(filesDir, "file.txt"), []byte(""), 0644)
				os.WriteFile(filepath.Join(filesDir, "file.md"), []byte(""), 0644)
				return dir
			},
			filesConfig: PartialFiles{Include: []string{"*.go", "*.txt"}},
			wantFiles:   []string{"file.go", "file.txt"},
		},
		{
			name: "respects exclude patterns",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filesDir := filepath.Join(dir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file1.txt"), []byte(""), 0644)
				os.WriteFile(filepath.Join(filesDir, "file2.txt"), []byte(""), 0644)
				os.WriteFile(filepath.Join(filesDir, "ignore.txt"), []byte(""), 0644)
				return dir
			},
			filesConfig: PartialFiles{Exclude: []string{"ignore*"}},
			wantFiles:   []string{"file1.txt", "file2.txt"},
		},
		{
			name: "handles nested directories",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filesDir := filepath.Join(dir, "files")
				nestedDir := filepath.Join(filesDir, "subdir")
				os.MkdirAll(nestedDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "root.txt"), []byte(""), 0644)
				os.WriteFile(filepath.Join(nestedDir, "nested.txt"), []byte(""), 0644)
				return dir
			},
			filesConfig: PartialFiles{},
			wantFiles:   []string{"root.txt", "subdir/nested.txt"},
		},
		{
			name: "empty files/ returns empty list",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.MkdirAll(filepath.Join(dir, "files"), 0755)
				return dir
			},
			filesConfig: PartialFiles{},
			wantFiles:   nil,
		},
		{
			name: "missing files/ returns empty list",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			filesConfig: PartialFiles{},
			wantFiles:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partialPath := tt.setup(t)
			got, err := ScanPartialFiles(partialPath, tt.filesConfig)

			if (err != nil) != tt.wantErr {
				t.Errorf("ScanPartialFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.wantFiles) {
				t.Errorf("ScanPartialFiles() got %d files, want %d", len(got), len(tt.wantFiles))
				return
			}

			// Check all expected files are present (order may vary)
			gotMap := make(map[string]bool)
			for _, f := range got {
				gotMap[f] = true
			}
			for _, want := range tt.wantFiles {
				if !gotMap[want] {
					t.Errorf("ScanPartialFiles() missing file %q", want)
				}
			}
		})
	}
}

func TestDetectConflicts(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (partialPath, targetPath string, files []string)
		conflicts    ConflictConfig
		extensions   []string
		wantCreates  int
		wantSkips    int
		wantStrategy FileAction
		wantErr      bool
	}{
		{
			name: "no conflicts when target is empty",
			setup: func(t *testing.T) (string, string, []string) {
				partialDir := t.TempDir()
				targetDir := t.TempDir()
				filesDir := filepath.Join(partialDir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file.txt"), []byte("content"), 0644)
				return partialDir, targetDir, []string{"file.txt"}
			},
			conflicts:   ConflictConfig{Strategy: "skip"},
			wantCreates: 1,
			wantSkips:   0,
		},
		{
			name: "detects existing files as conflicts",
			setup: func(t *testing.T) (string, string, []string) {
				partialDir := t.TempDir()
				targetDir := t.TempDir()
				filesDir := filepath.Join(partialDir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "existing.txt"), []byte("new"), 0644)
				os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("old"), 0644)
				return partialDir, targetDir, []string{"existing.txt"}
			},
			conflicts:    ConflictConfig{Strategy: "skip"},
			wantCreates:  0,
			wantSkips:    1,
			wantStrategy: ActionSkip,
		},
		{
			name: "respects preserve patterns",
			setup: func(t *testing.T) (string, string, []string) {
				partialDir := t.TempDir()
				targetDir := t.TempDir()
				filesDir := filepath.Join(partialDir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, ".gitignore"), []byte("new"), 0644)
				os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte("old"), 0644)
				return partialDir, targetDir, []string{".gitignore"}
			},
			conflicts:   ConflictConfig{Strategy: "overwrite", Preserve: []string{".gitignore"}},
			wantCreates: 0,
			wantSkips:   1,
		},
		{
			name: "overwrite strategy sets ActionOverwrite",
			setup: func(t *testing.T) (string, string, []string) {
				partialDir := t.TempDir()
				targetDir := t.TempDir()
				filesDir := filepath.Join(partialDir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file.txt"), []byte("new"), 0644)
				os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("old"), 0644)
				return partialDir, targetDir, []string{"file.txt"}
			},
			conflicts:    ConflictConfig{Strategy: "overwrite"},
			wantStrategy: ActionOverwrite,
		},
		{
			name: "backup strategy sets ActionBackup",
			setup: func(t *testing.T) (string, string, []string) {
				partialDir := t.TempDir()
				targetDir := t.TempDir()
				filesDir := filepath.Join(partialDir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file.txt"), []byte("new"), 0644)
				os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("old"), 0644)
				return partialDir, targetDir, []string{"file.txt"}
			},
			conflicts:    ConflictConfig{Strategy: "backup"},
			wantStrategy: ActionBackup,
		},
		{
			name: "strips .tmpl extension for destination",
			setup: func(t *testing.T) (string, string, []string) {
				partialDir := t.TempDir()
				targetDir := t.TempDir()
				filesDir := filepath.Join(partialDir, "files")
				os.MkdirAll(filesDir, 0755)
				os.WriteFile(filepath.Join(filesDir, "file.txt.tmpl"), []byte("{{VAR}}"), 0644)
				return partialDir, targetDir, []string{"file.txt.tmpl"}
			},
			conflicts:   ConflictConfig{Strategy: "skip"},
			extensions:  []string{".tmpl"},
			wantCreates: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partialPath, targetPath, files := tt.setup(t)

			extensions := tt.extensions
			if len(extensions) == 0 {
				extensions = []string{".tmpl"}
			}

			plan, err := DetectConflicts(files, partialPath, targetPath, tt.conflicts, extensions)

			if (err != nil) != tt.wantErr {
				t.Errorf("DetectConflicts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if plan.Creates != tt.wantCreates {
				t.Errorf("DetectConflicts() Creates = %d, want %d", plan.Creates, tt.wantCreates)
			}

			if plan.Skips != tt.wantSkips {
				t.Errorf("DetectConflicts() Skips = %d, want %d", plan.Skips, tt.wantSkips)
			}

			if tt.wantStrategy != 0 && len(plan.Files) > 0 && plan.Files[0].Action != tt.wantStrategy {
				t.Errorf("DetectConflicts() first file action = %v, want %v", plan.Files[0].Action, tt.wantStrategy)
			}
		})
	}
}

func TestProcessFile(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) (srcPath, destPath string)
		isTemplate bool
		vars       map[string]string
		wantErr    bool
		checkDest  func(t *testing.T, destPath string)
	}{
		{
			name: "copies non-template file correctly",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dest := filepath.Join(dir, "dest.txt")
				os.WriteFile(src, []byte("hello world"), 0644)
				return src, dest
			},
			isTemplate: false,
			vars:       nil,
			checkDest: func(t *testing.T, destPath string) {
				content, _ := os.ReadFile(destPath)
				if string(content) != "hello world" {
					t.Errorf("expected 'hello world', got %q", content)
				}
			},
		},
		{
			name: "substitutes variables in template files",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt.tmpl")
				dest := filepath.Join(dir, "dest.txt")
				os.WriteFile(src, []byte("Hello, {{NAME}}!"), 0644)
				return src, dest
			},
			isTemplate: true,
			vars:       map[string]string{"NAME": "World"},
			checkDest: func(t *testing.T, destPath string) {
				content, _ := os.ReadFile(destPath)
				if string(content) != "Hello, World!" {
					t.Errorf("expected 'Hello, World!', got %q", content)
				}
			},
		},
		{
			name: "creates parent directories",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dest := filepath.Join(dir, "nested", "deep", "dest.txt")
				os.WriteFile(src, []byte("content"), 0644)
				return src, dest
			},
			isTemplate: false,
			checkDest: func(t *testing.T, destPath string) {
				if _, err := os.Stat(destPath); err != nil {
					t.Errorf("file not created: %v", err)
				}
			},
		},
		{
			name: "preserves file permissions",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "script.sh")
				dest := filepath.Join(dir, "dest.sh")
				os.WriteFile(src, []byte("#!/bin/bash"), 0755)
				return src, dest
			},
			isTemplate: false,
			checkDest: func(t *testing.T, destPath string) {
				info, _ := os.Stat(destPath)
				if info.Mode().Perm() != 0755 {
					t.Errorf("expected permissions 0755, got %o", info.Mode().Perm())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcPath, destPath := tt.setup(t)

			err := ProcessFile(srcPath, destPath, tt.isTemplate, tt.vars, []string{".tmpl"})

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkDest != nil && !tt.wantErr {
				tt.checkDest(t, destPath)
			}
		})
	}
}

func TestExecuteBackup(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) string
		wantBackupExt string
	}{
		{
			name: "creates .bak file",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "file.txt")
				os.WriteFile(path, []byte("original"), 0644)
				return path
			},
			wantBackupExt: ".bak",
		},
		{
			name: "creates .bak.1 if .bak exists",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "file.txt")
				os.WriteFile(path, []byte("original"), 0644)
				os.WriteFile(path+".bak", []byte("backup1"), 0644)
				return path
			},
			wantBackupExt: ".bak.1",
		},
		{
			name: "creates .bak.2 if .bak and .bak.1 exist",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "file.txt")
				os.WriteFile(path, []byte("original"), 0644)
				os.WriteFile(path+".bak", []byte("backup1"), 0644)
				os.WriteFile(path+".bak.1", []byte("backup2"), 0644)
				return path
			},
			wantBackupExt: ".bak.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)

			backupPath, err := ExecuteBackup(path)
			if err != nil {
				t.Fatalf("ExecuteBackup() error = %v", err)
			}

			wantPath := path + tt.wantBackupExt
			if backupPath != wantPath {
				t.Errorf("ExecuteBackup() = %q, want %q", backupPath, wantPath)
			}

			// Check backup file exists and has correct content
			content, err := os.ReadFile(backupPath)
			if err != nil {
				t.Errorf("backup file not readable: %v", err)
			}
			if string(content) != "original" {
				t.Errorf("backup content = %q, want 'original'", content)
			}
		})
	}
}

func TestIsPreserved(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		patterns []string
		want     bool
	}{
		{"empty patterns returns false", "file.txt", nil, false},
		{"exact match", ".gitignore", []string{".gitignore"}, true},
		{"glob match", "config.json", []string{"*.json"}, true},
		{"no match", "file.txt", []string{"*.go"}, false},
		{"multiple patterns", "file.go", []string{"*.txt", "*.go"}, true},
		{"nested path match", "src/main.go", []string{"src/*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPreserved(tt.relPath, tt.patterns)
			if got != tt.want {
				t.Errorf("IsPreserved(%q, %v) = %v, want %v", tt.relPath, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Run("copies file with correct permissions", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dest := filepath.Join(dir, "dest.txt")

		os.WriteFile(src, []byte("test content"), 0644)

		err := CopyFile(src, dest, 0755)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		content, _ := os.ReadFile(dest)
		if string(content) != "test content" {
			t.Errorf("content = %q, want 'test content'", content)
		}

		info, _ := os.Stat(dest)
		if info.Mode().Perm() != 0755 {
			t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dest := filepath.Join(dir, "a", "b", "c", "dest.txt")

		os.WriteFile(src, []byte("content"), 0644)

		err := CopyFile(src, dest, 0644)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		if _, err := os.Stat(dest); err != nil {
			t.Errorf("file not created: %v", err)
		}
	})
}

func TestValidateTargetPath(t *testing.T) {
	t.Run("valid directory", func(t *testing.T) {
		dir := t.TempDir()
		err := ValidateTargetPath(dir)
		if err != nil {
			t.Errorf("ValidateTargetPath() error = %v, want nil", err)
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		err := ValidateTargetPath("/nonexistent/path/12345")
		if _, ok := err.(*TargetNotFoundError); !ok {
			t.Errorf("ValidateTargetPath() error = %T, want *TargetNotFoundError", err)
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file.txt")
		os.WriteFile(file, []byte(""), 0644)

		err := ValidateTargetPath(file)
		if err == nil {
			t.Error("ValidateTargetPath() expected error for file path")
		}
	})
}

func TestFormatDetection(t *testing.T) {
	tests := []struct {
		path        string
		isGitignore bool
		isJSON      bool
		isYAML      bool
		canMerge    bool
	}{
		{".gitignore", true, false, false, true},
		{".dockerignore", true, false, false, true},
		{".npmignore", true, false, false, true},
		{"config.json", false, true, false, true},
		{"settings.JSON", false, true, false, true},
		{"config.yaml", false, false, true, true},
		{"config.yml", false, false, true, true},
		{"config.YML", false, false, true, true},
		{"script.sh", false, false, false, false},
		{"readme.md", false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if IsGitignoreFile(tt.path) != tt.isGitignore {
				t.Errorf("IsGitignoreFile(%q) = %v, want %v", tt.path, !tt.isGitignore, tt.isGitignore)
			}
			if IsJSONFile(tt.path) != tt.isJSON {
				t.Errorf("IsJSONFile(%q) = %v, want %v", tt.path, !tt.isJSON, tt.isJSON)
			}
			if IsYAMLFile(tt.path) != tt.isYAML {
				t.Errorf("IsYAMLFile(%q) = %v, want %v", tt.path, !tt.isYAML, tt.isYAML)
			}
			if CanMerge(tt.path) != tt.canMerge {
				t.Errorf("CanMerge(%q) = %v, want %v", tt.path, !tt.canMerge, tt.canMerge)
			}
		})
	}
}

func TestResolveConflict(t *testing.T) {
	tests := []struct {
		name     string
		info     *FileInfo
		strategy string
		want     FileAction
	}{
		{"skip", &FileInfo{RelPath: "file.txt"}, "skip", ActionSkip},
		{"overwrite", &FileInfo{RelPath: "file.txt"}, "overwrite", ActionOverwrite},
		{"backup", &FileInfo{RelPath: "file.txt"}, "backup", ActionBackup},
		{"merge supported", &FileInfo{RelPath: "config.json"}, "merge", ActionMerge},
		{"merge unsupported", &FileInfo{RelPath: "readme.md"}, "merge", ActionPrompt},
		{"prompt", &FileInfo{RelPath: "file.txt"}, "prompt", ActionPrompt},
		{"unknown", &FileInfo{RelPath: "file.txt"}, "unknown", ActionPrompt},
		{"empty", &FileInfo{RelPath: "file.txt"}, "", ActionPrompt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveConflict(tt.info, tt.strategy)
			if got != tt.want {
				t.Errorf("ResolveConflict(%q) = %v, want %v", tt.strategy, got, tt.want)
			}
		})
	}
}
