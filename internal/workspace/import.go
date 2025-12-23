// Package workspace provides functions for creating and managing workspaces.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
)

// ImportOptions configures an import operation.
type ImportOptions struct {
	Owner   string // Workspace owner
	Project string // Project name

	// Extra files to include (paths relative to source)
	ExtraFiles     []string
	ExtraFilesDest string // Destination subfolder for extra files (empty = project root)

	// Callbacks for progress reporting (all optional)
	OnRepoMove func(repoName, srcPath, dstPath string)
	OnRepoSkip func(repoName, reason string)
	OnFileCopy func(relPath, dstPath string)
	OnWarning  func(msg string)
}

// ImportResult holds the result of an import operation.
type ImportResult struct {
	WorkspacePath string   // Full path to created/updated workspace
	WorkspaceSlug string   // Workspace slug (owner--project)
	ReposImported []string // Names of repos imported
	ReposSkipped  []string // Names of repos skipped (already exist, etc.)
	FilesCopied   []string // Paths of extra files copied
	SourceEmpty   bool     // True if source directory is now empty
	Errors        []string // Non-fatal errors encountered
}

// CreateWorkspace creates a new workspace from a source folder.
// It moves git repositories into the workspace and optionally copies extra files.
func CreateWorkspace(cfg *config.Config, sourcePath string, gitRoots []string, opts ImportOptions) (*ImportResult, error) {
	if opts.Owner == "" || opts.Project == "" {
		return nil, fmt.Errorf("owner and project are required")
	}

	slug := opts.Owner + "--" + opts.Project
	if !fs.IsValidWorkspaceSlug(slug) {
		return nil, fmt.Errorf("invalid workspace slug: %s", slug)
	}

	if fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return nil, fmt.Errorf("workspace already exists: %s", slug)
	}

	workspacePath := filepath.Join(cfg.CodeRoot, slug)
	reposPath := filepath.Join(workspacePath, "repos")

	// Create workspace directory structure
	if err := os.MkdirAll(reposPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	result := &ImportResult{
		WorkspacePath: workspacePath,
		WorkspaceSlug: slug,
	}

	// Create project model
	proj := model.NewProject(opts.Owner, opts.Project)

	// Move git repos
	for _, root := range gitRoots {
		repoName := DeriveRepoName(root, sourcePath)
		destPath := filepath.Join(reposPath, repoName)

		if opts.OnRepoMove != nil {
			opts.OnRepoMove(repoName, root, destPath)
		}

		if err := moveDir(root, destPath); err != nil {
			errMsg := fmt.Sprintf("failed to move %s: %v", root, err)
			result.Errors = append(result.Errors, errMsg)
			if opts.OnWarning != nil {
				opts.OnWarning(errMsg)
			}
			continue
		}

		// Get remote info from moved repo
		remote := ""
		if info, err := git.GetInfo(destPath); err == nil && info.Remote != "" {
			remote = info.Remote
		}
		proj.AddRepo(repoName, "repos/"+repoName, remote)
		result.ReposImported = append(result.ReposImported, repoName)
	}

	// Save project.json
	if err := proj.Save(workspacePath); err != nil {
		return nil, fmt.Errorf("failed to save project.json: %w", err)
	}

	// Copy extra files
	if len(opts.ExtraFiles) > 0 {
		copied, errs := CopyExtraFiles(sourcePath, workspacePath, opts.ExtraFiles, opts.ExtraFilesDest, opts.OnFileCopy)
		result.FilesCopied = copied
		result.Errors = append(result.Errors, errs...)
	}

	// Check if source is now empty
	result.SourceEmpty, _ = isDirEmpty(sourcePath)

	return result, nil
}

// AddToWorkspace adds repositories and files to an existing workspace.
func AddToWorkspace(cfg *config.Config, sourcePath string, gitRoots []string, slug string, opts ImportOptions) (*ImportResult, error) {
	if !fs.IsValidWorkspaceSlug(slug) {
		return nil, fmt.Errorf("invalid workspace slug: %s", slug)
	}

	if !fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return nil, fmt.Errorf("workspace does not exist: %s", slug)
	}

	workspacePath := filepath.Join(cfg.CodeRoot, slug)
	reposPath := filepath.Join(workspacePath, "repos")

	// Load existing project
	proj, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to load project.json: %w", err)
	}

	// Build set of existing repos
	existingRepos := make(map[string]bool)
	for _, r := range proj.Repos {
		existingRepos[r.Name] = true
	}

	result := &ImportResult{
		WorkspacePath: workspacePath,
		WorkspaceSlug: slug,
	}

	// Move git repos
	for _, root := range gitRoots {
		repoName := DeriveRepoName(root, sourcePath)
		destPath := filepath.Join(reposPath, repoName)

		if existingRepos[repoName] {
			if opts.OnRepoSkip != nil {
				opts.OnRepoSkip(repoName, "already exists")
			}
			result.ReposSkipped = append(result.ReposSkipped, repoName)
			continue
		}

		if opts.OnRepoMove != nil {
			opts.OnRepoMove(repoName, root, destPath)
		}

		if err := moveDir(root, destPath); err != nil {
			errMsg := fmt.Sprintf("failed to move %s: %v", root, err)
			result.Errors = append(result.Errors, errMsg)
			if opts.OnWarning != nil {
				opts.OnWarning(errMsg)
			}
			continue
		}

		// Get remote info from moved repo
		remote := ""
		if info, err := git.GetInfo(destPath); err == nil && info.Remote != "" {
			remote = info.Remote
		}
		proj.AddRepo(repoName, "repos/"+repoName, remote)
		result.ReposImported = append(result.ReposImported, repoName)
	}

	// Save updated project.json
	if len(result.ReposImported) > 0 {
		if err := proj.Save(workspacePath); err != nil {
			return nil, fmt.Errorf("failed to save project.json: %w", err)
		}
	}

	// Copy extra files
	if len(opts.ExtraFiles) > 0 {
		copied, errs := CopyExtraFiles(sourcePath, workspacePath, opts.ExtraFiles, opts.ExtraFilesDest, opts.OnFileCopy)
		result.FilesCopied = copied
		result.Errors = append(result.Errors, errs...)
	}

	// Check if source is now empty
	result.SourceEmpty, _ = isDirEmpty(sourcePath)

	return result, nil
}

// CopyExtraFiles copies selected files/folders from source to workspace.
// Returns the list of successfully copied paths and any errors encountered.
func CopyExtraFiles(sourcePath, workspacePath string, selectedPaths []string, destSubfolder string, onCopy func(relPath, dstPath string)) ([]string, []string) {
	var copied []string
	var errors []string

	destBase := workspacePath
	if destSubfolder != "" {
		destBase = filepath.Join(workspacePath, destSubfolder)
		if err := os.MkdirAll(destBase, 0755); err != nil {
			errors = append(errors, fmt.Sprintf("failed to create destination subfolder: %v", err))
			return copied, errors
		}
	}

	for _, relPath := range selectedPaths {
		srcPath := filepath.Join(sourcePath, relPath)
		dstPath := filepath.Join(destBase, relPath)

		info, err := os.Stat(srcPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("cannot access %s: %v", relPath, err))
			continue
		}

		if onCopy != nil {
			onCopy(relPath, dstPath)
		}

		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				errors = append(errors, fmt.Sprintf("failed to copy directory %s: %v", relPath, err))
				continue
			}
		} else {
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				errors = append(errors, fmt.Sprintf("failed to create parent dir for %s: %v", relPath, err))
				continue
			}
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				errors = append(errors, fmt.Sprintf("failed to copy file %s: %v", relPath, err))
				continue
			}
		}

		// Remove the source after successful copy
		if err := os.RemoveAll(srcPath); err != nil {
			errors = append(errors, fmt.Sprintf("failed to remove source %s: %v", relPath, err))
		}

		copied = append(copied, relPath)
	}

	return copied, errors
}

// DeriveRepoName derives a repo name from its path relative to the source folder.
func DeriveRepoName(repoPath, sourcePath string) string {
	if repoPath == sourcePath {
		return filepath.Base(sourcePath)
	}

	rel, err := filepath.Rel(sourcePath, repoPath)
	if err != nil {
		return filepath.Base(repoPath)
	}

	name := strings.ReplaceAll(rel, string(filepath.Separator), "-")
	return SanitizeSlugPart(name)
}

// SanitizeSlugPart cleans a string for use in a workspace slug.
func SanitizeSlugPart(s string) string {
	s = strings.ToLower(s)
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		} else if c == '_' || c == ' ' {
			result.WriteRune('-')
		}
	}
	return result.String()
}

// RemoveEmptySource removes the source directory if it's empty.
// Returns true if the directory was removed.
func RemoveEmptySource(sourcePath string) bool {
	empty, err := isDirEmpty(sourcePath)
	if err != nil || !empty {
		return false
	}
	if err := os.Remove(sourcePath); err != nil {
		return false
	}
	return true
}

// moveDir moves a directory, falling back to copy+delete for cross-device moves.
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		if isCrossDevice(err) {
			if err := copyDir(src, dst); err != nil {
				return err
			}
			return os.RemoveAll(src)
		}
		return err
	}
	return nil
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func isCrossDevice(err error) bool {
	return strings.Contains(err.Error(), "cross-device") ||
		strings.Contains(err.Error(), "invalid cross-device link")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}
