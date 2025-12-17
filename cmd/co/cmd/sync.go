package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/model"
	"github.com/tormodhaugland/co/internal/sync"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	syncForce        bool
	syncDryRun       bool
	syncNoGit        bool
	syncIncludeEnv   bool
	syncExcludes     []string
	syncExcludeFrom  string
	syncListExcludes bool
	syncInteractive  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync <workspace-slug> <server>",
	Short: "Sync workspace to a remote server",
	Long: `Syncs a workspace to a remote server via rsync or tar.
By default, does nothing if the remote path already exists (exit 10).
Use --force to sync regardless.

Default excludes include common build artifacts, dependency caches,
and sensitive files (node_modules/, target/, .env, etc.).
Use --exclude to add patterns, --include-env to sync .env files.

Use --interactive (-i) to launch a TUI for selecting files/directories
to exclude before syncing. Navigate with j/k, toggle with space.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --list-excludes
		if syncListExcludes {
			excludeList := fs.BuildExcludeList(fs.ExcludeOptions{
				NoGit:      syncNoGit,
				IncludeEnv: syncIncludeEnv,
				Additional: syncExcludes,
			})
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(excludeList.Patterns)
			}
			fmt.Println("Default exclude patterns:")
			for _, p := range excludeList.Patterns {
				fmt.Printf("  %s\n", p)
			}
			return nil
		}

		// Require both args for actual sync
		if len(args) != 2 {
			return fmt.Errorf("requires exactly 2 arguments: <workspace-slug> <server>")
		}

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
		opts.IncludeEnv = syncIncludeEnv
		opts.ExcludePatterns = syncExcludes
		opts.ExcludeFromFile = syncExcludeFrom

		// Load workspace config from project.json if it exists
		projectPath := filepath.Join(localPath, "project.json")
		if project, err := model.LoadProject(projectPath); err == nil {
			// Apply workspace sync config
			if project.Sync != nil && project.Sync.Excludes != nil {
				opts.WorkspaceAdd = project.Sync.Excludes.Add
				opts.WorkspaceRemove = project.Sync.Excludes.Remove
			}
			if project.Sync != nil && project.Sync.IncludeEnv {
				opts.IncludeEnv = true
			}
		}

		// Interactive mode: launch TUI picker to select excludes
		if syncInteractive {
			// Build default excludes list to show which patterns are pre-selected
			defaultExcludes := fs.BuiltinExcludes
			if syncNoGit {
				defaultExcludes = append(defaultExcludes, ".git/")
			}

			pickerResult, err := tui.RunExcludePicker(localPath, defaultExcludes)
			if err != nil {
				return fmt.Errorf("exclude picker failed: %w", err)
			}
			if pickerResult.Aborted {
				fmt.Println("Sync cancelled.")
				return nil
			}

			// Use only the user's selected excludes (skip default patterns)
			opts.ExcludePatterns = pickerResult.Excludes
			opts.SkipDefaultExcludes = true

			// Save excludes to project.json if requested
			if pickerResult.SaveRequested {
				if err := saveWorkspaceExcludes(localPath, pickerResult.Excludes); err != nil {
					fmt.Printf("Warning: failed to save excludes to project.json: %v\n", err)
				} else {
					fmt.Println("Saved exclude patterns to project.json")
				}
			}
		}

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
	syncCmd.Flags().BoolVar(&syncIncludeEnv, "include-env", false, "include .env files (overrides default exclude)")
	syncCmd.Flags().StringArrayVar(&syncExcludes, "exclude", nil, "add an exclude pattern (repeatable)")
	syncCmd.Flags().StringVar(&syncExcludeFrom, "exclude-from", "", "read exclude patterns from file (one per line, # comments ignored)")
	syncCmd.Flags().BoolVar(&syncListExcludes, "list-excludes", false, "print effective exclude list and exit")
	syncCmd.Flags().BoolVarP(&syncInteractive, "interactive", "i", false, "launch TUI to interactively select exclude patterns")
	rootCmd.AddCommand(syncCmd)
}

// saveWorkspaceExcludes saves the exclude patterns to project.json
func saveWorkspaceExcludes(workspacePath string, excludes []string) error {
	projectPath := filepath.Join(workspacePath, "project.json")

	// Try to load existing project, or create a new one
	project, err := model.LoadProject(projectPath)
	if err != nil {
		// If project.json doesn't exist, we can't save excludes
		// (user should create workspace first with co new)
		return fmt.Errorf("project.json not found: %w", err)
	}

	// Initialize Sync config if needed
	if project.Sync == nil {
		project.Sync = &model.SyncConfig{}
	}
	if project.Sync.Excludes == nil {
		project.Sync.Excludes = &model.ExcludeConfig{}
	}

	// Save the exclude patterns as "add" patterns
	project.Sync.Excludes.Add = excludes

	// Save back to file
	return project.Save(workspacePath)
}
