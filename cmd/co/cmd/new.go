package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
	"github.com/tormodhaugland/co/internal/template"
	"github.com/tormodhaugland/co/internal/tui"
)

var (
	newTemplateName  string
	newTemplateVars  []string
	newNoHooks       bool
	newDryRun        bool
	newListTemplates bool
	newShowTemplate  string
)

var newCmd = &cobra.Command{
	Use:   "new [owner] [project] [repo-url...]",
	Short: "Create a new workspace",
	Long: `Creates a new workspace with project.json and repos/ directory.
If repo URLs are provided, clones them into repos/<derived-name>/.
If owner and project are not provided, prompts interactively.

Template Support:
  -t, --template <name>  Use a template for workspace creation
  -v, --var <key=value>  Set template variable (can be repeated)
      --no-hooks         Skip running lifecycle hooks
      --dry-run          Preview creation without making changes
      --list-templates   List available templates
      --show-template    Show template details`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Handle --list-templates
		if newListTemplates {
			return listTemplates(cfg)
		}

		// Handle --show-template
		if newShowTemplate != "" {
			return showTemplate(cfg, newShowTemplate)
		}

		var owner, project string
		var repoURLs []string

		if len(args) >= 2 {
			owner = strings.ToLower(args[0])
			project = strings.ToLower(args[1])
			repoURLs = args[2:]
		} else {
			result, err := tui.RunNewPrompt()
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if result.Abort {
				return fmt.Errorf("aborted")
			}
			owner = result.Owner
			project = result.Project
		}

		slug := owner + "--" + project
		if !fs.IsValidWorkspaceSlug(slug) {
			return fmt.Errorf("invalid workspace slug: %s (must be lowercase alphanumeric with hyphens)", slug)
		}

		if fs.WorkspaceExists(cfg.CodeRoot, slug) && !newDryRun {
			return fmt.Errorf("workspace already exists: %s", slug)
		}

		// If template is specified, use template-based creation
		if newTemplateName != "" {
			return createWithTemplate(cfg, owner, project, repoURLs)
		}

		// Non-template creation (original flow)
		workspacePath, err := fs.CreateWorkspace(cfg.CodeRoot, slug)
		if err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		proj := model.NewProject(owner, project)

		for _, url := range repoURLs {
			repoName := deriveRepoName(url)
			repoPath := filepath.Join(workspacePath, "repos", repoName)

			fmt.Printf("Cloning %s into repos/%s...\n", url, repoName)
			if err := git.Clone(url, repoPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clone %s: %v\n", url, err)
				continue
			}

			proj.AddRepo(repoName, "repos/"+repoName, url)
		}

		if err := proj.Save(workspacePath); err != nil {
			return fmt.Errorf("failed to save project.json: %w", err)
		}

		fmt.Printf("Created workspace: %s\n", workspacePath)
		return nil
	},
}

func createWithTemplate(cfg *config.Config, owner, project string, extraRepoURLs []string) error {
	// Load template to check variables
	tmpl, err := template.LoadTemplate(cfg.TemplatesDir(), newTemplateName)
	if err != nil {
		return err
	}

	// Parse provided variables
	providedVars := parseVarFlags(newTemplateVars)

	// Get built-in variables for checking
	builtins := template.GetBuiltinVariables(owner, project, cfg.WorkspacePath(owner+"--"+project), cfg.CodeRoot)

	// Check for missing required variables and prompt
	missing := template.GetMissingRequiredVars(tmpl, providedVars, builtins)
	if len(missing) > 0 {
		fmt.Printf("Template '%s' requires the following variables:\n\n", newTemplateName)
		reader := bufio.NewReader(os.Stdin)

		for _, v := range missing {
			fmt.Printf("%s", v.Name)
			if v.Description != "" {
				fmt.Printf(" (%s)", v.Description)
			}
			if v.Type == template.VarTypeChoice && len(v.Choices) > 0 {
				fmt.Printf(" [choices: %s]", strings.Join(v.Choices, ", "))
			}
			fmt.Print(": ")

			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			input = strings.TrimSpace(input)
			if input == "" {
				return fmt.Errorf("required variable %s not provided", v.Name)
			}
			providedVars[v.Name] = input
		}
		fmt.Println()
	}

	// Create workspace with template
	opts := template.CreateOptions{
		TemplateName: newTemplateName,
		Variables:    providedVars,
		NoHooks:      newNoHooks,
		DryRun:       newDryRun,
		Verbose:      true,
	}

	result, err := template.CreateWorkspace(cfg, owner, project, opts)
	if err != nil {
		return err
	}

	// Handle extra repo URLs not in template
	if len(extraRepoURLs) > 0 && !newDryRun {
		for _, url := range extraRepoURLs {
			repoName := deriveRepoName(url)
			repoPath := filepath.Join(result.WorkspacePath, "repos", repoName)

			fmt.Printf("Cloning %s into repos/%s...\n", url, repoName)
			if err := git.Clone(url, repoPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clone %s: %v\n", url, err)
			}
		}
	}

	// Output result
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if newDryRun {
		fmt.Println("Dry run - no changes made")
		fmt.Printf("Would create workspace: %s\n", result.WorkspacePath)
		fmt.Printf("Template: %s\n", result.TemplateUsed)
		fmt.Printf("Files: %d global + %d template = %d total\n",
			result.GlobalFiles, result.TemplateFiles, result.FilesCreated)
		fmt.Printf("Repos: %d\n", result.ReposCreated)
		return nil
	}

	fmt.Printf("Created workspace: %s\n", result.WorkspacePath)
	fmt.Printf("  Template: %s\n", result.TemplateUsed)
	fmt.Printf("  Files created: %d\n", result.FilesCreated)
	if result.ReposCreated > 0 {
		fmt.Printf("  Repos initialized: %d\n", result.ReposCreated)
	}
	if result.ReposCloned > 0 {
		fmt.Printf("  Repos cloned: %d\n", result.ReposCloned)
	}
	if len(result.HooksRun) > 0 {
		fmt.Printf("  Hooks run: %s\n", strings.Join(result.HooksRun, ", "))
	}
	if len(result.Warnings) > 0 {
		fmt.Println("  Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("    - %s\n", w)
		}
	}

	return nil
}

func parseVarFlags(vars []string) map[string]string {
	result := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func listTemplates(cfg *config.Config) error {
	templates, err := template.ListTemplates(cfg.TemplatesDir())
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	if len(templates) == 0 {
		fmt.Println("No templates found")
		fmt.Printf("\nTemplates directory: %s\n", cfg.TemplatesDir())
		return nil
	}

	fmt.Println("Available templates:")
	for _, tmpl := range templates {
		fmt.Printf("  %s - %s\n", tmpl.Name, tmpl.Description)
	}
	return nil
}

func showTemplate(cfg *config.Config, name string) error {
	tmpl, err := template.LoadTemplate(cfg.TemplatesDir(), name)
	if err != nil {
		return err
	}

	fmt.Printf("Template: %s\n", tmpl.Name)
	fmt.Printf("Description: %s\n", tmpl.Description)

	if len(tmpl.Variables) > 0 {
		fmt.Println("\nVariables:")
		for _, v := range tmpl.Variables {
			required := ""
			if v.Required {
				required = " (required)"
			}
			fmt.Printf("  %s%s: %s\n", v.Name, required, v.Description)
		}
	}

	return nil
}

func deriveRepoName(url string) string {
	url = strings.TrimSuffix(url, ".git")

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	parts = strings.Split(url, ":")
	if len(parts) > 1 {
		subparts := strings.Split(parts[1], "/")
		if len(subparts) > 0 {
			return subparts[len(subparts)-1]
		}
	}

	return "repo"
}

func init() {
	rootCmd.AddCommand(newCmd)

	newCmd.Flags().StringVarP(&newTemplateName, "template", "t", "", "Template to use for workspace creation")
	newCmd.Flags().StringArrayVarP(&newTemplateVars, "var", "v", nil, "Set template variable (key=value)")
	newCmd.Flags().BoolVar(&newNoHooks, "no-hooks", false, "Skip running lifecycle hooks")
	newCmd.Flags().BoolVar(&newDryRun, "dry-run", false, "Preview creation without making changes")
	newCmd.Flags().BoolVar(&newListTemplates, "list-templates", false, "List available templates")
	newCmd.Flags().StringVar(&newShowTemplate, "show-template", "", "Show template details")
}
