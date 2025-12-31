// Package partial provides functionality for managing and applying partials -
// reusable file sets that can be applied to any directory at any time.
//
// Unlike workspace templates (which create entire workspaces with repos),
// partials are folder-agnostic snippets that add or update files in an
// existing directory.
package partial

import (
	"regexp"
	"time"

	"github.com/tormodhaugland/co/internal/template"
)

// CurrentPartialSchema is the current version of the partial manifest schema.
const CurrentPartialSchema = 1

// PartialManifestFile is the name of the partial manifest file.
const PartialManifestFile = "partial.json"

// PartialFilesDir is the name of the directory containing partial files.
const PartialFilesDir = "files"

// PartialHooksDir is the name of the directory containing hook scripts.
const PartialHooksDir = "hooks"

// DefaultHookTimeout is the default timeout for hook scripts.
const DefaultHookTimeout = "5m"

// ConflictStrategy defines how to handle file conflicts during apply.
type ConflictStrategy string

const (
	// StrategyPrompt asks the user for each conflict (default).
	StrategyPrompt ConflictStrategy = "prompt"
	// StrategySkip keeps existing files, doesn't overwrite.
	StrategySkip ConflictStrategy = "skip"
	// StrategyOverwrite replaces existing files.
	StrategyOverwrite ConflictStrategy = "overwrite"
	// StrategyBackup backs up existing to <file>.bak, then writes new.
	StrategyBackup ConflictStrategy = "backup"
	// StrategyMerge attempts three-way merge for supported formats.
	StrategyMerge ConflictStrategy = "merge"
)

// DefaultConflictStrategy is the default strategy when not specified.
const DefaultConflictStrategy = StrategyPrompt

// ValidConflictStrategies contains all valid conflict strategy values.
var ValidConflictStrategies = []ConflictStrategy{
	StrategyPrompt,
	StrategySkip,
	StrategyOverwrite,
	StrategyBackup,
	StrategyMerge,
}

// IsValidConflictStrategy checks if a strategy string is valid.
func IsValidConflictStrategy(s string) bool {
	for _, valid := range ValidConflictStrategies {
		if string(valid) == s {
			return true
		}
	}
	return false
}

// partialNameRegex validates partial names: lowercase alphanumeric with hyphens,
// must start with a letter or number.
var partialNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// IsValidPartialName checks if a name is a valid partial name.
func IsValidPartialName(name string) bool {
	return partialNameRegex.MatchString(name)
}

// Partial represents a partial definition loaded from partial.json.
type Partial struct {
	Schema      int            `json:"schema"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version,omitempty"`
	Variables   []PartialVar   `json:"variables,omitempty"`
	Files       PartialFiles   `json:"files,omitempty"`
	Conflicts   ConflictConfig `json:"conflicts,omitempty"`
	Hooks       PartialHooks   `json:"hooks,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Requires    Requirements   `json:"requires,omitempty"`
}

// PartialVar defines a variable that can be customized when applying the partial.
// This mirrors template.TemplateVar but uses VarType from the template package.
type PartialVar struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Type        template.VarType `json:"type"`
	Required    bool             `json:"required"`
	Default     interface{}      `json:"default,omitempty"`
	Validation  string           `json:"validation,omitempty"` // regex pattern
	Choices     []string         `json:"choices,omitempty"`    // for VarTypeChoice
}

// PartialFiles configures which files to include/exclude from the partial.
type PartialFiles struct {
	Include            []string `json:"include,omitempty"`
	Exclude            []string `json:"exclude,omitempty"`
	TemplateExtensions []string `json:"template_extensions,omitempty"` // default: [".tmpl"]
}

// ConflictConfig defines how to handle conflicts when files already exist.
type ConflictConfig struct {
	Strategy string   `json:"strategy,omitempty"` // prompt, skip, overwrite, backup, merge
	Preserve []string `json:"preserve,omitempty"` // glob patterns to never overwrite
}

// PartialHooks defines lifecycle hook scripts for partials.
type PartialHooks struct {
	PreApply  template.HookSpec `json:"pre_apply,omitempty"`
	PostApply template.HookSpec `json:"post_apply,omitempty"`
}

// Requirements defines prerequisites that must be met before applying a partial.
type Requirements struct {
	Commands []string `json:"commands,omitempty"` // e.g., ["git", "node"]
	Files    []string `json:"files,omitempty"`    // e.g., ["package.json"]
}

// ApplyOptions holds options for applying a partial to a directory.
type ApplyOptions struct {
	PartialName      string            // Name of the partial to apply
	TargetPath       string            // Directory to apply partial to
	Variables        map[string]string // User-provided variable values
	ConflictStrategy string            // Override conflict strategy
	DryRun           bool              // Preview changes without applying
	NoHooks          bool              // Skip lifecycle hooks
	Force            bool              // Apply even if prerequisites fail
	Yes              bool              // Accept all prompts (non-interactive)
}

// ApplyResult holds the result of applying a partial.
type ApplyResult struct {
	PartialName      string   `json:"partial_name"`
	TargetPath       string   `json:"target_path"`
	FilesCreated     []string `json:"files_created"`
	FilesSkipped     []string `json:"files_skipped"`
	FilesOverwritten []string `json:"files_overwritten"`
	FilesMerged      []string `json:"files_merged"`
	FilesBackedUp    []string `json:"files_backed_up"`
	HooksRun         []string `json:"hooks_run,omitempty"`
	HooksSkipped     []string `json:"hooks_skipped,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

// PartialInfo provides summary information about a partial for listing.
type PartialInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version,omitempty"`
	Path        string   `json:"path"`
	SourceDir   string   `json:"source_dir"` // Which directory it came from
	VarCount    int      `json:"var_count"`
	FileCount   int      `json:"file_count"`
	HookCount   int      `json:"hook_count"`
	Tags        []string `json:"tags,omitempty"`
}

// ToInfo converts a Partial to PartialInfo for listing.
func (p *Partial) ToInfo(path, sourceDir string, fileCount int) PartialInfo {
	hookCount := 0
	if !p.Hooks.PreApply.IsEmpty() {
		hookCount++
	}
	if !p.Hooks.PostApply.IsEmpty() {
		hookCount++
	}

	return PartialInfo{
		Name:        p.Name,
		Description: p.Description,
		Version:     p.Version,
		Path:        path,
		SourceDir:   sourceDir,
		VarCount:    len(p.Variables),
		FileCount:   fileCount,
		HookCount:   hookCount,
		Tags:        p.Tags,
	}
}

// GetTemplateExtensions returns the template extensions to use, defaulting to [".tmpl"].
func (p *Partial) GetTemplateExtensions() []string {
	if len(p.Files.TemplateExtensions) > 0 {
		return p.Files.TemplateExtensions
	}
	return []string{".tmpl"}
}

// GetConflictStrategy returns the conflict strategy, defaulting to "prompt".
func (p *Partial) GetConflictStrategy() string {
	if p.Conflicts.Strategy != "" {
		return p.Conflicts.Strategy
	}
	return string(DefaultConflictStrategy)
}

// HasPrerequisites returns true if the partial has any prerequisites defined.
func (p *Partial) HasPrerequisites() bool {
	return len(p.Requires.Commands) > 0 || len(p.Requires.Files) > 0
}

// PartialHookEnv holds environment information passed to partial hook scripts.
type PartialHookEnv struct {
	PartialName   string
	PartialPath   string
	TargetPath    string
	TargetDirname string
	DryRun        bool
	Verbose       bool
	IsGitRepo     bool
	GitRemoteURL  string
	GitBranch     string
	Variables     map[string]string
	Result        *ApplyResult // Only populated for post_apply hook
}

// FileAction represents what action to take for a file during apply.
type FileAction int

const (
	// ActionCreate creates a new file.
	ActionCreate FileAction = iota
	// ActionSkip skips the file (keep existing).
	ActionSkip
	// ActionOverwrite overwrites the existing file.
	ActionOverwrite
	// ActionBackup backs up existing and writes new.
	ActionBackup
	// ActionMerge merges with existing file.
	ActionMerge
	// ActionPrompt prompts user for action.
	ActionPrompt
)

// String returns a human-readable representation of the action.
func (a FileAction) String() string {
	switch a {
	case ActionCreate:
		return "create"
	case ActionSkip:
		return "skip"
	case ActionOverwrite:
		return "overwrite"
	case ActionBackup:
		return "backup"
	case ActionMerge:
		return "merge"
	case ActionPrompt:
		return "prompt"
	default:
		return "unknown"
	}
}

// FileInfo holds information about a file to be processed during apply.
type FileInfo struct {
	RelPath        string     // Relative path from partial files/ or target
	AbsSourcePath  string     // Absolute path to source file in partial
	AbsDestPath    string     // Absolute path to destination in target
	TargetModTime  time.Time  // Mod time of existing target file
	IsTemplate     bool       // Whether this is a .tmpl file
	ExistsInTarget bool       // Whether file already exists in target
	IsPreserved    bool       // Whether file matches a preserve pattern
	Action         FileAction // What action to take
}

// FilePlan holds the complete plan for applying files.
type FilePlan struct {
	Files      []FileInfo
	Creates    int
	Skips      int
	Overwrites int
	Backups    int
	Merges     int
	Prompts    int
}

// CountActions updates the action counts based on the files in the plan.
func (fp *FilePlan) CountActions() {
	fp.Creates = 0
	fp.Skips = 0
	fp.Overwrites = 0
	fp.Backups = 0
	fp.Merges = 0
	fp.Prompts = 0

	for _, f := range fp.Files {
		switch f.Action {
		case ActionCreate:
			fp.Creates++
		case ActionSkip:
			fp.Skips++
		case ActionOverwrite:
			fp.Overwrites++
		case ActionBackup:
			fp.Backups++
		case ActionMerge:
			fp.Merges++
		case ActionPrompt:
			fp.Prompts++
		}
	}
}
