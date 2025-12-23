package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGitRootsWithDepth(t *testing.T) {
	tmp := t.TempDir()

	// Create directory structure:
	// tmp/
	//   repo1/.git/          (depth 1)
	//   dir1/
	//     repo2/.git/        (depth 2)
	//     dir2/
	//       repo3/.git/      (depth 3)
	//       dir3/
	//         repo4/.git/    (depth 4)
	//         dir4/
	//           repo5/.git/  (depth 5)

	dirs := []string{
		"repo1/.git",
		"dir1/repo2/.git",
		"dir1/dir2/repo3/.git",
		"dir1/dir2/dir3/repo4/.git",
		"dir1/dir2/dir3/dir4/repo5/.git",
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	tests := []struct {
		name     string
		maxDepth int
		expected int
	}{
		{"depth 0 (root only)", 0, 0},
		{"depth 1", 1, 1},
		{"depth 2", 2, 2},
		{"depth 3", 3, 3},
		{"depth 4", 4, 4},
		{"depth 5", 5, 5},
		{"unlimited", -1, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roots, err := FindGitRootsWithDepth(tmp, tt.maxDepth)
			if err != nil {
				t.Fatalf("FindGitRootsWithDepth: %v", err)
			}
			if len(roots) != tt.expected {
				t.Errorf("expected %d roots, got %d: %v", tt.expected, len(roots), roots)
			}
		})
	}
}

func TestFindGitRootsSkipsDirs(t *testing.T) {
	tmp := t.TempDir()

	// Create a repo inside node_modules (should be skipped)
	nodeModulesRepo := filepath.Join(tmp, "node_modules", "some-pkg", ".git")
	if err := os.MkdirAll(nodeModulesRepo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a normal repo (should be found)
	normalRepo := filepath.Join(tmp, "myrepo", ".git")
	if err := os.MkdirAll(normalRepo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	roots, err := FindGitRootsWithDepth(tmp, -1)
	if err != nil {
		t.Fatalf("FindGitRootsWithDepth: %v", err)
	}

	if len(roots) != 1 {
		t.Errorf("expected 1 root (node_modules should be skipped), got %d: %v", len(roots), roots)
	}

	if len(roots) > 0 && filepath.Base(roots[0]) != "myrepo" {
		t.Errorf("expected 'myrepo', got %q", filepath.Base(roots[0]))
	}
}

func TestSkipDirsCompleteness(t *testing.T) {
	// Ensure common problematic directories are in the skip list
	mustSkip := []string{
		"node_modules",
		"vendor",
		".venv",
		"venv",
		"target",
		"build",
		"dist",
		".next",
		".nuxt",
		"__pycache__",
		".cache",
	}

	for _, dir := range mustSkip {
		if !skipDirs[dir] {
			t.Errorf("skipDirs should contain %q", dir)
		}
	}
}
