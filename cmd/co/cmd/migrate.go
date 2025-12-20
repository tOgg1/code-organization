package cmd

import (
	"bufio"
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
	migrateOwner        string
	migrateProject      string
	migrateDryRun       bool
	migrateAddTo        string
	migrateTemplateName string
	migrateTemplateVars []string
	migrateNoHooks      bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate <folder-path>",
	Short: "Migrate an existing folder to a new workspace",
	Long: `Migrates an existing folder containing code into a proper workspace.

Detects git repositories within the folder and creates the standard
workspace structure (project.json + repos/).

If the source folder itself is a git repo, it becomes the single repo.
If the source contains multiple git repos, each becomes a separate repo.

Use --add-to to add repos to an existing workspace instead of creating a new one.

Template Support:
  -t, --template <name>  Apply a template after migration
  -v, --var <key=value>  Set template variable (can be repeated)
      --no-hooks         Skip running lifecycle hooks`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourcePath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
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

		gitRoots, err := git.FindGitRoots(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to scan for git repos: %w", err)
		}

		if len(gitRoots) == 0 {
			return fmt.Errorf("no git repositories found in %s", sourcePath)
		}

		if migrateAddTo != "" {
			return runAddToWorkspace(cfg, sourcePath, gitRoots)
		}

		return runCreateWorkspace(cfg, sourcePath, gitRoots)
	},
}

func runAddToWorkspace(cfg *config.Config, sourcePath string, gitRoots []string) error {
	slug := migrateAddTo
	if !fs.IsValidWorkspaceSlug(slug) {
		return fmt.Errorf("invalid workspace slug: %s", slug)
	}

	if !fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return fmt.Errorf("workspace does not exist: %s", slug)
	}

	workspacePath := filepath.Join(cfg.CodeRoot, slug)
	reposPath := filepath.Join(workspacePath, "repos")

	proj, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		return fmt.Errorf("failed to load project.json: %w", err)
	}

	existingRepos := make(map[string]bool)
	for _, r := range proj.Repos {
		existingRepos[r.Name] = true
	}

	// Check for non-git files/folders to offer inclusion
	var extraFilesResult tui.ExtraFilesResult
	if !migrateDryRun {
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
	}

	if migrateDryRun {
		fmt.Printf("Dry run - would add to workspace: %s\n", slug)
		for _, root := range gitRoots {
			repoName := deriveRepoNameFromPath(root, sourcePath)
			if existingRepos[repoName] {
				fmt.Printf("  Skip %s (already exists)\n", repoName)
			} else {
				fmt.Printf("  Move %s -> repos/%s\n", root, repoName)
			}
		}
		return nil
	}

	added := 0
	for _, root := range gitRoots {
		repoName := deriveRepoNameFromPath(root, sourcePath)
		destPath := filepath.Join(reposPath, repoName)

		if existingRepos[repoName] {
			fmt.Printf("Skipping %s (already exists)\n", repoName)
			continue
		}

		fmt.Printf("Moving %s -> repos/%s\n", root, repoName)
		if err := os.Rename(root, destPath); err != nil {
			if isCrossDevice(err) {
				if err := copyDir(root, destPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to copy %s: %v\n", root, err)
					continue
				}
				os.RemoveAll(root)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to move %s: %v\n", root, err)
				continue
			}
		}

		remote := ""
		if info, err := git.GetInfo(destPath); err == nil && info.Remote != "" {
			remote = info.Remote
		}
		proj.AddRepo(repoName, "repos/"+repoName, remote)
		added++
	}

	if added > 0 {
		if err := proj.Save(workspacePath); err != nil {
			return fmt.Errorf("failed to save project.json: %w", err)
		}
	}

	// Copy selected extra files to workspace
	if extraFilesResult.Confirmed && len(extraFilesResult.SelectedPaths) > 0 {
		if err := copyExtraFiles(sourcePath, workspacePath, extraFilesResult); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to copy some extra files: %v\n", err)
		}
	}

	isEmpty, _ := isDirEmpty(sourcePath)
	if isEmpty {
		os.Remove(sourcePath)
		fmt.Printf("Removed empty source directory: %s\n", sourcePath)
	}

	fmt.Printf("\nAdded %d repo(s) to workspace: %s\n", added, slug)
	fmt.Printf("Run 'co index' to update the index.\n")
	return nil
}

func runCreateWorkspace(cfg *config.Config, sourcePath string, gitRoots []string) error {
	suggestedOwner := migrateOwner
	suggestedProject := migrateProject

	if suggestedProject == "" {
		suggestedProject = strings.ToLower(filepath.Base(sourcePath))
		suggestedProject = sanitizeSlugPart(suggestedProject)
	}

	var owner, project string

	if migrateOwner != "" && migrateProject != "" {
		owner = strings.ToLower(migrateOwner)
		project = strings.ToLower(migrateProject)
	} else {
		result, err := tui.RunMigratePrompt(sourcePath, gitRoots, suggestedOwner, suggestedProject)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if result.Abort {
			fmt.Println("Migration cancelled.")
			return nil
		}
		owner = result.Owner
		project = result.Project
	}

	slug := owner + "--" + project
	if !fs.IsValidWorkspaceSlug(slug) {
		return fmt.Errorf("invalid workspace slug: %s", slug)
	}

	if fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return fmt.Errorf("workspace already exists: %s", slug)
	}

	workspacePath := filepath.Join(cfg.CodeRoot, slug)
	reposPath := filepath.Join(workspacePath, "repos")

	// Check for non-git files/folders to offer inclusion
	var extraFilesResult tui.ExtraFilesResult
	if !migrateDryRun {
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
	}

	if migrateDryRun {
		fmt.Println("Dry run - would perform:")
		fmt.Printf("  Create workspace: %s\n", workspacePath)
		fmt.Printf("  Create repos dir: %s\n", reposPath)
		for _, root := range gitRoots {
			repoName := deriveRepoNameFromPath(root, sourcePath)
			fmt.Printf("  Move %s -> repos/%s\n", root, repoName)
		}
		return nil
	}

	if err := os.MkdirAll(reposPath, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	proj := model.NewProject(owner, project)

	for _, root := range gitRoots {
		repoName := deriveRepoNameFromPath(root, sourcePath)
		destPath := filepath.Join(reposPath, repoName)

		fmt.Printf("Moving %s -> repos/%s\n", root, repoName)
		if err := os.Rename(root, destPath); err != nil {
			if isCrossDevice(err) {
				if err := copyDir(root, destPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to copy %s: %v\n", root, err)
					continue
				}
				os.RemoveAll(root)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to move %s: %v\n", root, err)
				continue
			}
		}

		remote := ""
		if info, err := git.GetInfo(destPath); err == nil && info.Remote != "" {
			remote = info.Remote
		}
		proj.AddRepo(repoName, "repos/"+repoName, remote)
	}

	if err := proj.Save(workspacePath); err != nil {
		return fmt.Errorf("failed to save project.json: %w", err)
	}

	// Copy selected extra files to workspace
	if extraFilesResult.Confirmed && len(extraFilesResult.SelectedPaths) > 0 {
		if err := copyExtraFiles(sourcePath, workspacePath, extraFilesResult); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to copy some extra files: %v\n", err)
		}
	}

	isEmpty, _ := isDirEmpty(sourcePath)
	if isEmpty {
		os.Remove(sourcePath)
		fmt.Printf("Removed empty source directory: %s\n", sourcePath)
	} else {
		fmt.Printf("Note: source directory not empty, keeping: %s\n", sourcePath)
	}

	fmt.Printf("\nCreated workspace: %s\n", workspacePath)

	// Apply template if specified
	if migrateTemplateName != "" {
		fmt.Printf("\nApplying template: %s\n", migrateTemplateName)
		if err := applyMigrateTemplate(cfg, workspacePath); err != nil {
			return fmt.Errorf("failed to apply template: %w", err)
		}
	}

	fmt.Printf("Run 'co index' to update the index.\n")
	return nil
}

// copyExtraFiles copies the selected non-git files/folders to the workspace.
func copyExtraFiles(sourcePath, workspacePath string, result tui.ExtraFilesResult) error {
	destBase := workspacePath
	if result.DestSubfolder != "" {
		destBase = filepath.Join(workspacePath, result.DestSubfolder)
		if err := os.MkdirAll(destBase, 0755); err != nil {
			return fmt.Errorf("failed to create destination subfolder: %w", err)
		}
	}

	for _, relPath := range result.SelectedPaths {
		srcPath := filepath.Join(sourcePath, relPath)
		dstPath := filepath.Join(destBase, relPath)

		info, err := os.Stat(srcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot access %s: %v\n", relPath, err)
			continue
		}

		if info.IsDir() {
			fmt.Printf("Copying %s/ -> %s/\n", relPath, filepath.Join(result.DestSubfolder, relPath))
			if err := copyDir(srcPath, dstPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to copy directory %s: %v\n", relPath, err)
				continue
			}
		} else {
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create parent dir for %s: %v\n", relPath, err)
				continue
			}
			fmt.Printf("Copying %s -> %s\n", relPath, filepath.Join(result.DestSubfolder, relPath))
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to copy file %s: %v\n", relPath, err)
				continue
			}
		}

		// Remove the source after successful copy
		if err := os.RemoveAll(srcPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove source %s: %v\n", relPath, err)
		}
	}

	return nil
}

func applyMigrateTemplate(cfg *config.Config, workspacePath string) error {
	// Load template to check for required variables
	tmpl, err := template.LoadTemplate(cfg.TemplatesDir(), migrateTemplateName)
	if err != nil {
		return err
	}

	// Parse provided variables
	providedVars := parseMigrateVarFlags(migrateTemplateVars)

	// Get built-in variables
	slug := filepath.Base(workspacePath)
	owner, project := parseSlugForMigrate(slug)
	builtins := template.GetBuiltinVariables(owner, project, workspacePath, cfg.CodeRoot)

	// Check for missing required variables and prompt
	missing := template.GetMissingRequiredVars(tmpl, providedVars, builtins)
	if len(missing) > 0 {
		fmt.Printf("Template '%s' requires the following variables:\n\n", migrateTemplateName)
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
		TemplateName: migrateTemplateName,
		Variables:    providedVars,
		NoHooks:      migrateNoHooks,
		DryRun:       migrateDryRun,
		Verbose:      true,
	}

	result, err := template.ApplyTemplateToExisting(cfg, workspacePath, migrateTemplateName, opts)
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

func parseMigrateVarFlags(vars []string) map[string]string {
	result := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func parseSlugForMigrate(slug string) (owner, project string) {
	parts := strings.SplitN(slug, "--", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return slug, slug
}

func deriveRepoNameFromPath(repoPath, sourcePath string) string {
	if repoPath == sourcePath {
		return filepath.Base(sourcePath)
	}

	rel, err := filepath.Rel(sourcePath, repoPath)
	if err != nil {
		return filepath.Base(repoPath)
	}

	name := strings.ReplaceAll(rel, string(filepath.Separator), "-")
	return sanitizeSlugPart(name)
}

func sanitizeSlugPart(s string) string {
	s = strings.ToLower(s)
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		} else if c == '_' || c == ' ' {
			result.WriteRune('-')
		}
	}
	return result.String()
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func isCrossDevice(err error) bool {
	return strings.Contains(err.Error(), "cross-device") ||
		strings.Contains(err.Error(), "invalid cross-device link")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVarP(&migrateOwner, "owner", "o", "", "workspace owner (skip prompt)")
	migrateCmd.Flags().StringVarP(&migrateProject, "project", "p", "", "project name (skip prompt)")
	migrateCmd.Flags().StringVar(&migrateAddTo, "add-to", "", "add repos to existing workspace instead of creating new")
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "show what would be done without making changes")
	migrateCmd.Flags().StringVarP(&migrateTemplateName, "template", "t", "", "Template to apply after migration")
	migrateCmd.Flags().StringArrayVarP(&migrateTemplateVars, "var", "v", nil, "Set template variable (key=value)")
	migrateCmd.Flags().BoolVar(&migrateNoHooks, "no-hooks", false, "Skip running lifecycle hooks")
}
