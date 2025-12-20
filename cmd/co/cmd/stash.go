package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	stashDelete bool
	stashName   string
)

// StashResult holds the result of a stash operation.
type StashResult struct {
	ArchivePath string `json:"archive_path"`
	SourcePath  string `json:"source_path"`
	Name        string `json:"name"`
	Deleted     bool   `json:"deleted"`
}

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

		// Determine archive name
		name := stashName
		if name == "" {
			name = filepath.Base(sourcePath)
		}
		name = sanitizeArchiveName(name)

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

		result, err := stashFolder(cfg, sourcePath, name, stashDelete)
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

func stashFolder(cfg *config.Config, sourcePath, name string, deleteAfter bool) (*StashResult, error) {
	now := time.Now()
	year := now.Format("2006")
	timestamp := now.Format("20060102-150405")

	archiveDir := filepath.Join(cfg.ArchiveDir(), year)
	if err := fs.EnsureDir(archiveDir); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Create archive filename: name--timestamp--stash.tar.gz
	archiveName := fmt.Sprintf("%s--%s--stash.tar.gz", name, timestamp)
	archivePath := filepath.Join(archiveDir, archiveName)

	fmt.Printf("Archiving: %s\n", sourcePath)

	// Create the tar.gz archive
	cmd := exec.Command("tar", "-czf", archivePath, "-C", filepath.Dir(sourcePath), filepath.Base(sourcePath))
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	result := &StashResult{
		ArchivePath: archivePath,
		SourcePath:  sourcePath,
		Name:        name,
	}

	if deleteAfter {
		if err := os.RemoveAll(sourcePath); err != nil {
			return nil, fmt.Errorf("failed to delete source folder: %w", err)
		}
		result.Deleted = true
	}

	return result, nil
}

// sanitizeArchiveName cleans up a name for use in archive filenames.
func sanitizeArchiveName(s string) string {
	s = strings.ToLower(s)
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result.WriteRune(c)
		} else if c == ' ' {
			result.WriteRune('-')
		}
	}
	name := result.String()
	// Trim leading/trailing dashes
	name = strings.Trim(name, "-_")
	if name == "" {
		name = "folder"
	}
	return name
}

func init() {
	stashCmd.Flags().BoolVar(&stashDelete, "delete", false, "delete folder after archiving")
	stashCmd.Flags().StringVar(&stashName, "name", "", "custom name for the archive (defaults to folder name)")
	rootCmd.AddCommand(stashCmd)
}
