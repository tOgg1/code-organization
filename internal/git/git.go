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

func FindGitRoots(basePath string) ([]string, error) {
	var roots []string
	seen := make(map[string]bool)

	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == "node_modules" || name == ".venv" || name == "target" || name == "vendor" {
			return filepath.SkipDir
		}

		if name == ".git" {
			repoRoot := filepath.Dir(path)
			if !seen[repoRoot] {
				seen[repoRoot] = true
				roots = append(roots, repoRoot)
			}
			return filepath.SkipDir
		}

		return nil
	})

	return roots, err
}
