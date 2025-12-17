package sync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tormodhaugland/co/internal/fs"
)

func TestBuildExcludes(t *testing.T) {
	opts := &Options{
		NoGit:           true,
		IncludeEnv:      false,
		ExcludePatterns: []string{"custom/"},
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	// Should contain .git/ since NoGit is true
	hasGit := false
	hasCustom := false
	for _, p := range excludeList.Patterns {
		if p == ".git/" {
			hasGit = true
		}
		if p == "custom/" {
			hasCustom = true
		}
	}

	if !hasGit {
		t.Error("Expected .git/ in excludes when NoGit is true")
	}
	if !hasCustom {
		t.Error("Expected custom/ in excludes")
	}
}

func TestBuildExcludesIncludeEnv(t *testing.T) {
	opts := &Options{
		IncludeEnv: true,
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	// Should NOT contain .env patterns when IncludeEnv is true
	for _, p := range excludeList.Patterns {
		if p == ".env" || p == ".env.*" {
			t.Errorf("Expected .env patterns to be excluded when IncludeEnv is true, found: %s", p)
		}
	}
}

func TestBuildExcludesFromFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "excludes.txt")
	content := "# comment line\ncustom1/\n \n# another\ncustom2/\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	opts := &Options{
		ExcludeFromFile: path,
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	hasCustom1 := false
	hasCustom2 := false
	for _, p := range excludeList.Patterns {
		if p == "custom1/" {
			hasCustom1 = true
		}
		if p == "custom2/" {
			hasCustom2 = true
		}
	}

	if !hasCustom1 {
		t.Error("Expected custom1/ in excludes from file")
	}
	if !hasCustom2 {
		t.Error("Expected custom2/ in excludes from file")
	}
}

func TestParseExcludeFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "excludes.txt")
	content := "# comment line\nnode_modules/\n \n# another\n*.log\n dist/\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := fs.ParseExcludeFile(path)
	if err != nil {
		t.Fatalf("ParseExcludeFile returned error: %v", err)
	}

	want := []string{"node_modules/", "*.log", "dist/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseExcludeFile() = %v, want %v", got, want)
	}
}

func TestExcludeListToRsyncArgs(t *testing.T) {
	list := &fs.ExcludeList{
		Patterns: []string{"node_modules/", "*.log"},
	}

	got := list.ToRsyncArgs()
	want := []string{"--exclude=node_modules/", "--exclude=*.log"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToRsyncArgs() = %v, want %v", got, want)
	}
}

func TestExcludeListToTarArgs(t *testing.T) {
	list := &fs.ExcludeList{
		Patterns: []string{"node_modules/", "*.log"},
	}

	got := list.ToTarArgs()
	want := []string{"--exclude=*/node_modules/*", "--exclude=*.log"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToTarArgs() = %v, want %v", got, want)
	}
}
