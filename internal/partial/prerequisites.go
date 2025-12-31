package partial

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PrerequisiteResult reports which prerequisites are missing.
type PrerequisiteResult struct {
	Satisfied       bool
	MissingCommands []string
	MissingFiles    []string
}

// CheckPrerequisites validates required commands and files for a partial.
func CheckPrerequisites(p *Partial, targetPath string) (*PrerequisiteResult, error) {
	result := &PrerequisiteResult{
		Satisfied:       true,
		MissingCommands: []string{},
		MissingFiles:    []string{},
	}

	for _, cmd := range p.Requires.Commands {
		if strings.TrimSpace(cmd) == "" {
			continue
		}
		if !commandExists(cmd) {
			result.MissingCommands = append(result.MissingCommands, cmd)
		}
	}

	for _, relPath := range p.Requires.Files {
		if strings.TrimSpace(relPath) == "" {
			continue
		}
		if !fileExists(targetPath, relPath) {
			result.MissingFiles = append(result.MissingFiles, relPath)
		}
	}

	if len(result.MissingCommands) > 0 || len(result.MissingFiles) > 0 {
		result.Satisfied = false
	}

	return result, nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fileExists(targetPath, relPath string) bool {
	relPath = filepath.Clean(relPath)

	if hasGlob(relPath) {
		matches, err := filepath.Glob(filepath.Join(targetPath, relPath))
		return err == nil && len(matches) > 0
	}

	info, err := os.Stat(filepath.Join(targetPath, relPath))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func hasGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}
