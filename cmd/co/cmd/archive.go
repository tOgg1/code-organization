package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/archive"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	archiveDelete bool
	archiveReason string
	archiveFull   bool
)

var archiveCmd = &cobra.Command{
	Use:   "archive <workspace-slug>",
	Short: "Archive a workspace",
	Long: `Creates a git bundle archive for each repo in the workspace.
Archives are stored in _system/archive/YYYY/.
Use --delete to remove the workspace after archiving.
Use --full to archive the entire workspace folder instead of just git bundles.

Supports fuzzy matching - if no exact match is found, you'll be prompted to confirm.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		slug := query
		if !fs.WorkspaceExists(cfg.CodeRoot, query) {
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

			slug = best.Str
			result, err := tui.RunConfirm(fmt.Sprintf("Archive workspace '%s'?", slug))
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if result.Aborted || !result.Confirmed {
				return fmt.Errorf("aborted")
			}
		}

		if archiveFull {
			fmt.Printf("Archiving workspace (full): %s\n", slug)
		} else {
			fmt.Printf("Archiving workspace: %s\n", slug)
		}

		opts := archive.Options{
			Reason:      archiveReason,
			DeleteAfter: archiveDelete,
			Full:        archiveFull,
		}

		result, err := archive.ArchiveWorkspace(cfg, slug, opts)
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		fmt.Printf("Archive created: %s\n", result.ArchivePath)
		if result.FullArchive {
			fmt.Println("Type: full workspace")
		} else {
			fmt.Printf("Bundles: %d\n", result.BundleCount)
		}
		if result.Deleted {
			fmt.Println("Workspace deleted")
		}

		return nil
	},
}

func init() {
	archiveCmd.Flags().BoolVar(&archiveDelete, "delete", false, "delete workspace after archiving")
	archiveCmd.Flags().StringVar(&archiveReason, "reason", "", "reason for archiving")
	archiveCmd.Flags().BoolVar(&archiveFull, "full", false, "archive entire workspace folder, not just git bundles")
	rootCmd.AddCommand(archiveCmd)
}
