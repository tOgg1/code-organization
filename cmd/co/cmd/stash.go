package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/archive"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	stashDelete bool
	stashName   string
)

var stashCmd = &cobra.Command{
	Use:   "stash <folder-path>",
	Short: "Archive any folder to the system archive",
	Long: `Archives any folder (not necessarily an indexed workspace) to _system/archive/.

This is useful for archiving source folders after importing, or for
archiving any other folders you want to keep but not have cluttering
your filesystem.

The folder is compressed into a .tar.gz file in the archive directory.
Use --delete to remove the original folder after archiving.
Use --name to specify a custom name for the archive (defaults to folder name).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourcePath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("cannot access path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", sourcePath)
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Confirm if deleting
		if stashDelete {
			result, err := tui.RunConfirm(fmt.Sprintf("Archive and DELETE '%s'?", sourcePath))
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if result.Aborted || !result.Confirmed {
				fmt.Println("Stash cancelled.")
				return nil
			}
		}

		fmt.Printf("Archiving: %s\n", sourcePath)

		opts := archive.StashOptions{
			Name:        stashName,
			DeleteAfter: stashDelete,
		}
		result, err := archive.StashFolder(cfg, sourcePath, opts)
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		fmt.Printf("Archive created: %s\n", result.ArchivePath)
		if result.Deleted {
			fmt.Printf("Deleted: %s\n", result.SourcePath)
		}

		return nil
	},
}

func init() {
	stashCmd.Flags().BoolVar(&stashDelete, "delete", false, "delete folder after archiving")
	stashCmd.Flags().StringVar(&stashName, "name", "", "custom name for the archive (defaults to folder name)")
	rootCmd.AddCommand(stashCmd)
}
