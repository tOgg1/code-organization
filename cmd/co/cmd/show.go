package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
)

var showCmd = &cobra.Command{
	Use:   "show <workspace-slug>",
	Short: "Show workspace details",
	Long:  `Displays detailed information about a specific workspace.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		idx, err := model.LoadIndex(cfg.IndexPath())
		if err != nil {
			return fmt.Errorf("failed to load index: %w", err)
		}

		record := idx.FindBySlug(slug)
		if record == nil {
			return fmt.Errorf("workspace not found: %s", slug)
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(record)
		}

		fmt.Printf("Workspace: %s\n", record.Slug)
		fmt.Printf("Path:      %s\n", record.Path)
		fmt.Printf("Owner:     %s\n", record.Owner)
		fmt.Printf("State:     %s\n", record.State)
		if len(record.Tags) > 0 {
			fmt.Printf("Tags:      %v\n", record.Tags)
		}
		fmt.Printf("Repos:     %d\n", record.RepoCount)
		fmt.Printf("Dirty:     %d\n", record.DirtyRepos)
		fmt.Printf("Size:      %s\n", formatBytes(record.SizeBytes))

		if record.LastCommitAt != nil {
			fmt.Printf("Last commit: %s\n", record.LastCommitAt.Format("2006-01-02 15:04"))
		}
		if record.LastFSChangeAt != nil {
			fmt.Printf("Last change: %s\n", record.LastFSChangeAt.Format("2006-01-02 15:04"))
		}

		if len(record.Repos) > 0 {
			fmt.Println("\nRepositories:")
			for _, r := range record.Repos {
				dirtyMark := ""
				if r.Dirty {
					dirtyMark = " [dirty]"
				}
				fmt.Printf("  - %s (%s)%s\n", r.Name, r.Branch, dirtyMark)
				if r.Remote != "" {
					fmt.Printf("    remote: %s\n", r.Remote)
				}
			}
		}

		return nil
	},
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	rootCmd.AddCommand(showCmd)
}
