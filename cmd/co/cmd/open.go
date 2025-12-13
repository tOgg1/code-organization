package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
)

var openCmd = &cobra.Command{
	Use:   "open <workspace-slug>",
	Short: "Open a workspace",
	Long:  `Opens the workspace in the configured editor, or prints the path if no editor is set.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !fs.WorkspaceExists(cfg.CodeRoot, slug) {
			return fmt.Errorf("workspace not found: %s", slug)
		}

		workspacePath := cfg.WorkspacePath(slug)

		if cfg.Editor != "" {
			editorCmd := exec.Command(cfg.Editor, workspacePath)
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			return editorCmd.Start()
		}

		if runtime.GOOS == "darwin" {
			return exec.Command("open", workspacePath).Start()
		}

		fmt.Println(workspacePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
