package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/partial"
	"github.com/tormodhaugland/co/internal/template"
)

var (
	cfgFile   string
	jsonOut   bool
	jsonlOut  bool
	robotHelp bool
)

var rootCmd = &cobra.Command{
	Use:   "co",
	Short: "Code Organization - workspace manager with TUI and remote sync",
	Long: `co is a Go-powered workspace manager that enforces a single,
machine-readable project structure under ~/Code, provides a fast TUI
for navigating and operating on projects, and supports safe one-command
syncing to named remote servers.

Running 'co' without arguments launches the TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default behavior: launch TUI
		return tuiCmd.RunE(cmd, args)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/co/config.json)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&jsonlOut, "jsonl", false, "output in JSON Lines format")
	rootCmd.PersistentFlags().BoolVar(&robotHelp, "robot-help", false, "print detailed robot helper guidance and exit")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if robotHelp {
			fmt.Fprint(cmd.OutOrStdout(), robotHelpText())
			os.Exit(0)
		}
		return nil
	}

	template.RegisterPartialApplier(func(opts template.PartialApplyOptions, partialsDirs []string) error {
		_, err := partial.Apply(partial.ApplyOptions{
			PartialName: opts.PartialName,
			TargetPath:  opts.TargetPath,
			Variables:   opts.Variables,
			DryRun:      opts.DryRun,
			NoHooks:     opts.NoHooks,
		}, partialsDirs)
		return err
	})
}

func exitWithError(msg string, code int) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}

func robotHelpText() string {
	return `co --robot-help
Detailed guidance for automation and helper tools.

Purpose
  co manages a single workspace root (default: ~/Code) with a consistent
  owner--project layout and machine-readable metadata. It can run a TUI,
  but automation should prefer non-interactive commands.

Structure
  Root layout:
    ~/Code/
      _system/         internal data (index.jsonl, archive/, cache/, logs/)
      owner--project/  workspace directory
  Workspace layout:
    <workspace>/
      project.json     canonical metadata
      repos/           git repositories

Organization patterns
  - Workspace slug format: owner--project
  - Optional qualifier: owner--project--qualifier
    allowed: poc, demo, legacy, migration, infra
  - project.json is the source of truth; index.jsonl is generated from disk
  - State values: active, paused, archived, scratch

Templates and partials
  - Templates create workspaces and live in:
      ~/Code/_system/templates/ (primary)
      ~/.config/co/templates/  (fallback)
  - Partials apply files to an existing directory and live in:
      ~/Code/_system/partials/  (primary)
      ~/.config/co/partials/   (fallback)
  - Use -t/--template and -v/--var for template variables.
  - Use --dry-run and --no-hooks for safe previews.

Automation defaults
  - Prefer non-interactive commands. Avoid the TUI unless explicitly requested.
  - Use --json or --jsonl for machine-readable output when available.
  - Use --config to point at a specific config file when running in CI.

Common workflows
  1) Discover and inspect workspaces
     co index
     co ls --json
     co show <slug> --json
     co cd <slug> [repo]

  2) Create a workspace
     co new <owner> <project>
     co new <owner> <project> <repo-url...>
     co new <owner> <project> -t <template> -v key=value

  3) Work with templates and partials
     co template list
     co template show <name>
     co partial list
     co partial show <name> --files
     co partial apply <name> [path] --dry-run

  4) Import existing folders
     co import <folder-path>
     co import <folder-path> --add-to <workspace-slug>
     co import -i <folder-path>   (interactive browser)

  5) Sync and archive
     co sync <workspace-slug> <server> --dry-run
     co sync <workspace-slug> <server> --force
     co archive <workspace-slug> --reason "EOL"

  6) Semantic code search
     co vector index <workspace...>
     co vector search "<query>" --json

Command notes
  - co (no args) launches the TUI.
  - co open <slug> opens the workspace in the configured editor.
  - co ls supports --owner, --state, --tag filters plus --json/--jsonl output.
  - co show exposes full workspace metadata and repo status.

Safety defaults
  - Destructive actions are opt-in: co sync --force, co archive --delete.
  - Prefer --dry-run before any sync or import.

Exit codes
  0 success
  1 general error
  2 invalid arguments
  10 sync skipped (remote exists)

Config discovery
  1) --config <path>
  2) $XDG_CONFIG_HOME/co/config.json or ~/.config/co/config.json
  3) ~/Code/_system/config.json (optional)
`
}
