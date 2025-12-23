package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/template"
	"github.com/tormodhaugland/co/internal/tui"
	"github.com/tormodhaugland/co/internal/workspace"
)

var (
	importOwner        string
	importProject      string
	importDryRun       bool
	importAddTo        string
	importTemplateName string
	importTemplateVars []string
	importNoHooks      bool
	importInteractive  bool
)

var importCmd = &cobra.Command{
	Use:   "import <folder-path>",
	Short: "Import an existing folder into a new workspace",
	Long: `Imports an existing folder containing code into a proper workspace.

Detects git repositories within the folder and creates the standard
workspace structure (project.json + repos/).

If the source folder itself is a git repo, it becomes the single repo.
If the source contains multiple git repos, each becomes a separate repo.
Non-git files and folders can also be included via an interactive picker.

Use --add-to to add repos to an existing workspace instead of creating a new one.
Use -i/--interactive to launch a visual file browser for selecting folders to import.

Template Support:
  -t, --template <name>  Apply a template after import
  -v, --var <key=value>  Set template variable (can be repeated)
      --no-hooks         Skip running lifecycle hooks`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine source path
		var sourcePath string
		var err error
		if len(args) > 0 {
			sourcePath, err = filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
		} else {
			// No path provided - use current directory
			sourcePath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("cannot access path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", sourcePath)
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Interactive mode - launch import browser TUI
		if importInteractive {
			result, err := tui.RunImportBrowser(cfg, sourcePath)
			if err != nil {
				return fmt.Errorf("import browser failed: %w", err)
			}
			if result.Aborted {
				fmt.Println("Import cancelled.")
				return nil
			}
			if result.Error != nil {
				return result.Error
			}
			if result.Success {
				switch result.Action {
				case "import":
					fmt.Printf("Created workspace: %s\n", result.WorkspacePath)
				case "stash":
					fmt.Printf("Stashed: %s\n", result.ArchivePath)
				case "add-to":
					fmt.Printf("Added to workspace: %s\n", result.WorkspacePath)
				}
			}
			return nil
		}

		// Non-interactive mode requires a path argument
		if len(args) == 0 {
			return fmt.Errorf("folder path required (or use -i/--interactive for visual browser)")
		}

		gitRoots, err := git.FindGitRoots(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to scan for git repos: %w", err)
		}

		// Check if the source folder has any content at all
		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source directory: %w", err)
		}
		if len(entries) == 0 {
			return fmt.Errorf("source directory is empty: %s", sourcePath)
		}

		if importAddTo != "" {
			return runAddToWorkspace(cfg, sourcePath, gitRoots)
		}

		return runCreateWorkspace(cfg, sourcePath, gitRoots)
	},
}

func runAddToWorkspace(cfg *config.Config, sourcePath string, gitRoots []string) error {
	slug := importAddTo

	// Check for non-git files/folders to offer inclusion
	var extraFilesResult tui.ExtraFilesResult
	if !importDryRun {
		nonGitItems, err := tui.FindNonGitItems(sourcePath, gitRoots)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan for non-git files: %v\n", err)
		} else if len(nonGitItems) > 0 {
			extraFilesResult, err = tui.RunExtraFilesPicker(sourcePath, nonGitItems)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: extra files picker failed: %v\n", err)
			}
			if extraFilesResult.Aborted {
				fmt.Println("Import cancelled.")
				return nil
			}
		}
	}

	if importDryRun {
		fmt.Printf("Dry run - would add to workspace: %s\n", slug)
		for _, root := range gitRoots {
			repoName := workspace.DeriveRepoName(root, sourcePath)
			fmt.Printf("  Move %s -> repos/%s\n", root, repoName)
		}
		return nil
	}

	opts := workspace.ImportOptions{
		ExtraFiles:     extraFilesResult.SelectedPaths,
		ExtraFilesDest: extraFilesResult.DestSubfolder,
		OnRepoMove: func(repoName, srcPath, dstPath string) {
			fmt.Printf("Moving %s -> repos/%s\n", srcPath, repoName)
		},
		OnRepoSkip: func(repoName, reason string) {
			fmt.Printf("Skipping %s (%s)\n", repoName, reason)
		},
		OnFileCopy: func(relPath, dstPath string) {
			fmt.Printf("Copying %s\n", relPath)
		},
		OnWarning: func(msg string) {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
		},
	}

	result, err := workspace.AddToWorkspace(cfg, sourcePath, gitRoots, slug, opts)
	if err != nil {
		return err
	}

	// Print any errors encountered
	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", e)
	}

	if result.SourceEmpty {
		if workspace.RemoveEmptySource(sourcePath) {
			fmt.Printf("Removed empty source directory: %s\n", sourcePath)
		}
	}

	fmt.Printf("\nAdded %d repo(s) to workspace: %s\n", len(result.ReposImported), slug)
	if len(result.ReposSkipped) > 0 {
		fmt.Printf("Skipped %d repo(s) (already exist)\n", len(result.ReposSkipped))
	}
	fmt.Printf("Run 'co index' to update the index.\n")
	return nil
}

func runCreateWorkspace(cfg *config.Config, sourcePath string, gitRoots []string) error {
	suggestedOwner := importOwner
	suggestedProject := importProject

	if suggestedProject == "" {
		suggestedProject = strings.ToLower(filepath.Base(sourcePath))
		suggestedProject = workspace.SanitizeSlugPart(suggestedProject)
	}

	var owner, project string

	if importOwner != "" && importProject != "" {
		owner = strings.ToLower(importOwner)
		project = strings.ToLower(importProject)
	} else {
		result, err := tui.RunImportPrompt(sourcePath, gitRoots, suggestedOwner, suggestedProject)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if result.Abort {
			fmt.Println("Import cancelled.")
			return nil
		}
		owner = result.Owner
		project = result.Project
	}

	slug := owner + "--" + project
	workspacePath := filepath.Join(cfg.CodeRoot, slug)
	reposPath := filepath.Join(workspacePath, "repos")

	// Check for non-git files/folders to offer inclusion
	var extraFilesResult tui.ExtraFilesResult
	if !importDryRun {
		nonGitItems, err := tui.FindNonGitItems(sourcePath, gitRoots)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan for non-git files: %v\n", err)
		} else if len(nonGitItems) > 0 {
			extraFilesResult, err = tui.RunExtraFilesPicker(sourcePath, nonGitItems)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: extra files picker failed: %v\n", err)
			}
			if extraFilesResult.Aborted {
				fmt.Println("Migration cancelled.")
				return nil
			}
		}

		// If no git repos and no files selected, nothing to import
		if len(gitRoots) == 0 && len(extraFilesResult.SelectedPaths) == 0 {
			fmt.Println("No git repositories found and no files selected. Nothing to import.")
			return nil
		}
	}

	if importDryRun {
		fmt.Println("Dry run - would perform:")
		fmt.Printf("  Create workspace: %s\n", workspacePath)
		fmt.Printf("  Create repos dir: %s\n", reposPath)
		for _, root := range gitRoots {
			repoName := workspace.DeriveRepoName(root, sourcePath)
			fmt.Printf("  Move %s -> repos/%s\n", root, repoName)
		}
		return nil
	}

	opts := workspace.ImportOptions{
		Owner:          owner,
		Project:        project,
		ExtraFiles:     extraFilesResult.SelectedPaths,
		ExtraFilesDest: extraFilesResult.DestSubfolder,
		OnRepoMove: func(repoName, srcPath, dstPath string) {
			fmt.Printf("Moving %s -> repos/%s\n", srcPath, repoName)
		},
		OnFileCopy: func(relPath, dstPath string) {
			fmt.Printf("Copying %s\n", relPath)
		},
		OnWarning: func(msg string) {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
		},
	}

	result, err := workspace.CreateWorkspace(cfg, sourcePath, gitRoots, opts)
	if err != nil {
		return err
	}

	// Print any errors encountered
	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", e)
	}

	if result.SourceEmpty {
		if workspace.RemoveEmptySource(sourcePath) {
			fmt.Printf("Removed empty source directory: %s\n", sourcePath)
		}
	} else {
		fmt.Printf("Note: source directory not empty, keeping: %s\n", sourcePath)
	}

	fmt.Printf("\nCreated workspace: %s\n", result.WorkspacePath)

	// Apply template if specified
	if importTemplateName != "" {
		fmt.Printf("\nApplying template: %s\n", importTemplateName)
		if err := applyImportTemplate(cfg, result.WorkspacePath); err != nil {
			return fmt.Errorf("failed to apply template: %w", err)
		}
	}

	fmt.Printf("Run 'co index' to update the index.\n")
	return nil
}

func applyImportTemplate(cfg *config.Config, workspacePath string) error {
	// Load template to check for required variables
	tmpl, err := template.LoadTemplate(cfg.TemplatesDir(), importTemplateName)
	if err != nil {
		return err
	}

	// Parse provided variables
	providedVars := parseImportVarFlags(importTemplateVars)

	// Get built-in variables
	slug := filepath.Base(workspacePath)
	owner, project := parseSlugForImport(slug)
	builtins := template.GetBuiltinVariables(owner, project, workspacePath, cfg.CodeRoot)

	// Check for missing required variables and prompt
	missing := template.GetMissingRequiredVars(tmpl, providedVars, builtins)
	if len(missing) > 0 {
		fmt.Printf("Template '%s' requires the following variables:\n\n", importTemplateName)
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

	// Apply template to existing workspace
	opts := template.CreateOptions{
		TemplateName: importTemplateName,
		Variables:    providedVars,
		NoHooks:      importNoHooks,
		DryRun:       importDryRun,
		Verbose:      true,
	}

	result, err := template.ApplyTemplateToExisting(cfg, workspacePath, importTemplateName, opts)
	if err != nil {
		return err
	}

	// Output result
	fmt.Printf("  Files created: %d\n", result.FilesCreated)
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

func parseImportVarFlags(vars []string) map[string]string {
	result := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func parseSlugForImport(slug string) (owner, project string) {
	parts := strings.SplitN(slug, "--", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return slug, slug
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().BoolVarP(&importInteractive, "interactive", "i", false, "launch visual import browser")
	importCmd.Flags().StringVarP(&importOwner, "owner", "o", "", "workspace owner (skip prompt)")
	importCmd.Flags().StringVarP(&importProject, "project", "p", "", "project name (skip prompt)")
	importCmd.Flags().StringVar(&importAddTo, "add-to", "", "add repos to existing workspace instead of creating new")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "show what would be done without making changes")
	importCmd.Flags().StringVarP(&importTemplateName, "template", "t", "", "Template to apply after import")
	importCmd.Flags().StringArrayVarP(&importTemplateVars, "var", "v", nil, "Set template variable (key=value)")
	importCmd.Flags().BoolVar(&importNoHooks, "no-hooks", false, "Skip running lifecycle hooks")
}
