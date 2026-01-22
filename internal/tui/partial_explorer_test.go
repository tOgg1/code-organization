package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/partial"
	"github.com/tormodhaugland/co/internal/template"
)

func createTestPartialInfo(t *testing.T, root, name string, withVars bool) partial.PartialInfo {
	t.Helper()

	partialDir := filepath.Join(root, name)
	if err := os.MkdirAll(partialDir, 0o755); err != nil {
		t.Fatalf("mkdir partial dir: %v", err)
	}

	p := partial.Partial{
		Schema:      partial.CurrentPartialSchema,
		Name:        name,
		Description: "Test partial",
	}
	if withVars {
		p.Variables = []partial.PartialVar{
			{Name: "foo", Type: template.VarTypeString, Required: true},
		}
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal partial: %v", err)
	}
	if err := os.WriteFile(filepath.Join(partialDir, partial.PartialManifestFile), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	loaded, err := partial.LoadPartial(partialDir)
	if err != nil {
		t.Fatalf("load partial: %v", err)
	}

	return loaded.ToInfo(partialDir, root, 0)
}

func unwrapPartialExplorer(t *testing.T, model tea.Model) PartialExplorerModel {
	t.Helper()
	switch m := model.(type) {
	case PartialExplorerModel:
		return m
	case *PartialExplorerModel:
		return *m
	default:
		t.Fatalf("unexpected model type: %T", model)
		return PartialExplorerModel{}
	}
}

func TestPartialExplorerEnterBrowseSwitchesToApplyAndPromptsVars(t *testing.T) {
	root := t.TempDir()
	info := createTestPartialInfo(t, root, "test-partial", true)
	cfg := config.DefaultConfig()

	m := NewPartialExplorer(cfg, []partial.PartialInfo{info})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := unwrapPartialExplorer(t, updated)

	if next.activeTab != PartialTabApply {
		t.Fatalf("expected apply tab, got %v", next.activeTab)
	}
	if next.applyFocus != ApplyFocusVars {
		t.Fatalf("expected vars focus, got %v", next.applyFocus)
	}
	if cmd == nil {
		t.Fatal("expected variables prompt command")
	}
}

func TestPartialExplorerEnterBrowseNoVarsNoPrompt(t *testing.T) {
	root := t.TempDir()
	info := createTestPartialInfo(t, root, "test-partial", false)
	cfg := config.DefaultConfig()

	m := NewPartialExplorer(cfg, []partial.PartialInfo{info})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := unwrapPartialExplorer(t, updated)

	if next.activeTab != PartialTabApply {
		t.Fatalf("expected apply tab, got %v", next.activeTab)
	}
	if next.applyFocus != ApplyFocusTarget {
		t.Fatalf("expected target focus, got %v", next.applyFocus)
	}
	if cmd != nil {
		t.Fatal("expected no variables prompt command")
	}
}

func TestPartialExplorerApplyVarsHotkeyFromAnyFocus(t *testing.T) {
	root := t.TempDir()
	info := createTestPartialInfo(t, root, "test-partial", true)
	cfg := config.DefaultConfig()

	m := NewPartialExplorer(cfg, []partial.PartialInfo{info})
	m.activeTab = PartialTabApply
	m.applyFocus = ApplyFocusTarget

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	_ = unwrapPartialExplorer(t, updated)

	if cmd == nil {
		t.Fatal("expected variables prompt command")
	}
}
