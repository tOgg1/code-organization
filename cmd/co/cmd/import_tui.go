package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/tui"
)

var importTUICmd = &cobra.Command{
	Use:   "import-tui [path]",
	Short: "Interactive import browser for organizing folders into workspaces",
	Long: `Launch an interactive TUI for browsing folders and importing them as workspaces.

The import browser shows a tree view of the selected folder (or current directory),
highlighting git repositories. You can:

  - Navigate the folder tree with vim-style keys (j/k/h/l)
  - Import folders as new workspaces
  - Add repos to existing workspaces
  - Stash (archive) folders for later

If no path is provided, the current directory is used.

Examples:
  co import-tui                    # Browse current directory
  co import-tui ~/projects         # Browse ~/projects
  co import-tui ./legacy-code      # Browse ./legacy-code`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine the root path
		var rootPath string
		if len(args) > 0 {
			rootPath = args[0]
		} else {
			var err error
			rootPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Resolve to absolute path
		rootPath, err := filepath.Abs(rootPath)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}

		// Verify it's a directory
		info, err := os.Stat(rootPath)
		if err != nil {
			return fmt.Errorf("cannot access path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", rootPath)
		}

		// Load config
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Run the import browser
		result, err := tui.RunImportBrowser(cfg, rootPath)
		if err != nil {
			return fmt.Errorf("import browser failed: %w", err)
		}

		// Handle result
		if result.Aborted {
			fmt.Println("Import browser cancelled.")
			return nil
		}

		if result.Error != nil {
			return result.Error
		}

		// Report success based on action taken
		switch result.Action {
		case "import":
			fmt.Printf("Created workspace: %s\n", result.WorkspaceSlug)
			if len(result.ReposImported) > 0 {
				fmt.Printf("Imported %d repo(s)\n", len(result.ReposImported))
			}
			fmt.Println("Run 'co index' to update the index.")

		case "add-to":
			fmt.Printf("Added to workspace: %s\n", result.WorkspaceSlug)
			if len(result.ReposImported) > 0 {
				fmt.Printf("Added %d repo(s)\n", len(result.ReposImported))
			}
			fmt.Println("Run 'co index' to update the index.")

		case "stash":
			fmt.Printf("Archived: %s\n", result.ArchivePath)
			if result.SourceStashed != "" {
				fmt.Printf("Source: %s\n", result.SourceStashed)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(importTUICmd)
}
