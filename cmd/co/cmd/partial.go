package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/partial"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	partialListTag       string
	partialShowFiles     bool
	partialApplyVars     []string
	partialApplyConflict string
	partialApplyDryRun   bool
	partialApplyNoHooks  bool
	partialApplyForce    bool
	partialApplyYes      bool
)

var partialCmd = &cobra.Command{
	Use:     "partial",
	Aliases: []string{"p", "partials"},
	Short:   "Manage reusable file sets (partials)",
	Long: `Partials are reusable file sets that can be applied to any directory.
They are smaller than templates and can be used to add or update files in
existing projects without creating a new workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return cmd.Help()
		}
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return tui.RunPartialExplorer(cfg)
	},
}

var partialListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available partials",
	Long:    "Lists all available partials with descriptions and counts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		partials, err := partial.ListPartialsFiltered(cfg.AllPartialsDirs(), partialListTag)
		if err != nil {
			return fmt.Errorf("failed to list partials: %w", err)
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(partials)
		}

		if len(partials) == 0 {
			fmt.Println("No partials found")
			fmt.Printf("\nPartials directories:\n  %s\n  %s\n",
				cfg.PartialsDir(), cfg.FallbackPartialsDir())
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tFILES\tVARS")
		for _, info := range partials {
			desc := info.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\n",
				info.Name, desc, info.FileCount, info.VarCount)
		}
		w.Flush()

		return nil
	},
}

var partialShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show partial details",
	Long:  "Shows detailed information about a partial including variables, hooks, tags, and requirements.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		p, partialPath, err := partial.LoadPartialByName(args[0], cfg.AllPartialsDirs())
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(p)
		}

		fmt.Printf("Partial: %s\n", p.Name)
		fmt.Printf("Description: %s\n", p.Description)
		if p.Version != "" {
			fmt.Printf("Version: %s\n", p.Version)
		}
		fmt.Println()

		if len(p.Variables) > 0 {
			fmt.Println("Variables:")
			for _, v := range p.Variables {
				required := ""
				if v.Required {
					required = " (required)"
				}
				fmt.Printf("  - %s%s: %s [%s]\n", v.Name, required, v.Description, v.Type)
				if v.Default != nil {
					fmt.Printf("    Default: %v\n", v.Default)
				}
				if len(v.Choices) > 0 {
					fmt.Printf("    Choices: %v\n", v.Choices)
				}
				if v.Validation != "" {
					fmt.Printf("    Validation: %s\n", v.Validation)
				}
			}
			fmt.Println()
		}

		if partialShowFiles {
			files, err := listPartialFiles(partialPath, p)
			if err != nil {
				return err
			}
			fmt.Println("Files:")
			if len(files) == 0 {
				fmt.Println("  (none)")
			} else {
				for _, f := range files {
					fmt.Printf("  - %s\n", f)
				}
			}
			fmt.Println()
		}

		hookCount := 0
		if p.Hooks.PreApply.Script != "" {
			hookCount++
		}
		if p.Hooks.PostApply.Script != "" {
			hookCount++
		}
		if hookCount > 0 {
			fmt.Println("Hooks:")
			if p.Hooks.PreApply.Script != "" {
				timeout := p.Hooks.PreApply.Timeout
				if timeout == "" {
					timeout = partial.DefaultHookTimeout
				}
				fmt.Printf("  - pre_apply: %s (timeout: %s)\n", p.Hooks.PreApply.Script, timeout)
			}
			if p.Hooks.PostApply.Script != "" {
				timeout := p.Hooks.PostApply.Timeout
				if timeout == "" {
					timeout = partial.DefaultHookTimeout
				}
				fmt.Printf("  - post_apply: %s (timeout: %s)\n", p.Hooks.PostApply.Script, timeout)
			}
			fmt.Println()
		}

		if p.HasPrerequisites() {
			fmt.Println("Requirements:")
			if len(p.Requires.Commands) > 0 {
				fmt.Printf("  Commands: %v\n", p.Requires.Commands)
			}
			if len(p.Requires.Files) > 0 {
				fmt.Printf("  Files: %v\n", p.Requires.Files)
			}
			fmt.Println()
		}

		if len(p.Tags) > 0 {
			fmt.Printf("Tags: %v\n", p.Tags)
		}

		return nil
	},
}

var partialApplyCmd = &cobra.Command{
	Use:   "apply <name> [path]",
	Short: "Apply a partial to a directory",
	Long: `Applies a partial to a target directory.

Arguments:
  name    Name of the partial to apply (required)
  path    Target directory (default: current directory)

Flags:
  -v, --var key=value     Set variable (can be repeated)
      --conflict strategy Override conflict strategy (prompt|skip|overwrite|backup|merge)
      --dry-run           Preview changes without applying
      --no-hooks          Skip lifecycle hooks
      --force             Apply even if prerequisites fail
  -y, --yes               Accept all prompts automatically`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		partialName := args[0]
		targetPath := "."
		if len(args) > 1 {
			targetPath = args[1]
		}

		// Resolve target path
		absTargetPath, err := filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("resolving target path: %w", err)
		}

		// Parse variables
		vars := parsePartialVars(partialApplyVars)

		// Build options
		opts := partial.ApplyOptions{
			PartialName:      partialName,
			TargetPath:       absTargetPath,
			Variables:        vars,
			ConflictStrategy: partialApplyConflict,
			DryRun:           partialApplyDryRun,
			NoHooks:          partialApplyNoHooks,
			Force:            partialApplyForce,
			Yes:              partialApplyYes,
		}

		// Apply the partial
		result, err := partial.Apply(opts, cfg.AllPartialsDirs())
		if err != nil {
			return handleApplyError(err, cfg)
		}

		// Output result
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		return formatApplyResult(result, partialApplyDryRun)
	},
}

func parsePartialVars(vars []string) map[string]string {
	result := make(map[string]string)
	for _, v := range vars {
		if idx := strings.Index(v, "="); idx > 0 {
			result[v[:idx]] = v[idx+1:]
		}
	}
	return result
}

func handleApplyError(err error, cfg *config.Config) error {
	switch e := err.(type) {
	case *partial.PartialNotFoundError:
		fmt.Fprintf(os.Stderr, "Error: partial not found: %s\n\n", e.Name)
		fmt.Fprintln(os.Stderr, "Available partials:")
		partials, listErr := partial.ListPartials(cfg.AllPartialsDirs())
		if listErr == nil && len(partials) > 0 {
			for _, p := range partials {
				fmt.Fprintf(os.Stderr, "  - %s\n", p.Name)
			}
		} else if len(partials) == 0 {
			fmt.Fprintln(os.Stderr, "  (none found)")
		}
		return fmt.Errorf("partial not found: %s", e.Name)

	case *partial.TargetNotFoundError:
		return fmt.Errorf("target directory not found: %s\n\nCreate the directory first or check the path.", e.Path)

	case *partial.PrerequisiteFailedError:
		fmt.Fprintf(os.Stderr, "Error: prerequisites not met for partial '%s'\n", e.PartialName)
		if len(e.MissingCommands) > 0 {
			fmt.Fprintf(os.Stderr, "  Missing commands: %s\n", strings.Join(e.MissingCommands, ", "))
		}
		if len(e.MissingFiles) > 0 {
			fmt.Fprintf(os.Stderr, "  Missing files: %s\n", strings.Join(e.MissingFiles, ", "))
		}
		fmt.Fprintln(os.Stderr, "\nUse --force to apply anyway.")
		return fmt.Errorf("prerequisites not met")

	case *partial.MissingRequiredVarError:
		return fmt.Errorf("missing required variable: %s\n\nUse -v %s=VALUE to provide it.", e.VarName, e.VarName)

	default:
		return err
	}
}

func formatApplyResult(result *partial.ApplyResult, dryRun bool) error {
	if dryRun {
		fmt.Println("DRY RUN - No changes will be made")
		fmt.Println()
	}

	fmt.Printf("Partial: %s\n", result.PartialName)
	fmt.Printf("Target: %s\n", result.TargetPath)
	fmt.Println()

	if len(result.FilesCreated) > 0 {
		fmt.Println("Created:")
		for _, f := range result.FilesCreated {
			fmt.Printf("  + %s\n", f)
		}
	}

	if len(result.FilesOverwritten) > 0 {
		fmt.Println("Overwritten:")
		for _, f := range result.FilesOverwritten {
			fmt.Printf("  ~ %s\n", f)
		}
	}

	if len(result.FilesBackedUp) > 0 {
		fmt.Println("Backed up:")
		for _, f := range result.FilesBackedUp {
			fmt.Printf("  ! %s\n", f)
		}
	}

	if len(result.FilesMerged) > 0 {
		fmt.Println("Merged:")
		for _, f := range result.FilesMerged {
			fmt.Printf("  M %s\n", f)
		}
	}

	if len(result.FilesSkipped) > 0 {
		fmt.Println("Skipped:")
		for _, f := range result.FilesSkipped {
			fmt.Printf("  - %s\n", f)
		}
	}

	// Summary
	fmt.Println()
	total := len(result.FilesCreated) + len(result.FilesOverwritten) + len(result.FilesBackedUp) + len(result.FilesMerged)
	if dryRun {
		fmt.Printf("Would apply %d files (%d skipped)\n", total, len(result.FilesSkipped))
	} else {
		fmt.Printf("Applied %d files (%d skipped)\n", total, len(result.FilesSkipped))
	}

	if len(result.HooksRun) > 0 {
		fmt.Printf("Hooks run: %s\n", strings.Join(result.HooksRun, ", "))
	}
	if len(result.HooksSkipped) > 0 {
		fmt.Printf("Hooks skipped: %s\n", strings.Join(result.HooksSkipped, ", "))
	}

	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	return nil
}

var partialValidateCmd = &cobra.Command{
	Use:   "validate [name]",
	Short: "Validate partial manifests",
	Long:  "Validates one or all partials, checking manifest structure and referenced files.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(args) > 0 {
			partialPath, err := partial.FindPartial(args[0], cfg.AllPartialsDirs())
			if err != nil {
				return err
			}
			if err := partial.ValidatePartialDir(partialPath); err != nil {
				return fmt.Errorf("validation failed for %s: %w", args[0], err)
			}
			fmt.Printf("Partial %s is valid\n", args[0])
			return nil
		}

		partials, err := partial.ListPartials(cfg.AllPartialsDirs())
		if err != nil {
			return fmt.Errorf("failed to list partials: %w", err)
		}

		if len(partials) == 0 {
			fmt.Println("No partials to validate")
			return nil
		}

		hasErrors := false
		for _, info := range partials {
			if err := partial.ValidatePartialDir(info.Path); err != nil {
				fmt.Printf("ERR %s: %v\n", info.Name, err)
				hasErrors = true
			} else {
				fmt.Printf("OK %s\n", info.Name)
			}
		}

		if hasErrors {
			return fmt.Errorf("some partials have errors")
		}

		fmt.Printf("\nAll %d partials are valid\n", len(partials))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(partialCmd)
	partialCmd.AddCommand(partialListCmd)
	partialCmd.AddCommand(partialShowCmd)
	partialCmd.AddCommand(partialApplyCmd)
	partialCmd.AddCommand(partialValidateCmd)

	// List flags
	partialListCmd.Flags().StringVar(&partialListTag, "tag", "", "filter partials by tag")

	// Show flags
	partialShowCmd.Flags().BoolVar(&partialShowFiles, "files", false, "include file list in output")

	// Apply flags
	partialApplyCmd.Flags().StringArrayVarP(&partialApplyVars, "var", "v", nil, "set variable (key=value, can be repeated)")
	partialApplyCmd.Flags().StringVar(&partialApplyConflict, "conflict", "", "conflict strategy (prompt|skip|overwrite|backup|merge)")
	partialApplyCmd.Flags().BoolVar(&partialApplyDryRun, "dry-run", false, "preview changes without applying")
	partialApplyCmd.Flags().BoolVar(&partialApplyNoHooks, "no-hooks", false, "skip lifecycle hooks")
	partialApplyCmd.Flags().BoolVar(&partialApplyForce, "force", false, "apply even if prerequisites fail")
	partialApplyCmd.Flags().BoolVarP(&partialApplyYes, "yes", "y", false, "accept all prompts automatically")
}

func listPartialFiles(partialPath string, p *partial.Partial) ([]string, error) {
	infos, err := partial.ListPartialFilesWithInfo(partialPath, p.Files, p.GetTemplateExtensions())
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(infos))
	for _, info := range infos {
		files = append(files, info.RelPath)
	}
	return files, nil
}
