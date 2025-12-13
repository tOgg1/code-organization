package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/sync"
)

var (
	syncForce  bool
	syncDryRun bool
	syncNoGit  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync <workspace-slug> <server>",
	Short: "Sync workspace to a remote server",
	Long: `Syncs a workspace to a remote server via rsync or tar.
By default, does nothing if the remote path already exists (exit 10).
Use --force to sync regardless.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		serverName := args[1]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !fs.WorkspaceExists(cfg.CodeRoot, slug) {
			return fmt.Errorf("workspace not found: %s", slug)
		}

		localPath := cfg.WorkspacePath(slug)
		server := cfg.GetServer(serverName)

		opts := sync.DefaultOptions()
		opts.Force = syncForce
		opts.DryRun = syncDryRun
		opts.NoGit = syncNoGit

		fmt.Printf("Syncing %s to %s:%s/%s\n", slug, server.SSH, server.CodeRoot, slug)

		result, err := sync.SyncWorkspace(localPath, server, slug, opts)

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(result)
		} else {
			fmt.Print(sync.FormatResult(result))
		}

		if err != nil {
			return err
		}

		if result.ActionTaken == "skipped" {
			fmt.Println("Remote path already exists. Use --force to overwrite.")
			os.Exit(10)
		}

		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "sync even if remote path exists")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "preview changes without syncing")
	syncCmd.Flags().BoolVar(&syncNoGit, "no-git", false, "exclude .git directories")
	rootCmd.AddCommand(syncCmd)
}
