package template

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UnresolvedPlaceholder represents a variable placeholder that may be unresolved.
type UnresolvedPlaceholder struct {
	FilePath    string // Absolute path to the file
	FileRel     string // Relative path for display
	Line        int    // 1-indexed line number
	Column      int    // 1-indexed column number
	VarName     string // The variable name (without {{ }})
	Context     string // The line content for context
	IsAvailable bool   // True if the variable is defined (builtin, default, or user-provided)
}

// DiagnosticReport contains the results of scanning a template for issues.
type DiagnosticReport struct {
	TemplateName string
	TemplatesDir string
	Placeholders []UnresolvedPlaceholder
	TotalFiles   int
	TotalScanned int
}

// ScanForPlaceholders scans all template files in a template directory for {{VAR}} placeholders.
// It returns all placeholders found along with whether they would be resolved given the available variables.
func ScanForPlaceholders(templatesDir, templateName string, availableVars map[string]string) (*DiagnosticReport, error) {
	templatePath := filepath.Join(templatesDir, templateName)
	filesPath := GetTemplateFilesPath(templatesDir, templateName)

	report := &DiagnosticReport{
		TemplateName: templateName,
		TemplatesDir: templatesDir,
		Placeholders: []UnresolvedPlaceholder{},
	}

	// Check if files directory exists
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		return report, nil // No files to scan
	}

	// Walk template files
	err := filepath.Walk(filesPath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		report.TotalFiles++

		// Only scan .tmpl files (actual template files with placeholders)
		if !strings.HasSuffix(srcPath, ".tmpl") {
			return nil
		}

		report.TotalScanned++

		relPath, _ := filepath.Rel(templatePath, srcPath)
		placeholders, err := scanFileForPlaceholders(srcPath, relPath, availableVars)
		if err != nil {
			return nil // Skip files that can't be read
		}

		report.Placeholders = append(report.Placeholders, placeholders...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort placeholders by file, then line
	sort.Slice(report.Placeholders, func(i, j int) bool {
		if report.Placeholders[i].FileRel != report.Placeholders[j].FileRel {
			return report.Placeholders[i].FileRel < report.Placeholders[j].FileRel
		}
		return report.Placeholders[i].Line < report.Placeholders[j].Line
	})

	return report, nil
}

// scanFileForPlaceholders scans a single file for variable placeholders.
func scanFileForPlaceholders(filePath, relPath string, availableVars map[string]string) ([]UnresolvedPlaceholder, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var placeholders []UnresolvedPlaceholder
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Find all {{VAR}} matches
		matches := variableRefPattern.FindAllStringSubmatchIndex(line, -1)
		for _, match := range matches {
			if len(match) < 4 {
				continue
			}

			varName := line[match[2]:match[3]]
			column := match[0] + 1 // 1-indexed

			_, isAvailable := availableVars[varName]

			placeholders = append(placeholders, UnresolvedPlaceholder{
				FilePath:    filePath,
				FileRel:     relPath,
				Line:        lineNum,
				Column:      column,
				VarName:     varName,
				Context:     truncateLine(line, 80),
				IsAvailable: isAvailable,
			})
		}
	}

	return placeholders, scanner.Err()
}

// truncateLine truncates a line to maxLen characters, adding ellipsis if needed.
func truncateLine(line string, maxLen int) string {
	line = strings.TrimSpace(line)
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}

// GetUnresolvedPlaceholders returns only the placeholders that would be unresolved.
func (r *DiagnosticReport) GetUnresolvedPlaceholders() []UnresolvedPlaceholder {
	var unresolved []UnresolvedPlaceholder
	for _, p := range r.Placeholders {
		if !p.IsAvailable {
			unresolved = append(unresolved, p)
		}
	}
	return unresolved
}

// HasUnresolvedPlaceholders returns true if there are any unresolved placeholders.
func (r *DiagnosticReport) HasUnresolvedPlaceholders() bool {
	for _, p := range r.Placeholders {
		if !p.IsAvailable {
			return true
		}
	}
	return false
}

// FileDiagnostic contains pattern matching info for a single file.
type FileDiagnostic struct {
	FilePath    string      // Absolute path
	FileRel     string      // Relative path within template/global
	MatchResult MatchResult // Why included/excluded
	Origin      OriginType  // Global or Template
	IsTemplate  bool        // Has .tmpl extension
}

// DiagnoseTemplateFiles returns pattern match information for all files in a template.
// This shows why each file is included or excluded based on patterns.
func DiagnoseTemplateFiles(tmpl *Template, templatesDir string) ([]FileDiagnostic, error) {
	var diagnostics []FileDiagnostic

	templatePath := filepath.Join(templatesDir, tmpl.Name)
	filesPath := GetTemplateFilesPath(templatesDir, tmpl.Name)

	include := tmpl.Files.Include
	exclude := tmpl.Files.Exclude

	// Check if files directory exists
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		return diagnostics, nil
	}

	err := filepath.Walk(filesPath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(templatePath, srcPath)
		filesRelPath, _ := filepath.Rel(filesPath, srcPath)

		result := GetFileMatchDetails(filesRelPath, include, exclude)

		diagnostics = append(diagnostics, FileDiagnostic{
			FilePath:    srcPath,
			FileRel:     relPath,
			MatchResult: result,
			Origin:      OriginTemplate,
			IsTemplate:  strings.HasSuffix(srcPath, ".tmpl"),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by path
	sort.Slice(diagnostics, func(i, j int) bool {
		return diagnostics[i].FileRel < diagnostics[j].FileRel
	})

	return diagnostics, nil
}
