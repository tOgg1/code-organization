package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
)

// testConfig creates a test config with temp directories.
func testConfig(t *testing.T, tmpDir string) *config.Config {
	t.Helper()
	return &config.Config{
		Schema:   1,
		CodeRoot: tmpDir,
	}
}

// setupTestTemplate creates a test template structure in the templates directory.
func setupTestTemplate(t *testing.T, templatesDir, templateName string, tmpl *Template) {
	t.Helper()

	templatePath := filepath.Join(templatesDir, templateName)
	if err := os.MkdirAll(templatePath, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Write template.json
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatePath, TemplateManifestFile), data, 0644); err != nil {
		t.Fatalf("Failed to write template.json: %v", err)
	}
}

// setupTemplateFiles creates files in the template's files/ directory.
func setupTemplateFiles(t *testing.T, templatesDir, templateName string, files map[string]string) {
	t.Helper()

	filesDir := filepath.Join(templatesDir, templateName, TemplateFilesDir)
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("Failed to create files dir: %v", err)
	}

	for name, content := range files {
		path := filepath.Join(filesDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}
}

// setupGlobalFiles creates files in the _global directory.
func setupGlobalFiles(t *testing.T, templatesDir string, files map[string]string) {
	t.Helper()

	globalDir := filepath.Join(templatesDir, GlobalTemplateDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global dir: %v", err)
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
}

// setupHook creates a hook script in the template's hooks/ directory.
func setupHook(t *testing.T, templatesDir, templateName, hookName, content string) {
	t.Helper()

	hooksDir := filepath.Join(templatesDir, templateName, TemplateHooksDir)
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	hookPath := filepath.Join(hooksDir, hookName)
	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		t.Fatalf("Failed to write hook %s: %v", hookName, err)
	}
}

func TestCreateWorkspaceBasic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create test template
	tmpl := &Template{
		Schema:      1,
		Name:        "basic",
		Description: "A basic test template",
		Version:     "1.0.0",
	}
	setupTestTemplate(t, templatesDir, "basic", tmpl)

	// Create template files
	setupTemplateFiles(t, templatesDir, "basic", map[string]string{
		"README.md.tmpl": "# {{PROJECT}}\n\nOwner: {{OWNER}}",
		"config.json":    `{"name": "test"}`,
	})

	// Create global files
	setupGlobalFiles(t, templatesDir, map[string]string{
		"AGENTS.md.tmpl": "# Agents\n\nProject: {{PROJECT}}",
		".gitignore":     "node_modules/\n.env",
	})

	// Create workspace
	opts := CreateOptions{
		TemplateName: "basic",
		Variables:    map[string]string{},
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "testowner", "testproject", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify result
	if result.WorkspaceSlug != "testowner--testproject" {
		t.Errorf("WorkspaceSlug = %q, want %q", result.WorkspaceSlug, "testowner--testproject")
	}
	if result.TemplateUsed != "basic" {
		t.Errorf("TemplateUsed = %q, want %q", result.TemplateUsed, "basic")
	}
	if result.GlobalFiles != 2 {
		t.Errorf("GlobalFiles = %d, want 2", result.GlobalFiles)
	}
	if result.TemplateFiles != 2 {
		t.Errorf("TemplateFiles = %d, want 2", result.TemplateFiles)
	}

	// Verify workspace was created
	workspacePath := result.WorkspacePath
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		t.Error("Workspace directory was not created")
	}

	// Verify template files exist
	readme, err := os.ReadFile(filepath.Join(workspacePath, "README.md"))
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	expectedReadme := "# testproject\n\nOwner: testowner"
	if string(readme) != expectedReadme {
		t.Errorf("README.md = %q, want %q", string(readme), expectedReadme)
	}

	// Verify global files exist
	agents, err := os.ReadFile(filepath.Join(workspacePath, "AGENTS.md"))
	if err != nil {
		t.Fatalf("Failed to read AGENTS.md: %v", err)
	}
	expectedAgents := "# Agents\n\nProject: testproject"
	if string(agents) != expectedAgents {
		t.Errorf("AGENTS.md = %q, want %q", string(agents), expectedAgents)
	}

	// Verify project.json was created
	proj, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		t.Fatalf("Failed to load project.json: %v", err)
	}
	if proj.Owner != "testowner" {
		t.Errorf("Project owner = %q, want %q", proj.Owner, "testowner")
	}
	if proj.Name != "testproject" {
		t.Errorf("Project name = %q, want %q", proj.Name, "testproject")
	}
	if proj.Template != "basic" {
		t.Errorf("Project template = %q, want %q", proj.Template, "basic")
	}
}

func TestCreateWorkspaceWithVariables(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with variables
	tmpl := &Template{
		Schema:      1,
		Name:        "with-vars",
		Description: "Template with variables",
		Variables: []TemplateVar{
			{Name: "app_name", Type: VarTypeString, Default: "{{PROJECT}}"},
			{Name: "version", Type: VarTypeString, Default: "0.1.0"},
			{Name: "author", Type: VarTypeString, Required: false},
		},
	}
	setupTestTemplate(t, templatesDir, "with-vars", tmpl)

	// Create template files using variables
	setupTemplateFiles(t, templatesDir, "with-vars", map[string]string{
		"package.json.tmpl": `{"name": "{{app_name}}", "version": "{{version}}", "author": "{{author}}"}`,
	})

	// Create workspace with provided variables
	opts := CreateOptions{
		TemplateName: "with-vars",
		Variables: map[string]string{
			"app_name": "my-custom-app",
			"author":   "Test Author",
		},
		NoHooks: true,
	}

	result, err := CreateWorkspace(cfg, "owner", "project", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify variable substitution
	pkgJSON, err := os.ReadFile(filepath.Join(result.WorkspacePath, "package.json"))
	if err != nil {
		t.Fatalf("Failed to read package.json: %v", err)
	}

	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Author  string `json:"author"`
	}
	if err := json.Unmarshal(pkgJSON, &pkg); err != nil {
		t.Fatalf("Failed to parse package.json: %v", err)
	}

	if pkg.Name != "my-custom-app" {
		t.Errorf("package.name = %q, want %q", pkg.Name, "my-custom-app")
	}
	if pkg.Version != "0.1.0" {
		t.Errorf("package.version = %q, want %q", pkg.Version, "0.1.0")
	}
	if pkg.Author != "Test Author" {
		t.Errorf("package.author = %q, want %q", pkg.Author, "Test Author")
	}
}

func TestCreateWorkspaceWithDependentVariables(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with dependent variables
	tmpl := &Template{
		Schema:      1,
		Name:        "dependent-vars",
		Description: "Template with dependent variables",
		Variables: []TemplateVar{
			{Name: "base_name", Type: VarTypeString, Default: "{{PROJECT}}"},
			{Name: "api_name", Type: VarTypeString, Default: "{{base_name}}-api"},
			{Name: "client_name", Type: VarTypeString, Default: "{{base_name}}-client"},
		},
	}
	setupTestTemplate(t, templatesDir, "dependent-vars", tmpl)

	setupTemplateFiles(t, templatesDir, "dependent-vars", map[string]string{
		"names.txt.tmpl": "Base: {{base_name}}\nAPI: {{api_name}}\nClient: {{client_name}}",
	})

	opts := CreateOptions{
		TemplateName: "dependent-vars",
		Variables:    map[string]string{},
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "owner", "myapp", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	names, err := os.ReadFile(filepath.Join(result.WorkspacePath, "names.txt"))
	if err != nil {
		t.Fatalf("Failed to read names.txt: %v", err)
	}

	expected := "Base: myapp\nAPI: myapp-api\nClient: myapp-client"
	if string(names) != expected {
		t.Errorf("names.txt = %q, want %q", string(names), expected)
	}
}

func TestCreateWorkspaceWithRepos(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with repos
	tmpl := &Template{
		Schema:      1,
		Name:        "with-repos",
		Description: "Template with repositories",
		Repos: []TemplateRepo{
			{Name: "frontend", Init: true, DefaultBranch: "main"},
			{Name: "backend", Init: true, DefaultBranch: "main"},
		},
	}
	setupTestTemplate(t, templatesDir, "with-repos", tmpl)

	opts := CreateOptions{
		TemplateName: "with-repos",
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "owner", "myapp", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	if result.ReposCreated != 2 {
		t.Errorf("ReposCreated = %d, want 2", result.ReposCreated)
	}

	// Verify repo directories were created
	reposPath := filepath.Join(result.WorkspacePath, "repos")
	for _, repoName := range []string{"frontend", "backend"} {
		repoPath := filepath.Join(reposPath, repoName)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			t.Errorf("Repo directory %s was not created", repoName)
		}
	}

	// Verify repos are in project.json
	proj, err := model.LoadProject(filepath.Join(result.WorkspacePath, "project.json"))
	if err != nil {
		t.Fatalf("Failed to load project.json: %v", err)
	}
	if len(proj.Repos) != 2 {
		t.Errorf("project.repos length = %d, want 2", len(proj.Repos))
	}
}

func TestCreateWorkspaceWithTags(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with tags
	tmpl := &Template{
		Schema:      1,
		Name:        "with-tags",
		Description: "Template with tags",
		Tags:        []string{"web", "fullstack", "typescript"},
	}
	setupTestTemplate(t, templatesDir, "with-tags", tmpl)

	opts := CreateOptions{
		TemplateName: "with-tags",
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "owner", "myapp", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	proj, err := model.LoadProject(filepath.Join(result.WorkspacePath, "project.json"))
	if err != nil {
		t.Fatalf("Failed to load project.json: %v", err)
	}

	if len(proj.Tags) != 3 {
		t.Errorf("project.tags length = %d, want 3", len(proj.Tags))
	}

	expectedTags := map[string]bool{"web": true, "fullstack": true, "typescript": true}
	for _, tag := range proj.Tags {
		if !expectedTags[tag] {
			t.Errorf("Unexpected tag: %s", tag)
		}
	}
}

func TestCreateWorkspaceWithState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with state
	tmpl := &Template{
		Schema:      1,
		Name:        "scratch",
		Description: "Scratch workspace",
		State:       model.StateScratch,
	}
	setupTestTemplate(t, templatesDir, "scratch", tmpl)

	opts := CreateOptions{
		TemplateName: "scratch",
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "owner", "experiment", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	proj, err := model.LoadProject(filepath.Join(result.WorkspacePath, "project.json"))
	if err != nil {
		t.Fatalf("Failed to load project.json: %v", err)
	}

	if proj.State != model.StateScratch {
		t.Errorf("project.state = %q, want %q", proj.State, model.StateScratch)
	}
}

func TestCreateWorkspaceDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template
	tmpl := &Template{
		Schema:      1,
		Name:        "dry-run-test",
		Description: "Dry run test",
		Repos: []TemplateRepo{
			{Name: "repo1", Init: true},
		},
	}
	setupTestTemplate(t, templatesDir, "dry-run-test", tmpl)
	setupTemplateFiles(t, templatesDir, "dry-run-test", map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	})
	setupGlobalFiles(t, templatesDir, map[string]string{
		"global.txt": "global content",
	})

	opts := CreateOptions{
		TemplateName: "dry-run-test",
		NoHooks:      true,
		DryRun:       true,
	}

	result, err := CreateWorkspace(cfg, "owner", "project", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify counts are reported
	if result.TemplateFiles != 2 {
		t.Errorf("TemplateFiles = %d, want 2", result.TemplateFiles)
	}
	if result.GlobalFiles != 1 {
		t.Errorf("GlobalFiles = %d, want 1", result.GlobalFiles)
	}
	if result.ReposCreated != 1 {
		t.Errorf("ReposCreated = %d, want 1", result.ReposCreated)
	}

	// Verify workspace was NOT created
	workspacePath := cfg.WorkspacePath("owner--project")
	if _, err := os.Stat(workspacePath); !os.IsNotExist(err) {
		t.Error("Workspace should NOT be created in dry-run mode")
	}

	// Verify warnings include dry-run message
	foundDryRunWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(strings.ToLower(w), "dry run") {
			foundDryRunWarning = true
			break
		}
	}
	if !foundDryRunWarning {
		t.Error("Expected dry-run warning in result")
	}
}

func TestCreateWorkspaceWithHooks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with hooks
	tmpl := &Template{
		Schema:      1,
		Name:        "with-hooks",
		Description: "Template with hooks",
		Hooks: TemplateHooks{
			PostCreate: HookSpec{
				Script:  "post-create.sh",
				Timeout: "30s",
			},
		},
	}
	setupTestTemplate(t, templatesDir, "with-hooks", tmpl)

	// Create a simple hook that creates a marker file
	hookScript := `#!/bin/bash
echo "Hook ran" > "$CO_WORKSPACE_PATH/hook-marker.txt"
echo "Owner: $CO_OWNER" >> "$CO_WORKSPACE_PATH/hook-marker.txt"
echo "Project: $CO_PROJECT" >> "$CO_WORKSPACE_PATH/hook-marker.txt"
`
	setupHook(t, templatesDir, "with-hooks", "post-create.sh", hookScript)

	opts := CreateOptions{
		TemplateName: "with-hooks",
		NoHooks:      false, // Enable hooks
	}

	result, err := CreateWorkspace(cfg, "owner", "project", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify hook was run
	if len(result.HooksRun) != 1 || result.HooksRun[0] != "post_create" {
		t.Errorf("HooksRun = %v, want [post_create]", result.HooksRun)
	}

	// Verify hook created the marker file
	markerPath := filepath.Join(result.WorkspacePath, "hook-marker.txt")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("Failed to read hook marker file: %v", err)
	}

	markerContent := string(marker)
	if !strings.Contains(markerContent, "Hook ran") {
		t.Error("Hook marker should contain 'Hook ran'")
	}
	if !strings.Contains(markerContent, "Owner: owner") {
		t.Error("Hook marker should contain 'Owner: owner'")
	}
	if !strings.Contains(markerContent, "Project: project") {
		t.Error("Hook marker should contain 'Project: project'")
	}
}

func TestCreateWorkspaceNoHooksFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with hooks
	tmpl := &Template{
		Schema:      1,
		Name:        "with-hooks",
		Description: "Template with hooks",
		Hooks: TemplateHooks{
			PostCreate: HookSpec{
				Script:  "post-create.sh",
				Timeout: "30s",
			},
		},
	}
	setupTestTemplate(t, templatesDir, "with-hooks", tmpl)
	setupHook(t, templatesDir, "with-hooks", "post-create.sh", `#!/bin/bash
touch "$CO_WORKSPACE_PATH/hook-ran"
`)

	opts := CreateOptions{
		TemplateName: "with-hooks",
		NoHooks:      true, // Disable hooks
	}

	result, err := CreateWorkspace(cfg, "owner", "project", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify hook was NOT run
	if len(result.HooksRun) != 0 {
		t.Errorf("HooksRun = %v, want empty (hooks disabled)", result.HooksRun)
	}

	// Verify hook marker was NOT created
	markerPath := filepath.Join(result.WorkspacePath, "hook-ran")
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Error("Hook should NOT have run when NoHooks=true")
	}
}

func TestCreateWorkspaceSkipGlobalFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template that skips all global files
	tmpl := &Template{
		Schema:          1,
		Name:            "skip-global",
		Description:     "Template that skips global files",
		SkipGlobalFiles: true,
	}
	setupTestTemplate(t, templatesDir, "skip-global", tmpl)
	setupTemplateFiles(t, templatesDir, "skip-global", map[string]string{
		"template.txt": "template content",
	})
	setupGlobalFiles(t, templatesDir, map[string]string{
		"global.txt": "global content",
	})

	opts := CreateOptions{
		TemplateName: "skip-global",
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "owner", "project", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify global files were skipped
	if result.GlobalFiles != 0 {
		t.Errorf("GlobalFiles = %d, want 0 (skipped)", result.GlobalFiles)
	}
	if result.TemplateFiles != 1 {
		t.Errorf("TemplateFiles = %d, want 1", result.TemplateFiles)
	}

	// Verify global file was NOT created
	if _, err := os.Stat(filepath.Join(result.WorkspacePath, "global.txt")); !os.IsNotExist(err) {
		t.Error("global.txt should NOT be created when skip_global_files=true")
	}

	// Verify template file WAS created
	if _, err := os.Stat(filepath.Join(result.WorkspacePath, "template.txt")); os.IsNotExist(err) {
		t.Error("template.txt should be created")
	}
}

func TestCreateWorkspaceSkipSpecificGlobalFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template that skips specific global files
	tmpl := &Template{
		Schema:          1,
		Name:            "skip-some",
		Description:     "Template that skips some global files",
		SkipGlobalFiles: []interface{}{"skip.txt"},
	}
	setupTestTemplate(t, templatesDir, "skip-some", tmpl)
	setupGlobalFiles(t, templatesDir, map[string]string{
		"keep.txt": "keep content",
		"skip.txt": "skip content",
	})

	opts := CreateOptions{
		TemplateName: "skip-some",
		NoHooks:      true,
	}

	result, err := CreateWorkspace(cfg, "owner", "project", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	// Verify only non-skipped files were created
	if result.GlobalFiles != 1 {
		t.Errorf("GlobalFiles = %d, want 1", result.GlobalFiles)
	}

	if _, err := os.Stat(filepath.Join(result.WorkspacePath, "keep.txt")); os.IsNotExist(err) {
		t.Error("keep.txt should be created")
	}
	if _, err := os.Stat(filepath.Join(result.WorkspacePath, "skip.txt")); !os.IsNotExist(err) {
		t.Error("skip.txt should NOT be created")
	}
}

func TestCreateWorkspaceMissingRequiredVariable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with required variable
	tmpl := &Template{
		Schema:      1,
		Name:        "required-var",
		Description: "Template with required variable",
		Variables: []TemplateVar{
			{Name: "api_key", Type: VarTypeString, Required: true},
		},
	}
	setupTestTemplate(t, templatesDir, "required-var", tmpl)

	opts := CreateOptions{
		TemplateName: "required-var",
		Variables:    map[string]string{}, // No api_key provided
		NoHooks:      true,
	}

	_, err = CreateWorkspace(cfg, "owner", "project", opts)
	if err == nil {
		t.Error("Expected error for missing required variable")
	}
	if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "api_key") {
		t.Errorf("Error should mention missing required variable, got: %v", err)
	}
}

func TestCreateWorkspaceInvalidVariableValue(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with integer variable
	tmpl := &Template{
		Schema:      1,
		Name:        "int-var",
		Description: "Template with integer variable",
		Variables: []TemplateVar{
			{Name: "port", Type: VarTypeInteger},
		},
	}
	setupTestTemplate(t, templatesDir, "int-var", tmpl)

	opts := CreateOptions{
		TemplateName: "int-var",
		Variables:    map[string]string{"port": "not-a-number"},
		NoHooks:      true,
	}

	_, err = CreateWorkspace(cfg, "owner", "project", opts)
	if err == nil {
		t.Error("Expected error for invalid integer value")
	}
}

func TestCreateWorkspaceTemplateNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)

	// Create templates directory but no templates
	if err := os.MkdirAll(cfg.TemplatesDir(), 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	opts := CreateOptions{
		TemplateName: "nonexistent",
		NoHooks:      true,
	}

	_, err = CreateWorkspace(cfg, "owner", "project", opts)
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestApplyTemplateToExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create existing workspace
	workspacePath := cfg.WorkspacePath("owner--project")
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create existing project.json
	proj := model.NewProject("owner", "project")
	if err := proj.Save(workspacePath); err != nil {
		t.Fatalf("Failed to save project.json: %v", err)
	}

	// Create template
	tmpl := &Template{
		Schema:      1,
		Name:        "migrate-template",
		Description: "Template for migration",
	}
	setupTestTemplate(t, templatesDir, "migrate-template", tmpl)
	setupTemplateFiles(t, templatesDir, "migrate-template", map[string]string{
		"new-file.txt.tmpl": "Project: {{PROJECT}}",
	})
	setupGlobalFiles(t, templatesDir, map[string]string{
		"global.txt": "global content",
	})

	opts := CreateOptions{
		TemplateName: "migrate-template",
		NoHooks:      true,
	}

	result, err := ApplyTemplateToExisting(cfg, workspacePath, "migrate-template", opts)
	if err != nil {
		t.Fatalf("ApplyTemplateToExisting() error = %v", err)
	}

	// Verify files were created
	if result.GlobalFiles != 1 {
		t.Errorf("GlobalFiles = %d, want 1", result.GlobalFiles)
	}
	if result.TemplateFiles != 1 {
		t.Errorf("TemplateFiles = %d, want 1", result.TemplateFiles)
	}

	// Verify new file was created with variable substitution
	newFile, err := os.ReadFile(filepath.Join(workspacePath, "new-file.txt"))
	if err != nil {
		t.Fatalf("Failed to read new-file.txt: %v", err)
	}
	if string(newFile) != "Project: project" {
		t.Errorf("new-file.txt = %q, want %q", string(newFile), "Project: project")
	}

	// Verify project.json was updated
	updatedProj, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		t.Fatalf("Failed to load updated project.json: %v", err)
	}
	if updatedProj.Template != "migrate-template" {
		t.Errorf("project.template = %q, want %q", updatedProj.Template, "migrate-template")
	}
}

func TestApplyTemplateToExistingWithMigrateHook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create existing workspace
	workspacePath := cfg.WorkspacePath("owner--project")
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create existing project.json
	proj := model.NewProject("owner", "project")
	if err := proj.Save(workspacePath); err != nil {
		t.Fatalf("Failed to save project.json: %v", err)
	}

	// Create template with post_migrate hook
	tmpl := &Template{
		Schema:      1,
		Name:        "migrate-hook",
		Description: "Template with migrate hook",
		Hooks: TemplateHooks{
			PostMigrate: HookSpec{
				Script:  "post-migrate.sh",
				Timeout: "30s",
			},
		},
	}
	setupTestTemplate(t, templatesDir, "migrate-hook", tmpl)
	setupHook(t, templatesDir, "migrate-hook", "post-migrate.sh", `#!/bin/bash
touch "$CO_WORKSPACE_PATH/migrate-marker.txt"
`)

	opts := CreateOptions{
		TemplateName: "migrate-hook",
		NoHooks:      false,
	}

	result, err := ApplyTemplateToExisting(cfg, workspacePath, "migrate-hook", opts)
	if err != nil {
		t.Fatalf("ApplyTemplateToExisting() error = %v", err)
	}

	// Verify hook was run
	if len(result.HooksRun) != 1 || result.HooksRun[0] != "post_migrate" {
		t.Errorf("HooksRun = %v, want [post_migrate]", result.HooksRun)
	}

	// Verify hook marker was created
	if _, err := os.Stat(filepath.Join(workspacePath, "migrate-marker.txt")); os.IsNotExist(err) {
		t.Error("post_migrate hook should have created migrate-marker.txt")
	}
}

func TestCreateWorkspaceWithConditionals(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	// Create template with conditionals
	tmpl := &Template{
		Schema:      1,
		Name:        "conditionals",
		Description: "Template with conditionals",
		Variables: []TemplateVar{
			{Name: "include_tests", Type: VarTypeBoolean, Default: "true"},
			{Name: "framework", Type: VarTypeChoice, Choices: []string{"react", "vue", "none"}, Default: "react"},
		},
	}
	setupTestTemplate(t, templatesDir, "conditionals", tmpl)

	setupTemplateFiles(t, templatesDir, "conditionals", map[string]string{
		"README.md.tmpl": `# {{PROJECT}}

{{#if include_tests}}
## Testing

Run tests with: npm test
{{/if}}

{{#if framework == "react"}}
## React Setup

This project uses React.
{{/if}}

{{#if framework == "vue"}}
## Vue Setup

This project uses Vue.
{{/if}}
`,
	})

	// Test with tests enabled and React
	opts := CreateOptions{
		TemplateName: "conditionals",
		Variables: map[string]string{
			"include_tests": "true",
			"framework":     "react",
		},
		NoHooks: true,
	}

	result, err := CreateWorkspace(cfg, "owner", "myapp", opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(result.WorkspacePath, "README.md"))
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}

	content := string(readme)
	if !strings.Contains(content, "## Testing") {
		t.Error("README should contain Testing section when include_tests=true")
	}
	if !strings.Contains(content, "React Setup") {
		t.Error("README should contain React Setup when framework=react")
	}
	if strings.Contains(content, "Vue Setup") {
		t.Error("README should NOT contain Vue Setup when framework=react")
	}
}

func TestParseSlug(t *testing.T) {
	tests := []struct {
		slug        string
		wantOwner   string
		wantProject string
	}{
		{"owner--project", "owner", "project"},
		{"acme--web-app", "acme", "web-app"},
		{"single", "single", "single"},
		{"a--b--c", "a", "b"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			owner, project := parseSlug(tt.slug)
			if owner != tt.wantOwner {
				t.Errorf("parseSlug(%q) owner = %q, want %q", tt.slug, owner, tt.wantOwner)
			}
			if project != tt.wantProject {
				t.Errorf("parseSlug(%q) project = %q, want %q", tt.slug, project, tt.wantProject)
			}
		})
	}
}

func TestSplitSlug(t *testing.T) {
	tests := []struct {
		slug string
		want []string
	}{
		{"owner--project", []string{"owner", "project"}},
		{"a--b--c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a-b--c-d", []string{"a-b", "c-d"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := splitSlug(tt.slug)
			if len(got) != len(tt.want) {
				t.Errorf("splitSlug(%q) = %v, want %v", tt.slug, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitSlug(%q)[%d] = %q, want %q", tt.slug, i, got[i], tt.want[i])
				}
			}
		})
	}
}
