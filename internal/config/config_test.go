package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Schema != CurrentConfigSchema {
		t.Errorf("Schema = %d, want %d", cfg.Schema, CurrentConfigSchema)
	}

	home, _ := os.UserHomeDir()
	expectedRoot := filepath.Join(home, "Code")
	if cfg.CodeRoot != expectedRoot {
		t.Errorf("CodeRoot = %q, want %q", cfg.CodeRoot, expectedRoot)
	}

	if cfg.Servers == nil {
		t.Error("Servers should not be nil")
	}
}

func TestConfigSystemDir(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	if cfg.SystemDir() != "/home/user/Code/_system" {
		t.Errorf("SystemDir() = %q, want %q", cfg.SystemDir(), "/home/user/Code/_system")
	}
}

func TestConfigIndexPath(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	expected := "/home/user/Code/_system/index.jsonl"
	if cfg.IndexPath() != expected {
		t.Errorf("IndexPath() = %q, want %q", cfg.IndexPath(), expected)
	}
}

func TestConfigArchiveDir(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	expected := "/home/user/Code/_system/archive"
	if cfg.ArchiveDir() != expected {
		t.Errorf("ArchiveDir() = %q, want %q", cfg.ArchiveDir(), expected)
	}
}

func TestConfigLogsDir(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	expected := "/home/user/Code/_system/logs"
	if cfg.LogsDir() != expected {
		t.Errorf("LogsDir() = %q, want %q", cfg.LogsDir(), expected)
	}
}

func TestConfigCacheDir(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	expected := "/home/user/Code/_system/cache"
	if cfg.CacheDir() != expected {
		t.Errorf("CacheDir() = %q, want %q", cfg.CacheDir(), expected)
	}
}

func TestConfigPartialsDir(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	expected := "/home/user/Code/_system/partials"
	if cfg.PartialsDir() != expected {
		t.Errorf("PartialsDir() = %q, want %q", cfg.PartialsDir(), expected)
	}
}

func TestConfigFallbackPartialsDirWithXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	cfg := &Config{}
	expected := filepath.Join("/tmp/xdg-config", "co", "partials")
	if cfg.FallbackPartialsDir() != expected {
		t.Errorf("FallbackPartialsDir() = %q, want %q", cfg.FallbackPartialsDir(), expected)
	}
}

func TestConfigFallbackPartialsDirWithoutXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	cfg := &Config{}
	expected := filepath.Join(home, ".config", "co", "partials")
	if cfg.FallbackPartialsDir() != expected {
		t.Errorf("FallbackPartialsDir() = %q, want %q", cfg.FallbackPartialsDir(), expected)
	}
}

func TestConfigAllPartialsDirs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	cfg := &Config{CodeRoot: "/home/user/Code"}
	expected := []string{
		"/home/user/Code/_system/partials",
		filepath.Join("/tmp/xdg-config", "co", "partials"),
	}
	got := cfg.AllPartialsDirs()
	if len(got) != len(expected) {
		t.Fatalf("AllPartialsDirs() length = %d, want %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("AllPartialsDirs()[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestConfigWorkspacePath(t *testing.T) {
	cfg := &Config{CodeRoot: "/home/user/Code"}
	path := cfg.WorkspacePath("owner--project")
	expected := "/home/user/Code/owner--project"
	if path != expected {
		t.Errorf("WorkspacePath() = %q, want %q", path, expected)
	}
}

func TestConfigGetServer(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"prod": {SSH: "user@prod.example.com", CodeRoot: "/data/code"},
		},
	}

	prod := cfg.GetServer("prod")
	if prod.SSH != "user@prod.example.com" {
		t.Errorf("SSH = %q, want %q", prod.SSH, "user@prod.example.com")
	}
	if prod.CodeRoot != "/data/code" {
		t.Errorf("CodeRoot = %q, want %q", prod.CodeRoot, "/data/code")
	}

	unknown := cfg.GetServer("unknown-server")
	if unknown.SSH != "unknown-server" {
		t.Errorf("SSH = %q, want %q", unknown.SSH, "unknown-server")
	}
	if unknown.CodeRoot != "~/Code" {
		t.Errorf("CodeRoot = %q, want %q", unknown.CodeRoot, "~/Code")
	}
}

func TestConfigExpandPaths(t *testing.T) {
	home, _ := os.UserHomeDir()

	cfg := &Config{
		CodeRoot: "~/Projects",
		Servers:  map[string]ServerConfig{},
	}
	cfg.expandPaths()

	expected := filepath.Join(home, "Projects")
	if cfg.CodeRoot != expected {
		t.Errorf("CodeRoot = %q, want %q", cfg.CodeRoot, expected)
	}
}

func TestConfigExpandPathsNoTilde(t *testing.T) {
	cfg := &Config{
		CodeRoot: "/absolute/path",
		Servers:  map[string]ServerConfig{},
	}
	cfg.expandPaths()

	if cfg.CodeRoot != "/absolute/path" {
		t.Errorf("CodeRoot = %q, want %q", cfg.CodeRoot, "/absolute/path")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"schema": 1,
		"code_root": "/custom/code",
		"editor": "nvim",
		"servers": {
			"remote": {"ssh": "user@host", "code_root": "/remote/code"}
		}
	}`
	os.WriteFile(configPath, []byte(configJSON), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.CodeRoot != "/custom/code" {
		t.Errorf("CodeRoot = %q, want %q", cfg.CodeRoot, "/custom/code")
	}
	if cfg.Editor != "nvim" {
		t.Errorf("Editor = %q, want %q", cfg.Editor, "nvim")
	}
	if len(cfg.Servers) != 1 {
		t.Errorf("len(Servers) = %d, want 1", len(cfg.Servers))
	}
}

func TestLoadConfigWithTildePath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{"schema": 1, "code_root": "~/MyCode"}`
	os.WriteFile(configPath, []byte(configJSON), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "MyCode")
	if cfg.CodeRoot != expected {
		t.Errorf("CodeRoot = %q, want %q", cfg.CodeRoot, expected)
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Load should not error for missing file: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "Code")
	if cfg.CodeRoot != expected {
		t.Errorf("CodeRoot = %q, want %q", cfg.CodeRoot, expected)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte("not json"), 0644)

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetConfigPaths(t *testing.T) {
	paths := getConfigPaths("/explicit/config.json")

	if len(paths) < 2 {
		t.Fatalf("expected at least 2 paths, got %d", len(paths))
	}

	if paths[0] != "/explicit/config.json" {
		t.Errorf("paths[0] = %q, want explicit path", paths[0])
	}
}

func TestGetConfigPathsNoExplicit(t *testing.T) {
	paths := getConfigPaths("")

	if len(paths) < 2 {
		t.Fatalf("expected at least 2 paths, got %d", len(paths))
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			t.Errorf("path %q should be absolute", p)
		}
	}
}
