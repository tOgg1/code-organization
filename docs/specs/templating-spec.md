# Project Templating & Scripting System

**Status:** Draft
**Created:** 2025-12-14
**Author:** Claude (with human oversight)

---

## 1. Executive Summary

This specification defines a templating and scripting system for the `co` CLI that enables:

1. **Project Templates** — Reusable blueprints for creating new workspaces with predefined structure, files, and repositories
2. **Global Template Files** — Files that are always created for every new workspace regardless of template
3. **Template Variables** — User-provided values that are substituted into template files during creation
4. **Lifecycle Hooks** — Bash scripts that execute automatically at defined points in the workspace lifecycle

The system integrates with the existing `co new` command while maintaining backward compatibility with the current workflow.

---

## 2. Goals & Non-Goals

### 2.1 Goals

- **G1:** Enable reproducible workspace creation with predefined structures
- **G2:** Support both repository-based templates (mono-repo, multi-repo) and file-only templates
- **G3:** Allow global files that are created in every workspace (e.g., Agents.md, claude.md)
- **G4:** Provide variable substitution for customizing templates at creation time
- **G5:** Support lifecycle hooks (bash scripts) that run at workspace creation
- **G6:** Maintain backward compatibility — `co new` without a template works as before
- **G7:** Keep templates in a machine-readable, version-controllable format

### 2.2 Non-Goals

- **NG1:** Complex templating engines (Jinja2, Handlebars) — simple variable substitution is sufficient
- **NG2:** Remote template registries — templates are local-first
- **NG3:** Template inheritance/composition — keep it simple initially
- **NG4:** Windows batch script support — bash only for hooks

---

## 3. Template Storage & Discovery

### 3.1 Template Locations

Templates are stored in the system directory under a new `templates/` subdirectory:

```
~/Code/_system/
├── config.json
├── index.jsonl
├── templates/
│   ├── _global/                    # Global files (always applied)
│   │   ├── Agents.md
│   │   ├── claude.md
│   │   └── .gitignore
│   │
│   ├── single-repo/                # Template: single-repo
│   │   ├── template.json           # Template manifest
│   │   ├── files/                  # Template files to copy
│   │   │   ├── README.md.tmpl
│   │   │   └── .claude/
│   │   │       └── settings.json.tmpl
│   │   └── hooks/                  # Lifecycle hooks
│   │       ├── post-create.sh
│   │       └── post-clone.sh
│   │
│   ├── monorepo/                   # Template: monorepo
│   │   ├── template.json
│   │   ├── files/
│   │   │   ├── README.md.tmpl
│   │   │   └── CONTRIBUTING.md
│   │   └── hooks/
│   │       └── post-create.sh
│   │
│   └── python-service/             # Template: python-service
│       ├── template.json
│       ├── files/
│       │   ├── pyproject.toml.tmpl
│       │   └── src/
│       │       └── __init__.py.tmpl
│       └── hooks/
│           └── post-create.sh
```

### 3.2 Template Naming Convention

- Template names follow the same pattern as project names: lowercase alphanumeric with hyphens
- Pattern: `^[a-z0-9][a-z0-9-]*$`
- Reserved name: `_global` for global files

### 3.3 Discovery Order

When creating a workspace with a template:

1. Look in `~/Code/_system/templates/<template-name>/`
2. Validate `template.json` exists and is valid
3. Apply global files from `~/Code/_system/templates/_global/` (if any)
4. Apply template-specific files from `templates/<template-name>/files/`

---

## 4. Template Manifest (`template.json`)

Each template has a manifest file that defines its metadata, variables, and behavior.

### 4.1 Schema

```json
{
  "schema": 1,
  "name": "single-repo",
  "description": "A workspace with a single repository and standard AI-assisted development files",
  "version": "1.0.0",

  "variables": [
    {
      "name": "project_name",
      "description": "Human-readable project name",
      "type": "string",
      "required": true,
      "default": null,
      "validation": "^[A-Za-z][A-Za-z0-9 -]*$"
    },
    {
      "name": "author_name",
      "description": "Primary author or maintainer",
      "type": "string",
      "required": false,
      "default": "{{GIT_USER_NAME}}"
    },
    {
      "name": "description",
      "description": "Short project description",
      "type": "string",
      "required": true,
      "default": null
    },
    {
      "name": "license",
      "description": "License type",
      "type": "choice",
      "required": false,
      "default": "MIT",
      "choices": ["MIT", "Apache-2.0", "GPL-3.0", "BSD-3-Clause", "Proprietary", "None"]
    },
    {
      "name": "enable_ci",
      "description": "Include GitHub Actions CI configuration",
      "type": "boolean",
      "required": false,
      "default": true
    }
  ],

  "repos": [
    {
      "name": "main",
      "clone_url": null,
      "init": true,
      "default_branch": "main"
    }
  ],

  "files": {
    "include": ["**/*"],
    "exclude": ["*.bak", ".DS_Store"],
    "template_extensions": [".tmpl"]
  },

  "hooks": {
    "post_create": "hooks/post-create.sh",
    "post_clone": "hooks/post-clone.sh"
  },

  "tags": ["ai-assisted", "standard"],
  "state": "active"
}
```

### 4.2 Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `schema` | int | Yes | Schema version (currently 1) |
| `name` | string | Yes | Template identifier (matches directory name) |
| `description` | string | Yes | Human-readable description |
| `version` | string | No | Semantic version for the template |
| `variables` | array | No | Variable definitions for user input |
| `repos` | array | No | Repository specifications to create/clone |
| `files` | object | No | File handling configuration |
| `hooks` | object | No | Lifecycle hook script paths |
| `tags` | array | No | Default tags applied to created workspaces |
| `state` | string | No | Default state for created workspaces |

### 4.3 Variable Types

| Type | Description | UI Behavior |
|------|-------------|-------------|
| `string` | Free-form text input | Text field |
| `boolean` | True/false toggle | Checkbox or y/n prompt |
| `choice` | Selection from predefined options | Dropdown/select |
| `integer` | Numeric value | Number input |

### 4.4 Variable Interpolation

Variables can reference other variables in their default values:

```json
{
  "variables": [
    {
      "name": "author_name",
      "type": "string",
      "default": "{{GIT_USER_NAME}}"
    },
    {
      "name": "project_title",
      "type": "string",
      "default": "{{author_name}}'s {{PROJECT}} Project"
    }
  ]
}
```

**Cycle Detection:**

The system must detect and reject circular references:

```json
{
  "variables": [
    { "name": "a", "default": "{{b}}" },
    { "name": "b", "default": "{{a}}" }
  ]
}
```

Resolution algorithm:
1. Build a dependency graph from variable defaults
2. Topologically sort variables
3. If cycle detected, error with: `"Circular variable reference: a → b → a"`
4. Resolve variables in topological order

### 4.5 Built-in Variables

These variables are automatically available in all templates:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{OWNER}}` | Workspace owner | `acme` |
| `{{PROJECT}}` | Project name (slug) | `backend` |
| `{{SLUG}}` | Full workspace slug | `acme--backend` |
| `{{CREATED_DATE}}` | ISO 8601 creation date | `2025-12-14` |
| `{{CREATED_DATETIME}}` | ISO 8601 creation timestamp | `2025-12-14T10:30:00Z` |
| `{{YEAR}}` | Current year | `2025` |
| `{{GIT_USER_NAME}}` | Git user.name config | `John Doe` |
| `{{GIT_USER_EMAIL}}` | Git user.email config | `john@example.com` |
| `{{HOME}}` | User home directory | `/Users/john` |
| `{{CODE_ROOT}}` | Code root directory | `/Users/john/Code` |
| `{{WORKSPACE_PATH}}` | Full workspace path | `/Users/john/Code/acme--backend` |

---

## 5. Template Files

### 5.1 File Structure

Template files are stored in the `files/` subdirectory and are copied to the new workspace during creation.

```
templates/single-repo/files/
├── README.md.tmpl           # Template file (variables substituted)
├── CONTRIBUTING.md          # Static file (copied as-is)
├── .claude/
│   └── settings.json.tmpl
├── .github/
│   └── workflows/
│       └── ci.yml.tmpl
└── repos/
    └── main/                # Files for the 'main' repo
        ├── src/
        │   └── .gitkeep
        └── tests/
            └── .gitkeep
```

### 5.2 Template File Processing

1. Files with `.tmpl` extension are processed for variable substitution
2. The `.tmpl` extension is removed from the output filename
3. Files without `.tmpl` are copied verbatim
4. Directory structure is preserved

### 5.3 Variable Substitution Syntax

Variables use double-brace syntax: `{{VARIABLE_NAME}}`

**Example `README.md.tmpl`:**

```markdown
# {{project_name}}

{{description}}

## Overview

This project is owned by **{{OWNER}}** and was created on {{CREATED_DATE}}.

## Author

{{author_name}} ({{GIT_USER_EMAIL}})

## License

{{#if license != "None"}}
This project is licensed under the {{license}} license.
{{/if}}
```

### 5.4 Conditional Blocks

Simple conditional blocks for optional content:

```
{{#if VARIABLE_NAME}}
Content to include if VARIABLE_NAME is truthy
{{/if}}

{{#if VARIABLE_NAME == "value"}}
Content to include if VARIABLE_NAME equals "value"
{{/if}}

{{#if VARIABLE_NAME != "value"}}
Content to include if VARIABLE_NAME does not equal "value"
{{/if}}
```

### 5.5 Repository-Specific Files

Files placed under `files/repos/<repo-name>/` are copied into the corresponding repository directory:

```
templates/monorepo/files/
├── README.md.tmpl           # → workspace/README.md
└── repos/
    ├── frontend/            # → workspace/repos/frontend/
    │   └── package.json.tmpl
    └── backend/             # → workspace/repos/backend/
        └── go.mod.tmpl
```

---

## 6. Global Template Files

### 6.1 Purpose

Global files are applied to **every** new workspace regardless of which template is used. This ensures consistency across all projects.

### 6.2 Storage

Global files are stored in `~/Code/_system/templates/_global/`:

```
~/Code/_system/templates/_global/
├── Agents.md.tmpl           # AI agent coordination file
├── claude.md.tmpl           # Claude Code configuration
├── .gitignore               # Standard ignores (copied as-is)
└── .editorconfig            # Editor configuration
```

### 6.3 Processing Order

1. Global files are processed first
2. Template-specific files are processed second
3. Template files can override global files (same path = override)

### 6.4 Disabling Global Files

Templates can opt out of global files via the manifest:

```json
{
  "skip_global_files": true
}
```

Or exclude specific global files:

```json
{
  "skip_global_files": ["Agents.md", ".editorconfig"]
}
```

---

## 7. Lifecycle Hooks

### 7.1 Hook Types

| Hook | Trigger | Use Case |
|------|---------|----------|
| `pre_create` | Before workspace directory is created | Validation, prerequisites |
| `post_create` | After workspace and files created, before repos | Initialize git, install dependencies |
| `post_clone` | After repositories are cloned | Set up remotes, install repo deps |
| `post_complete` | After entire creation process completes | Final configuration, notifications |
| `post_migrate` | After `co migrate` imports existing folder | Setup for migrated projects |

### 7.2 Hook Environment Variables

Hooks receive the following environment variables:

| Variable | Description |
|----------|-------------|
| `CO_WORKSPACE_PATH` | Full path to the workspace directory |
| `CO_WORKSPACE_SLUG` | Workspace slug (owner--project) |
| `CO_OWNER` | Workspace owner |
| `CO_PROJECT` | Project name |
| `CO_CODE_ROOT` | Code root directory |
| `CO_TEMPLATE_NAME` | Name of the template being used |
| `CO_TEMPLATE_PATH` | Full path to the template directory |
| `CO_REPOS_PATH` | Path to the repos/ subdirectory |
| `CO_DRY_RUN` | "true" if running in dry-run mode |
| `CO_VERBOSE` | "true" if verbose output enabled |
| `CO_HOOK_OUTPUT_FILE` | Path to file where hook can write output for subsequent hooks |
| `CO_PREV_HOOK_OUTPUT` | Contents of previous hook's output (empty for first hook) |

Additionally, all user-defined variables are exported with a `CO_VAR_` prefix:

```bash
CO_VAR_project_name="My Project"
CO_VAR_author_name="John Doe"
CO_VAR_license="MIT"
CO_VAR_enable_ci="true"
```

### 7.3 Hook Script Requirements

1. Must be executable bash scripts
2. Must have a shebang line: `#!/usr/bin/env bash`
3. Exit code 0 = success, non-zero = failure (aborts creation)
4. Stdout/stderr are displayed to the user
5. Scripts run with the workspace directory as the working directory

### 7.4 Example Hook Script

**`hooks/post-create.sh`:**

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "Setting up workspace: ${CO_WORKSPACE_SLUG}"

# Initialize git in repo directories
for repo_dir in "${CO_REPOS_PATH}"/*; do
    if [[ -d "$repo_dir" ]] && [[ ! -d "$repo_dir/.git" ]]; then
        echo "Initializing git in $(basename "$repo_dir")"
        git -C "$repo_dir" init
        git -C "$repo_dir" checkout -b main
    fi
done

# Install dependencies if package.json exists
if [[ -f "${CO_REPOS_PATH}/main/package.json" ]]; then
    echo "Installing npm dependencies..."
    cd "${CO_REPOS_PATH}/main"
    npm install
fi

echo "Workspace setup complete!"
```

### 7.5 Hook Timeout

Hooks have a default timeout of 5 minutes. This can be configured per-hook:

```json
{
  "hooks": {
    "post_create": {
      "script": "hooks/post-create.sh",
      "timeout": "10m"
    }
  }
}
```

### 7.6 Hook Failure Handling

- `pre_create` failure: Aborts creation, no cleanup needed
- `post_create` failure: Workspace directory remains, user prompted for cleanup
- `post_clone` failure: Workspace and repos remain, user warned
- `post_complete` failure: Warning only, workspace considered created

---

## 8. CLI Interface

### 8.1 Command: `co new` (Extended)

The existing `co new` command gains template support:

```
co new [owner] [project] [flags]

Flags:
  -t, --template string     Template to use for workspace creation
  -v, --var stringArray     Set template variable (can be repeated: -v key=value)
      --list-templates      List available templates
      --show-template       Show template details and variables
      --no-hooks            Skip running lifecycle hooks
      --dry-run             Preview creation without making changes

Examples:
  co new                                    # Interactive mode (current behavior)
  co new acme backend                       # Create without template (current behavior)
  co new acme backend -t single-repo        # Create with template
  co new acme backend -t single-repo -v project_name="ACME Backend" -v license=MIT
  co new --list-templates                   # List available templates
  co new --show-template single-repo        # Show template details
```

### 8.2 Command: `co template`

New command for template management:

```
co template <subcommand>

Subcommands:
  list          List all available templates
  show <name>   Show details of a template
  create <name> Create a new template (interactive)
  validate      Validate all templates
  export        Export template to portable format
  import        Import template from file or URL

Flags:
  --json        Output in JSON format
  --jsonl       Output in JSON Lines format

Examples:
  co template list
  co template show single-repo
  co template create my-template
  co template validate
  co template export single-repo > single-repo.tar.gz
  co template import single-repo.tar.gz
```

### 8.3 Interactive Variable Input

When a template has required variables without defaults, `co new` prompts interactively:

```
$ co new acme backend -t single-repo

Using template: single-repo
A workspace with a single repository and standard AI-assisted development files

Variables:
  project_name (required): Human-readable project name
  > ACME Backend API

  description (required): Short project description
  > Internal API service for ACME Corp

  author_name [John Doe]: Primary author or maintainer
  > (enter to accept default)

  license [MIT]: License type
  [1] MIT (default)
  [2] Apache-2.0
  [3] GPL-3.0
  [4] BSD-3-Clause
  [5] Proprietary
  [6] None
  > 1

  enable_ci [true]: Include GitHub Actions CI configuration
  > y

Creating workspace: acme--backend
  ✓ Created workspace directory
  ✓ Applied global files (3 files)
  ✓ Applied template files (8 files)
  ✓ Initialized repository: main
  ✓ Running hook: post-create.sh
  ✓ Running hook: post-complete.sh

Workspace created: ~/Code/acme--backend
```

### 8.4 TUI Integration

The TUI's new workspace prompt (`n` key) gains a template selection step:

1. First prompt: Select template (or "No template")
2. Second prompt: Owner input
3. Third prompt: Project input
4. Fourth prompt: Variable inputs (if template selected)
5. Confirmation with preview

---

## 9. Data Model Changes

### 9.1 New Types

**File: `internal/model/template.go`**

```go
package model

type Template struct {
    Schema      int               `json:"schema"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Version     string            `json:"version,omitempty"`
    Variables   []TemplateVar     `json:"variables,omitempty"`
    Repos       []TemplateRepo    `json:"repos,omitempty"`
    Files       TemplateFiles     `json:"files,omitempty"`
    Hooks       TemplateHooks     `json:"hooks,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    State       ProjectState      `json:"state,omitempty"`
    SkipGlobal  interface{}       `json:"skip_global_files,omitempty"` // bool or []string
}

type TemplateVar struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Type        string      `json:"type"` // string, boolean, choice, integer
    Required    bool        `json:"required"`
    Default     interface{} `json:"default,omitempty"`
    Validation  string      `json:"validation,omitempty"` // regex pattern
    Choices     []string    `json:"choices,omitempty"`    // for type=choice
}

type TemplateRepo struct {
    Name          string `json:"name"`
    CloneURL      string `json:"clone_url,omitempty"`
    Init          bool   `json:"init,omitempty"`
    DefaultBranch string `json:"default_branch,omitempty"`
}

type TemplateFiles struct {
    Include            []string `json:"include,omitempty"`
    Exclude            []string `json:"exclude,omitempty"`
    TemplateExtensions []string `json:"template_extensions,omitempty"`
}

type TemplateHooks struct {
    PreCreate    HookSpec `json:"pre_create,omitempty"`
    PostCreate   HookSpec `json:"post_create,omitempty"`
    PostClone    HookSpec `json:"post_clone,omitempty"`
    PostComplete HookSpec `json:"post_complete,omitempty"`
    PostMigrate  HookSpec `json:"post_migrate,omitempty"`
}

type HookSpec struct {
    Script  string `json:"script,omitempty"`
    Timeout string `json:"timeout,omitempty"`
}
```

### 9.2 Project Model Extension

**Additions to `internal/model/project.go`:**

```go
type Project struct {
    // ... existing fields ...

    // New fields
    Template       string            `json:"template,omitempty"`       // Template used to create
    TemplateVars   map[string]string `json:"template_vars,omitempty"`  // Variables used
}
```

---

## 10. Internal Package Structure

### 10.1 New Package: `internal/template`

```
internal/template/
├── template.go       # Core template operations
├── loader.go         # Template discovery and loading
├── variables.go      # Variable handling and substitution
├── files.go          # File processing and copying
├── hooks.go          # Hook execution
└── validation.go     # Template validation
```

### 10.2 Key Functions

```go
package template

// Discovery and loading
func ListTemplates(codeRoot string) ([]Template, error)
func LoadTemplate(codeRoot, name string) (*Template, error)
func ValidateTemplate(tmpl *Template) error

// Variable handling
func ResolveVariables(tmpl *Template, provided map[string]string) (map[string]string, error)
func SubstituteVariables(content string, vars map[string]string) (string, error)

// File processing
func ProcessFiles(tmpl *Template, destPath string, vars map[string]string) error
func ProcessGlobalFiles(codeRoot, destPath string, vars map[string]string) error

// Hook execution
func RunHook(hookType string, tmpl *Template, env HookEnv) error
type HookEnv struct {
    WorkspacePath string
    Slug          string
    Owner         string
    Project       string
    CodeRoot      string
    TemplateName  string
    TemplatePath  string
    DryRun        bool
    Verbose       bool
    Variables     map[string]string
}
```

---

## 11. Creation Flow (Detailed)

### 11.1 Sequence Diagram

```
User                    CLI                     Template                 FS/Git
  |                      |                         |                       |
  |-- co new -t tmpl --->|                         |                       |
  |                      |-- LoadTemplate(tmpl) -->|                       |
  |                      |<-- Template ------------|                       |
  |                      |                         |                       |
  |                      |-- ResolveVariables ---->|                       |
  |<-- Prompt for vars --|                         |                       |
  |-- Variables -------->|                         |                       |
  |                      |<-- ResolvedVars --------|                       |
  |                      |                         |                       |
  |                      |-- RunHook(pre_create) ->|                       |
  |                      |<-- Success -------------|                       |
  |                      |                         |                       |
  |                      |-- CreateWorkspace ------|---------------------->|
  |                      |                         |                       |
  |                      |-- ProcessGlobalFiles -->|                       |
  |                      |                         |-- CopyFiles --------->|
  |                      |                         |                       |
  |                      |-- ProcessFiles -------->|                       |
  |                      |                         |-- CopyFiles --------->|
  |                      |                         |                       |
  |                      |-- RunHook(post_create)->|                       |
  |                      |                         |                       |
  |                      |-- CloneRepos -----------|---------------------->|
  |                      |                         |                       |
  |                      |-- RunHook(post_clone) ->|                       |
  |                      |                         |                       |
  |                      |-- SaveProject ----------|---------------------->|
  |                      |                         |                       |
  |                      |-- RunHook(post_complete)|                       |
  |                      |                         |                       |
  |<-- Success ----------|                         |                       |
```

### 11.2 Creation Steps

1. **Parse arguments** — Extract owner, project, template name, and variables
2. **Load template** — Read and validate `template.json`
3. **Resolve variables** — Merge provided, defaults, and built-in variables
4. **Prompt for missing** — Interactively request required variables
5. **Run pre_create hook** — Validate prerequisites
6. **Create workspace directory** — `~/Code/owner--project/`
7. **Create repos directory** — `repos/`
8. **Process global files** — Copy and substitute `_global/` files
9. **Process template files** — Copy and substitute template files
10. **Run post_create hook** — Post-file-creation setup
11. **Initialize/clone repos** — Based on `repos` spec
12. **Run post_clone hook** — Post-repo setup
13. **Save project.json** — With template info recorded
14. **Run post_complete hook** — Final notifications
15. **Update index** — Add to global index

---

## 12. Error Handling

### 12.1 Validation Errors

| Error | Trigger | Response |
|-------|---------|----------|
| `TemplateNotFound` | Template doesn't exist | List available templates |
| `InvalidManifest` | `template.json` parse failure | Show validation errors |
| `MissingRequiredVar` | Required variable not provided | Prompt interactively |
| `InvalidVarValue` | Value fails validation regex | Show error, re-prompt |
| `HookNotFound` | Referenced hook script missing | Error with path |
| `HookNotExecutable` | Script lacks execute permission | Auto-fix or error |

### 12.2 Creation Errors

| Error | Trigger | Response |
|-------|---------|----------|
| `WorkspaceExists` | Slug already in use | Error with suggestion |
| `HookFailed` | Hook returns non-zero | Show output, offer cleanup |
| `CloneFailed` | Git clone fails | Show error, workspace remains |
| `SubstitutionFailed` | Variable not found in template | Error with context |

### 12.3 Cleanup Policy

- **pre_create failure:** Nothing to clean up
- **post_create failure:** Prompt user to keep or delete workspace
- **post_clone failure:** Keep workspace, list which repos failed
- **post_complete failure:** Warning only, creation considered successful

---

## 13. Built-in Templates

### 13.1 Template: `single-repo`

**Purpose:** Standard workspace with one repository and AI-assisted development files.

**Files:**
- `README.md.tmpl`
- `Agents.md.tmpl`
- `claude.md.tmpl`
- `.gitignore`
- `repos/main/.gitkeep`

**Variables:**
- `project_name` (required)
- `description` (required)
- `author_name` (default: git user)
- `license` (default: MIT)

**Hooks:**
- `post_create`: Initialize git, create initial commit

### 13.2 Template: `monorepo`

**Purpose:** Multi-package workspace with shared tooling.

**Files:**
- `README.md.tmpl`
- `CONTRIBUTING.md.tmpl`
- `repos/frontend/package.json.tmpl`
- `repos/backend/go.mod.tmpl`

**Variables:**
- `project_name` (required)
- `description` (required)
- `frontend_framework` (choice: react, vue, svelte)
- `backend_language` (choice: go, python, typescript)

**Hooks:**
- `post_create`: Initialize git repos
- `post_clone`: Install dependencies

### 13.3 Template: `scratch`

**Purpose:** Minimal workspace for experiments and prototypes.

**Files:**
- `README.md.tmpl`
- `.gitignore`

**Variables:**
- `description` (optional)

**State:** `scratch`

**Hooks:** None

---

## 14. Security Considerations

### 14.1 Hook Script Security

1. **Scripts are local-only** — No remote script execution
2. **User must create scripts** — No automatic script creation
3. **Executable check** — Scripts must be executable before running
4. **Timeout enforcement** — Prevent runaway scripts
5. **No sudo** — Scripts run as current user

### 14.2 Variable Sanitization

1. **No shell expansion** — Variables are literal strings
2. **Regex validation** — Enforce patterns where specified
3. **Path traversal prevention** — Validate file paths
4. **Size limits** — Maximum variable value length (4KB)

### 14.3 File Processing Safety

1. **Destination validation** — All files must be within workspace
2. **Symlink handling** — Reject or dereference symlinks
3. **Permission preservation** — Maintain executable bits where appropriate
4. **Overwrite protection** — Prompt before overwriting existing files

---

## 15. Testing Strategy

### 15.1 Unit Tests

- Variable substitution with all types
- Conditional block parsing
- Template validation
- Path resolution
- Hook environment building

### 15.2 Integration Tests

- Full creation flow with each built-in template
- Variable prompting in interactive mode
- Hook execution and failure handling
- Global files with template override
- Dry-run mode verification

### 15.3 E2E Tests

- CLI commands with all flag combinations
- TUI template selection flow
- Error scenarios and cleanup
- Cross-platform path handling

---

## 16. Migration & Compatibility

### 16.1 Backward Compatibility

- `co new` without `-t` works exactly as before
- Existing workspaces are unaffected
- No schema changes to existing `project.json` files
- Index format unchanged

### 16.2 Initial Setup

On first use of templating features:

1. Create `~/Code/_system/templates/` if missing
2. Create `~/Code/_system/templates/_global/` if missing
3. Optionally seed with built-in templates

### 16.3 Template Versioning

- Templates include `version` field
- Future: migration support for template upgrades
- Future: template compatibility with `co` versions

---

## 17. Future Considerations

### 17.1 Potential Extensions

- **Template inheritance** — Templates extending other templates
- **Remote templates** — Download from Git URLs or registries
- **Template updates** — Sync changes to existing workspaces
- **Hook libraries** — Shared hook scripts across templates
- **Template testing** — Automated template validation

### 17.2 Not In Scope (For Now)

- Complex templating language (loops, functions)
- Template marketplace/sharing
- Workspace-to-template conversion
- Real-time template sync

---

## 18. Implementation Phases

### Phase 1: Core Foundation
- [ ] Template data model and validation
- [ ] Template discovery and loading
- [ ] Basic variable substitution (no conditionals)
- [ ] File copying without templates
- [ ] `co template list` command

### Phase 2: Variable System
- [ ] All variable types (string, boolean, choice, integer)
- [ ] Interactive variable prompting
- [ ] Built-in variables
- [ ] Validation patterns
- [ ] CLI `-v` flag support

### Phase 3: File Processing
- [ ] Template file substitution
- [ ] Conditional blocks
- [ ] Global files support
- [ ] Repository-specific files
- [ ] `co new -t` integration

### Phase 4: Hooks System
- [ ] Hook execution framework
- [ ] Environment variable injection
- [ ] Timeout enforcement
- [ ] Error handling and cleanup
- [ ] All hook types

### Phase 5: Polish & Templates
- [ ] Built-in templates (single-repo, monorepo, scratch)
- [ ] TUI integration
- [ ] `co template show/create` commands
- [ ] Documentation
- [ ] Tests

---

## 19. Design Decisions

The following questions were resolved during specification review:

1. **Post-import hooks for `co migrate`:** ✅ Yes — Templates can define a `post_migrate` hook that runs after `co migrate` imports an existing folder.

2. **Variable interpolation:** ✅ Yes — Variables can reference other variables in their defaults (e.g., `default: "{{author_name}}'s project"`). Cycle detection is required to prevent infinite loops.

3. **Maximum template file size:** No limit initially — Can be added later if needed.

4. **Hook output sharing:** ✅ Yes — Each hook can write to `CO_HOOK_OUTPUT_FILE` and subsequent hooks receive previous outputs via `CO_PREV_HOOK_OUTPUT`.

---

## 20. Glossary

| Term | Definition |
|------|------------|
| **Template** | A reusable blueprint for creating workspaces |
| **Global files** | Files applied to all workspaces regardless of template |
| **Variable** | A named placeholder that users provide values for |
| **Hook** | A bash script that runs at a specific point in creation |
| **Manifest** | The `template.json` file that defines a template |
| **Substitution** | Replacing `{{VAR}}` placeholders with actual values |
| **Workspace** | A directory under `~/Code` containing one or more repos |

---

## Appendix A: Example Global Files

### A.1 `_global/Agents.md.tmpl`

```markdown
# Agents

This document describes AI agents working on the {{project_name}} project.

## Active Agents

<!-- Register your agent here when starting work -->

| Agent | Task | Started | Status |
|-------|------|---------|--------|
| | | | |

## Communication Protocol

1. Check this file before starting work
2. Register your agent and task
3. Update status when completing work
4. Use the mail system for coordination

## Workspace Info

- **Owner:** {{OWNER}}
- **Project:** {{PROJECT}}
- **Created:** {{CREATED_DATE}}
```

### A.2 `_global/claude.md.tmpl`

```markdown
# Claude Configuration

## Project Context

This is the **{{project_name}}** project, owned by **{{OWNER}}**.

{{#if description}}
**Description:** {{description}}
{{/if}}

## Guidelines

- Follow existing code patterns
- Run tests before committing
- Update documentation for API changes
- Use conventional commits

## Key Files

- `project.json` - Workspace metadata
- `Agents.md` - Agent coordination
- `repos/` - Git repositories
```

---

## Appendix B: Example Hook

### B.1 `single-repo/hooks/post-create.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

# Post-create hook for single-repo template
# Initializes git and creates initial commit

echo "🚀 Setting up ${CO_VAR_project_name}..."

REPO_DIR="${CO_REPOS_PATH}/main"

# Create repo directory if it doesn't exist
mkdir -p "$REPO_DIR"

# Initialize git
if [[ ! -d "$REPO_DIR/.git" ]]; then
    echo "  → Initializing git repository"
    git -C "$REPO_DIR" init --initial-branch=main

    # Configure local git settings
    git -C "$REPO_DIR" config user.name "${CO_VAR_author_name:-$USER}"

    # Create initial commit if there are files
    if [[ -n "$(ls -A "$REPO_DIR" 2>/dev/null)" ]]; then
        git -C "$REPO_DIR" add .
        git -C "$REPO_DIR" commit -m "Initial commit

Created from template: ${CO_TEMPLATE_NAME}
Project: ${CO_VAR_project_name}"
    fi
fi

echo "✅ Repository initialized: ${REPO_DIR}"
```

---

*End of specification*
