package partial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tormodhaugland/co/internal/template"
)

// createTestPartial creates a partial directory with a valid partial.json
func createTestPartial(t *testing.T, dir, name string, p *Partial) string {
	t.Helper()
	partialDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(partialDir, 0755))

	// If no partial provided, create a minimal valid one
	if p == nil {
		p = &Partial{
			Schema:      1,
			Name:        name,
			Description: "Test partial: " + name,
		}
	}

	data, err := json.MarshalIndent(p, "", "  ")
	require.NoError(t, err)

	manifestPath := filepath.Join(partialDir, PartialManifestFile)
	require.NoError(t, os.WriteFile(manifestPath, data, 0644))

	return partialDir
}

// createTestPartialWithFiles creates a partial with files directory
func createTestPartialWithFiles(t *testing.T, dir, name string, p *Partial, files map[string]string) string {
	t.Helper()
	partialDir := createTestPartial(t, dir, name, p)

	filesDir := filepath.Join(partialDir, PartialFilesDir)
	require.NoError(t, os.MkdirAll(filesDir, 0755))

	for path, content := range files {
		filePath := filepath.Join(filesDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0755))
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
	}

	return partialDir
}

// TestListPartials_EmptyDirectory tests listing partials from an empty directory
func TestListPartials_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	infos, err := ListPartials([]string{dir})
	require.NoError(t, err)
	assert.Empty(t, infos)
}

// TestListPartials_SinglePartial tests listing a single partial
func TestListPartials_SinglePartial(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "test-partial", nil)

	infos, err := ListPartials([]string{dir})
	require.NoError(t, err)
	require.Len(t, infos, 1)

	assert.Equal(t, "test-partial", infos[0].Name)
	assert.Equal(t, "Test partial: test-partial", infos[0].Description)
}

// TestListPartials_MultiplePartials tests listing multiple partials
func TestListPartials_MultiplePartials(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "partial-a", nil)
	createTestPartial(t, dir, "partial-b", nil)
	createTestPartial(t, dir, "partial-c", nil)

	infos, err := ListPartials([]string{dir})
	require.NoError(t, err)
	assert.Len(t, infos, 3)

	// Collect names
	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Name] = true
	}
	assert.True(t, names["partial-a"])
	assert.True(t, names["partial-b"])
	assert.True(t, names["partial-c"])
}

// TestListPartials_InvalidPartialSkipped tests that invalid partials are skipped
func TestListPartials_InvalidPartialSkipped(t *testing.T) {
	dir := t.TempDir()

	// Create valid partial
	createTestPartial(t, dir, "valid-partial", nil)

	// Create invalid partial (bad JSON)
	invalidDir := filepath.Join(dir, "invalid-partial")
	require.NoError(t, os.MkdirAll(invalidDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, PartialManifestFile), []byte("not json"), 0644))

	infos, err := ListPartials([]string{dir})
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "valid-partial", infos[0].Name)
}

// TestListPartials_HiddenDirectoriesSkipped tests that hidden directories are skipped
func TestListPartials_HiddenDirectoriesSkipped(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "visible-partial", nil)
	createTestPartial(t, dir, ".hidden-partial", nil)

	infos, err := ListPartials([]string{dir})
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "visible-partial", infos[0].Name)
}

// TestListPartials_MultipleDirectoriesPriority tests that earlier directories take precedence
func TestListPartials_MultipleDirectoriesPriority(t *testing.T) {
	primaryDir := t.TempDir()
	fallbackDir := t.TempDir()

	// Create same-named partial in both with different descriptions
	createTestPartial(t, primaryDir, "shared-partial", &Partial{
		Schema:      1,
		Name:        "shared-partial",
		Description: "Primary version",
	})
	createTestPartial(t, fallbackDir, "shared-partial", &Partial{
		Schema:      1,
		Name:        "shared-partial",
		Description: "Fallback version",
	})
	// Create unique partial in fallback only
	createTestPartial(t, fallbackDir, "fallback-only", nil)

	infos, err := ListPartials([]string{primaryDir, fallbackDir})
	require.NoError(t, err)
	require.Len(t, infos, 2)

	// Find the shared partial
	var sharedInfo *PartialInfo
	for i := range infos {
		if infos[i].Name == "shared-partial" {
			sharedInfo = &infos[i]
			break
		}
	}
	require.NotNil(t, sharedInfo)
	assert.Equal(t, "Primary version", sharedInfo.Description)
}

// TestListPartialsFiltered_ByTag tests filtering partials by tag
func TestListPartialsFiltered_ByTag(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "agent-partial", &Partial{
		Schema:      1,
		Name:        "agent-partial",
		Description: "Agent tools",
		Tags:        []string{"agent", "ai"},
	})
	createTestPartial(t, dir, "python-partial", &Partial{
		Schema:      1,
		Name:        "python-partial",
		Description: "Python tools",
		Tags:        []string{"python", "tooling"},
	})

	// Filter by "agent" tag
	infos, err := ListPartialsFiltered([]string{dir}, "agent")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "agent-partial", infos[0].Name)

	// Filter by non-existent tag
	infos, err = ListPartialsFiltered([]string{dir}, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, infos)

	// Empty filter returns all
	infos, err = ListPartialsFiltered([]string{dir}, "")
	require.NoError(t, err)
	assert.Len(t, infos, 2)
}

// TestFindPartial_Exists tests finding an existing partial
func TestFindPartial_Exists(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "my-partial", nil)

	path, err := FindPartial("my-partial", []string{dir})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "my-partial"), path)
}

// TestFindPartial_NotFound tests finding a non-existent partial
func TestFindPartial_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := FindPartial("nonexistent", []string{dir})
	require.Error(t, err)

	var notFoundErr *PartialNotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent", notFoundErr.Name)
}

// TestFindPartial_EmptyName tests finding with empty name
func TestFindPartial_EmptyName(t *testing.T) {
	dir := t.TempDir()

	_, err := FindPartial("", []string{dir})
	require.Error(t, err)

	var validationErr *ValidationError
	assert.ErrorAs(t, err, &validationErr)
}

// TestFindPartial_PrimaryTakesPrecedence tests that primary directory wins
func TestFindPartial_PrimaryTakesPrecedence(t *testing.T) {
	primaryDir := t.TempDir()
	fallbackDir := t.TempDir()

	createTestPartial(t, primaryDir, "shared", nil)
	createTestPartial(t, fallbackDir, "shared", nil)

	path, err := FindPartial("shared", []string{primaryDir, fallbackDir})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(primaryDir, "shared"), path)
}

// TestLoadPartial_Valid tests loading a valid partial
func TestLoadPartial_Valid(t *testing.T) {
	dir := t.TempDir()
	partialDir := createTestPartial(t, dir, "test-partial", &Partial{
		Schema:      1,
		Name:        "test-partial",
		Description: "A test partial",
		Version:     "1.0.0",
		Variables: []PartialVar{
			{
				Name:        "project_name",
				Description: "Project name",
				Type:        template.VarTypeString,
				Default:     "{{DIRNAME}}",
			},
			{
				Name:     "stack",
				Type:     template.VarTypeChoice,
				Choices:  []string{"go", "python", "node"},
				Required: true,
			},
		},
		Tags: []string{"test", "example"},
		Conflicts: ConflictConfig{
			Strategy: "prompt",
			Preserve: []string{".beads/**"},
		},
	})

	p, err := LoadPartial(partialDir)
	require.NoError(t, err)

	assert.Equal(t, 1, p.Schema)
	assert.Equal(t, "test-partial", p.Name)
	assert.Equal(t, "A test partial", p.Description)
	assert.Equal(t, "1.0.0", p.Version)
	assert.Len(t, p.Variables, 2)
	assert.Equal(t, "project_name", p.Variables[0].Name)
	assert.Equal(t, "stack", p.Variables[1].Name)
	assert.Equal(t, []string{"test", "example"}, p.Tags)
	assert.Equal(t, "prompt", p.Conflicts.Strategy)
}

// TestLoadPartial_MissingManifest tests loading a partial without manifest
func TestLoadPartial_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "no-manifest")
	require.NoError(t, os.MkdirAll(partialDir, 0755))

	_, err := LoadPartial(partialDir)
	require.Error(t, err)

	var manifestErr *InvalidManifestError
	assert.ErrorAs(t, err, &manifestErr)
}

// TestLoadPartial_InvalidJSON tests loading a partial with invalid JSON
func TestLoadPartial_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "bad-json")
	require.NoError(t, os.MkdirAll(partialDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(partialDir, PartialManifestFile), []byte("not json {"), 0644))

	_, err := LoadPartial(partialDir)
	require.Error(t, err)

	var manifestErr *InvalidManifestError
	assert.ErrorAs(t, err, &manifestErr)
}

// TestLoadPartial_NameMismatch tests loading when name doesn't match directory
func TestLoadPartial_NameMismatch(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "actual-name")
	require.NoError(t, os.MkdirAll(partialDir, 0755))

	data, _ := json.Marshal(&Partial{
		Schema:      1,
		Name:        "different-name",
		Description: "Test",
	})
	require.NoError(t, os.WriteFile(filepath.Join(partialDir, PartialManifestFile), data, 0644))

	_, err := LoadPartial(partialDir)
	require.Error(t, err)

	var manifestErr *InvalidManifestError
	assert.ErrorAs(t, err, &manifestErr)
	assert.Contains(t, manifestErr.Error(), "does not match directory name")
}

// TestLoadPartial_DefaultsName tests that empty name defaults to directory name
func TestLoadPartial_DefaultsName(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "inferred-name")
	require.NoError(t, os.MkdirAll(partialDir, 0755))

	data, _ := json.Marshal(&Partial{
		Schema:      1,
		Name:        "", // Empty, should be inferred
		Description: "Test partial",
	})
	require.NoError(t, os.WriteFile(filepath.Join(partialDir, PartialManifestFile), data, 0644))

	p, err := LoadPartial(partialDir)
	require.NoError(t, err)
	assert.Equal(t, "inferred-name", p.Name)
}

// TestValidatePartial_Valid tests validation of a valid partial
func TestValidatePartial_Valid(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "valid-partial",
		Description: "A valid partial",
	}

	err := ValidatePartial(p, "")
	assert.NoError(t, err)
}

// TestValidatePartial_InvalidSchema tests validation with invalid schema
func TestValidatePartial_InvalidSchema(t *testing.T) {
	p := &Partial{
		Schema:      99,
		Name:        "test",
		Description: "Test",
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema")
}

// TestValidatePartial_InvalidName tests validation with invalid name patterns
func TestValidatePartial_InvalidName(t *testing.T) {
	testCases := []struct {
		name        string
		partialName string
	}{
		{"uppercase", "InvalidName"},
		{"starts with hyphen", "-invalid"},
		{"special chars", "invalid@name"},
		{"spaces", "invalid name"},
		// Note: "starts with number" is actually valid per spec: ^[a-z0-9][a-z0-9-]*$
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Partial{
				Schema:      1,
				Name:        tc.partialName,
				Description: "Test",
			}

			err := ValidatePartial(p, "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "name")
		})
	}
}

// TestValidatePartial_MissingDescription tests validation with missing description
func TestValidatePartial_MissingDescription(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "",
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

// TestValidatePartial_InvalidVariableType tests validation with invalid variable type
func TestValidatePartial_InvalidVariableType(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "Test",
		Variables: []PartialVar{
			{
				Name: "var1",
				Type: "invalid-type",
			},
		},
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

// TestValidatePartial_ChoiceWithoutChoices tests validation of choice type without choices
func TestValidatePartial_ChoiceWithoutChoices(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "Test",
		Variables: []PartialVar{
			{
				Name:    "var1",
				Type:    template.VarTypeChoice,
				Choices: []string{}, // Empty choices
			},
		},
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "choice")
}

// TestValidatePartial_InvalidConflictStrategy tests validation with invalid conflict strategy
func TestValidatePartial_InvalidConflictStrategy(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "Test",
		Conflicts: ConflictConfig{
			Strategy: "invalid-strategy",
		},
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strategy")
}

// TestValidatePartial_DuplicateVariableName tests validation with duplicate variable names
func TestValidatePartial_DuplicateVariableName(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "Test",
		Variables: []PartialVar{
			{Name: "duplicate", Type: template.VarTypeString},
			{Name: "duplicate", Type: template.VarTypeString},
		},
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// TestValidatePartial_InvalidValidationRegex tests validation with invalid regex
func TestValidatePartial_InvalidValidationRegex(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "Test",
		Variables: []PartialVar{
			{
				Name:       "var1",
				Type:       template.VarTypeString,
				Validation: "[invalid(regex",
			},
		},
	}

	err := ValidatePartial(p, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regex")
}

// TestLoadPartialByName tests loading partial by name from multiple directories
func TestLoadPartialByName(t *testing.T) {
	primaryDir := t.TempDir()
	fallbackDir := t.TempDir()

	createTestPartial(t, primaryDir, "primary-only", nil)
	createTestPartial(t, fallbackDir, "fallback-only", nil)

	// Load from primary
	p, path, err := LoadPartialByName("primary-only", []string{primaryDir, fallbackDir})
	require.NoError(t, err)
	assert.Equal(t, "primary-only", p.Name)
	assert.Equal(t, filepath.Join(primaryDir, "primary-only"), path)

	// Load from fallback
	p, path, err = LoadPartialByName("fallback-only", []string{primaryDir, fallbackDir})
	require.NoError(t, err)
	assert.Equal(t, "fallback-only", p.Name)
	assert.Equal(t, filepath.Join(fallbackDir, "fallback-only"), path)

	// Not found
	_, _, err = LoadPartialByName("nonexistent", []string{primaryDir, fallbackDir})
	require.Error(t, err)
}

// TestPartialExists tests checking if partial exists
func TestPartialExists(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "exists", nil)

	assert.True(t, PartialExists([]string{dir}, "exists"))
	assert.False(t, PartialExists([]string{dir}, "does-not-exist"))
}

// TestCountPartialFiles tests counting files in a partial
func TestCountPartialFiles(t *testing.T) {
	dir := t.TempDir()
	partialDir := createTestPartialWithFiles(t, dir, "with-files", nil, map[string]string{
		"file1.txt":           "content1",
		"file2.txt":           "content2",
		"subdir/file3.txt":    "content3",
		"subdir/nested/f.txt": "content4",
	})

	count := countPartialFiles(partialDir)
	assert.Equal(t, 4, count)
}

// TestGetPartialFilesPath tests getting the files directory path
func TestGetPartialFilesPath(t *testing.T) {
	path := GetPartialFilesPath("/some/partial")
	assert.Equal(t, "/some/partial/files", path)
}

// TestGetPartialHooksPath tests getting the hooks directory path
func TestGetPartialHooksPath(t *testing.T) {
	path := GetPartialHooksPath("/some/partial")
	assert.Equal(t, "/some/partial/hooks", path)
}

// TestPartialInfo_CountsCorrectly tests that ToInfo counts hooks correctly
func TestPartialInfo_CountsCorrectly(t *testing.T) {
	p := &Partial{
		Schema:      1,
		Name:        "test",
		Description: "Test",
		Variables: []PartialVar{
			{Name: "v1", Type: template.VarTypeString},
			{Name: "v2", Type: template.VarTypeBoolean},
		},
		Hooks: PartialHooks{
			PreApply:  template.HookSpec{Script: "pre.sh"},
			PostApply: template.HookSpec{Script: "post.sh"},
		},
		Tags: []string{"tag1", "tag2"},
	}

	info := p.ToInfo("/path", "/source", 5)
	assert.Equal(t, "test", info.Name)
	assert.Equal(t, 2, info.VarCount)
	assert.Equal(t, 5, info.FileCount)
	assert.Equal(t, 2, info.HookCount)
	assert.Equal(t, []string{"tag1", "tag2"}, info.Tags)
}

// TestIsValidPartialName tests the name validation function
func TestIsValidPartialName(t *testing.T) {
	validNames := []string{
		"agent-setup",
		"python-tooling",
		"eslint",
		"a",
		"a1",
		"a-b-c",
		"1abc", // Starts with number is allowed per regex
	}

	invalidNames := []string{
		"Agent-Setup",
		"-starts-with-hyphen",
		"has spaces",
		"has_underscore",
		"has.dot",
		"",
	}

	for _, name := range validNames {
		assert.True(t, IsValidPartialName(name), "expected %q to be valid", name)
	}

	for _, name := range invalidNames {
		assert.False(t, IsValidPartialName(name), "expected %q to be invalid", name)
	}
}

// TestIsValidConflictStrategy tests conflict strategy validation
func TestIsValidConflictStrategy(t *testing.T) {
	assert.True(t, IsValidConflictStrategy("prompt"))
	assert.True(t, IsValidConflictStrategy("skip"))
	assert.True(t, IsValidConflictStrategy("overwrite"))
	assert.True(t, IsValidConflictStrategy("backup"))
	assert.True(t, IsValidConflictStrategy("merge"))

	assert.False(t, IsValidConflictStrategy("invalid"))
	assert.False(t, IsValidConflictStrategy(""))
	assert.False(t, IsValidConflictStrategy("SKIP"))
}

// TestValidatePartial_NonExistentHookPath tests validation with non-existent hook script
func TestValidatePartial_NonExistentHookPath(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "hook-test")
	require.NoError(t, os.MkdirAll(partialDir, 0755))

	p := &Partial{
		Schema:      1,
		Name:        "hook-test",
		Description: "Test",
		Hooks: PartialHooks{
			PreApply: template.HookSpec{Script: "nonexistent.sh"},
		},
	}

	err := ValidatePartial(p, partialDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook")
	assert.Contains(t, err.Error(), "not found")
}

// TestValidatePartial_ValidHookInHooksDir tests validation with hook in hooks/ directory
func TestValidatePartial_ValidHookInHooksDir(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "hook-test")
	hooksDir := filepath.Join(partialDir, PartialHooksDir)
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create the hook script
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "setup.sh"), []byte("#!/bin/bash\necho setup"), 0755))

	p := &Partial{
		Schema:      1,
		Name:        "hook-test",
		Description: "Test",
		Hooks: PartialHooks{
			PreApply: template.HookSpec{Script: "setup.sh"},
		},
	}

	err := ValidatePartial(p, partialDir)
	assert.NoError(t, err)
}

// TestValidatePartial_ValidHookRelativeToRoot tests validation with hook relative to partial root
func TestValidatePartial_ValidHookRelativeToRoot(t *testing.T) {
	dir := t.TempDir()
	partialDir := filepath.Join(dir, "hook-test")
	require.NoError(t, os.MkdirAll(partialDir, 0755))

	// Create the hook script at root level
	require.NoError(t, os.WriteFile(filepath.Join(partialDir, "setup.sh"), []byte("#!/bin/bash\necho setup"), 0755))

	p := &Partial{
		Schema:      1,
		Name:        "hook-test",
		Description: "Test",
		Hooks: PartialHooks{
			PostApply: template.HookSpec{Script: "setup.sh"},
		},
	}

	err := ValidatePartial(p, partialDir)
	assert.NoError(t, err)
}

// TestValidatePartialDir_MissingFilesDir tests ValidatePartialDir with missing files directory
func TestValidatePartialDir_MissingFilesDir(t *testing.T) {
	dir := t.TempDir()
	partialDir := createTestPartial(t, dir, "no-files-dir", &Partial{
		Schema:      1,
		Name:        "no-files-dir",
		Description: "Test",
		Files: PartialFiles{
			Include: []string{"**/*"}, // Has include patterns but no files/ dir
		},
	})

	err := ValidatePartialDir(partialDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "files")
}

// TestValidatePartialDir_OptionalFilesDir tests that files directory is optional without include patterns
func TestValidatePartialDir_OptionalFilesDir(t *testing.T) {
	dir := t.TempDir()
	partialDir := createTestPartial(t, dir, "no-includes", &Partial{
		Schema:      1,
		Name:        "no-includes",
		Description: "Test",
		// No include patterns
	})

	err := ValidatePartialDir(partialDir)
	assert.NoError(t, err)
}

// TestEnsurePartialsDir tests creating the partials directory
func TestEnsurePartialsDir(t *testing.T) {
	dir := t.TempDir()
	partialsDir := filepath.Join(dir, "new-partials")

	err := EnsurePartialsDir(partialsDir)
	require.NoError(t, err)

	info, err := os.Stat(partialsDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestPartial_GetConflictStrategy tests the GetConflictStrategy method
func TestPartial_GetConflictStrategy(t *testing.T) {
	// With explicit strategy
	p := &Partial{Conflicts: ConflictConfig{Strategy: "skip"}}
	assert.Equal(t, "skip", p.GetConflictStrategy())

	// Without explicit strategy (default)
	p = &Partial{}
	assert.Equal(t, "prompt", p.GetConflictStrategy())
}

// TestPartial_GetTemplateExtensions tests the GetTemplateExtensions method
func TestPartial_GetTemplateExtensions(t *testing.T) {
	// With explicit extensions
	p := &Partial{Files: PartialFiles{TemplateExtensions: []string{".template", ".tpl"}}}
	assert.Equal(t, []string{".template", ".tpl"}, p.GetTemplateExtensions())

	// Without explicit extensions (default)
	p = &Partial{}
	assert.Equal(t, []string{".tmpl"}, p.GetTemplateExtensions())
}

// TestPartial_HasPrerequisites tests the HasPrerequisites method
func TestPartial_HasPrerequisites(t *testing.T) {
	// With commands
	p := &Partial{Requires: Requirements{Commands: []string{"git"}}}
	assert.True(t, p.HasPrerequisites())

	// With files
	p = &Partial{Requires: Requirements{Files: []string{"package.json"}}}
	assert.True(t, p.HasPrerequisites())

	// With both
	p = &Partial{Requires: Requirements{Commands: []string{"git"}, Files: []string{"package.json"}}}
	assert.True(t, p.HasPrerequisites())

	// With neither
	p = &Partial{}
	assert.False(t, p.HasPrerequisites())
}

// TestFindPartial_ExactMatchOnly tests that FindPartial requires exact match (no fuzzy)
func TestFindPartial_ExactMatchOnly(t *testing.T) {
	dir := t.TempDir()
	createTestPartial(t, dir, "my-partial", nil)

	// Partial match should fail
	_, err := FindPartial("my-par", []string{dir})
	require.Error(t, err)

	// Full name should work
	_, err = FindPartial("my-partial", []string{dir})
	require.NoError(t, err)
}

// TestListPartials_NestedDirectoriesNotScanned tests that nested directories are not scanned
func TestListPartials_NestedDirectoriesNotScanned(t *testing.T) {
	dir := t.TempDir()

	// Create top-level partial
	createTestPartial(t, dir, "top-level", nil)

	// Create nested directory with a partial inside (should NOT be discovered)
	nestedDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	createTestPartial(t, nestedDir, "nested", nil)

	infos, err := ListPartials([]string{dir})
	require.NoError(t, err)
	require.Len(t, infos, 1, "Should only find top-level partial")
	assert.Equal(t, "top-level", infos[0].Name)
}

// TestFindPartial_InFallbackDirectory tests finding partial in fallback directory
func TestFindPartial_InFallbackDirectory(t *testing.T) {
	primaryDir := t.TempDir()
	fallbackDir := t.TempDir()

	// Create partial only in fallback
	createTestPartial(t, fallbackDir, "fallback-partial", nil)

	path, err := FindPartial("fallback-partial", []string{primaryDir, fallbackDir})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(fallbackDir, "fallback-partial"), path)
}
