package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := tmpDir
	templateName := "test-template"
	templatePath := filepath.Join(templatesDir, templateName)

	if err := os.MkdirAll(templatePath, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	tmpl := &Template{
		Schema:      1,
		Name:        templateName,
		Description: "A test template",
		Version:     "1.0.0",
	}

	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatePath, TemplateManifestFile), data, 0644); err != nil {
		t.Fatalf("Failed to write template.json: %v", err)
	}

	loaded, err := LoadTemplate(templatesDir, templateName)
	if err != nil {
		t.Fatalf("LoadTemplate() error = %v", err)
	}

	if loaded.Name != templateName {
		t.Errorf("Name = %q, want %q", loaded.Name, templateName)
	}
	if loaded.Description != "A test template" {
		t.Errorf("Description = %q, want %q", loaded.Description, "A test template")
	}
}

func TestLoadTemplateNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = LoadTemplate(tmpDir, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
	if _, ok := err.(*TemplateNotFoundError); !ok {
		t.Errorf("Expected TemplateNotFoundError, got %T", err)
	}
}

func TestLoadTemplateMulti(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")

	// Create template in primary
	primaryTemplate := filepath.Join(primaryDir, "primary-only")
	if err := os.MkdirAll(primaryTemplate, 0755); err != nil {
		t.Fatalf("Failed to create primary template dir: %v", err)
	}
	primaryTmpl := &Template{Schema: 1, Name: "primary-only", Description: "Primary only template"}
	data, _ := json.MarshalIndent(primaryTmpl, "", "  ")
	if err := os.WriteFile(filepath.Join(primaryTemplate, TemplateManifestFile), data, 0644); err != nil {
		t.Fatalf("Failed to write primary template.json: %v", err)
	}

	// Create template in fallback only
	fallbackTemplate := filepath.Join(fallbackDir, "fallback-only")
	if err := os.MkdirAll(fallbackTemplate, 0755); err != nil {
		t.Fatalf("Failed to create fallback template dir: %v", err)
	}
	fallbackTmpl := &Template{Schema: 1, Name: "fallback-only", Description: "Fallback only template"}
	data, _ = json.MarshalIndent(fallbackTmpl, "", "  ")
	if err := os.WriteFile(filepath.Join(fallbackTemplate, TemplateManifestFile), data, 0644); err != nil {
		t.Fatalf("Failed to write fallback template.json: %v", err)
	}

	templatesDirs := []string{primaryDir, fallbackDir}

	// Test loading primary-only template
	tmpl, foundDir, err := LoadTemplateMulti(templatesDirs, "primary-only")
	if err != nil {
		t.Fatalf("LoadTemplateMulti(primary-only) error = %v", err)
	}
	if tmpl.Name != "primary-only" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "primary-only")
	}
	if foundDir != primaryDir {
		t.Errorf("foundDir = %q, want %q", foundDir, primaryDir)
	}

	// Test loading fallback-only template
	tmpl, foundDir, err = LoadTemplateMulti(templatesDirs, "fallback-only")
	if err != nil {
		t.Fatalf("LoadTemplateMulti(fallback-only) error = %v", err)
	}
	if tmpl.Name != "fallback-only" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "fallback-only")
	}
	if foundDir != fallbackDir {
		t.Errorf("foundDir = %q, want %q", foundDir, fallbackDir)
	}

	// Test nonexistent template
	_, _, err = LoadTemplateMulti(templatesDirs, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestLoadTemplateMultiPrimaryTakesPrecedence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")

	// Create same template in both directories with different descriptions
	for _, dir := range []string{primaryDir, fallbackDir} {
		templatePath := filepath.Join(dir, "shared-template")
		if err := os.MkdirAll(templatePath, 0755); err != nil {
			t.Fatalf("Failed to create template dir: %v", err)
		}

		desc := "fallback"
		if dir == primaryDir {
			desc = "primary"
		}

		tmpl := &Template{Schema: 1, Name: "shared-template", Description: desc}
		data, _ := json.MarshalIndent(tmpl, "", "  ")
		if err := os.WriteFile(filepath.Join(templatePath, TemplateManifestFile), data, 0644); err != nil {
			t.Fatalf("Failed to write template.json: %v", err)
		}
	}

	templatesDirs := []string{primaryDir, fallbackDir}

	tmpl, foundDir, err := LoadTemplateMulti(templatesDirs, "shared-template")
	if err != nil {
		t.Fatalf("LoadTemplateMulti() error = %v", err)
	}

	// Should get primary version
	if tmpl.Description != "primary" {
		t.Errorf("Description = %q, want %q (primary should take precedence)", tmpl.Description, "primary")
	}
	if foundDir != primaryDir {
		t.Errorf("foundDir = %q, want %q", foundDir, primaryDir)
	}
}

func TestListTemplatesMulti(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")

	// Create templates
	templates := []struct {
		dir  string
		name string
		desc string
	}{
		{primaryDir, "primary-only", "Primary only"},
		{primaryDir, "shared", "Shared from primary"},
		{fallbackDir, "shared", "Shared from fallback (should be ignored)"},
		{fallbackDir, "fallback-only", "Fallback only"},
	}

	for _, tt := range templates {
		templatePath := filepath.Join(tt.dir, tt.name)
		if err := os.MkdirAll(templatePath, 0755); err != nil {
			t.Fatalf("Failed to create template dir: %v", err)
		}
		tmpl := &Template{Schema: 1, Name: tt.name, Description: tt.desc}
		data, _ := json.MarshalIndent(tmpl, "", "  ")
		if err := os.WriteFile(filepath.Join(templatePath, TemplateManifestFile), data, 0644); err != nil {
			t.Fatalf("Failed to write template.json: %v", err)
		}
	}

	templatesDirs := []string{primaryDir, fallbackDir}

	list, err := ListTemplatesMulti(templatesDirs)
	if err != nil {
		t.Fatalf("ListTemplatesMulti() error = %v", err)
	}

	// Should have 3 unique templates
	if len(list) != 3 {
		t.Errorf("ListTemplatesMulti() len = %d, want 3", len(list))
	}

	// Verify shared template has primary description
	for _, tmpl := range list {
		if tmpl.Name == "shared" {
			if tmpl.Description != "Shared from primary" {
				t.Errorf("shared.Description = %q, want %q (primary should take precedence)",
					tmpl.Description, "Shared from primary")
			}
		}
	}
}

func TestFindTemplateDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")

	// Create template only in fallback
	templatePath := filepath.Join(fallbackDir, "fallback-template")
	if err := os.MkdirAll(templatePath, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}
	tmpl := &Template{Schema: 1, Name: "fallback-template", Description: "Fallback"}
	data, _ := json.MarshalIndent(tmpl, "", "  ")
	if err := os.WriteFile(filepath.Join(templatePath, TemplateManifestFile), data, 0644); err != nil {
		t.Fatalf("Failed to write template.json: %v", err)
	}

	// Create primary dir without the template
	if err := os.MkdirAll(primaryDir, 0755); err != nil {
		t.Fatalf("Failed to create primary dir: %v", err)
	}

	templatesDirs := []string{primaryDir, fallbackDir}

	foundDir, err := FindTemplateDir(templatesDirs, "fallback-template")
	if err != nil {
		t.Fatalf("FindTemplateDir() error = %v", err)
	}

	if foundDir != fallbackDir {
		t.Errorf("foundDir = %q, want %q", foundDir, fallbackDir)
	}

	// Test not found
	_, err = FindTemplateDir(templatesDirs, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestTemplateExistsMulti(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")

	// Create template only in fallback
	templatePath := filepath.Join(fallbackDir, "exists")
	if err := os.MkdirAll(templatePath, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}
	tmpl := &Template{Schema: 1, Name: "exists", Description: "Exists"}
	data, _ := json.MarshalIndent(tmpl, "", "  ")
	if err := os.WriteFile(filepath.Join(templatePath, TemplateManifestFile), data, 0644); err != nil {
		t.Fatalf("Failed to write template.json: %v", err)
	}

	// Create primary dir without templates
	if err := os.MkdirAll(primaryDir, 0755); err != nil {
		t.Fatalf("Failed to create primary dir: %v", err)
	}

	templatesDirs := []string{primaryDir, fallbackDir}

	if !TemplateExistsMulti(templatesDirs, "exists") {
		t.Error("TemplateExistsMulti(exists) = false, want true")
	}

	if TemplateExistsMulti(templatesDirs, "nonexistent") {
		t.Error("TemplateExistsMulti(nonexistent) = true, want false")
	}
}

func TestGetGlobalFilesPaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")
	noGlobalDir := filepath.Join(tmpDir, "no-global")

	// Create _global in primary and fallback
	for _, dir := range []string{primaryDir, fallbackDir} {
		globalPath := filepath.Join(dir, GlobalTemplateDir)
		if err := os.MkdirAll(globalPath, 0755); err != nil {
			t.Fatalf("Failed to create _global dir: %v", err)
		}
	}

	// Create noGlobalDir without _global
	if err := os.MkdirAll(noGlobalDir, 0755); err != nil {
		t.Fatalf("Failed to create no-global dir: %v", err)
	}

	templatesDirs := []string{primaryDir, noGlobalDir, fallbackDir}

	paths := GetGlobalFilesPaths(templatesDirs)

	// Should only have 2 paths (primary and fallback, not noGlobalDir)
	if len(paths) != 2 {
		t.Errorf("GetGlobalFilesPaths() len = %d, want 2, got: %v", len(paths), paths)
	}

	// Should be in order: primary first, then fallback
	expectedPrimary := filepath.Join(primaryDir, GlobalTemplateDir)
	expectedFallback := filepath.Join(fallbackDir, GlobalTemplateDir)

	if paths[0] != expectedPrimary {
		t.Errorf("paths[0] = %q, want %q", paths[0], expectedPrimary)
	}
	if paths[1] != expectedFallback {
		t.Errorf("paths[1] = %q, want %q", paths[1], expectedFallback)
	}
}

func TestHasGlobalFilesMulti(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	primaryDir := filepath.Join(tmpDir, "primary")
	fallbackDir := filepath.Join(tmpDir, "fallback")

	// Create fallback _global only
	if err := os.MkdirAll(filepath.Join(fallbackDir, GlobalTemplateDir), 0755); err != nil {
		t.Fatalf("Failed to create _global dir: %v", err)
	}
	if err := os.MkdirAll(primaryDir, 0755); err != nil {
		t.Fatalf("Failed to create primary dir: %v", err)
	}

	templatesDirs := []string{primaryDir, fallbackDir}

	if !HasGlobalFilesMulti(templatesDirs) {
		t.Error("HasGlobalFilesMulti() = false, want true (fallback has _global)")
	}

	// Test with no _global anywhere
	emptyDirs := []string{primaryDir}
	if HasGlobalFilesMulti(emptyDirs) {
		t.Error("HasGlobalFilesMulti() = true, want false (no _global)")
	}
}
