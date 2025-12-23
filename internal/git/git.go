package git

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RepoInfo struct {
	Path       string
	Head       string
	Branch     string
	Dirty      bool
	Remote     string
	LastCommit time.Time
}

func IsRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return false
	}
	_ = gitDir
	return true
}

func GetInfo(repoPath string) (*RepoInfo, error) {
	info := &RepoInfo{Path: repoPath}

	head, err := getHead(repoPath)
	if err != nil {
		return nil, err
	}
	info.Head = head

	branch, err := getBranch(repoPath)
	if err == nil {
		info.Branch = branch
	}

	info.Dirty = isDirty(repoPath)

	remote, err := getRemote(repoPath)
	if err == nil {
		info.Remote = remote
	}

	lastCommit, err := getLastCommitTime(repoPath)
	if err == nil {
		info.LastCommit = lastCommit
	}

	return info, nil
}

func getHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func isDirty(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func getRemote(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getLastCommitTime(repoPath string) (time.Time, error) {
	cmd := exec.Command("git", "-C", repoPath, "log", "-1", "--format=%cI")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
}

func CreateBundle(repoPath, bundlePath string) error {
	cmd := exec.Command("git", "-C", repoPath, "bundle", "create", bundlePath, "--all")
	return cmd.Run()
}

func Clone(url, destPath string) error {
	cmd := exec.Command("git", "clone", url, destPath)
	return cmd.Run()
}

// skipDirs contains directory names that should be skipped during git root scanning.
// These are typically large generated/dependency directories that slow down scanning.
var skipDirs = map[string]bool{
	// Package managers / dependencies
	"node_modules":     true,
	"vendor":           true,
	"bower_components": true,
	".pnpm-store":      true,
	"jspm_packages":    true,

	// Python
	".venv":         true,
	"venv":          true,
	".virtualenv":   true,
	"__pycache__":   true,
	".tox":          true,
	".nox":          true,
	".mypy_cache":   true,
	".pytest_cache": true,
	"site-packages": true,

	// Build outputs
	"target": true, // Rust, Java/Maven
	"build":  true,
	"dist":   true,
	"out":    true,
	"bin":    true,
	"obj":    true,
	"_build": true, // Elixir
	"deps":   true, // Elixir

	// Framework caches
	".next":         true,
	".nuxt":         true,
	".output":       true,
	".svelte-kit":   true,
	".vercel":       true,
	".netlify":      true,
	".turbo":        true,
	".cache":        true,
	".parcel-cache": true,

	// IDE / tools
	".idea":     true,
	".vscode":   true,
	".settings": true,

	// Other
	".terraform":  true,
	"coverage":    true,
	".nyc_output": true,
	"htmlcov":     true,
}

// FindGitRoots finds all git repositories under basePath with no depth limit.
// Consider using FindGitRootsWithDepth for better performance on large trees.
func FindGitRoots(basePath string) ([]string, error) {
	return FindGitRootsWithDepth(basePath, -1) // -1 means no limit
}

// FindGitRootsWithDepth finds all git repositories under basePath up to maxDepth levels deep.
// A maxDepth of 0 only checks basePath itself, 1 checks immediate children, etc.
// A maxDepth of -1 means no limit (scans entire tree).
func FindGitRootsWithDepth(basePath string, maxDepth int) ([]string, error) {
	var roots []string
	seen := make(map[string]bool)
	baseDepth := strings.Count(basePath, string(filepath.Separator))

	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Check for .git first (before depth limit) since we want to find repos
		// at the depth limit, and .git is one level deeper than the repo root
		if name == ".git" {
			repoRoot := filepath.Dir(path)
			if !seen[repoRoot] {
				seen[repoRoot] = true
				roots = append(roots, repoRoot)
			}
			return filepath.SkipDir
		}

		// Skip known large/generated directories
		if skipDirs[name] {
			return filepath.SkipDir
		}

		// Check depth limit (after .git check so we can find repos at maxDepth)
		if maxDepth >= 0 {
			currentDepth := strings.Count(path, string(filepath.Separator)) - baseDepth
			if currentDepth > maxDepth {
				return filepath.SkipDir
			}
		}

		return nil
	})

	return roots, err
}
