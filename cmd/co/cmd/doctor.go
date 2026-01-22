package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/doctor"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	doctorYes    bool
	doctorDryRun bool
)

type doctorResult struct {
	CodeRoot string                  `json:"code_root"`
	Missing  []doctor.MissingProject `json:"missing"`
	Planned  []string                `json:"planned,omitempty"`
	Created  []string                `json:"created,omitempty"`
	Skipped  []string                `json:"skipped,omitempty"`
	Errors   []string                `json:"errors,omitempty"`
	DryRun   bool                    `json:"dry_run"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check and repair workspace metadata",
	Long: `Scans workspaces for missing project.json files.
If any are missing, you can create them interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		missing, err := doctor.FindMissingProjects(cfg.CodeRoot)
		if err != nil {
			return fmt.Errorf("failed to scan workspaces: %w", err)
		}

		result := doctorResult{
			CodeRoot: cfg.CodeRoot,
			Missing:  missing,
			DryRun:   doctorDryRun,
		}

		if len(missing) == 0 {
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Println("All workspaces have project.json")
			return nil
		}

		if jsonOut {
			if doctorYes && doctorDryRun {
				result.Planned = collectSlugs(missing)
			}
			if doctorYes && !doctorDryRun {
				applyDoctorFixes(&result, true)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return err
			}
			if len(result.Errors) > 0 {
				return fmt.Errorf("doctor encountered %d errors", len(result.Errors))
			}
			return nil
		}

		fmt.Printf("Missing project.json in %d workspace(s):\n", len(missing))
		for _, entry := range missing {
			fmt.Printf("  - %s (%s)\n", entry.Slug, entry.Path)
		}

		if doctorDryRun {
			fmt.Println("Dry run - no changes made")
			for _, entry := range missing {
				fmt.Printf("Would create project.json for %s\n", entry.Slug)
			}
			return nil
		}

		if doctorYes {
			applyDoctorFixes(&result, false)
		} else {
			for _, entry := range missing {
				confirm, err := tui.RunConfirm(fmt.Sprintf("Create project.json for '%s'?", entry.Slug))
				if err != nil {
					return fmt.Errorf("prompt failed: %w", err)
				}
				if confirm.Aborted {
					return fmt.Errorf("aborted")
				}
				if !confirm.Confirmed {
					result.Skipped = append(result.Skipped, entry.Slug)
					continue
				}

				if err := createProjectJSON(entry, &result, false); err != nil {
					continue
				}
			}
		}

		printDoctorSummary(result)
		if len(result.Errors) > 0 {
			return fmt.Errorf("doctor encountered %d errors", len(result.Errors))
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVarP(&doctorYes, "yes", "y", false, "create missing project.json files without prompting")
	doctorCmd.Flags().BoolVar(&doctorDryRun, "dry-run", false, "preview missing project.json files without creating")
	rootCmd.AddCommand(doctorCmd)
}

func applyDoctorFixes(result *doctorResult, quiet bool) {
	for _, entry := range result.Missing {
		if err := createProjectJSON(entry, result, quiet); err != nil {
			continue
		}
	}
}

func createProjectJSON(entry doctor.MissingProject, result *doctorResult, quiet bool) error {
	project, err := doctor.CreateProjectJSON(entry.Slug, entry.Path)
	if err != nil {
		msg := fmt.Sprintf("%s: %v", entry.Slug, err)
		result.Errors = append(result.Errors, msg)
		fmt.Fprintln(os.Stderr, "Error:", msg)
		return err
	}

	result.Created = append(result.Created, entry.Slug)
	if !quiet {
		fmt.Printf("Created project.json for %s (repos: %d)\n", entry.Slug, len(project.Repos))
	}
	return nil
}

func printDoctorSummary(result doctorResult) {
	if len(result.Created) > 0 {
		fmt.Printf("Created %d project.json file(s)\n", len(result.Created))
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("Skipped %d workspace(s)\n", len(result.Skipped))
	}
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d (see stderr)\n", len(result.Errors))
	}
}

func collectSlugs(entries []doctor.MissingProject) []string {
	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		slugs = append(slugs, entry.Slug)
	}
	return slugs
}
