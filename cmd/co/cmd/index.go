package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/index"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Regenerate the workspace index",
	Long: `Scans code_root and regenerates _system/index.jsonl atomically.
Computes last commit dates, dirty flags, and workspace sizes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Indexing workspaces in %s...\n", cfg.CodeRoot)
		start := time.Now()

		builder := index.NewBuilder(cfg)
		idx, err := builder.Build()
		if err != nil {
			return fmt.Errorf("failed to build index: %w", err)
		}

		if err := builder.Save(idx); err != nil {
			return fmt.Errorf("failed to save index: %w", err)
		}

		duration := time.Since(start)
		fmt.Printf("Indexed %d workspaces in %v\n", len(idx.Records), duration.Round(time.Millisecond))
		fmt.Printf("Index saved to: %s\n", cfg.IndexPath())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}
