package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVariables(t *testing.T) {
	tests := []struct {
		name      string
		varsA     []TemplateVar
		varsB     []TemplateVar
		wantDiffs int
		wantTypes map[DiffType]int
	}{
		{
			name:      "no differences",
			varsA:     []TemplateVar{{Name: "FOO", Type: VarTypeString}},
			varsB:     []TemplateVar{{Name: "FOO", Type: VarTypeString}},
			wantDiffs: 0,
		},
		{
			name:      "variable added",
			varsA:     []TemplateVar{{Name: "FOO", Type: VarTypeString}},
			varsB:     []TemplateVar{{Name: "FOO", Type: VarTypeString}, {Name: "BAR", Type: VarTypeString}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffAdded: 1},
		},
		{
			name:      "variable removed",
			varsA:     []TemplateVar{{Name: "FOO", Type: VarTypeString}, {Name: "BAR", Type: VarTypeString}},
			varsB:     []TemplateVar{{Name: "FOO", Type: VarTypeString}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffRemoved: 1},
		},
		{
			name:      "variable type changed",
			varsA:     []TemplateVar{{Name: "FOO", Type: VarTypeString}},
			varsB:     []TemplateVar{{Name: "FOO", Type: VarTypeBoolean}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffChanged: 1},
		},
		{
			name:      "variable required changed",
			varsA:     []TemplateVar{{Name: "FOO", Type: VarTypeString, Required: false}},
			varsB:     []TemplateVar{{Name: "FOO", Type: VarTypeString, Required: true}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffChanged: 1},
		},
		{
			name:      "multiple changes",
			varsA:     []TemplateVar{{Name: "FOO", Type: VarTypeString}, {Name: "OLD", Type: VarTypeString}},
			varsB:     []TemplateVar{{Name: "FOO", Type: VarTypeBoolean}, {Name: "NEW", Type: VarTypeString}},
			wantDiffs: 3,
			wantTypes: map[DiffType]int{DiffAdded: 1, DiffRemoved: 1, DiffChanged: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := compareVariables(tt.varsA, tt.varsB)

			if len(diffs) != tt.wantDiffs {
				t.Errorf("got %d diffs, want %d", len(diffs), tt.wantDiffs)
			}

			if tt.wantTypes != nil {
				typeCounts := make(map[DiffType]int)
				for _, d := range diffs {
					typeCounts[d.DiffType]++
				}
				for dt, want := range tt.wantTypes {
					if got := typeCounts[dt]; got != want {
						t.Errorf("diff type %q: got %d, want %d", dt, got, want)
					}
				}
			}
		})
	}
}

func TestCompareRepos(t *testing.T) {
	tests := []struct {
		name      string
		reposA    []TemplateRepo
		reposB    []TemplateRepo
		wantDiffs int
		wantTypes map[DiffType]int
	}{
		{
			name:      "no differences",
			reposA:    []TemplateRepo{{Name: "api", CloneURL: "https://example.com/api"}},
			reposB:    []TemplateRepo{{Name: "api", CloneURL: "https://example.com/api"}},
			wantDiffs: 0,
		},
		{
			name:      "repo added",
			reposA:    []TemplateRepo{{Name: "api", CloneURL: "https://example.com/api"}},
			reposB:    []TemplateRepo{{Name: "api", CloneURL: "https://example.com/api"}, {Name: "web", CloneURL: "https://example.com/web"}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffAdded: 1},
		},
		{
			name:      "repo removed",
			reposA:    []TemplateRepo{{Name: "api", CloneURL: "https://example.com/api"}, {Name: "web", CloneURL: "https://example.com/web"}},
			reposB:    []TemplateRepo{{Name: "api", CloneURL: "https://example.com/api"}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffRemoved: 1},
		},
		{
			name:      "clone URL changed",
			reposA:    []TemplateRepo{{Name: "api", CloneURL: "https://old.com/api"}},
			reposB:    []TemplateRepo{{Name: "api", CloneURL: "https://new.com/api"}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffChanged: 1},
		},
		{
			name:      "init changed",
			reposA:    []TemplateRepo{{Name: "api", Init: false}},
			reposB:    []TemplateRepo{{Name: "api", Init: true}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffChanged: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := compareRepos(tt.reposA, tt.reposB)

			if len(diffs) != tt.wantDiffs {
				t.Errorf("got %d diffs, want %d", len(diffs), tt.wantDiffs)
			}

			if tt.wantTypes != nil {
				typeCounts := make(map[DiffType]int)
				for _, d := range diffs {
					typeCounts[d.DiffType]++
				}
				for dt, want := range tt.wantTypes {
					if got := typeCounts[dt]; got != want {
						t.Errorf("diff type %q: got %d, want %d", dt, got, want)
					}
				}
			}
		})
	}
}

func TestCompareHooks(t *testing.T) {
	tests := []struct {
		name      string
		hooksA    TemplateHooks
		hooksB    TemplateHooks
		wantDiffs int
		wantTypes map[DiffType]int
	}{
		{
			name:      "no differences",
			hooksA:    TemplateHooks{PostCreate: HookSpec{Script: "echo hello"}},
			hooksB:    TemplateHooks{PostCreate: HookSpec{Script: "echo hello"}},
			wantDiffs: 0,
		},
		{
			name:      "hook added",
			hooksA:    TemplateHooks{},
			hooksB:    TemplateHooks{PostCreate: HookSpec{Script: "echo hello"}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffAdded: 1},
		},
		{
			name:      "hook removed",
			hooksA:    TemplateHooks{PostCreate: HookSpec{Script: "echo hello"}},
			hooksB:    TemplateHooks{},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffRemoved: 1},
		},
		{
			name:      "hook script changed",
			hooksA:    TemplateHooks{PostCreate: HookSpec{Script: "echo old"}},
			hooksB:    TemplateHooks{PostCreate: HookSpec{Script: "echo new"}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffChanged: 1},
		},
		{
			name:      "hook timeout changed",
			hooksA:    TemplateHooks{PostCreate: HookSpec{Script: "echo hello", Timeout: "30s"}},
			hooksB:    TemplateHooks{PostCreate: HookSpec{Script: "echo hello", Timeout: "60s"}},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffChanged: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := compareHooks(&tt.hooksA, &tt.hooksB)

			if len(diffs) != tt.wantDiffs {
				t.Errorf("got %d diffs, want %d", len(diffs), tt.wantDiffs)
			}

			if tt.wantTypes != nil {
				typeCounts := make(map[DiffType]int)
				for _, d := range diffs {
					typeCounts[d.DiffType]++
				}
				for dt, want := range tt.wantTypes {
					if got := typeCounts[dt]; got != want {
						t.Errorf("diff type %q: got %d, want %d", dt, got, want)
					}
				}
			}
		})
	}
}

func TestCompareFiles(t *testing.T) {
	tests := []struct {
		name      string
		filesA    []string
		filesB    []string
		wantDiffs int
		wantTypes map[DiffType]int
	}{
		{
			name:      "no differences",
			filesA:    []string{"README.md", "main.go"},
			filesB:    []string{"README.md", "main.go"},
			wantDiffs: 0,
		},
		{
			name:      "file added",
			filesA:    []string{"README.md"},
			filesB:    []string{"README.md", "main.go"},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffAdded: 1},
		},
		{
			name:      "file removed",
			filesA:    []string{"README.md", "main.go"},
			filesB:    []string{"README.md"},
			wantDiffs: 1,
			wantTypes: map[DiffType]int{DiffRemoved: 1},
		},
		{
			name:      "multiple changes",
			filesA:    []string{"old.go", "shared.go"},
			filesB:    []string{"new.go", "shared.go"},
			wantDiffs: 2,
			wantTypes: map[DiffType]int{DiffAdded: 1, DiffRemoved: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := compareFiles(tt.filesA, tt.filesB, "/dirA", "/dirB")

			if len(diffs) != tt.wantDiffs {
				t.Errorf("got %d diffs, want %d", len(diffs), tt.wantDiffs)
			}

			if tt.wantTypes != nil {
				typeCounts := make(map[DiffType]int)
				for _, d := range diffs {
					typeCounts[d.DiffType]++
				}
				for dt, want := range tt.wantTypes {
					if got := typeCounts[dt]; got != want {
						t.Errorf("diff type %q: got %d, want %d", dt, got, want)
					}
				}
			}
		})
	}
}

func TestCompareTemplates(t *testing.T) {
	// Create temp directories with actual templates
	tempDir := t.TempDir()

	// Template A
	templateADir := filepath.Join(tempDir, "template-a")
	filesADir := filepath.Join(templateADir, "files")
	if err := os.MkdirAll(filesADir, 0755); err != nil {
		t.Fatalf("Failed to create template-a dirs: %v", err)
	}

	manifestA := `{
		"name": "template-a",
		"description": "Template A",
		"variables": [
			{"name": "PROJECT_NAME", "type": "string"},
			{"name": "OLD_VAR", "type": "string"}
		],
		"repos": [
			{"name": "api", "clone_url": "https://example.com/api"}
		],
		"hooks": {
			"post_create": {"script": "echo a"}
		}
	}`
	if err := os.WriteFile(filepath.Join(templateADir, "template.json"), []byte(manifestA), 0644); err != nil {
		t.Fatalf("Failed to write manifest A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesADir, "README.md"), []byte("# A"), 0644); err != nil {
		t.Fatalf("Failed to write file A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesADir, "old.go"), []byte("// old"), 0644); err != nil {
		t.Fatalf("Failed to write old.go: %v", err)
	}

	// Template B
	templateBDir := filepath.Join(tempDir, "template-b")
	filesBDir := filepath.Join(templateBDir, "files")
	if err := os.MkdirAll(filesBDir, 0755); err != nil {
		t.Fatalf("Failed to create template-b dirs: %v", err)
	}

	manifestB := `{
		"name": "template-b",
		"description": "Template B",
		"variables": [
			{"name": "PROJECT_NAME", "type": "string"},
			{"name": "NEW_VAR", "type": "boolean"}
		],
		"repos": [
			{"name": "api", "clone_url": "https://example.com/api-v2"}
		],
		"hooks": {
			"post_create": {"script": "echo b"}
		}
	}`
	if err := os.WriteFile(filepath.Join(templateBDir, "template.json"), []byte(manifestB), 0644); err != nil {
		t.Fatalf("Failed to write manifest B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesBDir, "README.md"), []byte("# B"), 0644); err != nil {
		t.Fatalf("Failed to write file B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesBDir, "new.go"), []byte("// new"), 0644); err != nil {
		t.Fatalf("Failed to write new.go: %v", err)
	}

	// Load templates
	tmplA, err := LoadTemplate(tempDir, "template-a")
	if err != nil {
		t.Fatalf("Failed to load template A: %v", err)
	}

	tmplB, err := LoadTemplate(tempDir, "template-b")
	if err != nil {
		t.Fatalf("Failed to load template B: %v", err)
	}

	// Compare
	result, err := CompareTemplates(tmplA, tmplB, tempDir, tempDir)
	if err != nil {
		t.Fatalf("CompareTemplates error: %v", err)
	}

	// Verify results
	if result.TemplateA != "template-a" {
		t.Errorf("TemplateA = %q, want %q", result.TemplateA, "template-a")
	}
	if result.TemplateB != "template-b" {
		t.Errorf("TemplateB = %q, want %q", result.TemplateB, "template-b")
	}

	// Should have 2 var diffs (OLD_VAR removed, NEW_VAR added)
	if len(result.Vars) != 2 {
		t.Errorf("Expected 2 var diffs, got %d", len(result.Vars))
	}

	// Should have 1 repo diff (URL changed)
	if len(result.Repos) != 1 {
		t.Errorf("Expected 1 repo diff, got %d", len(result.Repos))
	}

	// Should have 1 hook diff (script changed)
	if len(result.Hooks) != 1 {
		t.Errorf("Expected 1 hook diff, got %d", len(result.Hooks))
	}

	// Should have 2 file diffs (old.go removed, new.go added)
	if len(result.Files) != 2 {
		t.Errorf("Expected 2 file diffs, got %d", len(result.Files))
	}

	// Test HasDifferences
	if !result.HasDifferences() {
		t.Error("Expected HasDifferences() = true")
	}

	// Test TotalDiffs
	expected := len(result.Vars) + len(result.Repos) + len(result.Hooks) + len(result.Files)
	if result.TotalDiffs() != expected {
		t.Errorf("TotalDiffs() = %d, want %d", result.TotalDiffs(), expected)
	}
}

func TestCompareTemplatesIdentical(t *testing.T) {
	tempDir := t.TempDir()

	// Create identical templates
	for _, name := range []string{"template-a", "template-b"} {
		templateDir := filepath.Join(tempDir, name)
		filesDir := filepath.Join(templateDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("Failed to create dirs: %v", err)
		}

		manifest := `{
			"name": "` + name + `",
			"description": "Identical template",
			"variables": [{"name": "FOO", "type": "string"}]
		}`
		if err := os.WriteFile(filepath.Join(templateDir, "template.json"), []byte(manifest), 0644); err != nil {
			t.Fatalf("Failed to write manifest: %v", err)
		}
		if err := os.WriteFile(filepath.Join(filesDir, "same.txt"), []byte("same"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	tmplA, _ := LoadTemplate(tempDir, "template-a")
	tmplB, _ := LoadTemplate(tempDir, "template-b")

	result, err := CompareTemplates(tmplA, tmplB, tempDir, tempDir)
	if err != nil {
		t.Fatalf("CompareTemplates error: %v", err)
	}

	if result.HasDifferences() {
		t.Error("Expected no differences for identical templates")
	}

	if result.TotalDiffs() != 0 {
		t.Errorf("TotalDiffs() = %d, want 0", result.TotalDiffs())
	}
}
