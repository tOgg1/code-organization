# Sync Exclude Sources & Precedence Specification

**Status:** Draft  
**Created:** 2025-12-17  
**Related Issue:** code-organization-408 (Sync: smarter excludes + interactive exclude picker)

---

## 1. Executive Summary

This specification defines how `co sync` computes the effective exclude list from multiple sources. It covers:

1. **Built-in defaults** — Language/framework-aware excludes (Go, Rust, Node, Python, etc.)
2. **Global config overrides** — User-level customization via `config.json`
3. **Per-workspace overrides** — Workspace-level customization via `project.json`
4. **CLI flags** — `--exclude`, `--exclude-from`, `--include-env`, `--no-git`
5. **Interactive picker output** — Runtime selections from the TUI picker

The specification also defines pattern semantics and how patterns map consistently to both rsync and tar transports.

---

## 2. Goals & Non-Goals

### 2.1 Goals

- **G1:** Prevent syncing large build artifacts by default (node_modules, target/, etc.)
- **G2:** Support language-specific exclude sets without user configuration
- **G3:** Allow users to customize excludes at global, workspace, and invocation levels
- **G4:** Ensure consistent behavior between rsync and tar fallback transports
- **G5:** Provide clear precedence rules for combining exclude sources
- **G6:** Support both directory-only and file-pattern excludes

### 2.2 Non-Goals

- **NG1:** Full rsync filter syntax (include/exclude interleaving, `**` anchoring rules)
- **NG2:** Gitignore-style negation patterns (`!pattern`)
- **NG3:** Remote-side exclude evaluation
- **NG4:** Exclude different patterns for different transports

---

## 3. Exclude Sources

### 3.1 Source Hierarchy (Low to High Priority)

| Priority | Source | Location | Applies To |
|----------|--------|----------|------------|
| 1 (lowest) | Built-in defaults | Compiled into `co` | All syncs |
| 2 | Global config | `config.json` → `sync.excludes` | All syncs |
| 3 | Workspace config | `project.json` → `sync.excludes` | This workspace |
| 4 | CLI flags | `--exclude`, `--exclude-from` | This invocation |
| 5 (highest) | Interactive picker | TUI selection | This invocation |

Higher priority sources **add to** lower priority sources; they do not replace them (see §4 Precedence Rules).

### 3.2 Built-in Defaults

The built-in defaults target common build artifacts, dependency caches, and sensitive files across major ecosystems:

```go
var BuiltinExcludes = []string{
    // === Package managers & dependencies ===
    "node_modules/",      // Node.js
    "vendor/",            // Go, PHP, Ruby
    ".pnpm-store/",       // pnpm
    "bower_components/",  // Bower (legacy)
    
    // === Build outputs ===
    "target/",            // Rust, Scala, Java (Maven)
    "dist/",              // Generic build output
    "build/",             // Generic build output
    "out/",               // Generic build output
    "bin/",               // Go, generic binaries
    "obj/",               // .NET, C++
    "_build/",            // Elixir, Erlang
    ".output/",           // Nuxt.js
    ".nuxt/",             // Nuxt.js
    ".next/",             // Next.js
    ".svelte-kit/",       // SvelteKit
    ".vercel/",           // Vercel
    ".netlify/",          // Netlify
    
    // === Test & coverage ===
    "coverage/",          // Generic coverage reports
    ".nyc_output/",       // NYC (Istanbul)
    "htmlcov/",           // Python coverage.py
    ".tox/",              // Python tox
    ".nox/",              // Python nox
    
    // === Caches ===
    ".cache/",            // Generic cache
    "__pycache__/",       // Python bytecode
    ".pytest_cache/",     // pytest
    ".mypy_cache/",       // mypy
    ".ruff_cache/",       // ruff
    "*.pyc",              // Python compiled
    ".turbo/",            // Turborepo
    ".parcel-cache/",     // Parcel
    ".webpack/",          // Webpack
    ".eslintcache",       // ESLint
    ".stylelintcache",    // Stylelint
    
    // === Virtual environments ===
    ".venv/",             // Python venv
    "venv/",              // Python venv (alternate)
    ".virtualenv/",       // virtualenv
    "env/",               // Python env (common name)
    ".env.local",         // Local env overrides
    
    // === IDE & editors ===
    ".idea/",             // JetBrains IDEs
    ".vscode/",           // VS Code (optional - see §6.1)
    "*.swp",              // Vim swap
    "*.swo",              // Vim swap
    "*~",                 // Emacs backup
    ".project",           // Eclipse
    ".classpath",         // Eclipse
    ".settings/",         // Eclipse
    
    // === OS artifacts ===
    ".DS_Store",          // macOS
    "Thumbs.db",          // Windows
    "Desktop.ini",        // Windows
    
    // === Logs ===
    "*.log",              // Log files
    "logs/",              // Log directories
    "npm-debug.log*",     // npm
    "yarn-debug.log*",    // Yarn
    "yarn-error.log*",    // Yarn
    "pnpm-debug.log*",    // pnpm
    
    // === Secrets & sensitive ===
    ".env",               // Environment files
    ".env.*",             // Environment variants
    "!.env.example",      // (negation not supported - see §6.2)
    "secrets/",           // Secrets directory
    "*.pem",              // Certificates
    "*.key",              // Private keys
    ".secret*",           // Secret files
    
    // === Terraform & IaC ===
    ".terraform/",        // Terraform providers
    "*.tfstate",          // Terraform state
    "*.tfstate.*",        // Terraform state backups
    
    // === Containers & deployment ===
    ".docker/",           // Docker context
}
```

**Note:** The full list is defined in `internal/fs/excludes.go` and can be inspected via `co sync --list-excludes`.

### 3.3 Global Config (`config.json`)

Users can customize excludes globally in their config file:

```json
{
  "schema": 1,
  "code_root": "~/Code",
  "sync": {
    "excludes": {
      "add": [
        ".claude/",
        "*.sqlite"
      ],
      "remove": [
        ".vscode/"
      ]
    },
    "include_env": false
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `sync.excludes.add` | `[]string` | Patterns to add to built-in defaults |
| `sync.excludes.remove` | `[]string` | Patterns to remove from built-in defaults |
| `sync.include_env` | `bool` | If true, don't exclude `.env*` files (default: false) |

### 3.4 Workspace Config (`project.json`)

Per-workspace overrides in `project.json`:

```json
{
  "schema": 1,
  "slug": "acme--backend",
  "sync": {
    "excludes": {
      "add": [
        "data/*.csv",
        "fixtures/"
      ],
      "remove": [
        "vendor/"
      ]
    }
  }
}
```

Same schema as global config. Workspace settings further modify the effective list after global config is applied.

### 3.5 CLI Flags

| Flag | Syntax | Description |
|------|--------|-------------|
| `--exclude` | `--exclude <pattern>` | Add pattern to exclude list (repeatable) |
| `--exclude-from` | `--exclude-from <file>` | Read patterns from file (one per line) |
| `--no-git` | `--no-git` | Exclude `.git/` directories |
| `--include-env` | `--include-env` | Include `.env*` files (override default exclude) |

**`--exclude-from` file format:**
- One pattern per line
- Lines starting with `#` are comments
- Blank lines are ignored
- No inline comments

Example `.syncignore` file:
```
# Project-specific excludes
data/*.parquet
training_data/

# Large generated files
*.model
*.bin
```

### 3.6 Interactive Picker

When `co sync --interactive` is used, the TUI picker allows runtime selection. Selections work as follows:

1. All default excludes start **selected** (will be excluded)
2. User can toggle directories/files to include (deselect) or exclude (select)
3. Selecting a directory excludes its entire subtree
4. Final selection list is passed to the sync operation

The picker does not persist selections by default. Use `--save` to write selections to `project.json`.

---

## 4. Precedence & Combination Rules

### 4.1 Combination Algorithm

```
EffectiveExcludes = dedupe(
    BuiltinDefaults
    - GlobalConfig.remove
    + GlobalConfig.add
    - WorkspaceConfig.remove
    + WorkspaceConfig.add
    + CLI.exclude
    + InteractivePicker.selections
)
```

Where:
- `+` means add to set
- `-` means remove from set
- `dedupe()` removes exact duplicates (case-sensitive)

### 4.2 Special Flag Handling

| Flag | Effect on Pipeline |
|------|-------------------|
| `--no-git` | Adds `.git/` at the CLI stage |
| `--include-env` | Removes `.env`, `.env.*` patterns before CLI stage |

### 4.3 Pattern Matching for Removal

When removing patterns via `remove`:
- Exact string match only (no glob matching)
- `"vendor/"` removes `"vendor/"` but not `"**/vendor/"`
- Case-sensitive comparison

### 4.4 De-duplication

- Exact string comparison
- Trailing slash variants are **not** auto-normalized (see §5.4)
- `node_modules` and `node_modules/` are considered different patterns

---

## 5. Pattern Semantics

### 5.1 Pattern Types

| Pattern | Type | Matches |
|---------|------|---------|
| `node_modules/` | Directory | Any directory named `node_modules` at any depth |
| `*.log` | File glob | Any file ending in `.log` at any depth |
| `.env` | Exact file | File named `.env` at any depth |
| `.env.*` | File glob | Files like `.env.local`, `.env.production` |
| `data/*.csv` | Path glob | CSV files directly under any `data/` directory |
| `logs/` | Directory | Any directory named `logs` |

### 5.2 Rsync Mapping

Patterns are passed directly to rsync's `--exclude=` option:

```bash
rsync -az --partial --progress \
  --exclude='node_modules/' \
  --exclude='*.log' \
  --exclude='.env' \
  source/ dest/
```

Rsync interprets these patterns relative to the transfer root with implicit `**` prefix behavior for patterns without `/`.

### 5.3 Tar Mapping

GNU tar requires pattern translation. The `tarExcludePattern()` function normalizes patterns:

| Input Pattern | Tar `--exclude=` |
|---------------|------------------|
| `node_modules/` | `*/node_modules/*` |
| `*.log` | `*.log` |
| `.env` | `.env` |
| `data/*.csv` | `data/*.csv` |

**Translation rules:**
1. Directory patterns (`foo/`) → `*/foo/*` (match at any depth)
2. File patterns → passed as-is
3. Path patterns with `/` → passed as-is (relative to archive root)

```go
func tarExcludePattern(pattern string) string {
    if strings.HasSuffix(pattern, "/") {
        dirName := strings.TrimSuffix(pattern, "/")
        return "*/" + dirName + "/*"
    }
    return pattern
}
```

### 5.4 Trailing Slash Semantics

| Pattern | Meaning |
|---------|---------|
| `vendor/` | Directory only — excludes directories named `vendor` |
| `vendor` | Ambiguous — matches both files and directories named `vendor` |

**Recommendation:** Always use trailing slash for directories to ensure consistent behavior across transports.

### 5.5 Path Separator Handling

- All patterns use forward slash `/` regardless of OS
- Backslashes are not supported and will cause undefined behavior
- Patterns are normalized to forward slash on input

---

## 6. Edge Cases

### 6.1 Dotfiles

Dotfiles (files/directories starting with `.`) are handled normally:

| Pattern | Effect |
|---------|--------|
| `.env` | Excludes file named `.env` |
| `.git/` | Excludes `.git` directory |
| `.*` | Excludes all dotfiles (not recommended) |

**Note:** `.vscode/` is in the default exclude list. Users who want to sync VS Code settings should add `".vscode/"` to their `sync.excludes.remove`.

### 6.2 Negation Patterns

**Not supported.** Unlike `.gitignore`, we do not support `!pattern` negation.

Workaround: Use the `remove` field to un-exclude specific patterns:

```json
{
  "sync": {
    "excludes": {
      "remove": [".env.example"]
    }
  }
}
```

### 6.3 Symlinks

- Symlinks to files: Excluded if the symlink name matches an exclude pattern
- Symlinks to directories: Same as above
- Symlink targets: Not evaluated (we don't follow symlinks to check target names)
- Dangling symlinks: Excluded from sync (rsync default behavior)

**Warning:** Symlink loops can cause issues. Both rsync and tar have built-in protection, but the interactive picker must avoid following symlinks during tree enumeration.

### 6.4 Nested Repositories

Subdirectories that are git repositories (contain `.git/`) are handled normally. The `.git/` exclude only applies if `--no-git` is specified:

```
workspace/
├── repos/
│   ├── frontend/          # Git repo
│   │   └── .git/          # Excluded with --no-git
│   └── backend/           # Git repo
│       ├── .git/          # Excluded with --no-git
│       └── vendor/        # Excluded by default
└── project.json
```

**Note:** `vendor/` inside a nested repo is still excluded by the `vendor/` pattern (patterns match at any depth).

### 6.5 Case Sensitivity

- All pattern matching is **case-sensitive**
- `Node_modules/` does not match `node_modules/`
- This matches rsync and tar default behavior
- Cross-platform consistency: Use lowercase for patterns

### 6.6 Unicode & Special Characters

- Unicode filenames are supported
- Spaces in patterns must be quoted in CLI: `--exclude='my files/'`
- Shell metacharacters (`*`, `?`, `[`, `]`) are interpreted as globs
- Literal `*` in filename: not directly supported (escape with `\*` for rsync, but tar differs)

### 6.7 Empty Directories

- Empty directories are excluded by rsync by default (unless `--dirs` is used)
- Our implementation does not add `--dirs`
- Empty directories that would be excluded anyway are not a concern

---

## 7. Language-Specific Examples

### 7.1 Go Workspace

Default excludes handle Go projects well:

```
go-project/
├── cmd/
├── internal/
├── vendor/           # ✓ Excluded (dependency cache)
├── bin/              # ✓ Excluded (compiled binaries)
├── .git/             # ✓ Excluded with --no-git
├── go.mod
└── go.sum
```

No additional configuration typically needed.

### 7.2 Rust Workspace

```
rust-project/
├── src/
├── target/           # ✓ Excluded (build artifacts, can be >1GB)
│   ├── debug/
│   └── release/
├── .git/             # ✓ Excluded with --no-git
├── Cargo.toml
└── Cargo.lock
```

No additional configuration typically needed.

### 7.3 Node.js Workspace

```
node-project/
├── src/
├── node_modules/     # ✓ Excluded (dependencies, often >500MB)
├── dist/             # ✓ Excluded (build output)
├── .next/            # ✓ Excluded (Next.js cache)
├── coverage/         # ✓ Excluded (test coverage)
├── .env              # ✓ Excluded (secrets)
├── .env.local        # ✓ Excluded (local overrides)
├── .git/             # ✓ Excluded with --no-git
├── package.json
└── package-lock.json
```

No additional configuration typically needed.

### 7.4 Python Workspace

```
python-project/
├── src/
├── .venv/            # ✓ Excluded (virtual environment)
├── __pycache__/      # ✓ Excluded (bytecode cache)
├── .pytest_cache/    # ✓ Excluded (pytest cache)
├── .mypy_cache/      # ✓ Excluded (mypy cache)
├── htmlcov/          # ✓ Excluded (coverage HTML)
├── *.pyc             # ✓ Excluded (compiled Python)
├── .env              # ✓ Excluded (secrets)
├── .git/             # ✓ Excluded with --no-git
├── pyproject.toml
└── requirements.txt
```

No additional configuration typically needed.

### 7.5 Monorepo Workspace

```
monorepo/
├── packages/
│   ├── frontend/
│   │   ├── node_modules/  # ✓ Excluded
│   │   └── dist/          # ✓ Excluded
│   └── backend/
│       ├── target/        # ✓ Excluded (if Rust)
│       └── node_modules/  # ✓ Excluded (if Node)
├── node_modules/          # ✓ Excluded (hoisted deps)
├── .turbo/                # ✓ Excluded (Turborepo cache)
├── .git/                  # ✓ Excluded with --no-git
└── package.json
```

No additional configuration typically needed.

### 7.6 Data Science / ML Workspace

May need additional excludes for large data files:

```json
{
  "sync": {
    "excludes": {
      "add": [
        "data/raw/",
        "models/*.h5",
        "models/*.pt",
        "*.parquet",
        "*.feather",
        "wandb/",
        "mlruns/"
      ]
    }
  }
}
```

---

## 8. Implementation Notes

### 8.1 Data Structures

```go
// ExcludeConfig represents exclude configuration from config/project files
type ExcludeConfig struct {
    Add    []string `json:"add,omitempty"`
    Remove []string `json:"remove,omitempty"`
}

// SyncConfig holds sync-related configuration
type SyncConfig struct {
    Excludes   *ExcludeConfig `json:"excludes,omitempty"`
    IncludeEnv bool           `json:"include_env,omitempty"`
}

// ExcludeList is the computed effective exclude list
type ExcludeList struct {
    Patterns []string
    Sources  map[string]string // pattern -> source (for debugging)
}
```

### 8.2 Key Functions

```go
// BuildExcludeList computes the effective exclude list from all sources
func BuildExcludeList(
    globalConfig *SyncConfig,
    workspaceConfig *SyncConfig,
    cliExcludes []string,
    cliExcludeFromFile string,
    cliNoGit bool,
    cliIncludeEnv bool,
    pickerSelections []string,
) (*ExcludeList, error)

// NormalizePatterns ensures consistent pattern format
func NormalizePatterns(patterns []string) []string

// ToRsyncArgs converts exclude list to rsync --exclude arguments
func (e *ExcludeList) ToRsyncArgs() []string

// ToTarArgs converts exclude list to tar --exclude arguments
func (e *ExcludeList) ToTarArgs() []string
```

### 8.3 Config File Updates

**`internal/config/config.go`** additions:

```go
type Config struct {
    // ... existing fields ...
    Sync *SyncConfig `json:"sync,omitempty"`
}
```

**`internal/model/project.go`** additions:

```go
type Project struct {
    // ... existing fields ...
    Sync *SyncConfig `json:"sync,omitempty"`
}
```

### 8.4 CLI Changes

New flags for `sync` command:

```go
syncCmd.Flags().StringArrayVar(&syncExcludes, "exclude", nil, 
    "additional pattern to exclude (can be repeated)")
syncCmd.Flags().StringVar(&syncExcludeFrom, "exclude-from", "", 
    "read exclude patterns from file")
syncCmd.Flags().BoolVar(&syncIncludeEnv, "include-env", false,
    "include .env files (overrides default exclude)")
syncCmd.Flags().BoolVarP(&syncInteractive, "interactive", "i", false,
    "launch interactive exclude picker")
syncCmd.Flags().BoolVar(&syncListExcludes, "list-excludes", false,
    "print effective exclude list and exit")
```

---

## 9. Testing Strategy

### 9.1 Unit Tests

- Pattern normalization
- Exclude list combination (add/remove logic)
- Rsync argument generation
- Tar argument generation
- `--exclude-from` file parsing
- Edge cases (empty patterns, duplicates, special characters)

### 9.2 Integration Tests

- Full exclude pipeline with all sources
- Config file loading and merging
- CLI flag parsing
- Pattern matching against test directory structures

### 9.3 Transport Verification Tests

Verify rsync and tar produce identical results for representative patterns:

```go
func TestTransportParity(t *testing.T) {
    patterns := []string{
        "node_modules/",
        "*.log",
        ".env",
        "data/*.csv",
    }
    
    // Create test directory with various files
    // Run rsync with patterns
    // Run tar with patterns
    // Compare resulting file lists
}
```

### 9.4 Example Test Cases

| Test Case | Input | Expected Effective List |
|-----------|-------|------------------------|
| Defaults only | No config | Built-in list |
| Global add | `{add: ["foo/"]}` | Built-in + `foo/` |
| Global remove | `{remove: ["vendor/"]}` | Built-in - `vendor/` |
| Workspace override | Global removes X, workspace adds X | X is included |
| CLI override | `--exclude=bar/` | Built-in + `bar/` |
| Include env | `--include-env` | Built-in - `.env*` patterns |
| No git | `--no-git` | Built-in + `.git/` |
| Interactive | User deselects `node_modules/` | Built-in - `node_modules/` |

---

## 10. Migration & Compatibility

### 10.1 Backward Compatibility

- Existing `co sync` behavior is preserved
- Default excludes are expanded but existing excludes remain
- No breaking changes to CLI interface
- New flags are additive

### 10.2 Config Schema

No schema version bump required — new fields are optional and additive.

---

## 11. Future Considerations

### 11.1 Potential Extensions

- **Include patterns** — `--include` to force-include specific paths
- **Profile-based excludes** — Named exclude profiles (e.g., "minimal", "full")
- **Remote exclude files** — Read `.syncignore` from workspace root
- **Dry-run exclude preview** — Show what would be excluded without syncing
- **Exclude size estimation** — Show size savings from excludes

### 11.2 Not In Scope

- Full rsync filter syntax
- Negation patterns (`!pattern`)
- Per-file exclude decisions (too slow for large trees)
- Exclude patterns with regex (glob only)

---

## Appendix A: Full Built-in Exclude List

See `internal/fs/excludes.go` for the authoritative list. Use `co sync --list-excludes` to view the current defaults.

---

## Appendix B: Example Configurations

### B.1 Global Config for ML Developer

```json
{
  "schema": 1,
  "code_root": "~/Code",
  "sync": {
    "excludes": {
      "add": [
        "*.h5",
        "*.pt",
        "*.onnx",
        "*.parquet",
        "*.feather",
        "wandb/",
        "mlruns/",
        "checkpoints/",
        "lightning_logs/"
      ]
    }
  }
}
```

### B.2 Workspace Config for API Project

```json
{
  "schema": 1,
  "slug": "acme--api",
  "sync": {
    "excludes": {
      "add": [
        "testdata/fixtures/",
        "*.sqlite"
      ],
      "remove": [
        ".vscode/"
      ]
    }
  }
}
```

### B.3 Exclude-from File

```
# .syncignore
# Large data files
data/raw/
data/processed/*.parquet

# Model artifacts
models/checkpoints/
models/*.bin

# Temporary files
tmp/
*.tmp
```

---

*End of specification*
