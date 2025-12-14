package template

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ShouldIncludeFile checks if a file should be included based on include/exclude patterns.
// Uses the PatternMatcher from pattern.go for glob matching.
func ShouldIncludeFile(path string, include, exclude []string) bool {
	pm := NewPatternMatcher(include, exclude)
	return pm.Match(path)
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
