package template

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateWorkspaceAppliesPartialsAfterRepos(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	tmpl := &Template{
		Schema:      1,
		Name:        "partials",
		Description: "partials test",
		Repos: []TemplateRepo{
			{Name: "app", Init: true},
		},
		Partials: []PartialRef{
			{Name: "lint"},
		},
	}
	setupTestTemplate(t, templatesDir, "partials", tmpl)

	workspacePath := filepath.Join(cfg.CodeRoot, "owner--project")
	repoPath := filepath.Join(workspacePath, "repos", "app")

	partialCalled := false
	RegisterPartialApplier(func(opts PartialApplyOptions, _ []string) error {
		if _, err := os.Stat(repoPath); err != nil {
			return fmt.Errorf("repo not created before partial apply: %w", err)
		}
		if opts.TargetPath != workspacePath {
			return fmt.Errorf("target path = %q, want %q", opts.TargetPath, workspacePath)
		}
		partialCalled = true
		return nil
	})
	t.Cleanup(func() { RegisterPartialApplier(nil) })

	opts := CreateOptions{
		TemplateName: "partials",
		NoHooks:      true,
	}
	if _, err := CreateWorkspace(cfg, "owner", "project", opts); err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	if !partialCalled {
		t.Error("expected partial applier to be called")
	}
}

func TestCreateWorkspaceResolvesPartialVariables(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	tmpl := &Template{
		Schema:      1,
		Name:        "vars",
		Description: "partials variables test",
		Variables: []TemplateVar{
			{Name: "frontend_stack", Type: VarTypeString, Default: "node"},
		},
		Partials: []PartialRef{
			{
				Name: "agent-setup",
				Variables: map[string]string{
					"stack":   "{{frontend_stack}}",
					"project": "{{PROJECT}}",
				},
			},
		},
	}
	setupTestTemplate(t, templatesDir, "vars", tmpl)

	RegisterPartialApplier(func(opts PartialApplyOptions, _ []string) error {
		if opts.Variables["stack"] != "node" {
			return fmt.Errorf("stack = %q, want %q", opts.Variables["stack"], "node")
		}
		if opts.Variables["project"] != "project" {
			return fmt.Errorf("project = %q, want %q", opts.Variables["project"], "project")
		}
		return nil
	})
	t.Cleanup(func() { RegisterPartialApplier(nil) })

	opts := CreateOptions{
		TemplateName: "vars",
		NoHooks:      true,
	}
	if _, err := CreateWorkspace(cfg, "owner", "project", opts); err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
}

func TestCreateWorkspaceAppliesConditionals(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	tmpl := &Template{
		Schema:      1,
		Name:        "conditional",
		Description: "partials conditional test",
		Variables: []TemplateVar{
			{Name: "frontend_stack", Type: VarTypeString, Default: "node"},
		},
		Partials: []PartialRef{
			{Name: "node-only", When: "{{frontend_stack}} == 'node'"},
			{Name: "non-node", When: "{{frontend_stack}} != 'node'"},
		},
	}
	setupTestTemplate(t, templatesDir, "conditional", tmpl)

	var applied []string
	RegisterPartialApplier(func(opts PartialApplyOptions, _ []string) error {
		applied = append(applied, opts.PartialName)
		return nil
	})
	t.Cleanup(func() { RegisterPartialApplier(nil) })

	opts := CreateOptions{
		TemplateName: "conditional",
		Variables: map[string]string{
			"frontend_stack": "go",
		},
		NoHooks: true,
	}
	if _, err := CreateWorkspace(cfg, "owner", "project", opts); err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	if len(applied) != 1 || applied[0] != "non-node" {
		t.Errorf("applied = %v, want [non-node]", applied)
	}
}

func TestCreateWorkspacePartialApplyError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	tmpl := &Template{
		Schema:      1,
		Name:        "error",
		Description: "partials error test",
		Partials: []PartialRef{
			{Name: "bad-partial"},
		},
	}
	setupTestTemplate(t, templatesDir, "error", tmpl)

	RegisterPartialApplier(func(opts PartialApplyOptions, _ []string) error {
		return fmt.Errorf("partial not found: %s", opts.PartialName)
	})
	t.Cleanup(func() { RegisterPartialApplier(nil) })

	opts := CreateOptions{
		TemplateName: "error",
		NoHooks:      true,
	}
	if _, err := CreateWorkspace(cfg, "owner", "project", opts); err == nil {
		t.Fatal("expected error for partial apply failure")
	}
}

func TestCreateWorkspacePartialApplierMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(t, tmpDir)
	templatesDir := cfg.TemplatesDir()

	tmpl := &Template{
		Schema:      1,
		Name:        "missing-applier",
		Description: "partials missing applier test",
		Partials: []PartialRef{
			{Name: "agent-setup"},
		},
	}
	setupTestTemplate(t, templatesDir, "missing-applier", tmpl)

	t.Cleanup(func() { RegisterPartialApplier(nil) })

	opts := CreateOptions{
		TemplateName: "missing-applier",
		NoHooks:      true,
	}
	if _, err := CreateWorkspace(cfg, "owner", "project", opts); err == nil {
		t.Fatal("expected error when no partial applier is registered")
	}
}
