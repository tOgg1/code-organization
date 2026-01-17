package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
	"github.com/tormodhaugland/co/internal/sync"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	syncBatchForce       bool
	syncBatchDryRun      bool
	syncBatchNoGit       bool
	syncBatchIncludeEnv  bool
	syncBatchExcludes    []string
	syncBatchExcludeFrom string
)

var syncBatchCmd = &cobra.Command{
	Use:   "sync-batch <server>",
	Short: "Interactively sync multiple workspaces to a remote server",
	Long: `Select multiple workspaces, then sync each to a remote server.
Repos are cloned on the target machine; existing repos are skipped.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		idx, err := model.LoadIndex(cfg.IndexPath())
		if err != nil {
			return fmt.Errorf("failed to load index (run 'co index' first): %w", err)
		}

		pickerResult, err := tui.RunSyncPicker(idx.Records)
		if err != nil {
			return fmt.Errorf("sync picker failed: %w", err)
		}
		if pickerResult.Aborted || len(pickerResult.Slugs) == 0 {
			fmt.Println("Sync cancelled.")
			return nil
		}

		server := cfg.GetServer(serverName)
		results := make([]batchResult, 0, len(pickerResult.Slugs))

		for _, slug := range pickerResult.Slugs {
			localPath := cfg.WorkspacePath(slug)
			projectPath := filepath.Join(localPath, "project.json")
			project, err := model.LoadProject(projectPath)
			if err != nil {
				results = append(results, batchResult{
					Slug:  slug,
					Error: fmt.Errorf("project.json required for sync: %w", err).Error(),
				})
				continue
			}

			opts := sync.DefaultOptions()
			opts.Force = syncBatchForce
			opts.DryRun = syncBatchDryRun
			opts.NoGit = syncBatchNoGit
			opts.IncludeEnv = syncBatchIncludeEnv
			opts.ExcludePatterns = syncBatchExcludes
			opts.ExcludeFromFile = syncBatchExcludeFrom
			opts.Project = project

			if project.Sync != nil && project.Sync.Excludes != nil {
				opts.WorkspaceAdd = project.Sync.Excludes.Add
				opts.WorkspaceRemove = project.Sync.Excludes.Remove
			}
			if project.Sync != nil && project.Sync.IncludeEnv {
				opts.IncludeEnv = true
			}

			fmt.Printf("Syncing %s to %s:%s/%s\n", slug, server.SSH, server.CodeRoot, slug)

			result, err := sync.SyncWorkspace(localPath, server, slug, opts)
			entry := batchResult{Slug: slug, Result: result}
			if err != nil {
				entry.Error = err.Error()
			}
			results = append(results, entry)
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		for _, entry := range results {
			fmt.Printf("\n%s\n", entry.Slug)
			if entry.Result != nil {
				fmt.Print(sync.FormatResult(entry.Result))
			}
			if entry.Error != "" {
				fmt.Printf("Error: %s\n", entry.Error)
			}
		}

		return nil
	},
}

type batchResult struct {
	Slug   string       `json:"slug"`
	Result *sync.Result `json:"result,omitempty"`
	Error  string       `json:"error,omitempty"`
}

func init() {
	syncBatchCmd.Flags().BoolVar(&syncBatchForce, "force", false, "sync even if remote path exists")
	syncBatchCmd.Flags().BoolVar(&syncBatchDryRun, "dry-run", false, "preview changes without syncing")
	syncBatchCmd.Flags().BoolVar(&syncBatchNoGit, "no-git", false, "exclude .git directories")
	syncBatchCmd.Flags().BoolVar(&syncBatchIncludeEnv, "include-env", false, "include .env files (overrides default exclude)")
	syncBatchCmd.Flags().StringArrayVar(&syncBatchExcludes, "exclude", nil, "add an exclude pattern (repeatable)")
	syncBatchCmd.Flags().StringVar(&syncBatchExcludeFrom, "exclude-from", "", "read exclude patterns from file (one per line, # comments ignored)")
	rootCmd.AddCommand(syncBatchCmd)
}
