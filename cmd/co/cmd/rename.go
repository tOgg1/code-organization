package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
	"github.com/tormodhaugland/co/internal/tui"
	"github.com/tormodhaugland/co/internal/workspace"
)

var renameCmd = &cobra.Command{
	Use:   "rename [current-slug] [new-owner] [new-project]",
	Short: "Rename a workspace",
	Long: `Rename a workspace by changing its owner and/or project name.

This command:
1. Renames the workspace folder
2. Updates project.json with the new slug, owner, and name
3. Re-indexes the workspace

Examples:
  # Interactive mode - select workspace and enter new name
  co rename

  # Rename with positional arguments
  co rename old-owner--old-project new-owner new-project

  # Just change the project name (keep owner)
  co rename myowner--oldname myowner newname`,
	Args: cobra.MaximumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var currentSlug, newOwner, newProject string

		if len(args) == 0 {
			// Interactive mode
			result, err := tui.RunRenamePrompt(cfg)
			if err != nil {
				return err
			}
			if result.Cancelled {
				fmt.Println("Rename cancelled")
				return nil
			}
			currentSlug = result.CurrentSlug
			newOwner = result.NewOwner
			newProject = result.NewProject
		} else if len(args) == 3 {
			currentSlug = args[0]
			newOwner = args[1]
			newProject = args[2]
		} else {
			return fmt.Errorf("requires 0 arguments (interactive) or 3 arguments: current-slug new-owner new-project")
		}

		// Validate inputs
		if currentSlug == "" || newOwner == "" || newProject == "" {
			return fmt.Errorf("current slug, new owner, and new project are required")
		}

		// Perform the rename
		result, err := workspace.RenameWorkspace(cfg, currentSlug, newOwner, newProject)
		if err != nil {
			return err
		}

		fmt.Printf("Renamed: %s -> %s\n", result.OldSlug, result.NewSlug)
		fmt.Printf("Path: %s\n", result.NewPath)

		// Re-index
		idx, err := model.LoadIndex(cfg.IndexPath())
		if err != nil {
			// Index might not exist, create new one
			idx = model.NewIndex()
		}

		// Remove old entry
		idx.Remove(result.OldSlug)

		// Add new entry by scanning
		record, err := scanWorkspace(result.NewPath, result.NewSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan renamed workspace: %v\n", err)
		} else {
			idx.Add(record)
		}

		if err := idx.Save(cfg.IndexPath()); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update index: %v\n", err)
		}

		return nil
	},
}

// scanWorkspace scans a single workspace and returns an index record.
func scanWorkspace(workspacePath, slug string) (*model.IndexRecord, error) {
	parts := strings.SplitN(slug, "--", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid slug format: %s", slug)
	}

	record := model.NewIndexRecord(slug, workspacePath)
	record.Owner = parts[0]

	// Load project.json if it exists
	projectPath := filepath.Join(workspacePath, "project.json")
	if proj, err := model.LoadProject(projectPath); err == nil {
		record.State = proj.State
		record.Tags = proj.Tags

		// Scan repos
		for _, repo := range proj.Repos {
			repoPath := filepath.Join(workspacePath, repo.Path)
			info, err := os.Stat(repoPath)
			if err != nil {
				continue
			}
			if !info.IsDir() {
				continue
			}

			repoRecord := model.IndexRepoInfo{
				Name:   repo.Name,
				Path:   repo.Path,
				Remote: repo.Remote,
			}

			// Get git info
			// Note: simplified scan, full scan is in index.go
			record.RepoCount++
			record.Repos = append(record.Repos, repoRecord)
		}
	}

	return record, nil
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
