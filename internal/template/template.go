package template

import "github.com/tormodhaugland/co/internal/model"

// CurrentTemplateSchema is the current version of the template manifest schema.
const CurrentTemplateSchema = 1

// VarType represents the type of a template variable.
type VarType string

const (
	VarTypeString  VarType = "string"
	VarTypeBoolean VarType = "boolean"
	VarTypeChoice  VarType = "choice"
	VarTypeInteger VarType = "integer"
)

// Template represents a workspace template definition.
type Template struct {
	Schema          int                `json:"schema"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Version         string             `json:"version,omitempty"`
	Variables       []TemplateVar      `json:"variables,omitempty"`
	Repos           []TemplateRepo     `json:"repos,omitempty"`
	Files           TemplateFiles      `json:"files,omitempty"`
	Hooks           TemplateHooks      `json:"hooks,omitempty"`
	Partials        []PartialRef       `json:"partials,omitempty"`
	Tags            []string           `json:"tags,omitempty"`
	State           model.ProjectState `json:"state,omitempty"`
	SkipGlobalFiles interface{}        `json:"skip_global_files,omitempty"` // bool or []string
}

// TemplateVar defines a variable that can be customized when using the template.
type TemplateVar struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Type        VarType     `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Validation  string      `json:"validation,omitempty"` // regex pattern
	Choices     []string    `json:"choices,omitempty"`    // for VarTypeChoice
}

// TemplateRepo defines a repository to create or clone in the workspace.
type TemplateRepo struct {
	Name          string `json:"name"`
	CloneURL      string `json:"clone_url,omitempty"`
	Init          bool   `json:"init,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// PartialRef defines a partial to apply during template creation.
type PartialRef struct {
	Name      string            `json:"name"`
	Target    string            `json:"target,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	When      string            `json:"when,omitempty"`
}

// TemplateFiles configures file processing behavior.
type TemplateFiles struct {
	Include            []string `json:"include,omitempty"`
	Exclude            []string `json:"exclude,omitempty"`
	TemplateExtensions []string `json:"template_extensions,omitempty"` // default: [".tmpl"]
}

// TemplateHooks defines lifecycle hook scripts.
type TemplateHooks struct {
	PreCreate    HookSpec `json:"pre_create,omitempty"`
	PostCreate   HookSpec `json:"post_create,omitempty"`
	PostClone    HookSpec `json:"post_clone,omitempty"`
	PostComplete HookSpec `json:"post_complete,omitempty"`
	PostMigrate  HookSpec `json:"post_migrate,omitempty"`
}

// HookSpec defines a hook script and its configuration.
type HookSpec struct {
	Script  string `json:"script,omitempty"`
	Timeout string `json:"timeout,omitempty"` // e.g., "5m", "30s"
}

// IsEmpty returns true if the hook spec has no script defined.
func (h HookSpec) IsEmpty() bool {
	return h.Script == ""
}

// HookEnv holds environment information passed to hook scripts.
type HookEnv struct {
	WorkspacePath  string
	WorkspaceSlug  string
	Owner          string
	Project        string
	CodeRoot       string
	TemplateName   string
	TemplatePath   string
	ReposPath      string
	DryRun         bool
	Verbose        bool
	Variables      map[string]string
	PrevHookOutput string
}

// CreateOptions holds options for template-based workspace creation.
type CreateOptions struct {
	TemplateName string
	Variables    map[string]string
	NoHooks      bool
	DryRun       bool
	Verbose      bool
}

// PartialApplyOptions holds the partial apply parameters for template integration.
type PartialApplyOptions struct {
	PartialName string
	TargetPath  string
	Variables   map[string]string
	DryRun      bool
	NoHooks     bool
}

// PartialApplier applies a partial during template creation.
type PartialApplier func(opts PartialApplyOptions, partialsDirs []string) error

var partialApplier PartialApplier

// RegisterPartialApplier registers a partial applier for template integration.
func RegisterPartialApplier(applier PartialApplier) {
	partialApplier = applier
}

// CreateResult holds the result of template-based workspace creation.
type CreateResult struct {
	WorkspacePath string   `json:"workspace_path"`
	WorkspaceSlug string   `json:"workspace_slug"`
	TemplateUsed  string   `json:"template_used,omitempty"`
	FilesCreated  int      `json:"files_created"`
	GlobalFiles   int      `json:"global_files"`
	TemplateFiles int      `json:"template_files"`
	ReposCreated  int      `json:"repos_created"`
	ReposCloned   int      `json:"repos_cloned"`
	HooksRun      []string `json:"hooks_run,omitempty"`
	HooksSkipped  []string `json:"hooks_skipped,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// TemplateInfo provides summary information about a template for listing.
type TemplateInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	VarCount    int    `json:"var_count"`
	RepoCount   int    `json:"repo_count"`
	HookCount   int    `json:"hook_count"`
	HasGlobal   bool   `json:"has_global,omitempty"` // true for _global pseudo-template
}

// ToInfo converts a Template to TemplateInfo for listing.
func (t *Template) ToInfo() TemplateInfo {
	hookCount := 0
	if !t.Hooks.PreCreate.IsEmpty() {
		hookCount++
	}
	if !t.Hooks.PostCreate.IsEmpty() {
		hookCount++
	}
	if !t.Hooks.PostClone.IsEmpty() {
		hookCount++
	}
	if !t.Hooks.PostComplete.IsEmpty() {
		hookCount++
	}
	if !t.Hooks.PostMigrate.IsEmpty() {
		hookCount++
	}

	return TemplateInfo{
		Name:        t.Name,
		Description: t.Description,
		Version:     t.Version,
		VarCount:    len(t.Variables),
		RepoCount:   len(t.Repos),
		HookCount:   hookCount,
	}
}

// GetTemplateExtensions returns the template extensions to use, defaulting to [".tmpl"].
func (t *Template) GetTemplateExtensions() []string {
	if len(t.Files.TemplateExtensions) > 0 {
		return t.Files.TemplateExtensions
	}
	return []string{".tmpl"}
}

// ShouldSkipGlobal checks if global files should be skipped entirely.
func (t *Template) ShouldSkipGlobal() bool {
	if skip, ok := t.SkipGlobalFiles.(bool); ok {
		return skip
	}
	return false
}

// GetSkippedGlobalFiles returns the list of global files to skip.
func (t *Template) GetSkippedGlobalFiles() []string {
	if files, ok := t.SkipGlobalFiles.([]interface{}); ok {
		result := make([]string, 0, len(files))
		for _, f := range files {
			if s, ok := f.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if files, ok := t.SkipGlobalFiles.([]string); ok {
		return files
	}
	return nil
}

// DefaultHookTimeout is the default timeout for hook scripts.
const DefaultHookTimeout = "5m"

// GlobalTemplateDir is the name of the global template directory.
const GlobalTemplateDir = "_global"

// TemplateManifestFile is the name of the template manifest file.
const TemplateManifestFile = "template.json"

// TemplateFilesDir is the name of the directory containing template files.
const TemplateFilesDir = "files"

// TemplateHooksDir is the name of the directory containing hook scripts.
const TemplateHooksDir = "hooks"
