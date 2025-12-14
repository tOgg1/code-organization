package template

import (
	"path/filepath"
	"strings"
)

// PatternMatcher handles file pattern matching for include/exclude globs.
type PatternMatcher struct {
	includePatterns []string
	excludePatterns []string
}

// NewPatternMatcher creates a new PatternMatcher with the given include and exclude patterns.
// If include patterns are empty, all files are included by default.
func NewPatternMatcher(include, exclude []string) *PatternMatcher {
	return &PatternMatcher{
		includePatterns: include,
		excludePatterns: exclude,
	}
}

// Match returns true if the path should be included based on include/exclude patterns.
// The path should be relative to the template files directory.
func (pm *PatternMatcher) Match(path string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Check exclude patterns first - exclude takes precedence
	for _, pattern := range pm.excludePatterns {
		if MatchGlob(pattern, path) {
			return false
		}
	}

	// If no include patterns, include everything not excluded
	if len(pm.includePatterns) == 0 {
		return true
	}

	// Check include patterns
	for _, pattern := range pm.includePatterns {
		if MatchGlob(pattern, path) {
			return true
		}
	}

	return false
}

// MatchGlob matches a glob pattern against a path.
// Supports:
//   - * matches any sequence of non-separator characters
//   - ** matches any sequence of characters including separators
//   - ? matches any single non-separator character
//
// The pattern and path are expected to use forward slashes.
func MatchGlob(pattern, path string) bool {
	// Normalize to forward slashes
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	return matchGlobRecursive(pattern, path)
}

func matchGlobRecursive(pattern, path string) bool {
	// Empty pattern only matches empty path
	if pattern == "" {
		return path == ""
	}

	// Handle ** (double star) - matches zero or more path segments
	if strings.HasPrefix(pattern, "**/") {
		// Try matching with zero or more directories
		restPattern := pattern[3:]

		// Try matching here (zero directories consumed)
		if matchGlobRecursive(restPattern, path) {
			return true
		}

		// Try consuming each directory level
		for i := 0; i <= len(path); i++ {
			if i == len(path) || path[i] == '/' {
				remaining := ""
				if i < len(path) {
					remaining = path[i+1:]
				}
				if matchGlobRecursive(restPattern, remaining) {
					return true
				}
				// Also try matching with ** consuming more
				if remaining != "" && matchGlobRecursive(pattern, remaining) {
					return true
				}
			}
		}
		return false
	}

	// Handle trailing **
	if pattern == "**" {
		return true // matches everything remaining
	}

	// Handle ** in the middle or end
	if strings.Contains(pattern, "/**/") {
		idx := strings.Index(pattern, "/**/")
		prefix := pattern[:idx]
		suffix := pattern[idx+4:]

		// The prefix must match the start of the path
		if !matchPrefix(prefix, path) {
			return false
		}

		// Find where the prefix ends in the path
		prefixEnd := findPrefixEnd(prefix, path)
		if prefixEnd == -1 {
			return false
		}

		remaining := path[prefixEnd:]
		if len(remaining) > 0 && remaining[0] == '/' {
			remaining = remaining[1:]
		}

		// The ** can match zero or more directories
		return matchGlobRecursive("**/"+suffix, remaining)
	}

	// Handle single * - matches any characters except /
	if strings.HasPrefix(pattern, "*") {
		restPattern := pattern[1:]

		// If this is the last character and there's no more pattern
		if restPattern == "" {
			// * at end matches rest of segment (no slashes)
			return !strings.Contains(path, "/")
		}

		// If next char in pattern is /, * matches up to /
		if strings.HasPrefix(restPattern, "/") {
			// Find next / in path
			slashIdx := strings.Index(path, "/")
			if slashIdx == -1 {
				// No more slashes in path, * matches rest and /... must fail
				return false
			}
			return matchGlobRecursive(restPattern[1:], path[slashIdx+1:])
		}

		// Try matching * with different lengths
		for i := 0; i <= len(path); i++ {
			// * cannot cross directory boundaries
			if i > 0 && path[i-1] == '/' {
				break
			}
			if matchGlobRecursive(restPattern, path[i:]) {
				return true
			}
			if i < len(path) && path[i] == '/' {
				break
			}
		}
		return false
	}

	// Handle ? - matches any single character except /
	if strings.HasPrefix(pattern, "?") {
		if len(path) == 0 || path[0] == '/' {
			return false
		}
		return matchGlobRecursive(pattern[1:], path[1:])
	}

	// Literal character match
	if len(path) == 0 {
		return false
	}
	if pattern[0] != path[0] {
		return false
	}
	return matchGlobRecursive(pattern[1:], path[1:])
}

// matchPrefix checks if the pattern prefix matches the start of the path
func matchPrefix(pattern, path string) bool {
	if pattern == "" {
		return true
	}

	// Handle wildcards in prefix
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
		// For patterns with wildcards, we need to check if it could match
		// We do a simpler check: the pattern should match some prefix of the path
		for i := 0; i <= len(path); i++ {
			if matchGlobRecursive(pattern, path[:i]) {
				return true
			}
		}
		return false
	}

	// Literal prefix match
	return strings.HasPrefix(path, pattern)
}

// findPrefixEnd finds where the prefix pattern ends in the path
func findPrefixEnd(pattern, path string) int {
	if pattern == "" {
		return 0
	}

	// For literal patterns, just return the length
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
		if strings.HasPrefix(path, pattern) {
			return len(pattern)
		}
		return -1
	}

	// For patterns with wildcards, find the end of matching portion
	for i := 0; i <= len(path); i++ {
		if matchGlobRecursive(pattern, path[:i]) {
			return i
		}
	}
	return -1
}

// ShouldProcessFile determines if a file should be processed based on template config.
// Returns true if the file passes include/exclude patterns.
func ShouldProcessFile(files TemplateFiles, relativePath string) bool {
	pm := NewPatternMatcher(files.Include, files.Exclude)
	return pm.Match(relativePath)
}

// IsTemplateFile checks if a file should be processed as a template based on its extension.
func IsTemplateFile(path string, extensions []string) bool {
	if len(extensions) == 0 {
		extensions = []string{".tmpl"}
	}

	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// StripTemplateExtension removes the template extension from a filename.
func StripTemplateExtension(path string, extensions []string) string {
	if len(extensions) == 0 {
		extensions = []string{".tmpl"}
	}

	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return strings.TrimSuffix(path, ext)
		}
	}
	return path
}

// OutputFileName returns the output filename for a template file.
// This is an alias for StripTemplateExtension for use in file processing.
func OutputFileName(path string, extensions []string) string {
	return StripTemplateExtension(path, extensions)
}
