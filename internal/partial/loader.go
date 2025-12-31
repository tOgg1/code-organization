package partial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tormodhaugland/co/internal/template"
)

// ListPartials returns all available partials from the given directories.
// Partials in earlier directories take precedence over later ones.
func ListPartials(partialsDirs []string) ([]PartialInfo, error) {
	seen := make(map[string]bool)
	var infos []PartialInfo

	for _, dir := range partialsDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading partials directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				continue
			}

			// Skip if already seen (earlier directory takes precedence)
			if seen[name] {
				continue
			}

			// Check if it has a valid partial.json
			partialPath := filepath.Join(dir, name)
			p, err := LoadPartial(partialPath)
			if err != nil {
				// Skip invalid partials in listing
				continue
			}

			// Count files in the partial
			fileCount := countPartialFiles(partialPath)

			seen[name] = true
			infos = append(infos, p.ToInfo(partialPath, dir, fileCount))
		}
	}

	return infos, nil
}

// ListPartialsFiltered returns partials filtered by tag.
func ListPartialsFiltered(partialsDirs []string, tag string) ([]PartialInfo, error) {
	all, err := ListPartials(partialsDirs)
	if err != nil {
		return nil, err
	}

	if tag == "" {
		return all, nil
	}

	var filtered []PartialInfo
	for _, info := range all {
		for _, t := range info.Tags {
			if t == tag {
				filtered = append(filtered, info)
				break
			}
		}
	}

	return filtered, nil
}

// FindPartial locates a partial by name across multiple directories.
// Returns the path to the partial directory (containing partial.json).
func FindPartial(name string, partialsDirs []string) (string, error) {
	if name == "" {
		return "", &ValidationError{Field: "name", Reason: "partial name is required"}
	}

	for _, dir := range partialsDirs {
		partialPath := filepath.Join(dir, name)
		manifestPath := filepath.Join(partialPath, PartialManifestFile)
		if _, err := os.Stat(manifestPath); err == nil {
			return partialPath, nil
		}
	}

	return "", &PartialNotFoundError{Name: name}
}

// LoadPartial loads a partial from its directory path.
func LoadPartial(partialPath string) (*Partial, error) {
	manifestPath := filepath.Join(partialPath, PartialManifestFile)

	// Check if manifest exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, &InvalidManifestError{
			Path: manifestPath,
			Err:  fmt.Errorf("partial.json not found"),
		}
	}

	// Read and parse manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, &InvalidManifestError{Path: manifestPath, Err: err}
	}

	var p Partial
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, &InvalidManifestError{Path: manifestPath, Err: err}
	}

	// Ensure name matches directory if not set
	dirName := filepath.Base(partialPath)
	if p.Name == "" {
		p.Name = dirName
	} else if p.Name != dirName {
		return nil, &InvalidManifestError{
			Path: manifestPath,
			Err:  fmt.Errorf("partial name %q does not match directory name %q", p.Name, dirName),
		}
	}

	// Validate the partial
	if err := ValidatePartial(&p, partialPath); err != nil {
		return nil, err
	}

	return &p, nil
}

// LoadPartialByName loads a partial by name, searching multiple directories.
func LoadPartialByName(name string, partialsDirs []string) (*Partial, string, error) {
	partialPath, err := FindPartial(name, partialsDirs)
	if err != nil {
		return nil, "", err
	}

	p, err := LoadPartial(partialPath)
	if err != nil {
		return nil, "", err
	}

	return p, partialPath, nil
}

// PartialExists checks if a partial exists by name in any of the given directories.
func PartialExists(partialsDirs []string, name string) bool {
	for _, dir := range partialsDirs {
		partialPath := filepath.Join(dir, name)
		manifestPath := filepath.Join(partialPath, PartialManifestFile)
		info, err := os.Stat(manifestPath)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// ValidatePartial validates a partial manifest.
func ValidatePartial(p *Partial, partialPath string) error {
	errs := &MultiError{}

	// Required fields
	if p.Name == "" {
		errs.Add(&ValidationError{Field: "name", Reason: "is required"})
	} else if !IsValidPartialName(p.Name) {
		errs.Add(&ValidationError{
			Field:  "name",
			Reason: fmt.Sprintf("must match pattern %s (lowercase alphanumeric with hyphens)", partialNameRegex.String()),
		})
	}

	if p.Description == "" {
		errs.Add(&ValidationError{Field: "description", Reason: "is required"})
	}

	// Schema version
	if p.Schema == 0 {
		p.Schema = CurrentPartialSchema
	} else if p.Schema > CurrentPartialSchema {
		errs.Add(&ValidationError{
			Field:  "schema",
			Reason: fmt.Sprintf("version %d is newer than supported version %d", p.Schema, CurrentPartialSchema),
		})
	}

	// Validate conflict strategy
	if p.Conflicts.Strategy != "" && !IsValidConflictStrategy(p.Conflicts.Strategy) {
		errs.Add(&ValidationError{
			Field:  "conflicts.strategy",
			Reason: fmt.Sprintf("invalid strategy: %s (must be prompt, skip, overwrite, backup, or merge)", p.Conflicts.Strategy),
		})
	}

	// Validate variables
	varNames := make(map[string]bool)
	for i, v := range p.Variables {
		if v.Name == "" {
			errs.Add(&ValidationError{
				Field:  fmt.Sprintf("variables[%d].name", i),
				Reason: "is required",
			})
			continue
		}

		if varNames[v.Name] {
			errs.Add(&ValidationError{
				Field:  fmt.Sprintf("variables[%d].name", i),
				Reason: fmt.Sprintf("duplicate variable name: %s", v.Name),
			})
		}
		varNames[v.Name] = true

		// Validate variable type
		switch v.Type {
		case template.VarTypeString, template.VarTypeBoolean, template.VarTypeInteger:
			// Valid
		case template.VarTypeChoice:
			if len(v.Choices) == 0 {
				errs.Add(&ValidationError{
					Field:  fmt.Sprintf("variables[%d].choices", i),
					Reason: "choice type requires at least one choice",
				})
			}
		case "":
			// Default to string
			p.Variables[i].Type = template.VarTypeString
		default:
			errs.Add(&ValidationError{
				Field:  fmt.Sprintf("variables[%d].type", i),
				Reason: fmt.Sprintf("invalid type: %s (must be string, boolean, choice, or integer)", v.Type),
			})
		}

		// Validate regex pattern if provided
		if v.Validation != "" {
			if _, err := regexp.Compile(v.Validation); err != nil {
				errs.Add(&ValidationError{
					Field:  fmt.Sprintf("variables[%d].validation", i),
					Reason: fmt.Sprintf("invalid regex pattern: %v", err),
				})
			}
		}
	}

	// Validate hook scripts exist (if partialPath provided)
	if partialPath != "" {
		validateHookScript := func(hookName string, spec template.HookSpec) {
			if spec.Script == "" {
				return
			}
			// Try hooks directory first
			hooksPath := filepath.Join(partialPath, PartialHooksDir)
			scriptPath := filepath.Join(hooksPath, spec.Script)
			if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
				// Try relative to partial root
				scriptPath = filepath.Join(partialPath, spec.Script)
				if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
					errs.Add(&ValidationError{
						Field:  fmt.Sprintf("hooks.%s", hookName),
						Reason: fmt.Sprintf("hook script not found: %s", spec.Script),
					})
				}
			}
		}

		validateHookScript("pre_apply", p.Hooks.PreApply)
		validateHookScript("post_apply", p.Hooks.PostApply)
	}

	return errs.ErrorOrNil()
}

// ValidatePartialDir validates a partial including its files and hooks.
func ValidatePartialDir(partialPath string) error {
	p, err := LoadPartial(partialPath)
	if err != nil {
		return err
	}

	errs := &MultiError{}

	// Check files directory exists
	filesPath := GetPartialFilesPath(partialPath)
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		// Files directory is optional, but warn if include patterns are defined
		if len(p.Files.Include) > 0 {
			errs.Add(&ValidationError{
				Field:  "files",
				Reason: fmt.Sprintf("files directory not found: %s", filesPath),
			})
		}
	}

	return errs.ErrorOrNil()
}

// GetPartialFilesPath returns the path to a partial's files directory.
func GetPartialFilesPath(partialPath string) string {
	return filepath.Join(partialPath, PartialFilesDir)
}

// GetPartialHooksPath returns the path to a partial's hooks directory.
func GetPartialHooksPath(partialPath string) string {
	return filepath.Join(partialPath, PartialHooksDir)
}

// countPartialFiles counts the number of files in a partial's files directory.
func countPartialFiles(partialPath string) int {
	filesPath := GetPartialFilesPath(partialPath)
	count := 0

	_ = filepath.WalkDir(filesPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})

	return count
}

// EnsurePartialsDir creates the partials directory if it doesn't exist.
func EnsurePartialsDir(partialsDir string) error {
	if err := os.MkdirAll(partialsDir, 0755); err != nil {
		return fmt.Errorf("creating partials directory: %w", err)
	}
	return nil
}
