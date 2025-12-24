package fs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var workspacePattern = regexp.MustCompile(`^[a-z0-9-]+--[a-z0-9-]+(--(poc|demo|legacy|migration|infra))?$`)
var tmpWorkspacePattern = regexp.MustCompile(`^tmp--[a-z0-9-]+$`)

func IsValidWorkspaceSlug(name string) bool {
	return workspacePattern.MatchString(name)
}

// IsTmpSlug returns true if the name matches the tmp workspace pattern (tmp--name)
func IsTmpSlug(name string) bool {
	return tmpWorkspacePattern.MatchString(name)
}

func ListWorkspaces(codeRoot string) ([]string, error) {
	entries, err := os.ReadDir(codeRoot)
	if err != nil {
		return nil, err
	}

	var workspaces []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "_system" {
			continue
		}
		if IsValidWorkspaceSlug(name) {
			workspaces = append(workspaces, name)
		}
	}

	return workspaces, nil
}

// ListTmpWorkspaces returns all tmp--* directories in the code root
func ListTmpWorkspaces(codeRoot string) ([]string, error) {
	entries, err := os.ReadDir(codeRoot)
	if err != nil {
		return nil, err
	}

	var workspaces []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if IsTmpSlug(name) {
			workspaces = append(workspaces, name)
		}
	}

	return workspaces, nil
}

func WorkspaceExists(codeRoot, slug string) bool {
	path := filepath.Join(codeRoot, slug)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func HasProjectJSON(workspacePath string) bool {
	projectPath := filepath.Join(workspacePath, "project.json")
	_, err := os.Stat(projectPath)
	return err == nil
}

func HasReposDir(workspacePath string) bool {
	reposPath := filepath.Join(workspacePath, "repos")
	info, err := os.Stat(reposPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func ListRepos(workspacePath string) ([]string, error) {
	reposPath := filepath.Join(workspacePath, "repos")
	entries, err := os.ReadDir(reposPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var repos []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			repos = append(repos, entry.Name())
		}
	}

	return repos, nil
}

func CalculateSize(path string) (int64, error) {
	var size int64

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() && shouldExcludeDir(d.Name()) {
			return filepath.SkipDir
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}

		return nil
	})

	return size, err
}

func shouldExcludeDir(name string) bool {
	for _, exclude := range BuiltinExcludes {
		exclude = strings.TrimSuffix(exclude, "/")
		if name == exclude {
			return true
		}
	}
	return false
}

func GetLastModTime(path string) (int64, error) {
	var latest int64

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() && shouldExcludeDir(d.Name()) {
			return filepath.SkipDir
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				modTime := info.ModTime().Unix()
				if modTime > latest {
					latest = modTime
				}
			}
		}

		return nil
	})

	return latest, err
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func CreateWorkspace(codeRoot, slug string) (string, error) {
	workspacePath := filepath.Join(codeRoot, slug)

	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return "", err
	}

	reposPath := filepath.Join(workspacePath, "repos")
	if err := os.MkdirAll(reposPath, 0755); err != nil {
		return "", err
	}

	return workspacePath, nil
}

func DefaultExcludes() []string {
	return append([]string{}, BuiltinExcludes...)
}
