package fs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildExcludeListDefaults(t *testing.T) {
	list := BuildExcludeList(ExcludeOptions{})

	// Should contain key patterns
	hasNodeModules := false
	hasEnv := false
	for _, p := range list.Patterns {
		if p == "node_modules/" {
			hasNodeModules = true
		}
		if p == ".env" {
			hasEnv = true
		}
	}

	if !hasNodeModules {
		t.Error("Expected node_modules/ in default excludes")
	}
	if !hasEnv {
		t.Error("Expected .env in default excludes")
	}
}

func TestBuildExcludeListNoGit(t *testing.T) {
	list := BuildExcludeList(ExcludeOptions{NoGit: true})

	hasGit := false
	for _, p := range list.Patterns {
		if p == ".git/" {
			hasGit = true
		}
	}

	if !hasGit {
		t.Error("Expected .git/ in excludes when NoGit is true")
	}
}

func TestBuildExcludeListIncludeEnv(t *testing.T) {
	list := BuildExcludeList(ExcludeOptions{IncludeEnv: true})

	for _, p := range list.Patterns {
		if p == ".env" || p == ".env.*" {
			t.Errorf("Expected .env patterns to be excluded when IncludeEnv is true, found: %s", p)
		}
	}
}

func TestBuildExcludeListAdditional(t *testing.T) {
	list := BuildExcludeList(ExcludeOptions{
		Additional: []string{"custom1/", "custom2/"},
	})

	hasCustom1 := false
	hasCustom2 := false
	for _, p := range list.Patterns {
		if p == "custom1/" {
			hasCustom1 = true
		}
		if p == "custom2/" {
			hasCustom2 = true
		}
	}

	if !hasCustom1 {
		t.Error("Expected custom1/ in excludes")
	}
	if !hasCustom2 {
		t.Error("Expected custom2/ in excludes")
	}
}

func TestBuildExcludeListRemove(t *testing.T) {
	list := BuildExcludeList(ExcludeOptions{
		Remove: []string{"node_modules/", "vendor/"},
	})

	for _, p := range list.Patterns {
		if p == "node_modules/" {
			t.Error("Expected node_modules/ to be removed from excludes")
		}
		if p == "vendor/" {
			t.Error("Expected vendor/ to be removed from excludes")
		}
	}
}

func TestBuildExcludeListDeduplication(t *testing.T) {
	list := BuildExcludeList(ExcludeOptions{
		Additional: []string{"node_modules/", "node_modules/", "custom/"},
	})

	count := 0
	for _, p := range list.Patterns {
		if p == "node_modules/" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected node_modules/ to appear exactly once, found %d times", count)
	}
}

func TestParseExcludeFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "excludes.txt")
	content := "# comment line\nnode_modules/\n \n# another\n*.log\n dist/\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ParseExcludeFile(path)
	if err != nil {
		t.Fatalf("ParseExcludeFile returned error: %v", err)
	}

	want := []string{"node_modules/", "*.log", "dist/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseExcludeFile() = %v, want %v", got, want)
	}
}

func TestParseExcludeFileNotFound(t *testing.T) {
	_, err := ParseExcludeFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestExcludeListToRsyncArgs(t *testing.T) {
	list := &ExcludeList{
		Patterns: []string{"node_modules/", "*.log", ".env"},
	}

	got := list.ToRsyncArgs()
	want := []string{"--exclude=node_modules/", "--exclude=*.log", "--exclude=.env"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToRsyncArgs() = %v, want %v", got, want)
	}
}

func TestExcludeListToTarArgs(t *testing.T) {
	list := &ExcludeList{
		Patterns: []string{"node_modules/", "*.log", ".env"},
	}

	got := list.ToTarArgs()
	want := []string{"--exclude=*/node_modules/*", "--exclude=*.log", "--exclude=.env"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToTarArgs() = %v, want %v", got, want)
	}
}

func TestTarExcludePattern(t *testing.T) {
	// Test at least 10 representative patterns for rsync/tar parity
	// per acceptance criteria for code-organization-408.7
	cases := []struct {
		in   string
		want string
		desc string
	}{
		// Directory patterns (trailing /) - converted for tar
		{"node_modules/", "*/node_modules/*", "directory: common deps"},
		{"build/", "*/build/*", "directory: build output"},
		{".git/", "*/.git/*", "directory: dotfile dir"},
		{"target/", "*/target/*", "directory: Rust build"},
		{"__pycache__/", "*/__pycache__/*", "directory: Python cache"},
		{"coverage/", "*/coverage/*", "directory: test coverage"},

		// File patterns - passed as-is
		{"*.log", "*.log", "glob: log files"},
		{".env", ".env", "exact: dotfile"},
		{".env.*", ".env.*", "glob: env variants"},
		{"*.pyc", "*.pyc", "glob: Python compiled"},

		// Path patterns with / but not trailing - passed as-is
		{"data/*.csv", "data/*.csv", "path glob: CSV in data"},
		// Note: nested paths with trailing / still get converted - this matches
		// the current behavior (any trailing / triggers conversion)
		{"repos/*/node_modules/", "*/repos/*/node_modules/*", "nested path with trailing /"},

		// Edge cases
		{"Thumbs.db", "Thumbs.db", "exact: Windows artifact"},
		{".DS_Store", ".DS_Store", "exact: macOS artifact"},
		{"*.tfstate", "*.tfstate", "glob: Terraform state"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if got := tarExcludePattern(tc.in); got != tc.want {
				t.Errorf("tarExcludePattern(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
