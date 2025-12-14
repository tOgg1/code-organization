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
```

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

## Remote Sync

### How It Works

1. Check if remote path exists
2. If exists: **stop** (exit code 10) unless `--force`
3. If not exists: create directory and sync

### Transport

- **Preferred:** rsync over SSH (`rsync -az --partial --progress`)
- **Fallback:** tar streaming over SSH

### Default Excludes

```
**/node_modules/
**/target/
**/.next/
**/dist/
**/build/
**/.venv/
**/.pytest_cache/
**/.DS_Store
**/*.log
.env
.env.*
**/secrets/
```

### Options

| Flag | Effect |
|------|--------|
| `--force` | Sync even if remote exists |
| `--dry-run` | Preview without changes |
| `--no-git` | Exclude `.git/` directories |
| `--include-env` | Include `.env` files |

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
