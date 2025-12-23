# co — Code Organization

> A Go-powered workspace manager that enforces a single, machine-readable project structure under `~/Code`, provides a fast TUI for navigating and operating on projects, and supports safe one-command syncing to remote servers.

---

## Overview

`co` solves the problem of disorganized code directories by enforcing a strict, predictable filesystem layout. Every project follows the same structure, making it trivial for both humans and automation (agents, scripts, tools) to locate and operate on projects.

### Key Features

- **Unified workspace structure** — Every project lives under `~/Code` with a consistent `<owner>--<project>` naming convention
- **Machine-readable metadata** — `project.json` in every workspace; global `index.jsonl` for fast querying
- **Interactive TUI** — Browse, search, and manage projects with a polished terminal interface
- **Remote sync** — One-command rsync to named servers with safety checks
- **Safe archiving** — Git-bundle based archives that preserve full history
- **Semantic code search** — Find code by meaning using AST-aware chunking and vector embeddings (via Ollama)

---

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/youruser/co.git
cd co

# Build
make build

# Install to $GOPATH/bin
make install
```

### Requirements

- Go 1.21+
- Git (for repo operations)
- rsync (for remote sync, optional)

---

## Quick Start

```bash
# Launch the TUI (default action)
co

# Create a new workspace
co new personal my-project

# Create a workspace with repos
co new acme webapp https://github.com/acme/frontend.git https://github.com/acme/backend.git

# Create a workspace using a template
co new acme dashboard -t fullstack

# List available templates
co new --list-templates

# Regenerate the global index
co index

# Sync a workspace to a remote server
co sync acme--webapp prod
```

---

## Filesystem Layout

### Root Structure

All workspaces live under a single root (default: `~/Code`):

```
~/Code/
├── _system/                    # Reserved for co internals
│   ├── archive/                # Archived workspaces
│   ├── cache/                  # Temporary data
│   ├── index.jsonl             # Global project index
│   └── logs/                   # Debug logs
├── acme--dashboard/            # Workspace: owner=acme, project=dashboard
├── acme--api/                  # Workspace: owner=acme, project=api
├── personal--dotfiles/         # Workspace: owner=personal, project=dotfiles
└── oss--contrib-rails/         # Workspace: owner=oss, project=contrib-rails
```

### Workspace Structure

Every workspace follows this exact layout—no exceptions:

```
<workspace>/
├── project.json                # Canonical metadata
└── repos/                      # Git repositories
    ├── frontend/
    ├── backend/
    └── ...
```

### Naming Convention

Workspace slugs follow the format: `<owner>--<project>`

| Owner Type | Examples |
|------------|----------|
| Client/org | `acme--dashboard`, `bigcorp--analytics` |
| Personal | `personal--dotfiles`, `personal--blog` |
| Open source | `oss--neovim-config`, `oss--contrib-rails` |
| Company | `mycompany--platform`, `startup--mvp` |

**Variants** use a controlled qualifier suffix: `owner--project--qualifier`
- Allowed: `poc`, `demo`, `legacy`, `migration`, `infra`
- Forbidden: `-old`, `-new`, `-2`, `final-final`

---

## Data Model

### project.json (v1)

Each workspace contains a `project.json` file as the source of truth:

```json
{
  "schema": 1,
  "slug": "acme--dashboard",
  "owner": "acme",
  "name": "dashboard",
  "state": "active",
  "tags": ["client", "web"],
  "created": "2025-12-13",
  "updated": "2025-12-13",
  "repos": [
    {
      "name": "frontend",
      "path": "repos/frontend",
      "remote": "git@github.com:acme/dashboard-frontend.git"
    },
    {
      "name": "backend",
      "path": "repos/backend",
      "remote": "git@github.com:acme/dashboard-backend.git"
    }
  ],
  "notes": "Main dashboard application"
}
```

**State vocabulary:** `active` | `paused` | `archived` | `scratch`

### index.jsonl

The global index at `~/Code/_system/index.jsonl` is computed from disk and provides fast access for the TUI and CLI:

```json
{
  "schema": 1,
  "slug": "acme--dashboard",
  "path": "/Users/you/Code/acme--dashboard",
  "owner": "acme",
  "state": "active",
  "tags": ["client", "web"],
  "repo_count": 2,
  "last_commit_at": "2025-12-10T12:34:56Z",
  "last_fs_change_at": "2025-12-13T09:21:00Z",
  "dirty_repos": 1,
  "size_bytes": 128731293,
  "repos": [
    { "name": "frontend", "path": "repos/frontend", "head": "a1b2c3d", "branch": "main", "dirty": true },
    { "name": "backend", "path": "repos/backend", "head": "d4e5f6g", "branch": "main", "dirty": false }
  ]
}
```

---

## CLI Reference

### Default Behavior

Running `co` with no arguments launches the TUI.

### Commands

#### `co tui`

Launch the interactive TUI dashboard.

```bash
co          # Same as `co tui`
co tui
```

#### `co new <owner> <project> [repo-url...]`

Create a new workspace with the standard structure.

```bash
# Empty workspace
co new personal my-project

# With cloned repos
co new acme webapp \
  https://github.com/acme/frontend.git \
  https://github.com/acme/backend.git

# Using a template
co new acme dashboard -t fullstack

# With template variables
co new acme api -t backend -v port=8080 -v db_name=app_db

# Interactive mode (prompts for owner, project, and template)
co new

# Preview template creation without making changes
co new acme app -t fullstack --dry-run
```

**Template flags:**

| Flag | Description |
|------|-------------|
| `-t, --template <name>` | Use a template for workspace creation |
| `-v, --var <key=value>` | Set template variable (can be repeated) |
| `--no-hooks` | Skip running lifecycle hooks |
| `--dry-run` | Preview creation without making changes |
| `--list-templates` | List available templates |
| `--show-template <name>` | Show template details |

#### `co index`

Scan `~/Code` and regenerate `_system/index.jsonl`. Computes:
- Last commit date across repos
- Last filesystem change
- Repo dirty flags
- Workspace size

```bash
co index
```

#### `co ls`

List workspaces with optional filters.

```bash
co ls                          # All workspaces
co ls --owner acme             # Filter by owner
co ls --state active           # Filter by state
co ls --tag client             # Filter by tag
co ls --json                   # JSON output
```

#### `co show <workspace-slug>`

Display detailed workspace information.

```bash
co show acme--dashboard
co show acme--dashboard --json
```

#### `co template`

Launch the Template Explorer TUI to browse, inspect, and create workspaces from templates.

```bash
co template            # Launch Template Explorer TUI
co template list       # List templates (non-interactive)
co template show <name>    # Show template details
co template validate [name]    # Validate one or all templates
```

The Template Explorer provides an interactive interface for:
- Browsing available templates across all configured directories
- Viewing template details, variables, repos, and hooks
- Creating new workspaces with variable prompting
- Validating template manifests

See the [Template Explorer TUI](#template-explorer-tui) section for keybindings.

#### `co import-tui [path]`

Launch an interactive TUI for browsing folders and importing them as workspaces. This is useful for organizing existing codebases into the `co` workspace structure.

```bash
co import-tui                    # Browse current directory
co import-tui ~/projects         # Browse ~/projects
co import-tui ./legacy-code      # Browse ./legacy-code
```

The import browser provides:
- Tree view of the folder structure with git repository detection
- Import folders as new workspaces with owner/project configuration
- Add repos to existing workspaces
- Apply templates during import
- Stash (archive) folders for later
- Batch operations on multiple selected folders

See the [Import Browser TUI](#import-browser-tui) section for the full workflow and keybindings.

#### `co open <workspace-slug>`

Open a workspace in your configured editor.

```bash
co open acme--dashboard
```

#### `co archive <workspace-slug>`

Archive a workspace to `_system/archive/`.

```bash
co archive old--project                    # Archive only
co archive old--project --delete           # Archive and delete local
co archive old--project --reason "EOL"     # Add reason to metadata
```

#### `co sync <workspace-slug> <server>`

Sync a workspace to a remote server.

```bash
co sync acme--dashboard prod                # Safe: fails if remote exists
co sync acme--dashboard prod --force        # Overwrite remote
co sync acme--dashboard prod --dry-run      # Preview only
co sync acme--dashboard prod --no-git       # Exclude .git directories
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 10 | Sync skipped (remote exists) |

### Machine-Readable Output

All listing/show commands support `--json` or `--jsonl` for scripting:

```bash
co ls --json | jq '.[] | select(.dirty_repos > 0)'
co show my--project --json
```

---

## Configuration

### Config File Location

Configuration is discovered in order:

1. `--config <path>` flag
2. `$XDG_CONFIG_HOME/co/config.json` or `~/.config/co/config.json`
3. `~/Code/_system/config.json` (optional)

### Config Schema

```json
{
  "schema": 1,
  "code_root": "~/Code",
  "editor": "code",
  "servers": {
    "prod": {
      "ssh": "prod",
      "code_root": "~/Code"
    },
    "devbox": {
      "ssh": "devbox",
      "code_root": "/home/ubuntu/Code"
    }
  }
}
```

**Server definitions:**
- `ssh` — SSH alias or host (uses `~/.ssh/config`)
- `code_root` — Remote code root (defaults to `~/Code`)

---

## Templates

Templates provide reusable workspace scaffolding with pre-configured files, repos, and setup scripts.

> **Tip:** Use `co template` to launch the [Template Explorer TUI](#template-explorer-tui) for interactive template browsing and workspace creation.

### Template Location

Templates are searched in multiple directories with the following precedence:

1. **Primary:** `<code_root>/_system/templates/` (e.g., `~/Code/_system/templates/`)
2. **Fallback:** `~/.config/co/templates/` (or `$XDG_CONFIG_HOME/co/templates/`)

When templates with the same name exist in multiple locations, the primary location takes precedence. Each template is a directory containing a `template.json` manifest.

```
~/Code/_system/templates/
├── fullstack/
│   ├── template.json
│   ├── .claude/
│   │   └── AGENTS.md
│   └── docker-compose.yml
├── backend/
│   └── template.json
└── _global/                # Files copied to ALL template-based workspaces
    └── .editorconfig
```

### Template Manifest

Each template has a `template.json` file:

```json
{
  "name": "fullstack",
  "description": "Full-stack web application with frontend and backend",
  "variables": [
    {
      "name": "port",
      "type": "integer",
      "default": 3000,
      "description": "Development server port"
    },
    {
      "name": "db_type",
      "type": "choice",
      "choices": ["postgres", "mysql", "sqlite"],
      "default": "postgres",
      "required": true
    }
  ],
  "repos": [
    {
      "name": "frontend",
      "clone_url": "https://github.com/example/frontend-template.git",
      "tags": ["web", "react"]
    },
    {
      "name": "backend",
      "init": true,
      "tags": ["api"]
    }
  ],
  "files": {
    "include": ["**/*"],
    "exclude": ["template.json", "*.md"]
  },
  "hooks": {
    "post_create": "echo 'Workspace created!'",
    "post_clone": {
      "command": "./setup.sh",
      "env": {"PORT": "{{port}}"}
    }
  },
  "tags": ["web", "fullstack"],
  "state": "active"
}
```

### Built-in Variables

These variables are automatically available in all templates:

| Variable | Description | Example |
|----------|-------------|---------|
| `owner` | Workspace owner | `acme` |
| `project` | Project name | `dashboard` |
| `slug` | Full workspace slug | `acme--dashboard` |
| `workspace_path` | Absolute path | `/Users/you/Code/acme--dashboard` |
| `code_root` | Code root directory | `/Users/you/Code` |

### Variable Substitution

Template files support `{{variable}}` substitution:

```yaml
# docker-compose.yml
services:
  app:
    ports:
      - "{{port}}:{{port}}"
    environment:
      DB_TYPE: {{db_type}}
      PROJECT: {{project}}
```

### Variable Types

| Type | Description |
|------|-------------|
| `string` | Free-form text |
| `integer` | Whole numbers |
| `boolean` | true/false (prompted as yes/no toggle) |
| `choice` | Selection from predefined list |

### Lifecycle Hooks

Templates support hooks that run at different stages:

| Hook | When it runs |
|------|--------------|
| `pre_create` | Before workspace directory is created |
| `post_create` | After workspace structure is created |
| `post_clone` | After each repo is cloned |
| `post_complete` | After all setup is complete |
| `post_migrate` | After applying template to existing workspace |

Hooks can be a simple command string or an object with `command`, `workdir`, and `env` fields.

### Global Template Files

Files in `~/Code/_system/templates/_global/` are copied to every workspace created with any template. Use this for shared configuration like `.editorconfig`, `.gitattributes`, or shared scripts.

### Creating Templates

1. Create a directory in `~/Code/_system/templates/`:
   ```bash
   mkdir -p ~/Code/_system/templates/my-template
   ```

2. Add a `template.json` manifest:
   ```json
   {
     "name": "my-template",
     "description": "My custom workspace template"
   }
   ```

3. Add any files you want copied to new workspaces.

4. Use the template:
   ```bash
   co new acme project -t my-template
   ```

---

## Remote Sync

### How It Works

1. Check if remote path exists
2. If exists: **stop** (exit code 10) unless `--force`
3. If not exists: create directory and sync

### Transport

- **Preferred:** rsync over SSH (`rsync -az --partial --progress`)
- **Fallback:** tar streaming over SSH

### Default Excludes

Common build artifacts, dependency caches, and sensitive files are excluded by default:

| Category | Patterns |
|----------|----------|
| **Dependencies** | `node_modules/`, `vendor/`, `.pnpm-store/`, `bower_components/` |
| **Build outputs** | `target/`, `dist/`, `build/`, `out/`, `bin/`, `obj/`, `_build/` |
| **Frameworks** | `.next/`, `.nuxt/`, `.output/`, `.svelte-kit/`, `.vercel/`, `.netlify/` |
| **Caches** | `.cache/`, `__pycache__/`, `.pytest_cache/`, `.mypy_cache/`, `.turbo/` |
| **Virtual envs** | `.venv/`, `venv/`, `.virtualenv/` |
| **Coverage** | `coverage/`, `.nyc_output/`, `htmlcov/`, `.tox/`, `.nox/` |
| **IDE** | `.idea/`, `*.swp`, `*.swo`, `*~`, `.settings/` |
| **OS** | `.DS_Store`, `Thumbs.db`, `Desktop.ini` |
| **Logs** | `*.log`, `logs/`, `npm-debug.log*`, `yarn-*.log*` |
| **Secrets** | `.env`, `.env.*`, `secrets/`, `*.pem`, `*.key` |
| **Terraform** | `.terraform/`, `*.tfstate`, `*.tfstate.*` |

Use `co sync --list-excludes` to see the full list.

### Options

| Flag | Effect |
|------|--------|
| `--force` | Sync even if remote exists |
| `--dry-run` | Preview without changes |
| `--no-git` | Exclude `.git/` directories |
| `-i, --interactive` | Launch TUI to select excludes before syncing |
| `--exclude <pattern>` | Add pattern to exclude (repeatable) |
| `--exclude-from <file>` | Read patterns from file (one per line, `#` comments) |
| `--include-env` | Include `.env` files (override default exclude) |
| `--list-excludes` | Print effective exclude list and exit |

### Interactive Mode

Launch with `co sync <workspace> <server> --interactive` to select files/directories to exclude:

```bash
co sync acme--backend prod -i
```

**Keybindings:**

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate list |
| `space` | Toggle exclude (red ✗ = excluded) |
| `h/l` or `←/→` | Collapse/expand directory |
| `g/G` | Jump to top/bottom |
| `c` | Clear all exclusions |
| `r` | Reset to defaults |
| `enter` | Confirm and sync |
| `S` | Save selections to project.json and sync |
| `q` or `esc` | Cancel |

Default excludes start pre-selected. Excluding a directory excludes its entire subtree.

### Per-Workspace Excludes

Exclude patterns can be persisted in `project.json` so they apply automatically on future syncs:

```json
{
  "sync": {
    "excludes": {
      "add": ["data/raw/", "*.sqlite"],
      "remove": ["vendor/"]
    },
    "include_env": false
  }
}
```

- `add`: Patterns to add to default excludes
- `remove`: Patterns to remove from default excludes
- `include_env`: Set to `true` to include `.env` files

Use `S` in interactive mode to save your current selections to `project.json`.

---

## Archiving

Archives use git bundles to preserve full history without copying build artifacts.

### Archive Format

```
~/Code/_system/archive/2025/acme--dashboard--20251213-143022.tar.gz
├── project.json
├── archive-meta.json
├── repos__frontend.bundle
└── repos__backend.bundle
```

### Why Bundles?

- Avoids copying `node_modules`, `target/`, etc.
- Preserves all git history and unpushed work
- Restores without network access

---

## Semantic Code Search

`co` includes a semantic code search feature that lets you find code by meaning rather than exact text matching. For example, searching for "authentication middleware" can find auth-related functions even if they don't contain those exact words.

### How It Works

1. **AST-aware chunking** — Uses tree-sitter to extract semantic units (functions, classes, methods) rather than arbitrary line splits
2. **Local embeddings** — Generates vector embeddings via Ollama (runs locally, free)
3. **Vector similarity** — Stores embeddings in SQLite with sqlite-vec for fast similarity search

### Prerequisites

Install and run [Ollama](https://ollama.ai):

```bash
# Install Ollama (macOS)
brew install ollama

# Start the server
ollama serve

# Pull the embedding model (768 dimensions, ~270MB)
ollama pull nomic-embed-text
```

### Commands

#### `co vector index [codebase...]`

Index codebases for semantic search.

```bash
# Index a specific codebase
co vector index acme--backend

# Index multiple codebases
co vector index acme--api acme--web

# Index all active codebases
co vector index --all

# Index with verbose output
co vector index acme--backend -v
```

Indexing is **incremental** — unchanged files are skipped based on content hash.

#### `co vector search <query>`

Search for similar code across indexed codebases.

```bash
# Natural language query
co vector search "authentication middleware"

# Code pattern query
co vector search "func handleError(err error)"

# With code content preview
co vector search "database connection" --content

# Limit results
co vector search "api endpoint" -n 5

# Filter by codebase
co vector search "user model" --codebase acme--backend

# Use a file as query
co vector search --file example.go

# JSON output for scripting
co vector search "config parsing" --json
```

#### `co vector stats [codebase]`

Show index statistics.

```bash
co vector stats                     # All codebases
co vector stats --codebase acme     # Specific codebase
co vector stats --json              # JSON output
```

Output includes:
- Total files and chunks indexed
- Per-codebase breakdown
- Language distribution
- Index freshness

#### `co vector clear [codebase]`

Clear index data.

```bash
co vector clear acme--backend       # Clear one codebase
co vector clear --all               # Clear entire index
```

### Configuration

Add to your `config.json`:

```json
{
  "embeddings": {
    "backend": "ollama",
    "ollama_url": "http://localhost:11434",
    "ollama_model": "nomic-embed-text"
  },
  "indexing": {
    "chunk_max_lines": 100,
    "chunk_min_lines": 5,
    "max_file_size_bytes": 1048576,
    "batch_size": 50,
    "workers": 4,
    "exclude_patterns": [
      "**/node_modules/**",
      "**/vendor/**",
      "**/.git/**"
    ]
  }
}
```

### Supported Languages

Tree-sitter parsing extracts semantic chunks from:
- Go
- Python
- JavaScript / TypeScript
- Rust
- Java
- C / C++

Other file types fall back to line-based chunking.

### Understanding Scores

Search results include a similarity score (0-1):
- **0.15-0.20** — Strong semantic match
- **0.10-0.15** — Good match
- **0.05-0.10** — Weak match
- **< 0.05** — Marginal relevance

Use `--min-score` to filter results:

```bash
co vector search "error handling" --min-score 0.1
```

### Data Storage

Vector data is stored in `~/Code/_system/vectors.db` (SQLite with sqlite-vec extension).

---

## TUI

The TUI provides:

- **Project list** — Browse all workspaces with status indicators
- **Search** — Fuzzy-find projects by name, owner, or tags
- **Details panel** — View repos, last activity, dirty state
- **Quick actions** — Open in editor, archive, sync

### Keybindings

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate list |
| `/` | Search |
| `Enter` | Open workspace in editor |
| `a` | Archive workspace |
| `s` | Sync to server |
| `r` | Refresh index |
| `q` | Quit |

---

## Template Explorer TUI

The Template Explorer (`co template`) provides a dedicated interface for working with workspace templates.

### Tabs

| Tab | Purpose |
|-----|---------|
| **Browse** | View all templates with details pane |
| **Files** | Browse template source files *(coming soon)* |
| **Create** | Create new workspace from selected template |
| **Validate** | Validate template manifests |

### Keybindings

#### Global (all tabs)

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next / previous tab |
| `1-4` | Jump to tab by number |
| `q` / `Ctrl+C` | Quit |

#### Browse Tab

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate template list |
| `/` | Search templates |
| `l` / `→` | Switch to details pane |
| `h` / `←` | Switch to list pane |
| `o` | Open template directory in editor |
| `v` | Validate selected template |

#### Create Tab

| Key | Action |
|-----|--------|
| `Tab` / `↓` | Next field |
| `Shift+Tab` / `↑` | Previous field |
| `Space` | Toggle checkbox (dry-run, no-hooks) |
| `Enter` | Submit / confirm |
| `Esc` | Cancel variable prompt or creation |

#### Variable Prompting

When a template has variables, you'll be prompted for each:

| Key | Action |
|-----|--------|
| `Enter` | Confirm value |
| `y/n` | Toggle boolean |
| `j/k` | Navigate choice list |
| `Esc` | Cancel and return to Create tab |

#### Creation Result Screen

| Key | Action |
|-----|--------|
| `o` | Open created workspace in editor |
| `Enter` / `Esc` | Return to Create tab |
| `q` | Quit |

#### Validate Tab

| Key | Action |
|-----|--------|
| `v` | Validate selected template |
| `V` | Validate all templates |
| `j/k` or `↑/↓` | Navigate results |
| `l/h` | Switch panes |

### Template Search Paths

Templates are searched in multiple directories with the following precedence:

1. **Primary:** `<code_root>/_system/templates/` (e.g., `~/Code/_system/templates/`)
2. **Fallback:** `~/.config/co/templates/` (or `$XDG_CONFIG_HOME/co/templates/`)

When templates with the same name exist in multiple locations, the primary location takes precedence. The Template Explorer shows the source directory for each template.

### Global Files

The special `_global` directory contains files copied to ALL template-based workspaces:

```
~/Code/_system/templates/_global/
├── .editorconfig
├── .gitattributes
└── .claude/
    └── settings.json
```

Global files from all template directories are merged, with primary taking precedence over fallback.

---

## Import Browser TUI

The Import Browser (`co import-tui`) provides an interactive interface for migrating existing folders into the `co` workspace structure. It's ideal for organizing legacy codebases, downloaded projects, or any folder structure that needs to be converted into proper workspaces.

### Workflow

The import process follows this flow:

```
Browse → Select → Configure → (Template) → (Extra Files) → Preview → Execute → Post-Import
```

1. **Browse** — Navigate the folder tree to find projects to import
2. **Select** — Choose a folder (single) or multiple folders (batch mode)
3. **Configure** — Enter owner and project name for the workspace slug
4. **Template** *(optional)* — Select a template to apply to the new workspace
5. **Extra Files** *(optional)* — Select non-git files to include in the import
6. **Preview** — Review the import operation before execution
7. **Execute** — Create the workspace and move repositories
8. **Post-Import** — Choose what to do with the source folder (keep/stash/delete)

### Keybindings

#### Browse Mode

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate up/down |
| `h/l` or `←/→` | Collapse/expand directory |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Enter` | Toggle expand/collapse |
| `Space` | Toggle selection (for batch operations) |
| `/` | Enter filter mode |
| `.` | Toggle hidden files |
| `r` | Refresh tree |
| `Tab` | Switch between tree and details pane |
| `i` | Import selected folder(s) |
| `s` | Stash selected folder(s) (keep source) |
| `S` | Stash selected folder(s) (delete source) |
| `a` | Add to existing workspace |
| `q` | Quit |

#### Import Config

| Key | Action |
|-----|--------|
| `Tab` / `↓` | Next field |
| `Shift+Tab` / `↑` | Previous field |
| `Enter` | Continue to next step |
| `Esc` | Cancel and return to browse |

#### Template Selection

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate template list |
| `g/G` | Jump to top/bottom |
| `Enter` | Select template |
| `Esc` | Skip template selection |

#### Extra Files Selection

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate file list |
| `Space` | Toggle file selection |
| `a` | Select all |
| `n` | Select none |
| `Enter` | Confirm selection |
| `Esc` | Skip extra files |

#### Import Preview

| Key | Action |
|-----|--------|
| `Enter` | Execute import |
| `d` | Toggle dry-run mode |
| `Esc` | Go back |

#### Post-Import Options

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate options |
| `1/2/3` | Quick-select option |
| `Enter` | Confirm selection |

Options:
1. **Keep** — Leave source folder in place
2. **Stash** — Archive source folder to `_system/archive/`
3. **Delete** — Remove source folder (with confirmation)

### Features

#### Git Repository Detection

The import browser automatically detects git repositories and displays:
- Repository status (clean/dirty)
- Current branch
- Nested repositories within folders

Folders containing git repos are highlighted with a special indicator.

#### Batch Operations

Select multiple folders using `Space`, then:
- Press `i` to batch import all selected folders
- Press `s` or `S` to batch stash all selected folders

Batch import prompts for a common owner, then creates separate workspaces using each folder's name as the project.

#### Template Application

When importing, you can optionally apply a template to the new workspace. The template's files and hooks are applied after the repositories are moved into place.

#### Add to Existing Workspace

Press `a` to add the selected folder's contents to an existing workspace instead of creating a new one. This is useful for consolidating related repositories.

### Display Indicators

| Indicator | Meaning |
|-----------|---------|
| `📁` | Regular directory |
| `📦` | Git repository |
| `📂` | Expanded directory |
| `✓` | Selected for batch operation |
| `*` | Dirty git repository |

### Examples

**Import a single project:**
```
1. co import-tui ~/old-projects
2. Navigate to the project folder
3. Press 'i' to import
4. Enter owner: "personal"
5. Confirm project name (auto-filled from folder name)
6. Press Enter to continue through preview
7. Choose to keep or stash the source
```

**Batch import multiple projects:**
```
1. co import-tui ~/client-work
2. Use j/k to navigate and Space to select folders
3. Press 'i' to start batch import
4. Enter common owner: "acme"
5. Review and confirm the batch
```

**Add repos to existing workspace:**
```
1. co import-tui ~/downloads
2. Navigate to a cloned repository
3. Press 'a' to add to existing workspace
4. Select the target workspace
5. Confirm the operation
```

---

## Project Structure

```
.
├── cmd/
│   └── co/
│       ├── main.go          # Entry point
│       └── cmd/             # Cobra commands
├── internal/
│   ├── archive/             # Git bundle archive builder
│   ├── chunker/             # AST-aware code chunking (tree-sitter)
│   ├── config/              # Config parsing + server resolution
│   ├── embedder/            # Embedding generation (Ollama client)
│   ├── fs/                  # Workspace scanning, directory walking
│   ├── git/                 # Git inspection (head, branch, dirty)
│   ├── index/               # Index generation + atomic write
│   ├── model/               # Data structures (project, index)
│   ├── search/              # Vector search indexing + querying
│   ├── sync/                # Remote sync (rsync/tar transport)
│   ├── template/            # Workspace templates + variable substitution
│   ├── tui/                 # Bubble Tea models/views
│   └── vectordb/            # SQLite + sqlite-vec database
├── build/                   # Build output
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Shell Integration

### Quick Navigation with `ccd`

Add this function to your `~/.zshrc` or `~/.bashrc` for instant workspace navigation:

```bash
ccd() { cd "$(co cd "$1")" }
```

The `co cd` command supports fuzzy matching, so you can type partial names:

```bash
ccd dashboard      # matches acme--dashboard
ccd api            # matches acme--api-server
ccd acme           # matches first acme--* workspace
```

After adding the function, reload your shell:

```bash
source ~/.zshrc  # or source ~/.bashrc
```

---

## Development

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Build and run
make run

# Full check (fmt + lint + test)
make check
```

---

## Philosophy

### Hard Invariants

1. **Single root** — All workspaces under one directory
2. **Uniform structure** — Every workspace has `project.json` + `repos/`
3. **Metadata is truth** — `project.json` is canonical; index is derived
4. **Index is generated** — System can regenerate state from disk
5. **Safe by default** — Destructive actions require `--force` or `--delete`

### What We Optimize For

- **Machine readability** — Stable JSON schemas, predictable paths
- **Searchability** — Fast filtering/sorting via index
- **Lifecycle** — Archiving and syncing are first-class operations
- **Speed** — TUI feels instantaneous with hundreds of projects

---

## License

MIT
