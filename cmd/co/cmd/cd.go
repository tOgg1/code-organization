package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/tui"
)

var cdRepoFlag bool

var cdCmd = &cobra.Command{
	Use:   "cd <workspace-slug> [repo-name]",
	Short: "Print workspace path (use with: cd $(co cd <slug>))",
	Long: `Prints the absolute path to a workspace or a repo within it.

Supports fuzzy matching - you can type partial names:
  co cd webapp      # matches acme--webapp
  co cd api         # matches acme--api-server

Use --repo or -r to change into a specific repo:
  co cd webapp --repo        # interactive repo selection
  co cd webapp --repo myrepo # direct repo selection
  co cd webapp myrepo        # shorthand for above

Usage with shell:
  cd $(co cd acme--webapp)
  cd $(co cd acme--webapp --repo)

Or add a shell function to your .zshrc/.bashrc:
  ccd() { cd "$(co cd "$@")" }

Then use:
  ccd webapp
  ccd webapp -r`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Resolve workspace path
		var workspacePath string
		var slug string

		if fs.WorkspaceExists(cfg.CodeRoot, query) {
			slug = query
			workspacePath = cfg.WorkspacePath(query)
		} else {
			workspaces, err := fs.ListWorkspaces(cfg.CodeRoot)
			if err != nil {
				return fmt.Errorf("failed to list workspaces: %w", err)
			}

			matches := fuzzy.Find(query, workspaces)
			if len(matches) == 0 {
				return fmt.Errorf("no workspace found matching: %s", query)
			}

			best := matches[0]
			if best.Score < -10 {
				return fmt.Errorf("no workspace found matching: %s", query)
			}

			if len(matches) > 1 && matches[0].Score == matches[1].Score {
				fmt.Fprintf(os.Stderr, "Ambiguous match, using: %s\n", best.Str)
			}

			slug = best.Str
			workspacePath = cfg.WorkspacePath(slug)
		}

		// Check if we're targeting a repo
		repoName := ""
		if len(args) > 1 {
			repoName = args[1]
		}

		// If --repo flag is set or repo name provided, handle repo selection
		if cdRepoFlag || repoName != "" {
			repos, err := fs.ListRepos(workspacePath)
			if err != nil {
				return fmt.Errorf("failed to list repos: %w", err)
			}

			if len(repos) == 0 {
				return fmt.Errorf("no repositories found in workspace: %s", slug)
			}

			// If repo name is provided, use fuzzy matching
			if repoName != "" {
				matches := fuzzy.Find(repoName, repos)
				if len(matches) == 0 {
					return fmt.Errorf("no repo found matching: %s", repoName)
				}

				best := matches[0]
				if best.Score < -10 {
					return fmt.Errorf("no repo found matching: %s", repoName)
				}

				if len(matches) > 1 && matches[0].Score == matches[1].Score {
					fmt.Fprintf(os.Stderr, "Ambiguous match, using: %s\n", best.Str)
				}

				repoPath := filepath.Join(workspacePath, "repos", best.Str)
				fmt.Println(repoPath)
				return nil
			}

			// Interactive repo selection
			result, err := tui.RunRepoSelect(repos, workspacePath)
			if err != nil {
				return fmt.Errorf("repo selection failed: %w", err)
			}

			if result.Abort {
				return fmt.Errorf("selection cancelled")
			}

			fmt.Println(result.Path)
			return nil
		}

		fmt.Println(workspacePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
	cdCmd.Flags().BoolVarP(&cdRepoFlag, "repo", "r", false, "Change into a repo within the workspace (interactive if no repo name given)")
}
