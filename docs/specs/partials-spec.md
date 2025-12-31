# Partials: Reusable File Sets for Any Directory

**Status:** Draft
**Created:** 2025-12-31
**Author:** Claude (with human oversight)
**Related:** [templating-spec.md](./templating-spec.md)

---

## 1. Executive Summary

This specification defines **Partials** — lightweight, reusable file collections that can be applied to any directory at any time. Unlike workspace templates (which create entire workspaces with repos), partials are folder-agnostic snippets that add or update files in an existing directory.

**Primary Use Cases:**
- Adding agent configuration files (`AGENTS.md`, `.claude/`, `agent_docs/`) to any repository
- Bootstrapping language-specific tooling (`.eslintrc`, `pyproject.toml`, etc.)
- Applying organizational standards to existing codebases
- Retrofitting projects with new conventions without recreating them

---

## 2. Goals & Non-Goals

### 2.1 Goals

- **G1:** Apply file sets to any directory, inside or outside `~/Code`
- **G2:** Reuse the existing variable substitution and conditional system from templates
- **G3:** Support lifecycle hooks (`pre_apply`, `post_apply`)
- **G4:** Provide clear conflict handling (skip, overwrite, merge, prompt)
- **G5:** Enable discovery of what a partial would do before applying (dry-run)
- **G6:** Allow partials to be applied multiple times (idempotent where sensible)
- **G7:** Integrate with workspace templates (templates can reference partials)

### 2.2 Non-Goals

- **NG1:** Creating directories or repositories (that's what templates do)
- **NG2:** Managing workspace-level metadata (`project.json`)
- **NG3:** Remote partial registries (local-first, like templates)
- **NG4:** Automatic detection of "which partial should I apply" (explicit invocation)

---

## 3. Conceptual Model

### 3.1 Templates vs Partials

| Aspect | Templates | Partials |
|--------|-----------|----------|
| **Scope** | Workspace-level | Any directory |
| **Creates structure** | Yes (workspace + repos/) | No (files only) |
| **Manages project.json** | Yes | No |
| **Can be applied multiple times** | No (workspace exists) | Yes |
| **Typical size** | 5-20+ files | 1-10 files |
| **Primary use** | New project scaffolding | Augmenting existing projects |

### 3.2 Mental Model

Think of partials as "copy-paste with intelligence":
- Copy a set of files into a target directory
- Substitute variables for customization
- Run hooks for additional setup
- Handle conflicts gracefully

---

## 4. Storage & Discovery

### 4.1 Partial Locations

Partials are stored alongside templates in a `partials/` subdirectory:

```
~/Code/_system/
├── templates/
│   ├── _global/
│   ├── single-repo/
│   └── ...
└── partials/
    ├── agent-setup/              # Partial: agent-setup
    │   ├── partial.json          # Manifest (required)
    │   ├── files/                # Files to copy
    │   │   ├── AGENTS.md.tmpl
    │   │   ├── .claude/
    │   │   │   └── settings.json.tmpl
    │   │   └── agent_docs/
    │   │       ├── README.md.tmpl
    │   │       ├── gotchas.md
    │   │       └── runbooks/
    │   │           └── dev.md.tmpl
    │   └── hooks/
    │       └── post-apply.sh
    │
    ├── eslint/                   # Partial: eslint
    │   ├── partial.json
    │   └── files/
    │       ├── .eslintrc.json.tmpl
    │       └── .eslintignore
    │
    ├── python-tooling/           # Partial: python-tooling
    │   ├── partial.json
    │   └── files/
    │       ├── pyproject.toml.tmpl
    │       ├── .python-version
    │       └── ruff.toml
    │
    └── github-actions/           # Partial: github-actions
        ├── partial.json
        └── files/
            └── .github/
                └── workflows/
                    ├── ci.yml.tmpl
                    └── release.yml.tmpl
```

### 4.2 Fallback Locations

Like templates, partials support fallback directories:

1. **Primary:** `~/Code/_system/partials/`
2. **Fallback:** `~/.config/co/partials/`

### 4.3 Naming Convention

- Partial names: lowercase alphanumeric with hyphens
- Pattern: `^[a-z0-9][a-z0-9-]*$`
- No reserved names (unlike `_global` for templates)

---

## 5. Partial Manifest (`partial.json`)

### 5.1 Schema

```json
{
  "schema": 1,
  "name": "agent-setup",
  "description": "AI agent configuration files for multi-agent coordination",
  "version": "1.0.0",

  "variables": [
    {
      "name": "project_name",
      "description": "Human-readable project name",
      "type": "string",
      "required": false,
      "default": "{{DIRNAME}}"
    },
    {
      "name": "primary_stack",
      "description": "Primary technology stack",
      "type": "choice",
      "choices": ["bun", "node", "python", "go", "rust", "other"],
      "required": true
    },
    {
      "name": "use_beads",
      "description": "Include Beads task tracking configuration",
      "type": "boolean",
      "default": true
    }
  ],

  "files": {
    "include": ["**/*"],
    "exclude": ["*.bak", ".DS_Store"],
    "template_extensions": [".tmpl"]
  },

  "conflicts": {
    "strategy": "prompt",
    "preserve": [".beads/**", "*.local.*"]
  },

  "hooks": {
    "pre_apply": "hooks/pre-apply.sh",
    "post_apply": "hooks/post-apply.sh"
  },

  "tags": ["agent", "ai", "coordination"],
  
  "requires": {
    "commands": ["git"],
    "files": []
  }
}
```

### 5.2 Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `schema` | int | Yes | Schema version (currently 1) |
| `name` | string | Yes | Partial identifier (matches directory name) |
| `description` | string | Yes | Human-readable description |
| `version` | string | No | Semantic version |
| `variables` | array | No | Variable definitions (same as templates) |
| `files` | object | No | File handling configuration |
| `conflicts` | object | No | Conflict resolution settings |
| `hooks` | object | No | Lifecycle hooks |
| `tags` | array | No | Categorization tags |
| `requires` | object | No | Prerequisites to check before applying |

### 5.3 Conflict Strategies

The `conflicts.strategy` field controls behavior when a file already exists:

| Strategy | Behavior |
|----------|----------|
| `prompt` | Ask user for each conflict (default) |
| `skip` | Keep existing file, don't overwrite |
| `overwrite` | Replace existing file |
| `backup` | Backup existing to `<file>.bak`, then write new |
| `merge` | Attempt three-way merge (for supported formats) |

The `conflicts.preserve` array lists glob patterns for files that should **never** be overwritten, regardless of strategy.

### 5.4 Built-in Variables

In addition to the standard template built-ins, partials have:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{DIRNAME}}` | Name of the target directory | `backend` |
| `{{DIRPATH}}` | Absolute path to target directory | `/Users/you/Code/acme--app/repos/backend` |
| `{{PARENT_DIRNAME}}` | Name of parent directory | `repos` |
| `{{IS_GIT_REPO}}` | "true" if target is a git repo | `true` |
| `{{GIT_REMOTE_URL}}` | Git remote origin URL (if git repo) | `git@github.com:acme/backend.git` |
| `{{GIT_BRANCH}}` | Current git branch (if git repo) | `main` |

Standard built-ins also available: `{{GIT_USER_NAME}}`, `{{GIT_USER_EMAIL}}`, `{{YEAR}}`, etc.

---

## 6. CLI Interface

### 6.1 Command: `co partial`

```
co partial <subcommand>

Subcommands:
  list                    List available partials
  show <name>             Show partial details and files
  apply <name> [path]     Apply partial to a directory
  validate [name]         Validate partial manifest(s)

Aliases:
  co p                    Shorthand for 'co partial'
```

### 6.2 Command: `co partial list`

```
co partial list [flags]

Flags:
  --json          Output in JSON format
  --tag <tag>     Filter by tag

Examples:
  co partial list
  co partial list --tag agent
  co partial list --json
```

**Output:**
```
NAME              DESCRIPTION                                      FILES  VARS
agent-setup       AI agent configuration files                     8      3
eslint            ESLint configuration for JavaScript/TypeScript   2      1
python-tooling    Python project tooling (ruff, pyproject)         3      2
github-actions    GitHub Actions CI/CD workflows                   2      4
```

### 6.3 Command: `co partial show`

```
co partial show <name> [flags]

Flags:
  --json          Output in JSON format
  --files         List all files that would be created

Examples:
  co partial show agent-setup
  co partial show agent-setup --files
```

**Output:**
```
Partial: agent-setup
Description: AI agent configuration files for multi-agent coordination
Version: 1.0.0

Variables:
  - project_name (string): Human-readable project name
    Default: {{DIRNAME}}
  - primary_stack (choice, required): Primary technology stack
    Choices: bun, node, python, go, rust, other
  - use_beads (boolean): Include Beads task tracking configuration
    Default: true

Files:
  - AGENTS.md
  - .claude/settings.json
  - agent_docs/README.md
  - agent_docs/gotchas.md
  - agent_docs/runbooks/dev.md

Hooks:
  - post_apply: hooks/post-apply.sh

Tags: agent, ai, coordination
```

### 6.4 Command: `co partial apply`

```
co partial apply <name> [path] [flags]

Arguments:
  name            Partial name to apply
  path            Target directory (default: current directory)

Flags:
  -v, --var <key=value>   Set variable (repeatable)
  --conflict <strategy>   Override conflict strategy (prompt|skip|overwrite|backup)
  --dry-run               Preview changes without applying
  --no-hooks              Skip lifecycle hooks
  --force                 Apply even if prerequisites fail
  --yes, -y               Accept all prompts (use conflict strategy without prompting)

Examples:
  # Apply to current directory
  co partial apply agent-setup

  # Apply to specific path
  co partial apply agent-setup ./repos/backend

  # With variables
  co partial apply agent-setup -v primary_stack=go -v use_beads=false

  # Preview mode
  co partial apply agent-setup --dry-run

  # Non-interactive (CI/scripts)
  co partial apply agent-setup --conflict skip -y
```

### 6.5 Interactive Flow

When running `co partial apply agent-setup ./repos/backend`:

```
Applying partial: agent-setup
Target: /Users/you/Code/acme--app/repos/backend

Variables:
  project_name [backend]: My Backend Service
  primary_stack (bun/node/python/go/rust/other): go
  use_beads [true]: (enter)

Files to create:
  ✓ AGENTS.md (new)
  ✓ .claude/settings.json (new)
  ✓ agent_docs/README.md (new)
  ✓ agent_docs/gotchas.md (new)
  ✓ agent_docs/runbooks/dev.md (new)

Conflicts:
  ? .gitignore (exists)
    [s]kip  [o]verwrite  [b]ackup  [d]iff  [a]ll-skip  [A]ll-overwrite: d

    --- existing .gitignore
    +++ partial .gitignore
    @@ -1,3 +1,5 @@
     node_modules/
    +.beads/
    +.co-*
     .env

    [s]kip  [o]verwrite  [b]ackup  [m]erge: m

Applying partial...
  ✓ Created AGENTS.md
  ✓ Created .claude/settings.json
  ✓ Created agent_docs/README.md
  ✓ Created agent_docs/gotchas.md
  ✓ Created agent_docs/runbooks/dev.md
  ✓ Merged .gitignore
  ✓ Running hook: post-apply.sh

Partial applied successfully!
```

---

## 7. Hooks

### 7.1 Hook Types

| Hook | Trigger | Use Case |
|------|---------|----------|
| `pre_apply` | Before any files are written | Validation, prerequisites |
| `post_apply` | After all files are written | Setup commands, git operations |

### 7.2 Hook Environment Variables

Hooks receive all template variables plus partial-specific ones:

```bash
# Standard
CO_PARTIAL_NAME=agent-setup
CO_PARTIAL_PATH=/Users/you/Code/_system/partials/agent-setup
CO_TARGET_PATH=/Users/you/Code/acme--app/repos/backend
CO_TARGET_DIRNAME=backend
CO_DRY_RUN=false
CO_VERBOSE=false

# Git info (if target is a git repo)
CO_IS_GIT_REPO=true
CO_GIT_REMOTE_URL=git@github.com:acme/backend.git
CO_GIT_BRANCH=main

# User variables (prefixed)
CO_VAR_project_name=My Backend Service
CO_VAR_primary_stack=go
CO_VAR_use_beads=true

# Files created (newline-separated)
CO_FILES_CREATED=/path/to/AGENTS.md
/path/to/.claude/settings.json
...

# Conflicts resolved
CO_FILES_SKIPPED=...
CO_FILES_OVERWRITTEN=...
CO_FILES_MERGED=...
```

### 7.3 Example Hook

```bash
#!/usr/bin/env bash
# hooks/post-apply.sh
set -euo pipefail

echo "Setting up agent configuration in ${CO_TARGET_DIRNAME}..."

# Initialize beads if requested
if [[ "${CO_VAR_use_beads}" == "true" ]]; then
    if command -v bd &> /dev/null; then
        echo "Initializing Beads..."
        cd "$CO_TARGET_PATH"
        bd init --quiet || true
    else
        echo "Note: Install 'bd' (Beads) for task tracking"
    fi
fi

# Add to .gitignore if git repo
if [[ "${CO_IS_GIT_REPO}" == "true" ]]; then
    if ! grep -q ".beads/" "$CO_TARGET_PATH/.gitignore" 2>/dev/null; then
        echo -e "\n# Agent tooling\n.beads/*.db\n.beads/*.log" >> "$CO_TARGET_PATH/.gitignore"
    fi
fi

echo "Agent setup complete!"
```

---

## 8. Integration with Templates

### 8.1 Templates Referencing Partials

Templates can declare partials to apply during workspace creation:

```json
{
  "schema": 1,
  "name": "fullstack",
  "description": "Full-stack application",
  
  "repos": [
    { "name": "frontend", "init": true },
    { "name": "backend", "init": true }
  ],

  "partials": [
    {
      "name": "agent-setup",
      "target": ".",
      "variables": {
        "project_name": "{{PROJECT}}",
        "primary_stack": "{{frontend_stack}}"
      }
    },
    {
      "name": "agent-setup", 
      "target": "repos/frontend",
      "variables": {
        "project_name": "{{PROJECT}} Frontend",
        "primary_stack": "{{frontend_stack}}"
      }
    },
    {
      "name": "agent-setup",
      "target": "repos/backend",
      "variables": {
        "project_name": "{{PROJECT}} Backend",
        "primary_stack": "{{backend_stack}}"
      }
    },
    {
      "name": "eslint",
      "target": "repos/frontend",
      "when": "{{frontend_stack}} == 'node'"
    }
  ]
}
```

### 8.2 Partial Application Order

When a template references partials:

1. Workspace structure is created
2. Global files are applied
3. Template files are applied
4. `post_create` hook runs
5. Repos are cloned/initialized
6. `post_clone` hook runs
7. **Partials are applied** (in order listed)
8. `post_complete` hook runs

### 8.3 Conditional Partials

The `when` field supports simple conditions:

```json
{
  "name": "eslint",
  "target": "repos/frontend",
  "when": "{{use_eslint}} == 'true'"
}
```

Supported operators: `==`, `!=`

---

## 9. Conflict Resolution

### 9.1 Detection

Before applying, `co` scans the target directory to identify conflicts:

```go
type ConflictInfo struct {
    Path         string      // Relative path in target
    SourcePath   string      // Path in partial
    ExistsInTarget bool
    TargetModTime  time.Time
    Strategy     ConflictStrategy
}
```

### 9.2 Resolution Strategies

**Skip:**
- Keep existing file unchanged
- Log that file was skipped
- Continue with next file

**Overwrite:**
- Replace existing file with partial's version
- No backup created
- Variables are substituted in new content

**Backup:**
- Rename existing to `<filename>.bak` (or `<filename>.bak.1`, `.bak.2`, etc.)
- Write new file
- Log backup location

**Merge (for supported formats):**
- For `.gitignore`, `.env.example`: append unique lines
- For JSON: deep merge (partial values override)
- For YAML: deep merge (partial values override)
- For other formats: fall back to prompt

### 9.3 Preserve Patterns

Files matching `conflicts.preserve` patterns are **never** modified:

```json
{
  "conflicts": {
    "strategy": "overwrite",
    "preserve": [
      ".beads/**",
      "*.local.*",
      ".env",
      "secrets/**"
    ]
  }
}
```

If a partial includes a file matching a preserve pattern and that file exists, it's silently skipped.

---

## 10. Dry Run Mode

`--dry-run` shows exactly what would happen without making changes:

```
$ co partial apply agent-setup ./repos/backend --dry-run

DRY RUN - No changes will be made

Partial: agent-setup
Target: /Users/you/Code/acme--app/repos/backend

Variables (resolved):
  project_name = "backend"
  primary_stack = "go"
  use_beads = "true"

Actions:
  CREATE  AGENTS.md
  CREATE  .claude/settings.json
  CREATE  agent_docs/README.md
  CREATE  agent_docs/gotchas.md
  CREATE  agent_docs/runbooks/dev.md
  SKIP    .gitignore (exists, strategy: skip)

Hooks:
  WOULD RUN  post-apply.sh

Summary:
  5 files would be created
  1 file would be skipped
  0 files would be overwritten
```

---

## 11. Data Model

### 11.1 New Types

**File: `internal/partial/partial.go`**

```go
package partial

type Partial struct {
    Schema      int              `json:"schema"`
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Version     string           `json:"version,omitempty"`
    Variables   []PartialVar     `json:"variables,omitempty"`
    Files       PartialFiles     `json:"files,omitempty"`
    Conflicts   ConflictConfig   `json:"conflicts,omitempty"`
    Hooks       PartialHooks     `json:"hooks,omitempty"`
    Tags        []string         `json:"tags,omitempty"`
    Requires    Requirements     `json:"requires,omitempty"`
}

type PartialVar struct {
    Name        string      `json:"name"`
    Description string      `json:"description,omitempty"`
    Type        VarType     `json:"type"`
    Required    bool        `json:"required"`
    Default     interface{} `json:"default,omitempty"`
    Validation  string      `json:"validation,omitempty"`
    Choices     []string    `json:"choices,omitempty"`
}

type PartialFiles struct {
    Include            []string `json:"include,omitempty"`
    Exclude            []string `json:"exclude,omitempty"`
    TemplateExtensions []string `json:"template_extensions,omitempty"`
}

type ConflictConfig struct {
    Strategy string   `json:"strategy,omitempty"` // prompt, skip, overwrite, backup, merge
    Preserve []string `json:"preserve,omitempty"` // glob patterns to never overwrite
}

type PartialHooks struct {
    PreApply  HookSpec `json:"pre_apply,omitempty"`
    PostApply HookSpec `json:"post_apply,omitempty"`
}

type Requirements struct {
    Commands []string `json:"commands,omitempty"` // e.g., ["git", "node"]
    Files    []string `json:"files,omitempty"`    // e.g., ["package.json"]
}

type ApplyOptions struct {
    PartialName      string
    TargetPath       string
    Variables        map[string]string
    ConflictStrategy string
    DryRun           bool
    NoHooks          bool
    Force            bool
    Yes              bool // Accept all prompts
}

type ApplyResult struct {
    PartialName     string   `json:"partial_name"`
    TargetPath      string   `json:"target_path"`
    FilesCreated    []string `json:"files_created"`
    FilesSkipped    []string `json:"files_skipped"`
    FilesOverwritten []string `json:"files_overwritten"`
    FilesMerged     []string `json:"files_merged"`
    FilesBackedUp   []string `json:"files_backed_up"`
    HooksRun        []string `json:"hooks_run,omitempty"`
    Warnings        []string `json:"warnings,omitempty"`
}
```

### 11.2 Config Extension

**Addition to `internal/config/config.go`:**

```go
// PartialsDir returns the path to the primary partials directory.
func (c *Config) PartialsDir() string {
    return filepath.Join(c.SystemDir(), "partials")
}

// FallbackPartialsDir returns the XDG config partials directory.
func (c *Config) FallbackPartialsDir() string {
    xdgConfig := os.Getenv("XDG_CONFIG_HOME")
    if xdgConfig == "" {
        home, _ := os.UserHomeDir()
        xdgConfig = filepath.Join(home, ".config")
    }
    return filepath.Join(xdgConfig, "co", "partials")
}

// AllPartialsDirs returns all partial directories to search, in priority order.
func (c *Config) AllPartialsDirs() []string {
    return []string{c.PartialsDir(), c.FallbackPartialsDir()}
}
```

---

## 12. Package Structure

```
internal/partial/
├── partial.go        # Core types and constants
├── loader.go         # Discovery, loading, validation
├── apply.go          # Main apply logic
├── variables.go      # Variable resolution (reuses template.SubstituteVariables)
├── files.go          # File processing and conflict detection
├── conflicts.go      # Conflict resolution strategies
├── merge.go          # Format-specific merge implementations
├── hooks.go          # Hook execution
└── errors.go         # Error types

cmd/co/cmd/
└── partial.go        # CLI commands (list, show, apply, validate)
```

---

## 13. Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Partial data model and types
- [ ] Loader (discovery, validation)
- [ ] Basic `co partial list` command
- [ ] Basic `co partial show` command

### Phase 2: Apply Logic
- [ ] File copying with variable substitution (reuse template code)
- [ ] Conflict detection
- [ ] Simple conflict strategies (skip, overwrite, backup)
- [ ] `co partial apply` command (non-interactive)
- [ ] Dry-run mode

### Phase 3: Interactive & Hooks
- [ ] Interactive conflict prompts
- [ ] Hook execution (pre_apply, post_apply)
- [ ] Built-in variables (DIRNAME, IS_GIT_REPO, etc.)

### Phase 4: Advanced Features
- [ ] Merge strategy for supported formats
- [ ] Template integration (partials in template.json)
- [ ] Conditional partials (`when` field)
- [ ] Prerequisites checking (`requires` field)

### Phase 5: Polish
- [ ] TUI for partial selection and application
- [ ] Built-in partials (agent-setup, etc.)
- [ ] Documentation
- [ ] Tests

---

## 14. Example Partials

### 14.1 `agent-setup`

**`partial.json`:**
```json
{
  "schema": 1,
  "name": "agent-setup",
  "description": "AI agent configuration for multi-agent development workflows",
  "version": "1.0.0",
  "variables": [
    {
      "name": "project_name",
      "description": "Project name for documentation",
      "type": "string",
      "default": "{{DIRNAME}}"
    },
    {
      "name": "primary_stack",
      "description": "Primary technology stack",
      "type": "choice",
      "choices": ["bun", "node", "python", "go", "rust", "other"],
      "required": true
    }
  ],
  "files": {
    "include": ["**/*"],
    "exclude": ["partial.json", "hooks/**"]
  },
  "conflicts": {
    "strategy": "prompt",
    "preserve": [".beads/beads.db", ".beads/*.log"]
  },
  "hooks": {
    "post_apply": "hooks/post-apply.sh"
  },
  "tags": ["agent", "ai", "workflow"]
}
```

**`files/AGENTS.md.tmpl`:**
```markdown
# AGENTS.md — Project Agent Operating Manual

This repo is commonly worked on by MULTIPLE AGENTS IN PARALLEL.
Assume the workspace may change while you work.

---

## 0) Repo Quick Facts

- Project: {{project_name}}
- Primary stack: {{primary_stack}}
- "How to run" (authoritative): `agent_docs/runbooks/dev.md`
- "How to test" (authoritative): `agent_docs/runbooks/test.md`

---

## 1) Non-negotiables

### Multi-agent coordination is mandatory
- Before editing files, coordinate via **MCP Agent Mail**
- Reserve the paths you will touch (leases) and announce intent

... (rest of AGENTS.md content)
```

### 14.2 `python-tooling`

**`partial.json`:**
```json
{
  "schema": 1,
  "name": "python-tooling",
  "description": "Modern Python project tooling with ruff and pyproject.toml",
  "version": "1.0.0",
  "variables": [
    {
      "name": "python_version",
      "description": "Python version",
      "type": "string",
      "default": "3.12"
    },
    {
      "name": "package_name",
      "description": "Package name",
      "type": "string",
      "default": "{{DIRNAME}}"
    }
  ],
  "conflicts": {
    "strategy": "prompt"
  },
  "requires": {
    "commands": ["python3"]
  },
  "tags": ["python", "tooling"]
}
```

---

## 15. Error Handling

### 15.1 Error Types

| Error | Trigger | Response |
|-------|---------|----------|
| `PartialNotFound` | Partial doesn't exist | List available partials |
| `InvalidManifest` | `partial.json` parse failure | Show validation errors |
| `TargetNotFound` | Target directory doesn't exist | Error with suggestion to create |
| `PrerequisiteFailed` | Required command/file missing | Show what's missing, suggest --force |
| `HookFailed` | Hook returns non-zero | Show output, partial may be partially applied |
| `ConflictAborted` | User cancelled during prompt | Clean exit, no partial changes |

### 15.2 Partial Application (Atomicity)

Partials are **not atomic** — if application fails midway, some files may have been written. This is intentional:

- Allows user to fix issues and re-run
- Avoids complexity of transaction rollback
- `--dry-run` exists for preview

On failure:
- Show which files were written
- Show which files remain to be written
- Suggest re-running with `--conflict skip` to continue

---

## 16. Security Considerations

### 16.1 Path Safety

- Target path must be a directory
- No path traversal (`../`) in file paths within partial
- All output files must be within target directory

### 16.2 Hook Safety

Same as templates:
- Local scripts only
- Run as current user
- Timeout enforcement
- No sudo

### 16.3 Variable Sanitization

Same as templates:
- No shell expansion in substitution
- Regex validation supported
- Size limits on values

---

## 17. Future Considerations

### 17.1 Potential Extensions

- **Partial composition:** Partials that include other partials
- **Partial updates:** Re-apply partial to update files (with smarter merging)
- **Partial removal:** `co partial remove` to undo application
- **Workspace-level partial tracking:** Record which partials were applied in `project.json`

### 17.2 Not In Scope (For Now)

- Remote partial fetching
- Partial versioning/updates after application
- Automatic partial detection ("this looks like a Python project")

---

## 18. Glossary

| Term | Definition |
|------|------------|
| **Partial** | A reusable set of files that can be applied to any directory |
| **Template** | A workspace-level blueprint (creates workspace + repos) |
| **Conflict** | When a partial file already exists in the target |
| **Strategy** | How to handle conflicts (skip, overwrite, backup, merge, prompt) |
| **Preserve** | Files that should never be overwritten by the partial |

---

*End of specification*
