package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchWithDetails(t *testing.T) {
	tests := []struct {
		name         string
		include      []string
		exclude      []string
		path         string
		wantIncluded bool
		wantRule     string
		wantPattern  string
	}{
		{
			name:         "default include when no patterns",
			include:      nil,
			exclude:      nil,
			path:         "foo.txt",
			wantIncluded: true,
			wantRule:     "default",
			wantPattern:  "",
		},
		{
			name:         "explicit include match",
			include:      []string{"*.txt"},
			exclude:      nil,
			path:         "foo.txt",
			wantIncluded: true,
			wantRule:     "include",
			wantPattern:  "*.txt",
		},
		{
			name:         "explicit exclude match",
			include:      nil,
			exclude:      []string{"*.log"},
			path:         "app.log",
			wantIncluded: false,
			wantRule:     "exclude",
			wantPattern:  "*.log",
		},
		{
			name:         "exclude takes precedence over include",
			include:      []string{"*"},
			exclude:      []string{"*.log"},
			path:         "app.log",
			wantIncluded: false,
			wantRule:     "exclude",
			wantPattern:  "*.log",
		},
		{
			name:         "not matched by include patterns",
			include:      []string{"*.txt"},
			exclude:      nil,
			path:         "foo.md",
			wantIncluded: false,
			wantRule:     "not_matched",
			wantPattern:  "",
		},
		{
			name:         "directory glob include",
			include:      []string{"src/**/*.go"},
			exclude:      nil,
			path:         "src/pkg/main.go",
			wantIncluded: true,
			wantRule:     "include",
			wantPattern:  "src/**/*.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPatternMatcher(tt.include, tt.exclude)
			result := pm.MatchWithDetails(tt.path)

			if result.Included != tt.wantIncluded {
				t.Errorf("Included = %v, want %v", result.Included, tt.wantIncluded)
			}
			if result.Rule != tt.wantRule {
				t.Errorf("Rule = %q, want %q", result.Rule, tt.wantRule)
			}
			if result.MatchedPattern != tt.wantPattern {
				t.Errorf("MatchedPattern = %q, want %q", result.MatchedPattern, tt.wantPattern)
			}
		})
	}
}

func TestScanForPlaceholders(t *testing.T) {
	// Create a temp directory structure
	tempDir := t.TempDir()

	// Create template structure
	templateName := "test-template"
	templateDir := filepath.Join(tempDir, templateName)
	filesDir := filepath.Join(templateDir, "files")

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Create template.json
	manifest := `{
		"name": "test-template",
		"description": "Test template for diagnostics",
		"variables": [
			{"name": "PROJECT_NAME", "type": "string"},
			{"name": "AUTHOR", "type": "string", "default": "Unknown"}
		]
	}`
	if err := os.WriteFile(filepath.Join(templateDir, "template.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	// Create a template file with placeholders
	tmplContent := `# {{PROJECT_NAME}}
Author: {{AUTHOR}}
Year: {{YEAR}}
Unknown: {{UNKNOWN_VAR}}
`
	if err := os.WriteFile(filepath.Join(filesDir, "README.md.tmpl"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Available variables (simulating builtins + defaults)
	availableVars := map[string]string{
		"PROJECT_NAME": "my-project",
		"AUTHOR":       "Unknown",
		"YEAR":         "2025",
		"OWNER":        "testowner",
	}

	// Run scan
	report, err := ScanForPlaceholders(tempDir, templateName, availableVars)
	if err != nil {
		t.Fatalf("ScanForPlaceholders error: %v", err)
	}

	// Should have found 4 placeholders total
	if len(report.Placeholders) != 4 {
		t.Errorf("Expected 4 placeholders, got %d", len(report.Placeholders))
	}

	// Check unresolved - only UNKNOWN_VAR should be unresolved
	unresolved := report.GetUnresolvedPlaceholders()
	if len(unresolved) != 1 {
		t.Errorf("Expected 1 unresolved placeholder, got %d", len(unresolved))
	}

	if len(unresolved) > 0 && unresolved[0].VarName != "UNKNOWN_VAR" {
		t.Errorf("Expected unresolved var 'UNKNOWN_VAR', got %q", unresolved[0].VarName)
	}

	// Check that line numbers are correct
	for _, p := range report.Placeholders {
		if p.VarName == "PROJECT_NAME" && p.Line != 1 {
			t.Errorf("PROJECT_NAME should be on line 1, got %d", p.Line)
		}
		if p.VarName == "UNKNOWN_VAR" && p.Line != 4 {
			t.Errorf("UNKNOWN_VAR should be on line 4, got %d", p.Line)
		}
	}
}

func TestDiagnoseTemplateFiles(t *testing.T) {
	// Create a temp directory structure
	tempDir := t.TempDir()

	// Create template structure
	templateName := "test-template"
	templateDir := filepath.Join(tempDir, templateName)
	filesDir := filepath.Join(templateDir, "files")
	subDir := filepath.Join(filesDir, "src")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Create files
	files := []struct {
		path    string
		content string
	}{
		{"README.md.tmpl", "# Test"},
		{"config.json", "{}"},
		{"src/main.go.tmpl", "package main"},
		{"src/test.log", "log content"},
	}

	for _, f := range files {
		fullPath := filepath.Join(filesDir, f.path)
		if err := os.WriteFile(fullPath, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", f.path, err)
		}
	}

	// Create template with include/exclude patterns
	tmpl := &Template{
		Name:        templateName,
		Description: "Test template",
		Files: TemplateFiles{
			Include: []string{"**/*.go.tmpl", "**/*.md.tmpl", "*.json"},
			Exclude: []string{"**/*.log"},
		},
	}

	// Run diagnostics
	diagnostics, err := DiagnoseTemplateFiles(tmpl, tempDir)
	if err != nil {
		t.Fatalf("DiagnoseTemplateFiles error: %v", err)
	}

	// Should have 4 files diagnosed
	if len(diagnostics) != 4 {
		t.Errorf("Expected 4 diagnostics, got %d", len(diagnostics))
	}

	// Check specific results
	for _, d := range diagnostics {
		switch filepath.Base(d.FilePath) {
		case "README.md.tmpl":
			if !d.MatchResult.Included {
				t.Errorf("README.md.tmpl should be included")
			}
			if d.MatchResult.MatchedPattern != "**/*.md.tmpl" {
				t.Errorf("README.md.tmpl should match **/*.md.tmpl, got %q", d.MatchResult.MatchedPattern)
			}
		case "test.log":
			if d.MatchResult.Included {
				t.Errorf("test.log should be excluded")
			}
			if d.MatchResult.Rule != "exclude" {
				t.Errorf("test.log should have rule 'exclude', got %q", d.MatchResult.Rule)
			}
		case "config.json":
			if !d.MatchResult.Included {
				t.Errorf("config.json should be included")
			}
		case "main.go.tmpl":
			if !d.MatchResult.Included {
				t.Errorf("main.go.tmpl should be included")
			}
		}
	}
}

func TestGetFileMatchDetails(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		include []string
		exclude []string
		want    bool
	}{
		{
			name:    "basic include",
			path:    "README.md",
			include: []string{"*.md"},
			exclude: nil,
			want:    true,
		},
		{
			name:    "basic exclude",
			path:    "test.log",
			include: nil,
			exclude: []string{"*.log"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFileMatchDetails(tt.path, tt.include, tt.exclude)
			if result.Included != tt.want {
				t.Errorf("GetFileMatchDetails(%q) = %v, want %v", tt.path, result.Included, tt.want)
			}
		})
	}
}
