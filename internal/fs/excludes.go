package fs

import (
	"bufio"
	"os"
	"strings"
)

// BuiltinExcludes contains the default patterns excluded from sync operations.
// These target common build artifacts, dependency caches, and sensitive files
// across major development ecosystems.
var BuiltinExcludes = []string{
	// === Package managers & dependencies ===
	"node_modules/",     // Node.js
	"vendor/",           // Go, PHP, Ruby
	".pnpm-store/",      // pnpm
	"bower_components/", // Bower (legacy)

	// === Build outputs ===
	"target/",      // Rust, Scala, Java (Maven)
	"dist/",        // Generic build output
	"build/",       // Generic build output
	"out/",         // Generic build output
	"bin/",         // Go, generic binaries
	"obj/",         // .NET, C++
	"_build/",      // Elixir, Erlang
	".output/",     // Nuxt.js
	".nuxt/",       // Nuxt.js
	".next/",       // Next.js
	".svelte-kit/", // SvelteKit
	".vercel/",     // Vercel
	".netlify/",    // Netlify

	// === Test & coverage ===
	"coverage/",    // Generic coverage reports
	".nyc_output/", // NYC (Istanbul)
	"htmlcov/",     // Python coverage.py
	".tox/",        // Python tox
	".nox/",        // Python nox

	// === Caches ===
	".cache/",         // Generic cache
	"__pycache__/",    // Python bytecode
	".pytest_cache/",  // pytest
	".mypy_cache/",    // mypy
	".ruff_cache/",    // ruff
	"*.pyc",           // Python compiled
	".turbo/",         // Turborepo
	".parcel-cache/",  // Parcel
	".webpack/",       // Webpack
	".eslintcache",    // ESLint
	".stylelintcache", // Stylelint

	// === Virtual environments ===
	".venv/",       // Python venv
	"venv/",        // Python venv (alternate)
	".virtualenv/", // virtualenv

	// === IDE & editors ===
	".idea/",     // JetBrains IDEs
	"*.swp",      // Vim swap
	"*.swo",      // Vim swap
	"*~",         // Emacs backup
	".project",   // Eclipse
	".classpath", // Eclipse
	".settings/", // Eclipse

	// === OS artifacts ===
	".DS_Store",   // macOS
	"Thumbs.db",   // Windows
	"Desktop.ini", // Windows

	// === Logs ===
	"*.log",           // Log files
	"logs/",           // Log directories
	"npm-debug.log*",  // npm
	"yarn-debug.log*", // Yarn
	"yarn-error.log*", // Yarn
	"pnpm-debug.log*", // pnpm

	// === Secrets & sensitive ===
	".env",     // Environment files
	".env.*",   // Environment variants
	"secrets/", // Secrets directory
	"*.pem",    // Certificates
	"*.key",    // Private keys
	".secret*", // Secret files

	// === Terraform & IaC ===
	".terraform/", // Terraform providers
	"*.tfstate",   // Terraform state
	"*.tfstate.*", // Terraform state backups
}

// EnvExcludePatterns contains patterns that exclude .env files.
// These can be removed via --include-env flag.
var EnvExcludePatterns = []string{
	".env",
	".env.*",
}

// ExcludeList holds the computed effective exclude list with source tracking.
type ExcludeList struct {
	Patterns []string
}

// ExcludeOptions configures how the exclude list is built.
type ExcludeOptions struct {
	// Additional patterns to add
	Additional []string
	// Patterns to remove from defaults
	Remove []string
	// Include .git directories (default: excluded when NoGit is false)
	NoGit bool
	// Include .env files (default: excluded)
	IncludeEnv bool
}

// BuildExcludeList computes the effective exclude list from all sources.
func BuildExcludeList(opts ExcludeOptions) *ExcludeList {
	// Start with built-in defaults
	patterns := make([]string, 0, len(BuiltinExcludes)+len(opts.Additional)+1)

	// Build a set for removal lookup
	removeSet := make(map[string]bool, len(opts.Remove))
	for _, p := range opts.Remove {
		removeSet[p] = true
	}

	// If including env files, add those patterns to remove set
	if opts.IncludeEnv {
		for _, p := range EnvExcludePatterns {
			removeSet[p] = true
		}
	}

	// Add built-in patterns (excluding removed ones)
	for _, p := range BuiltinExcludes {
		if !removeSet[p] {
			patterns = append(patterns, p)
		}
	}

	// Add .git if requested
	if opts.NoGit {
		patterns = append(patterns, ".git/")
	}

	// Add additional patterns
	patterns = append(patterns, opts.Additional...)

	// Deduplicate
	patterns = dedupePatterns(patterns)

	return &ExcludeList{Patterns: patterns}
}

// ParseExcludeFile reads exclude patterns from a file.
// Lines starting with # are comments, blank lines are ignored.
func ParseExcludeFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

// dedupePatterns removes duplicate patterns while preserving order.
func dedupePatterns(patterns []string) []string {
	seen := make(map[string]bool, len(patterns))
	result := make([]string, 0, len(patterns))

	for _, p := range patterns {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}

	return result
}

// ToRsyncArgs converts the exclude list to rsync --exclude arguments.
func (e *ExcludeList) ToRsyncArgs() []string {
	args := make([]string, 0, len(e.Patterns))
	for _, p := range e.Patterns {
		args = append(args, "--exclude="+p)
	}
	return args
}

// ToTarArgs converts the exclude list to tar --exclude arguments.
func (e *ExcludeList) ToTarArgs() []string {
	args := make([]string, 0, len(e.Patterns))
	for _, p := range e.Patterns {
		args = append(args, "--exclude="+tarExcludePattern(p))
	}
	return args
}

// tarExcludePattern converts a pattern to tar-compatible format.
// Directory patterns (ending in /) are converted to match at any depth.
func tarExcludePattern(pattern string) string {
	if dir, found := strings.CutSuffix(pattern, "/"); found {
		return "*/" + dir + "/*"
	}
	return pattern
}
