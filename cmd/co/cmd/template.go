package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/template"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage workspace templates",
	Long:  `List, show, and validate workspace templates.`,
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	Long:  `Lists all available workspace templates with their descriptions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		templates, err := template.ListTemplates(cfg.TemplatesDir())
		if err != nil {
			return fmt.Errorf("failed to list templates: %w", err)
		}

		if jsonOut {
			infos := make([]template.TemplateInfo, len(templates))
			for i, tmpl := range templates {
				infos[i] = tmpl.ToInfo()
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(infos)
		}

		if len(templates) == 0 {
			fmt.Println("No templates found")
			fmt.Printf("\nTemplates directory: %s\n", cfg.TemplatesDir())
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tVARS\tREPOS\tHOOKS")
		for _, tmpl := range templates {
			info := tmpl.ToInfo()
			desc := info.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n",
				info.Name, desc, info.VarCount, info.RepoCount, info.HookCount)
		}
		w.Flush()

		return nil
	},
}

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show template details",
	Long:  `Shows detailed information about a specific template including variables, repos, and hooks.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		tmpl, err := template.LoadTemplate(cfg.TemplatesDir(), args[0])
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(tmpl)
		}

		fmt.Printf("Template: %s\n", tmpl.Name)
		fmt.Printf("Description: %s\n", tmpl.Description)
		if tmpl.Version != "" {
			fmt.Printf("Version: %s\n", tmpl.Version)
		}
		fmt.Println()

		if len(tmpl.Variables) > 0 {
			fmt.Println("Variables:")
			for _, v := range tmpl.Variables {
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

		if len(tmpl.Repos) > 0 {
			fmt.Println("Repositories:")
			for _, r := range tmpl.Repos {
				if r.CloneURL != "" {
					fmt.Printf("  - %s (clone: %s)\n", r.Name, r.CloneURL)
				} else if r.Init {
					branch := r.DefaultBranch
					if branch == "" {
						branch = "main"
					}
					fmt.Printf("  - %s (init, branch: %s)\n", r.Name, branch)
				}
			}
			fmt.Println()
		}

		hooks := template.ListHooks(tmpl)
		if len(hooks) > 0 {
			fmt.Println("Hooks:")
			for _, h := range hooks {
				spec := template.GetHookSpec(tmpl, h)
				timeout := spec.Timeout
				if timeout == "" {
					timeout = template.DefaultHookTimeout
				}
				fmt.Printf("  - %s: %s (timeout: %s)\n", h, spec.Script, timeout)
			}
			fmt.Println()
		}

		if len(tmpl.Tags) > 0 {
			fmt.Printf("Default tags: %v\n", tmpl.Tags)
		}
		if tmpl.State != "" {
			fmt.Printf("Default state: %s\n", tmpl.State)
		}

		return nil
	},
}

var templateValidateCmd = &cobra.Command{
	Use:   "validate [name]",
	Short: "Validate templates",
	Long:  `Validates one or all templates, checking for errors in the manifest and missing files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(args) > 0 {
			// Validate specific template
			err := template.ValidateTemplateDir(cfg.TemplatesDir(), args[0])
			if err != nil {
				return fmt.Errorf("validation failed for %s: %w", args[0], err)
			}
			fmt.Printf("Template %s is valid\n", args[0])
			return nil
		}

		// Validate all templates
		templates, err := template.ListTemplates(cfg.TemplatesDir())
		if err != nil {
			return fmt.Errorf("failed to list templates: %w", err)
		}

		if len(templates) == 0 {
			fmt.Println("No templates to validate")
			return nil
		}

		hasErrors := false
		for _, tmpl := range templates {
			err := template.ValidateTemplateDir(cfg.TemplatesDir(), tmpl.Name)
			if err != nil {
				fmt.Printf("✗ %s: %v\n", tmpl.Name, err)
				hasErrors = true
			} else {
				fmt.Printf("✓ %s\n", tmpl.Name)
			}
		}

		if hasErrors {
			return fmt.Errorf("some templates have errors")
		}

		fmt.Printf("\nAll %d templates are valid\n", len(templates))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(templateCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateValidateCmd)
}
