package partial

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tormodhaugland/co/internal/template"
)

// ResolveConflict maps a conflict strategy string to a FileAction.
func ResolveConflict(info *FileInfo, strategy string) FileAction {
	if info != nil && info.IsPreserved {
		return ActionSkip
	}

	switch ConflictStrategy(strategy) {
	case StrategySkip:
		return ActionSkip
	case StrategyOverwrite:
		return ActionOverwrite
	case StrategyBackup:
		return ActionBackup
	case StrategyMerge:
		if info != nil {
			path := info.RelPath
			if info.AbsDestPath != "" {
				path = info.AbsDestPath
			} else if path == "" {
				path = info.AbsSourcePath
			}
			if path != "" && CanMerge(path) {
				return ActionMerge
			}
		}
		return ActionPrompt
	case StrategyPrompt:
		return ActionPrompt
	default:
		return ActionPrompt
	}
}

// ExecuteBackup creates a backup of an existing file before overwriting.
// Returns the path to the backup file.
func ExecuteBackup(destPath string) (string, error) {
	backupPath := destPath + ".bak"

	counter := 1
	for {
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			break
		}
		backupPath = fmt.Sprintf("%s.bak.%d", destPath, counter)
		counter++
		if counter > 100 {
			return "", fmt.Errorf("too many backup files for %s", destPath)
		}
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return "", fmt.Errorf("stat original file: %w", err)
	}

	if err := CopyFile(destPath, backupPath, info.Mode()); err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}

	return backupPath, nil
}

// IsPreserved checks if a path matches any preserve pattern.
func IsPreserved(relPath string, preservePatterns []string) bool {
	if len(preservePatterns) == 0 {
		return false
	}

	for _, pattern := range preservePatterns {
		if template.MatchGlob(pattern, relPath) {
			return true
		}
	}
	return false
}

// GetDefaultExtensions returns the default template file extensions.
func GetDefaultExtensions() []string {
	return []string{".tmpl"}
}

// IsGitignoreFile checks if a file is a .gitignore or similar ignore file.
func IsGitignoreFile(path string) bool {
	base := filepath.Base(path)
	return base == ".gitignore" || base == ".dockerignore" || base == ".npmignore"
}

// IsJSONFile checks if a file has a .json extension.
func IsJSONFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".json")
}

// IsYAMLFile checks if a file has a .yaml or .yml extension.
func IsYAMLFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// CanMerge returns true if the file format supports merging.
func CanMerge(path string) bool {
	return IsGitignoreFile(path) || IsJSONFile(path) || IsYAMLFile(path)
}
