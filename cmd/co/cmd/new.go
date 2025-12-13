package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
	"github.com/tormodhaugland/co/internal/tui"
)

var newCmd = &cobra.Command{
	Use:   "new [owner] [project] [repo-url...]",
	Short: "Create a new workspace",
	Long: `Creates a new workspace with project.json and repos/ directory.
If repo URLs are provided, clones them into repos/<derived-name>/.
If owner and project are not provided, prompts interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var owner, project string
		var repoURLs []string

		if len(args) >= 2 {
			owner = strings.ToLower(args[0])
			project = strings.ToLower(args[1])
			repoURLs = args[2:]
		} else {
			result, err := tui.RunNewPrompt()
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if result.Abort {
				return fmt.Errorf("aborted")
			}
			owner = result.Owner
			project = result.Project
		}

		slug := owner + "--" + project
		if !fs.IsValidWorkspaceSlug(slug) {
			return fmt.Errorf("invalid workspace slug: %s (must be lowercase alphanumeric with hyphens)", slug)
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if fs.WorkspaceExists(cfg.CodeRoot, slug) {
			return fmt.Errorf("workspace already exists: %s", slug)
		}

		workspacePath, err := fs.CreateWorkspace(cfg.CodeRoot, slug)
		if err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		proj := model.NewProject(owner, project)

		for _, url := range repoURLs {
			repoName := deriveRepoName(url)
			repoPath := filepath.Join(workspacePath, "repos", repoName)

			fmt.Printf("Cloning %s into repos/%s...\n", url, repoName)
			if err := git.Clone(url, repoPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clone %s: %v\n", url, err)
				continue
			}

			proj.AddRepo(repoName, "repos/"+repoName, url)
		}

		if err := proj.Save(workspacePath); err != nil {
			return fmt.Errorf("failed to save project.json: %w", err)
		}

		fmt.Printf("Created workspace: %s\n", workspacePath)
		return nil
	},
}

func deriveRepoName(url string) string {
	url = strings.TrimSuffix(url, ".git")

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	parts = strings.Split(url, ":")
	if len(parts) > 1 {
		subparts := strings.Split(parts[1], "/")
		if len(subparts) > 0 {
			return subparts[len(subparts)-1]
		}
	}

	return "repo"
}

func init() {
	rootCmd.AddCommand(newCmd)
}
