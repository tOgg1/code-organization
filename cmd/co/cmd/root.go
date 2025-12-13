package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	jsonOut  bool
	jsonlOut bool
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
}

func exitWithError(msg string, code int) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}
