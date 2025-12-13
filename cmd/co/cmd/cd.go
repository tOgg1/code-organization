package cmd

import (
	"fmt"
	"os"

	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
)

var cdCmd = &cobra.Command{
	Use:   "cd <workspace-slug>",
	Short: "Print workspace path (use with: cd $(co cd <slug>))",
	Long: `Prints the absolute path to a workspace.

Supports fuzzy matching - you can type partial names:
  co cd webapp      # matches acme--webapp
  co cd api         # matches acme--api-server

Usage with shell:
  cd $(co cd acme--webapp)

Or add a shell function to your .zshrc/.bashrc:
  ccd() { cd "$(co cd "$1")" }

Then use:
  ccd webapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if fs.WorkspaceExists(cfg.CodeRoot, query) {
			fmt.Println(cfg.WorkspacePath(query))
			return nil
		}

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

		fmt.Println(cfg.WorkspacePath(best.Str))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
}
