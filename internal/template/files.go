package template

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// OriginType indicates where a file comes from.
type OriginType string

const (
	OriginGlobal   OriginType = "global"
	OriginTemplate OriginType = "template"
)

// OutputMapping represents a file that would be created in the workspace.
type OutputMapping struct {
	OutputPath   string     // Workspace-relative output path (after stripping .tmpl)
	SourcePath   string     // Absolute path to source file
	OriginType   OriginType // Whether from global or template
	OriginDir    string     // The templates directory or template path this came from
	IsOverride   bool       // True if this template file overrides a global file
	IsTemplate   bool       // True if source is a template file (.tmpl)
	SourceRel    string     // Relative path within origin (for display)
	OverriddenBy string     // If overridden, the path of the overriding file
}

// BuildOutputMapping builds a map of output paths to their source files.
// This shows the effective set of files that would be created, with origin info.
// Returns mappings sorted by output path.
func BuildOutputMapping(tmpl *Template, templatesDirs []string, templatePath string) ([]OutputMapping, error) {
	// Map output path -> mapping (allows tracking overrides)
	outputMap := make(map[string]*OutputMapping)
	extensions := []string{".tmpl"}

	// Determine skip list from template
	var skipList []string
	switch v := tmpl.SkipGlobalFiles.(type) {
	case bool:
		if v {
			skipList = nil // Will skip all - handled below
		}
	case []string:
		skipList = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				skipList = append(skipList, s)
			}
		}
	}

	// Skip all global files if SkipGlobalFiles is true
	skipAllGlobal := false
	if b, ok := tmpl.SkipGlobalFiles.(bool); ok && b {
		skipAllGlobal = true
	}

	// Process global files from all directories (first wins among globals)
	if !skipAllGlobal {
		for _, templatesDir := range templatesDirs {
			globalPath := GetGlobalFilesPath(templatesDir)

			if _, err := os.Stat(globalPath); os.IsNotExist(err) {
				continue
			}

			err := filepath.Walk(globalPath, func(srcPath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				relPath, err := filepath.Rel(globalPath, srcPath)
				if err != nil {
					return err
				}

				// Check if this file should be skipped
				for _, skip := range skipList {
					if relPath == skip || filepath.Base(relPath) == skip {
						return nil
					}
				}

				// Determine output path
				outputPath := relPath
				isTemplate := IsTemplateFile(relPath, extensions)
				if isTemplate {
					outputPath = StripTemplateExtension(relPath, extensions)
				}

				// Skip if already seen from earlier global directory
				if _, exists := outputMap[outputPath]; exists {
					return nil
				}

				outputMap[outputPath] = &OutputMapping{
					OutputPath: outputPath,
					SourcePath: srcPath,
					OriginType: OriginGlobal,
					OriginDir:  templatesDir,
					IsTemplate: isTemplate,
					SourceRel:  filepath.Join("_global", relPath),
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walking global dir %s: %w", globalPath, err)
			}
		}
	}

	// Process template files (may override global files)
	filesPath := filepath.Join(templatePath, TemplateFilesDir)
	if _, err := os.Stat(filesPath); err == nil {
		tmplExtensions := tmpl.GetTemplateExtensions()
		include := tmpl.Files.Include
		exclude := tmpl.Files.Exclude

		err := filepath.Walk(filesPath, func(srcPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(filesPath, srcPath)
			if err != nil {
				return err
			}

			// Check include/exclude patterns
			if !ShouldIncludeFile(relPath, include, exclude) {
				return nil
			}

			// Determine output path
			outputPath := relPath
			isTemplate := IsTemplateFile(relPath, tmplExtensions)
			if isTemplate {
				outputPath = StripTemplateExtension(relPath, tmplExtensions)
			}

			// Check if this overrides a global file
			isOverride := false
			var overriddenSource string
			if existing, exists := outputMap[outputPath]; exists && existing.OriginType == OriginGlobal {
				isOverride = true
				overriddenSource = existing.SourcePath
				existing.OverriddenBy = srcPath
			}

			outputMap[outputPath] = &OutputMapping{
				OutputPath: outputPath,
				SourcePath: srcPath,
				OriginType: OriginTemplate,
				OriginDir:  templatePath,
				IsOverride: isOverride,
				IsTemplate: isTemplate,
				SourceRel:  filepath.Join(TemplateFilesDir, relPath),
			}

			// Keep track of what was overridden for reference
			if isOverride && overriddenSource != "" {
				// Store the overridden global mapping separately if needed
				_ = overriddenSource
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking template files %s: %w", filesPath, err)
		}
	}

	// Convert map to sorted slice
	result := make([]OutputMapping, 0, len(outputMap))
	for _, mapping := range outputMap {
		result = append(result, *mapping)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].OutputPath < result[j].OutputPath
	})

	return result, nil
}

// GetOverriddenGlobalFiles returns global files that would be overridden by template files.
func GetOverriddenGlobalFiles(tmpl *Template, templatesDirs []string, templatePath string) ([]OutputMapping, error) {
	mappings, err := BuildOutputMapping(tmpl, templatesDirs, templatePath)
	if err != nil {
		return nil, err
	}

	var overridden []OutputMapping
	for _, m := range mappings {
		if m.IsOverride {
			overridden = append(overridden, m)
		}
	}
	return overridden, nil
}

// ShouldIncludeFile checks if a file should be included based on include/exclude patterns.
// Uses the PatternMatcher from pattern.go for glob matching.
func ShouldIncludeFile(path string, include, exclude []string) bool {
	pm := NewPatternMatcher(include, exclude)
	return pm.Match(path)
}

// GetFileMatchDetails returns detailed information about why a file is included or excluded.
// This is useful for debugging include/exclude patterns.
func GetFileMatchDetails(path string, include, exclude []string) MatchResult {
	pm := NewPatternMatcher(include, exclude)
	return pm.MatchWithDetails(path)
}

// ProcessGlobalFiles copies and processes files from the _global directory.
func ProcessGlobalFiles(templatesDir, destPath string, vars map[string]string, skipFiles interface{}) (int, error) {
	globalPath := GetGlobalFilesPath(templatesDir)

	// Check if global directory exists
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		return 0, nil // No global files
	}

	// Determine which files to skip
	var skipList []string
	switch v := skipFiles.(type) {
	case bool:
		if v {
			return 0, nil // Skip all global files
		}
	case []string:
		skipList = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				skipList = append(skipList, s)
			}
		}
	}

	count := 0
	extensions := []string{".tmpl"}

	err := filepath.Walk(globalPath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path from global dir
		relPath, err := filepath.Rel(globalPath, srcPath)
		if err != nil {
			return err
		}

		// Check if this file should be skipped
		for _, skip := range skipList {
			if relPath == skip || filepath.Base(relPath) == skip {
				return nil
			}
		}

		// Determine output path
		outputPath := relPath
		isTemplate := IsTemplateFile(relPath, extensions)
		if isTemplate {
			outputPath = StripTemplateExtension(relPath, extensions)
		}

		destFilePath := filepath.Join(destPath, outputPath)

		// Process the file
		if err := processFile(srcPath, destFilePath, isTemplate, vars, extensions); err != nil {
			return &FileProcessingError{SrcPath: srcPath, DestPath: destFilePath, Err: err}
		}

		count++
		return nil
	})

	return count, err
}

// ProcessTemplateFiles copies and processes files from a template's files directory.
func ProcessTemplateFiles(tmpl *Template, templatePath, destPath string, vars map[string]string) (int, error) {
	filesPath := filepath.Join(templatePath, TemplateFilesDir)

	// Check if files directory exists
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		return 0, nil // No template files
	}

	extensions := tmpl.GetTemplateExtensions()
	include := tmpl.Files.Include
	exclude := tmpl.Files.Exclude

	count := 0

	err := filepath.Walk(filesPath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path from files dir
		relPath, err := filepath.Rel(filesPath, srcPath)
		if err != nil {
			return err
		}

		// Check include/exclude patterns
		if !ShouldIncludeFile(relPath, include, exclude) {
			return nil
		}

		// Determine output path
		outputPath := relPath
		isTemplate := IsTemplateFile(relPath, extensions)
		if isTemplate {
			outputPath = StripTemplateExtension(relPath, extensions)
		}

		destFilePath := filepath.Join(destPath, outputPath)

		// Validate path doesn't escape workspace
		absDestPath, err := filepath.Abs(destFilePath)
		if err != nil {
			return err
		}
		absWorkspace, err := filepath.Abs(destPath)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(absDestPath, absWorkspace) {
			return &PathTraversalError{Path: destFilePath, WorkspacePath: destPath}
		}

		// Process the file
		if err := processFile(srcPath, destFilePath, isTemplate, vars, extensions); err != nil {
			return &FileProcessingError{SrcPath: srcPath, DestPath: destFilePath, Err: err}
		}

		count++
		return nil
	})

	return count, err
}

// processFile copies or processes a single file.
func processFile(srcPath, destPath string, isTemplate bool, vars map[string]string, extensions []string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", destDir, err)
	}

	// Get source file info for permissions
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if isTemplate {
		// Read, process, and write template file
		content, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("reading template: %w", err)
		}

		processed, err := ProcessTemplateContent(string(content), vars)
		if err != nil {
			return fmt.Errorf("processing template: %w", err)
		}

		if err := os.WriteFile(destPath, []byte(processed), srcInfo.Mode()); err != nil {
			return fmt.Errorf("writing processed file: %w", err)
		}
	} else {
		// Copy file as-is
		if err := copyFile(srcPath, destPath, srcInfo.Mode()); err != nil {
			return fmt.Errorf("copying file: %w", err)
		}
	}

	return nil
}

// copyFile copies a file from src to dest preserving permissions.
func copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

// ProcessAllFiles processes both global and template files.
func ProcessAllFiles(tmpl *Template, templatesDir, templatePath, destPath string, vars map[string]string) (globalCount, templateCount int, err error) {
	// Process global files first
	globalCount, err = ProcessGlobalFiles(templatesDir, destPath, vars, tmpl.SkipGlobalFiles)
	if err != nil {
		return globalCount, 0, fmt.Errorf("processing global files: %w", err)
	}

	// Process template files (may override global files)
	templateCount, err = ProcessTemplateFiles(tmpl, templatePath, destPath, vars)
	if err != nil {
		return globalCount, templateCount, fmt.Errorf("processing template files: %w", err)
	}

	return globalCount, templateCount, nil
}

// ListTemplateFiles returns a list of files that would be created by a template.
func ListTemplateFiles(tmpl *Template, templatePath string) ([]string, error) {
	filesPath := filepath.Join(templatePath, TemplateFilesDir)

	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	extensions := tmpl.GetTemplateExtensions()
	include := tmpl.Files.Include
	exclude := tmpl.Files.Exclude

	var files []string

	err := filepath.Walk(filesPath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(filesPath, srcPath)
		if err != nil {
			return err
		}

		if !ShouldIncludeFile(relPath, include, exclude) {
			return nil
		}

		// Get output name
		outputPath := relPath
		if IsTemplateFile(relPath, extensions) {
			outputPath = StripTemplateExtension(relPath, extensions)
		}

		files = append(files, outputPath)
		return nil
	})

	return files, err
}

// ListGlobalFiles returns a list of files in the _global directory.
func ListGlobalFiles(templatesDir string) ([]string, error) {
	return ListGlobalFilesMulti([]string{templatesDir})
}

// ListGlobalFilesMulti returns a deduplicated list of files from all _global directories.
// Files from earlier directories take precedence (are listed first, duplicates removed).
func ListGlobalFilesMulti(templatesDirs []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	extensions := []string{".tmpl"}

	for _, templatesDir := range templatesDirs {
		globalPath := GetGlobalFilesPath(templatesDir)

		if _, err := os.Stat(globalPath); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(globalPath, func(srcPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(globalPath, srcPath)
			if err != nil {
				return err
			}

			outputPath := relPath
			if IsTemplateFile(relPath, extensions) {
				outputPath = StripTemplateExtension(relPath, extensions)
			}

			// Skip if already seen (earlier directory takes precedence)
			if seen[outputPath] {
				return nil
			}

			seen[outputPath] = true
			files = append(files, outputPath)
			return nil
		})
		if err != nil {
			return files, err
		}
	}

	return files, nil
}

// ProcessGlobalFilesMulti processes global files from multiple directories.
// Files from earlier directories take precedence (won't be overwritten by later ones).
func ProcessGlobalFilesMulti(templatesDirs []string, destPath string, vars map[string]string, skipFiles interface{}) (int, error) {
	// Determine which files to skip
	var skipList []string
	switch v := skipFiles.(type) {
	case bool:
		if v {
			return 0, nil // Skip all global files
		}
	case []string:
		skipList = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				skipList = append(skipList, s)
			}
		}
	}

	processed := make(map[string]bool)
	count := 0
	extensions := []string{".tmpl"}

	for _, templatesDir := range templatesDirs {
		globalPath := GetGlobalFilesPath(templatesDir)

		if _, err := os.Stat(globalPath); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(globalPath, func(srcPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(globalPath, srcPath)
			if err != nil {
				return err
			}

			// Check if this file should be skipped
			for _, skip := range skipList {
				if relPath == skip || filepath.Base(relPath) == skip {
					return nil
				}
			}

			// Determine output path
			outputPath := relPath
			isTemplate := IsTemplateFile(relPath, extensions)
			if isTemplate {
				outputPath = StripTemplateExtension(relPath, extensions)
			}

			// Skip if already processed from an earlier directory
			if processed[outputPath] {
				return nil
			}

			destFilePath := filepath.Join(destPath, outputPath)

			// Process the file
			if err := processFile(srcPath, destFilePath, isTemplate, vars, extensions); err != nil {
				return &FileProcessingError{SrcPath: srcPath, DestPath: destFilePath, Err: err}
			}

			processed[outputPath] = true
			count++
			return nil
		})
		if err != nil {
			return count, err
		}
	}

	return count, nil
}

// ProcessAllFilesMulti processes files from multiple template directories.
// Global files are merged from all directories (first wins), template files from the specific template path.
func ProcessAllFilesMulti(tmpl *Template, templatesDirs []string, templatePath, destPath string, vars map[string]string) (globalCount, templateCount int, err error) {
	// Process global files from all directories (first wins)
	globalCount, err = ProcessGlobalFilesMulti(templatesDirs, destPath, vars, tmpl.SkipGlobalFiles)
	if err != nil {
		return globalCount, 0, fmt.Errorf("processing global files: %w", err)
	}

	// Process template files (may override global files)
	templateCount, err = ProcessTemplateFiles(tmpl, templatePath, destPath, vars)
	if err != nil {
		return globalCount, templateCount, fmt.Errorf("processing template files: %w", err)
	}

	return globalCount, templateCount, nil
}
