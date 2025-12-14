package template

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
)

// CreateWorkspace creates a new workspace using a template.
func CreateWorkspace(cfg *config.Config, owner, project string, opts CreateOptions) (*CreateResult, error) {
	result := &CreateResult{
		WorkspaceSlug: owner + "--" + project,
	}

	// Load template
	tmpl, err := LoadTemplate(cfg.TemplatesDir(), opts.TemplateName)
	if err != nil {
		return nil, err
	}

	templatePath := filepath.Join(cfg.TemplatesDir(), opts.TemplateName)
	workspacePath := cfg.WorkspacePath(result.WorkspaceSlug)
	reposPath := filepath.Join(workspacePath, "repos")

	result.WorkspacePath = workspacePath
	result.TemplateUsed = opts.TemplateName

	// Get built-in variables
	builtins := GetBuiltinVariables(owner, project, workspacePath, cfg.CodeRoot)

	// Resolve all variables
	vars, err := ResolveVariables(tmpl, opts.Variables, builtins)
	if err != nil {
		return nil, fmt.Errorf("resolving variables: %w", err)
	}

	// Create hook environment
	hookEnv := HookEnv{
		WorkspacePath: workspacePath,
		WorkspaceSlug: result.WorkspaceSlug,
		Owner:         owner,
		Project:       project,
		CodeRoot:      cfg.CodeRoot,
		TemplateName:  opts.TemplateName,
		TemplatePath:  templatePath,
		ReposPath:     reposPath,
		DryRun:        opts.DryRun,
		Verbose:       opts.Verbose,
		Variables:     vars,
	}

	// Set up output writer (nil for no output, os.Stdout for verbose)
	var output io.Writer
	if opts.Verbose {
		output = os.Stdout
	}

	// Run pre_create hook
	if !opts.NoHooks && HasHook(tmpl, HookPreCreate) {
		hookResult, err := RunHook(HookPreCreate, tmpl.Hooks.PreCreate, templatePath, hookEnv, output)
		if err != nil {
			return result, fmt.Errorf("pre_create hook failed: %w", err)
		}
		if !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, string(HookPreCreate))
		}
	}

	if opts.DryRun {
		// In dry-run mode, just return what would be created
		globalFiles, _ := ListGlobalFiles(cfg.TemplatesDir())
		templateFiles, _ := ListTemplateFiles(tmpl, templatePath)
		result.GlobalFiles = len(globalFiles)
		result.TemplateFiles = len(templateFiles)
		result.FilesCreated = result.GlobalFiles + result.TemplateFiles
		result.ReposCreated = len(tmpl.Repos)
		result.Warnings = append(result.Warnings, "Dry run - no changes made")
		return result, nil
	}

	// Create workspace directory
	workspacePath, err = fs.CreateWorkspace(cfg.CodeRoot, result.WorkspaceSlug)
	if err != nil {
		return result, fmt.Errorf("creating workspace: %w", err)
	}
	result.WorkspacePath = workspacePath

	// Process files
	globalCount, templateCount, err := ProcessAllFiles(tmpl, cfg.TemplatesDir(), templatePath, workspacePath, vars)
	if err != nil {
		return result, fmt.Errorf("processing files: %w", err)
	}
	result.GlobalFiles = globalCount
	result.TemplateFiles = templateCount
	result.FilesCreated = globalCount + templateCount

	// Run post_create hook
	if !opts.NoHooks && HasHook(tmpl, HookPostCreate) {
		hookResult, err := RunHook(HookPostCreate, tmpl.Hooks.PostCreate, templatePath, hookEnv, output)
		if err != nil {
			return result, fmt.Errorf("post_create hook failed: %w", err)
		}
		if !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, string(HookPostCreate))
			hookEnv.PrevHookOutput = hookResult.Output
		}
	}

	// Create/clone repositories
	for _, repoSpec := range tmpl.Repos {
		repoPath := filepath.Join(reposPath, repoSpec.Name)

		if repoSpec.CloneURL != "" {
			// Clone repository
			if err := git.Clone(repoSpec.CloneURL, repoPath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to clone %s: %v", repoSpec.Name, err))
				continue
			}
			result.ReposCloned++
		} else if repoSpec.Init {
			// Initialize new repository
			if err := os.MkdirAll(repoPath, 0755); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to create %s: %v", repoSpec.Name, err))
				continue
			}
			result.ReposCreated++
		}
	}

	// Run post_clone hook
	if !opts.NoHooks && HasHook(tmpl, HookPostClone) {
		hookResult, err := RunHook(HookPostClone, tmpl.Hooks.PostClone, templatePath, hookEnv, output)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("post_clone hook failed: %v", err))
		} else if !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, string(HookPostClone))
			hookEnv.PrevHookOutput = hookResult.Output
		}
	}

	// Create project.json
	proj := model.NewProject(owner, project)
	proj.Template = opts.TemplateName
	proj.TemplateVars = vars

	// Apply template defaults
	if len(tmpl.Tags) > 0 {
		proj.Tags = tmpl.Tags
	}
	if tmpl.State != "" {
		proj.State = tmpl.State
	}

	// Add repo specs
	for _, repoSpec := range tmpl.Repos {
		proj.AddRepo(repoSpec.Name, filepath.Join("repos", repoSpec.Name), repoSpec.CloneURL)
	}

	if err := proj.Save(workspacePath); err != nil {
		return result, fmt.Errorf("saving project.json: %w", err)
	}

	// Run post_complete hook
	if !opts.NoHooks && HasHook(tmpl, HookPostComplete) {
		hookResult, err := RunHook(HookPostComplete, tmpl.Hooks.PostComplete, templatePath, hookEnv, output)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("post_complete hook failed: %v", err))
		} else if !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, string(HookPostComplete))
		}
	}

	// Cleanup
	CleanupHookOutputFile(workspacePath)

	return result, nil
}

// ApplyTemplateToExisting applies template files to an existing workspace.
// Used by co migrate --template.
func ApplyTemplateToExisting(cfg *config.Config, workspacePath, templateName string, opts CreateOptions) (*CreateResult, error) {
	result := &CreateResult{
		WorkspacePath: workspacePath,
		TemplateUsed:  templateName,
	}

	// Extract owner and project from path
	slug := filepath.Base(workspacePath)
	owner, project := parseSlug(slug)
	result.WorkspaceSlug = slug

	// Load template
	tmpl, err := LoadTemplate(cfg.TemplatesDir(), templateName)
	if err != nil {
		return nil, err
	}

	templatePath := filepath.Join(cfg.TemplatesDir(), templateName)
	reposPath := filepath.Join(workspacePath, "repos")

	// Get built-in variables
	builtins := GetBuiltinVariables(owner, project, workspacePath, cfg.CodeRoot)

	// Resolve all variables
	vars, err := ResolveVariables(tmpl, opts.Variables, builtins)
	if err != nil {
		return nil, fmt.Errorf("resolving variables: %w", err)
	}

	// Process files (skip global if workspace already has them)
	globalCount, templateCount, err := ProcessAllFiles(tmpl, cfg.TemplatesDir(), templatePath, workspacePath, vars)
	if err != nil {
		return result, fmt.Errorf("processing files: %w", err)
	}
	result.GlobalFiles = globalCount
	result.TemplateFiles = templateCount
	result.FilesCreated = globalCount + templateCount

	// Create hook environment
	hookEnv := HookEnv{
		WorkspacePath: workspacePath,
		WorkspaceSlug: slug,
		Owner:         owner,
		Project:       project,
		CodeRoot:      cfg.CodeRoot,
		TemplateName:  templateName,
		TemplatePath:  templatePath,
		ReposPath:     reposPath,
		DryRun:        opts.DryRun,
		Verbose:       opts.Verbose,
		Variables:     vars,
	}

	var output io.Writer
	if opts.Verbose {
		output = os.Stdout
	}

	// Run post_migrate hook
	if !opts.NoHooks && HasHook(tmpl, HookPostMigrate) {
		hookResult, err := RunHook(HookPostMigrate, tmpl.Hooks.PostMigrate, templatePath, hookEnv, output)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("post_migrate hook failed: %v", err))
		} else if !hookResult.Skipped {
			result.HooksRun = append(result.HooksRun, string(HookPostMigrate))
		}
	}

	// Update project.json if it exists
	projectPath := filepath.Join(workspacePath, "project.json")
	if _, err := os.Stat(projectPath); err == nil {
		proj, err := model.LoadProject(projectPath)
		if err == nil {
			proj.Template = templateName
			proj.TemplateVars = vars
			if err := proj.Save(workspacePath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to update project.json: %v", err))
			}
		}
	}

	CleanupHookOutputFile(workspacePath)

	return result, nil
}

// parseSlug extracts owner and project from a workspace slug.
func parseSlug(slug string) (owner, project string) {
	parts := splitSlug(slug)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return slug, slug
}

// splitSlug splits a slug by "--".
func splitSlug(slug string) []string {
	result := []string{}
	current := ""
	i := 0
	for i < len(slug) {
		if i+1 < len(slug) && slug[i] == '-' && slug[i+1] == '-' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
			i += 2
		} else {
			current += string(slug[i])
			i++
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
