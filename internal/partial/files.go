package partial

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tormodhaugland/co/internal/template"
)

// ScanPartialFiles scans the files/ directory of a partial and returns a list of relative paths.
// It applies include/exclude patterns from the partial configuration.
func ScanPartialFiles(partialPath string, filesConfig PartialFiles) ([]string, error) {
	filesDir := GetPartialFilesPath(partialPath)

	// Check if files directory exists
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	pm := template.NewPatternMatcher(filesConfig.Include, filesConfig.Exclude)
	var files []string

	err := filepath.WalkDir(filesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Get relative path from files directory
		relPath, err := filepath.Rel(filesDir, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes for pattern matching
		relPath = filepath.ToSlash(relPath)

		// Check include/exclude patterns
		if !pm.Match(relPath) {
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// DetectConflicts examines which files already exist in the target and builds a plan.
// It returns a FilePlan with actions for each file based on the conflict strategy.
func DetectConflicts(files []string, partialPath, targetPath string, conflicts ConflictConfig, extensions []string) (*FilePlan, error) {
	if len(extensions) == 0 {
		extensions = []string{".tmpl"}
	}

	plan := &FilePlan{
		Files: make([]FileInfo, 0, len(files)),
	}

	filesDir := GetPartialFilesPath(partialPath)

	for _, relPath := range files {
		srcPath := filepath.Join(filesDir, relPath)

		// Determine output path (strip template extension if present)
		outputPath := relPath
		isTemplate := template.IsTemplateFile(relPath, extensions)
		if isTemplate {
			outputPath = template.StripTemplateExtension(relPath, extensions)
		}

		destPath := filepath.Join(targetPath, outputPath)

		// Validate path doesn't escape target directory
		absDestPath, err := filepath.Abs(destPath)
		if err != nil {
			return nil, err
		}
		absTargetPath, err := filepath.Abs(targetPath)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(absDestPath, absTargetPath) {
			return nil, &PathTraversalError{Path: destPath, TargetPath: targetPath}
		}

		// Check if file exists in target
		existsInTarget := false
		var targetModTime time.Time
		if info, err := os.Stat(destPath); err == nil {
			existsInTarget = true
			targetModTime = info.ModTime()
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		// Check if file is preserved (never overwritten)
		isPreserved := IsPreserved(outputPath, conflicts.Preserve)

		// Determine action
		action := determineAction(outputPath, existsInTarget, isPreserved, conflicts.Strategy)

		plan.Files = append(plan.Files, FileInfo{
			RelPath:        outputPath,
			AbsSourcePath:  srcPath,
			AbsDestPath:    absDestPath,
			TargetModTime:  targetModTime,
			IsTemplate:     isTemplate,
			ExistsInTarget: existsInTarget,
			IsPreserved:    isPreserved,
			Action:         action,
		})
	}

	plan.CountActions()
	return plan, nil
}

// determineAction determines what action to take for a file.
func determineAction(relPath string, existsInTarget, isPreserved bool, strategy string) FileAction {
	// Preserved files are always skipped
	if isPreserved && existsInTarget {
		return ActionSkip
	}

	// New file - create it
	if !existsInTarget {
		return ActionCreate
	}

	return ResolveConflict(&FileInfo{RelPath: relPath, IsPreserved: isPreserved}, strategy)
}

// CopyFile copies a file from src to dest, creating parent directories as needed.
func CopyFile(src, dest string, mode os.FileMode) error {
	// Create parent directories
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", destDir, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("copying content: %w", err)
	}

	return nil
}

// ProcessFile processes a single file - either copying it directly or processing it as a template.
func ProcessFile(srcPath, destPath string, isTemplate bool, vars map[string]string, extensions []string) error {
	// Get source file info for permissions
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return &FileProcessingError{SrcPath: srcPath, DestPath: destPath, Err: fmt.Errorf("stat source: %w", err)}
	}

	if isTemplate && len(extensions) > 0 && template.IsTemplateFile(destPath, extensions) {
		destPath = template.StripTemplateExtension(destPath, extensions)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &FileProcessingError{SrcPath: srcPath, DestPath: destPath, Err: fmt.Errorf("creating directory: %w", err)}
	}

	if isTemplate {
		// Read, process, and write template file
		content, err := os.ReadFile(srcPath)
		if err != nil {
			return &FileProcessingError{SrcPath: srcPath, DestPath: destPath, Err: fmt.Errorf("reading template: %w", err)}
		}

		processed, err := template.ProcessTemplateContent(string(content), vars)
		if err != nil {
			return &FileProcessingError{SrcPath: srcPath, DestPath: destPath, Err: fmt.Errorf("processing template: %w", err)}
		}

		if err := os.WriteFile(destPath, []byte(processed), srcInfo.Mode()); err != nil {
			return &FileProcessingError{SrcPath: srcPath, DestPath: destPath, Err: fmt.Errorf("writing processed file: %w", err)}
		}
	} else {
		// Copy file as-is
		if err := CopyFile(srcPath, destPath, srcInfo.Mode()); err != nil {
			return &FileProcessingError{SrcPath: srcPath, DestPath: destPath, Err: err}
		}
	}

	return nil
}


// ValidateTargetPath checks if the target path is valid and writable.
func ValidateTargetPath(targetPath string) error {
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return &TargetNotFoundError{Path: targetPath}
	}
	if err != nil {
		return fmt.Errorf("checking target: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("target is not a directory: %s", targetPath)
	}

	// Check if writable by attempting to create a temp file
	testFile := filepath.Join(targetPath, ".co-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("target is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// ListPartialFilesWithInfo returns detailed information about files in a partial.
func ListPartialFilesWithInfo(partialPath string, filesConfig PartialFiles, extensions []string) ([]FileInfo, error) {
	if len(extensions) == 0 {
		extensions = []string{".tmpl"}
	}

	files, err := ScanPartialFiles(partialPath, filesConfig)
	if err != nil {
		return nil, err
	}

	filesDir := GetPartialFilesPath(partialPath)
	infos := make([]FileInfo, 0, len(files))

	for _, relPath := range files {
		srcPath := filepath.Join(filesDir, relPath)

		// Determine output path
		outputPath := relPath
		isTemplate := template.IsTemplateFile(relPath, extensions)
		if isTemplate {
			outputPath = template.StripTemplateExtension(relPath, extensions)
		}

		infos = append(infos, FileInfo{
			RelPath:       outputPath,
			AbsSourcePath: srcPath,
			IsTemplate:    isTemplate,
		})
	}

	return infos, nil
}
