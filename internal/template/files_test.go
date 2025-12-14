package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldIncludeFile(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		include []string
		exclude []string
		want    bool
	}{
		{
			name:    "no patterns includes all",
			path:    "file.txt",
			include: nil,
			exclude: nil,
			want:    true,
		},
		{
			name:    "include match",
			path:    "main.go",
			include: []string{"*.go"},
			exclude: nil,
			want:    true,
		},
		{
			name:    "include no match",
			path:    "main.js",
			include: []string{"*.go"},
			exclude: nil,
			want:    false,
		},
		{
			name:    "exclude match",
			path:    "file.bak",
			include: nil,
			exclude: []string{"*.bak"},
			want:    false,
		},
		{
			name:    "exclude no match",
			path:    "file.txt",
			include: nil,
			exclude: []string{"*.bak"},
			want:    true,
		},
		{
			name:    "include and exclude combined",
			path:    "main_test.go",
			include: []string{"**/*.go"},
			exclude: []string{"*_test.go"},
			want:    false,
		},
		{
			name:    "include and exclude combined passes",
			path:    "main.go",
			include: []string{"**/*.go"},
			exclude: []string{"*_test.go"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIncludeFile(tt.path, tt.include, tt.exclude)
			if got != tt.want {
				t.Errorf("ShouldIncludeFile(%q, %v, %v) = %v, want %v",
					tt.path, tt.include, tt.exclude, got, tt.want)
			}
		})
	}
}

func TestProcessGlobalFiles(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	destDir := filepath.Join(tmpDir, "dest")

	// Create directories
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create test files in global dir
	files := map[string]string{
		"README.md.tmpl":    "# {{PROJECT}}\n\nOwner: {{OWNER}}",
		"plain.txt":         "Plain text file",
		".gitignore":        "*.log\nnode_modules/",
		"subdir/config.txt": "config content",
	}

	for name, content := range files {
		path := filepath.Join(globalDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Test processing
	vars := map[string]string{
		"PROJECT": "TestProject",
		"OWNER":   "TestOwner",
	}

	count, err := ProcessGlobalFiles(templatesDir, destDir, vars, nil)
	if err != nil {
		t.Fatalf("ProcessGlobalFiles() error = %v", err)
	}

	if count != 4 {
		t.Errorf("ProcessGlobalFiles() count = %d, want 4", count)
	}

	// Verify files exist
	expectedFiles := []string{
		"README.md",      // .tmpl stripped
		"plain.txt",      // unchanged
		".gitignore",     // unchanged
		"subdir/config.txt",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(destDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not created: %s", f)
		}
	}

	// Verify template was processed
	readme, err := os.ReadFile(filepath.Join(destDir, "README.md"))
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	expectedContent := "# TestProject\n\nOwner: TestOwner"
	if string(readme) != expectedContent {
		t.Errorf("README.md content = %q, want %q", string(readme), expectedContent)
	}

	// Verify plain file was copied unchanged
	plain, err := os.ReadFile(filepath.Join(destDir, "plain.txt"))
	if err != nil {
		t.Fatalf("Failed to read plain.txt: %v", err)
	}
	if string(plain) != "Plain text file" {
		t.Errorf("plain.txt content = %q, want %q", string(plain), "Plain text file")
	}
}

func TestProcessGlobalFilesSkipAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create a test file
	if err := os.WriteFile(filepath.Join(globalDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test with skipFiles = true
	count, err := ProcessGlobalFiles(templatesDir, destDir, nil, true)
	if err != nil {
		t.Fatalf("ProcessGlobalFiles() error = %v", err)
	}

	if count != 0 {
		t.Errorf("ProcessGlobalFiles() with skip=true count = %d, want 0", count)
	}

	// Verify file was NOT created
	if _, err := os.Stat(filepath.Join(destDir, "test.txt")); !os.IsNotExist(err) {
		t.Error("Expected file to NOT be created when skip=true")
	}
}

func TestProcessGlobalFilesSkipList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create test files
	files := []string{"keep.txt", "skip.txt", "also-keep.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(globalDir, f), []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", f, err)
		}
	}

	// Test with skipFiles list
	skipList := []string{"skip.txt"}
	count, err := ProcessGlobalFiles(templatesDir, destDir, nil, skipList)
	if err != nil {
		t.Fatalf("ProcessGlobalFiles() error = %v", err)
	}

	if count != 2 {
		t.Errorf("ProcessGlobalFiles() count = %d, want 2", count)
	}

	// Verify skip.txt was NOT created
	if _, err := os.Stat(filepath.Join(destDir, "skip.txt")); !os.IsNotExist(err) {
		t.Error("Expected skip.txt to NOT be created")
	}

	// Verify other files were created
	if _, err := os.Stat(filepath.Join(destDir, "keep.txt")); os.IsNotExist(err) {
		t.Error("Expected keep.txt to be created")
	}
}

func TestProcessGlobalFilesNoGlobalDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Don't create global dir, just templates dir
	templatesDir := filepath.Join(tmpDir, "templates")
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	count, err := ProcessGlobalFiles(templatesDir, destDir, nil, nil)
	if err != nil {
		t.Fatalf("ProcessGlobalFiles() error = %v", err)
	}

	if count != 0 {
		t.Errorf("ProcessGlobalFiles() count = %d, want 0 for non-existent global dir", count)
	}
}

func TestProcessTemplateFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, "my-template")
	filesDir := filepath.Join(templatePath, TemplateFilesDir)
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create test files
	files := map[string]string{
		"README.md.tmpl":   "# {{PROJECT}}",
		"src/main.go":      "package main",
		"src/main_test.go": "package main",
	}

	for name, content := range files {
		path := filepath.Join(filesDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Create template with exclude for test files
	tmpl := &Template{
		Name: "test-template",
		Files: TemplateFiles{
			Exclude: []string{"**/*_test.go"},
		},
	}

	vars := map[string]string{"PROJECT": "MyProject"}

	count, err := ProcessTemplateFiles(tmpl, templatePath, destDir, vars)
	if err != nil {
		t.Fatalf("ProcessTemplateFiles() error = %v", err)
	}

	if count != 2 { // README.md and src/main.go (test file excluded)
		t.Errorf("ProcessTemplateFiles() count = %d, want 2", count)
	}

	// Verify files
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); os.IsNotExist(err) {
		t.Error("Expected README.md to be created")
	}
	if _, err := os.Stat(filepath.Join(destDir, "src", "main.go")); os.IsNotExist(err) {
		t.Error("Expected src/main.go to be created")
	}
	if _, err := os.Stat(filepath.Join(destDir, "src", "main_test.go")); !os.IsNotExist(err) {
		t.Error("Expected main_test.go to NOT be created (excluded)")
	}

	// Verify template processing
	readme, err := os.ReadFile(filepath.Join(destDir, "README.md"))
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	if string(readme) != "# MyProject" {
		t.Errorf("README.md content = %q, want %q", string(readme), "# MyProject")
	}
}

func TestProcessTemplateFilesNoFilesDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, "my-template")
	destDir := filepath.Join(tmpDir, "dest")

	// Create template dir without files subdir
	if err := os.MkdirAll(templatePath, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	tmpl := &Template{Name: "test"}

	count, err := ProcessTemplateFiles(tmpl, templatePath, destDir, nil)
	if err != nil {
		t.Fatalf("ProcessTemplateFiles() error = %v", err)
	}

	if count != 0 {
		t.Errorf("ProcessTemplateFiles() count = %d, want 0 for non-existent files dir", count)
	}
}

func TestListTemplateFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, "my-template")
	filesDir := filepath.Join(templatePath, TemplateFilesDir)

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Failed to create files dir: %v", err)
	}

	// Create test files
	testFiles := []string{"README.md.tmpl", "config.json", "src/main.go"}
	for _, f := range testFiles {
		path := filepath.Join(filesDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", f, err)
		}
	}

	tmpl := &Template{Name: "test"}

	files, err := ListTemplateFiles(tmpl, templatePath)
	if err != nil {
		t.Fatalf("ListTemplateFiles() error = %v", err)
	}

	if len(files) != 3 {
		t.Errorf("ListTemplateFiles() len = %d, want 3", len(files))
	}

	// Check that .tmpl extension is stripped
	found := false
	for _, f := range files {
		if f == "README.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected README.md (stripped from README.md.tmpl) in list, got %v", files)
	}
}

func TestListGlobalFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global dir: %v", err)
	}

	// Create test files
	testFiles := []string{"AGENTS.md.tmpl", "claude.md.tmpl", ".gitignore"}
	for _, f := range testFiles {
		if err := os.WriteFile(filepath.Join(globalDir, f), []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", f, err)
		}
	}

	files, err := ListGlobalFiles(templatesDir)
	if err != nil {
		t.Fatalf("ListGlobalFiles() error = %v", err)
	}

	if len(files) != 3 {
		t.Errorf("ListGlobalFiles() len = %d, want 3", len(files))
	}

	// Check that .tmpl extensions are stripped
	expectedFiles := map[string]bool{
		"AGENTS.md":  true,
		"claude.md":  true,
		".gitignore": true,
	}

	for _, f := range files {
		if !expectedFiles[f] {
			t.Errorf("Unexpected file in list: %s", f)
		}
	}
}

func TestProcessAllFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	templatePath := filepath.Join(templatesDir, "my-template")
	filesDir := filepath.Join(templatePath, TemplateFilesDir)
	destDir := filepath.Join(tmpDir, "dest")

	// Create directories
	for _, dir := range []string{globalDir, filesDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	// Create global files
	if err := os.WriteFile(filepath.Join(globalDir, "global.txt"), []byte("global"), 0644); err != nil {
		t.Fatalf("Failed to write global file: %v", err)
	}

	// Create template files
	if err := os.WriteFile(filepath.Join(filesDir, "template.txt"), []byte("template"), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	tmpl := &Template{Name: "test"}

	globalCount, templateCount, err := ProcessAllFiles(tmpl, templatesDir, templatePath, destDir, nil)
	if err != nil {
		t.Fatalf("ProcessAllFiles() error = %v", err)
	}

	if globalCount != 1 {
		t.Errorf("ProcessAllFiles() globalCount = %d, want 1", globalCount)
	}
	if templateCount != 1 {
		t.Errorf("ProcessAllFiles() templateCount = %d, want 1", templateCount)
	}

	// Verify both files exist
	if _, err := os.Stat(filepath.Join(destDir, "global.txt")); os.IsNotExist(err) {
		t.Error("Expected global.txt to be created")
	}
	if _, err := os.Stat(filepath.Join(destDir, "template.txt")); os.IsNotExist(err) {
		t.Error("Expected template.txt to be created")
	}
}

func TestProcessFilePreservesPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "src.sh")
	dstFile := filepath.Join(tmpDir, "dst.sh")

	// Create source file with executable permission
	if err := os.WriteFile(srcFile, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Process file
	if err := processFile(srcFile, dstFile, false, nil, nil); err != nil {
		t.Fatalf("processFile() error = %v", err)
	}

	// Check permissions are preserved
	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("Failed to stat dest file: %v", err)
	}

	if dstInfo.Mode()&0100 == 0 {
		t.Error("Expected executable permission to be preserved")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "src.txt")
	dstFile := filepath.Join(tmpDir, "dst.txt")
	content := "Hello, World!"

	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	if err := copyFile(srcFile, dstFile, 0644); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify content
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}

	if string(data) != content {
		t.Errorf("copyFile() content = %q, want %q", string(data), content)
	}
}

// TestProcessAllFilesGlobalOverride tests that template files override global files with the same name.
func TestProcessAllFilesGlobalOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	templatePath := filepath.Join(templatesDir, "my-template")
	filesDir := filepath.Join(templatePath, TemplateFilesDir)
	destDir := filepath.Join(tmpDir, "dest")

	// Create directories
	for _, dir := range []string{globalDir, filesDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	// Create global README that will be overridden
	if err := os.WriteFile(filepath.Join(globalDir, "README.md.tmpl"), []byte("# Global {{PROJECT}}"), 0644); err != nil {
		t.Fatalf("Failed to write global README: %v", err)
	}

	// Create global-only file
	if err := os.WriteFile(filepath.Join(globalDir, "global-only.txt"), []byte("global only content"), 0644); err != nil {
		t.Fatalf("Failed to write global-only file: %v", err)
	}

	// Create template README that should override global
	if err := os.WriteFile(filepath.Join(filesDir, "README.md.tmpl"), []byte("# Template {{PROJECT}}"), 0644); err != nil {
		t.Fatalf("Failed to write template README: %v", err)
	}

	// Create template-only file
	if err := os.WriteFile(filepath.Join(filesDir, "template-only.txt"), []byte("template only content"), 0644); err != nil {
		t.Fatalf("Failed to write template-only file: %v", err)
	}

	tmpl := &Template{Name: "test"}
	vars := map[string]string{"PROJECT": "MyApp"}

	globalCount, templateCount, err := ProcessAllFiles(tmpl, templatesDir, templatePath, destDir, vars)
	if err != nil {
		t.Fatalf("ProcessAllFiles() error = %v", err)
	}

	// Global should process 2 files, template should process 2 files (one overrides)
	if globalCount != 2 {
		t.Errorf("ProcessAllFiles() globalCount = %d, want 2", globalCount)
	}
	if templateCount != 2 {
		t.Errorf("ProcessAllFiles() templateCount = %d, want 2", templateCount)
	}

	// Verify README.md has TEMPLATE content (not global)
	readme, err := os.ReadFile(filepath.Join(destDir, "README.md"))
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	expectedReadme := "# Template MyApp"
	if string(readme) != expectedReadme {
		t.Errorf("README.md should have template content, got %q, want %q", string(readme), expectedReadme)
	}

	// Verify global-only.txt exists (from global)
	globalOnly, err := os.ReadFile(filepath.Join(destDir, "global-only.txt"))
	if err != nil {
		t.Fatalf("Failed to read global-only.txt: %v", err)
	}
	if string(globalOnly) != "global only content" {
		t.Errorf("global-only.txt content = %q, want %q", string(globalOnly), "global only content")
	}

	// Verify template-only.txt exists (from template)
	templateOnly, err := os.ReadFile(filepath.Join(destDir, "template-only.txt"))
	if err != nil {
		t.Fatalf("Failed to read template-only.txt: %v", err)
	}
	if string(templateOnly) != "template only content" {
		t.Errorf("template-only.txt content = %q, want %q", string(templateOnly), "template only content")
	}
}

// TestProcessGlobalFilesSkipInterfaceSlice tests skipFiles with []interface{} (as from JSON).
func TestProcessGlobalFilesSkipInterfaceSlice(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create test files
	if err := os.WriteFile(filepath.Join(globalDir, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatalf("Failed to write keep.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "skip.txt"), []byte("skip"), 0644); err != nil {
		t.Fatalf("Failed to write skip.txt: %v", err)
	}

	// Test with []interface{} (as it might come from JSON unmarshaling)
	skipList := []interface{}{"skip.txt"}
	count, err := ProcessGlobalFiles(templatesDir, destDir, nil, skipList)
	if err != nil {
		t.Fatalf("ProcessGlobalFiles() error = %v", err)
	}

	if count != 1 {
		t.Errorf("ProcessGlobalFiles() count = %d, want 1", count)
	}

	if _, err := os.Stat(filepath.Join(destDir, "keep.txt")); os.IsNotExist(err) {
		t.Error("Expected keep.txt to be created")
	}
	if _, err := os.Stat(filepath.Join(destDir, "skip.txt")); !os.IsNotExist(err) {
		t.Error("Expected skip.txt to NOT be created")
	}
}

// TestProcessTemplateFilesWithCustomExtension tests custom template extensions.
func TestProcessTemplateFilesWithCustomExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, "my-template")
	filesDir := filepath.Join(templatePath, TemplateFilesDir)
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create file with custom template extension
	if err := os.WriteFile(filepath.Join(filesDir, "config.yaml.template"), []byte("name: {{PROJECT}}"), 0644); err != nil {
		t.Fatalf("Failed to write config.yaml.template: %v", err)
	}

	tmpl := &Template{
		Name: "test",
		Files: TemplateFiles{
			TemplateExtensions: []string{".template"},
		},
	}

	vars := map[string]string{"PROJECT": "MyApp"}

	count, err := ProcessTemplateFiles(tmpl, templatePath, destDir, vars)
	if err != nil {
		t.Fatalf("ProcessTemplateFiles() error = %v", err)
	}

	if count != 1 {
		t.Errorf("ProcessTemplateFiles() count = %d, want 1", count)
	}

	// Verify .template extension was stripped and content was processed
	content, err := os.ReadFile(filepath.Join(destDir, "config.yaml"))
	if err != nil {
		t.Fatalf("Failed to read config.yaml: %v", err)
	}
	expected := "name: MyApp"
	if string(content) != expected {
		t.Errorf("config.yaml content = %q, want %q", string(content), expected)
	}

	// Verify .template file doesn't exist
	if _, err := os.Stat(filepath.Join(destDir, "config.yaml.template")); !os.IsNotExist(err) {
		t.Error("Expected config.yaml.template to NOT exist")
	}
}

// TestOutputFileName tests the OutputFileName function.
func TestOutputFileName(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       string
	}{
		{
			name:       "default tmpl extension",
			path:       "README.md.tmpl",
			extensions: nil,
			want:       "README.md",
		},
		{
			name:       "explicit tmpl extension",
			path:       "README.md.tmpl",
			extensions: []string{".tmpl"},
			want:       "README.md",
		},
		{
			name:       "custom extension",
			path:       "file.template",
			extensions: []string{".template"},
			want:       "file",
		},
		{
			name:       "no matching extension",
			path:       "plain.txt",
			extensions: nil,
			want:       "plain.txt",
		},
		{
			name:       "nested path",
			path:       "src/pkg/main.go.tmpl",
			extensions: nil,
			want:       "src/pkg/main.go",
		},
		{
			name:       "multiple extensions matches first",
			path:       "file.tpl",
			extensions: []string{".tmpl", ".tpl"},
			want:       "file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OutputFileName(tt.path, tt.extensions)
			if got != tt.want {
				t.Errorf("OutputFileName(%q, %v) = %q, want %q", tt.path, tt.extensions, got, tt.want)
			}
		})
	}
}

// TestFileProcessingError tests the FileProcessingError type.
func TestFileProcessingError(t *testing.T) {
	baseErr := os.ErrNotExist
	err := &FileProcessingError{
		SrcPath:  "/src/file.txt",
		DestPath: "/dest/file.txt",
		Err:      baseErr,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}

	// Test that source path is in the message
	if !contains(msg, "/src/file.txt") {
		t.Errorf("Error message should contain source path: %q", msg)
	}

	// Test Unwrap
	if err.Unwrap() != baseErr {
		t.Error("Unwrap() should return the base error")
	}
}

// TestPathTraversalError tests the PathTraversalError type.
func TestPathTraversalError(t *testing.T) {
	err := &PathTraversalError{
		Path:          "/workspace/../../../etc/passwd",
		WorkspacePath: "/workspace",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}

	if !contains(msg, "path traversal") {
		t.Errorf("Error message should mention path traversal: %q", msg)
	}
	if !contains(msg, "/workspace") {
		t.Errorf("Error message should contain workspace path: %q", msg)
	}
}

// TestProcessFileCreatesDirectories tests that processFile creates nested directories.
func TestProcessFileCreatesDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Destination with nested non-existent directories
	dstFile := filepath.Join(tmpDir, "a", "b", "c", "dst.txt")

	if err := processFile(srcFile, dstFile, false, nil, nil); err != nil {
		t.Fatalf("processFile() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("Content = %q, want %q", string(content), "content")
	}
}

// TestProcessFileAsTemplate tests processFile with template processing.
func TestProcessFileAsTemplate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "src.txt.tmpl")
	dstFile := filepath.Join(tmpDir, "dst.txt")

	// Create template file
	if err := os.WriteFile(srcFile, []byte("Hello {{NAME}}!"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	vars := map[string]string{"NAME": "World"}

	if err := processFile(srcFile, dstFile, true, vars, []string{".tmpl"}); err != nil {
		t.Fatalf("processFile() error = %v", err)
	}

	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}

	expected := "Hello World!"
	if string(content) != expected {
		t.Errorf("Content = %q, want %q", string(content), expected)
	}
}

// TestListTemplateFilesWithExclude tests ListTemplateFiles respects exclude patterns.
func TestListTemplateFilesWithExclude(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, "my-template")
	filesDir := filepath.Join(templatePath, TemplateFilesDir)

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Failed to create files dir: %v", err)
	}

	// Create various files
	testFiles := []string{"keep.txt", "exclude.bak", "src/main.go", "src/main_test.go"}
	for _, f := range testFiles {
		path := filepath.Join(filesDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", f, err)
		}
	}

	tmpl := &Template{
		Name: "test",
		Files: TemplateFiles{
			Exclude: []string{"*.bak", "**/*_test.go"},
		},
	}

	files, err := ListTemplateFiles(tmpl, templatePath)
	if err != nil {
		t.Fatalf("ListTemplateFiles() error = %v", err)
	}

	// Should have 2 files (excluding .bak and _test.go)
	if len(files) != 2 {
		t.Errorf("ListTemplateFiles() len = %d, want 2, got files: %v", len(files), files)
	}

	// Verify excluded files are not in the list
	for _, f := range files {
		if f == "exclude.bak" {
			t.Error("exclude.bak should not be in the list")
		}
		if filepath.Base(f) == "main_test.go" {
			t.Error("main_test.go should not be in the list")
		}
	}
}

// TestListGlobalFilesNoDir tests ListGlobalFiles when _global doesn't exist.
func TestListGlobalFilesNoDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Don't create _global directory
	files, err := ListGlobalFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListGlobalFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ListGlobalFiles() len = %d, want 0 when _global doesn't exist", len(files))
	}
}

// TestListTemplateFilesNoDir tests ListTemplateFiles when files/ doesn't exist.
func TestListTemplateFilesNoDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, "my-template")
	if err := os.MkdirAll(templatePath, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Don't create files/ directory
	tmpl := &Template{Name: "test"}

	files, err := ListTemplateFiles(tmpl, templatePath)
	if err != nil {
		t.Fatalf("ListTemplateFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ListTemplateFiles() len = %d, want 0 when files/ doesn't exist", len(files))
	}
}

// contains is a helper for string containment checks.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstr(s, substr)))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
