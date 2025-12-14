package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// templateNamePattern validates template names (lowercase alphanumeric with hyphens).
var templateNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ListTemplates returns all available templates in the templates directory.
func ListTemplates(templatesDir string) ([]Template, error) {
	return ListTemplatesMulti([]string{templatesDir})
}

// ListTemplatesMulti returns all available templates from multiple directories.
// Templates in earlier directories take precedence over later ones.
func ListTemplatesMulti(templatesDirs []string) ([]Template, error) {
	seen := make(map[string]bool)
	var templates []Template

	for _, dir := range templatesDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading templates directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Skip _global directory and hidden directories
			if name == GlobalTemplateDir || strings.HasPrefix(name, ".") {
				continue
			}

			// Skip if already seen (earlier directory takes precedence)
			if seen[name] {
				continue
			}

			// Check if it has a valid template.json
			tmpl, err := LoadTemplate(dir, name)
			if err != nil {
				// Skip invalid templates in listing, but could log warning
				continue
			}

			seen[name] = true
			templates = append(templates, *tmpl)
		}
	}

	return templates, nil
}

// ListTemplateInfos returns summary information for all templates.
func ListTemplateInfos(templatesDir string) ([]TemplateInfo, error) {
	return ListTemplateInfosMulti([]string{templatesDir})
}

// ListTemplateInfosMulti returns summary information for templates from multiple directories.
func ListTemplateInfosMulti(templatesDirs []string) ([]TemplateInfo, error) {
	templates, err := ListTemplatesMulti(templatesDirs)
	if err != nil {
		return nil, err
	}

	infos := make([]TemplateInfo, len(templates))
	for i, tmpl := range templates {
		infos[i] = tmpl.ToInfo()
	}

	return infos, nil
}

// LoadTemplate loads a template by name from the templates directory.
func LoadTemplate(templatesDir, name string) (*Template, error) {
	if name == "" {
		return nil, &ValidationError{Field: "name", Reason: "template name is required"}
	}

	templatePath := filepath.Join(templatesDir, name)
	manifestPath := filepath.Join(templatePath, TemplateManifestFile)

	// Check if template directory exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return nil, &TemplateNotFoundError{Name: name}
	}

	// Check if manifest exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, &InvalidManifestError{
			Path: manifestPath,
			Err:  fmt.Errorf("template.json not found"),
		}
	}

	// Read and parse manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, &InvalidManifestError{Path: manifestPath, Err: err}
	}

	var tmpl Template
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, &InvalidManifestError{Path: manifestPath, Err: err}
	}

	// Ensure name matches directory
	if tmpl.Name == "" {
		tmpl.Name = name
	} else if tmpl.Name != name {
		return nil, &InvalidManifestError{
			Path: manifestPath,
			Err:  fmt.Errorf("template name %q does not match directory name %q", tmpl.Name, name),
		}
	}

	// Validate the template
	if err := ValidateTemplate(&tmpl); err != nil {
		return nil, err
	}

	return &tmpl, nil
}

// LoadTemplateMulti loads a template by searching multiple directories in order.
// Returns the template from the first directory where it's found.
func LoadTemplateMulti(templatesDirs []string, name string) (*Template, string, error) {
	if name == "" {
		return nil, "", &ValidationError{Field: "name", Reason: "template name is required"}
	}

	for _, dir := range templatesDirs {
		tmpl, err := LoadTemplate(dir, name)
		if err == nil {
			return tmpl, dir, nil
		}
		// Continue searching if template not found in this directory
		if _, ok := err.(*TemplateNotFoundError); ok {
			continue
		}
		// Return other errors immediately
		return nil, "", err
	}

	return nil, "", &TemplateNotFoundError{Name: name}
}

// FindTemplateDir returns the directory containing a template, searching multiple directories.
func FindTemplateDir(templatesDirs []string, name string) (string, error) {
	for _, dir := range templatesDirs {
		templatePath := filepath.Join(dir, name)
		manifestPath := filepath.Join(templatePath, TemplateManifestFile)
		if _, err := os.Stat(manifestPath); err == nil {
			return dir, nil
		}
	}
	return "", &TemplateNotFoundError{Name: name}
}

// TemplateExists checks if a template exists by name.
func TemplateExists(templatesDir, name string) bool {
	return TemplateExistsMulti([]string{templatesDir}, name)
}

// TemplateExistsMulti checks if a template exists in any of the given directories.
func TemplateExistsMulti(templatesDirs []string, name string) bool {
	for _, dir := range templatesDirs {
		templatePath := filepath.Join(dir, name)
		manifestPath := filepath.Join(templatePath, TemplateManifestFile)
		info, err := os.Stat(manifestPath)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// GetGlobalFilesPath returns the path to the _global template directory.
func GetGlobalFilesPath(templatesDir string) string {
	return filepath.Join(templatesDir, GlobalTemplateDir)
}

// GetGlobalFilesPaths returns all _global directories that exist, in priority order.
func GetGlobalFilesPaths(templatesDirs []string) []string {
	var paths []string
	for _, dir := range templatesDirs {
		globalPath := filepath.Join(dir, GlobalTemplateDir)
		if info, err := os.Stat(globalPath); err == nil && info.IsDir() {
			paths = append(paths, globalPath)
		}
	}
	return paths
}

// HasGlobalFiles checks if global template files exist.
func HasGlobalFiles(templatesDir string) bool {
	return HasGlobalFilesMulti([]string{templatesDir})
}

// HasGlobalFilesMulti checks if global template files exist in any of the directories.
func HasGlobalFilesMulti(templatesDirs []string) bool {
	return len(GetGlobalFilesPaths(templatesDirs)) > 0
}

// GetTemplateFilesPath returns the path to a template's files directory.
func GetTemplateFilesPath(templatesDir, name string) string {
	return filepath.Join(templatesDir, name, TemplateFilesDir)
}

// GetTemplateHooksPath returns the path to a template's hooks directory.
func GetTemplateHooksPath(templatesDir, name string) string {
	return filepath.Join(templatesDir, name, TemplateHooksDir)
}

// ValidateTemplate validates a template manifest.
func ValidateTemplate(tmpl *Template) error {
	errs := &MultiError{}

	// Required fields
	if tmpl.Name == "" {
		errs.Add(&ValidationError{Field: "name", Reason: "is required"})
	} else if !templateNamePattern.MatchString(tmpl.Name) {
		errs.Add(&ValidationError{
			Field:  "name",
			Reason: fmt.Sprintf("must match pattern %s", templateNamePattern.String()),
		})
	}

	if tmpl.Description == "" {
		errs.Add(&ValidationError{Field: "description", Reason: "is required"})
	}

	// Schema version
	if tmpl.Schema == 0 {
		tmpl.Schema = CurrentTemplateSchema
	} else if tmpl.Schema > CurrentTemplateSchema {
		errs.Add(&ValidationError{
			Field:  "schema",
			Reason: fmt.Sprintf("version %d is newer than supported version %d", tmpl.Schema, CurrentTemplateSchema),
		})
	}

	// Validate variables
	varNames := make(map[string]bool)
	for i, v := range tmpl.Variables {
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
		case VarTypeString, VarTypeBoolean, VarTypeInteger:
			// Valid
		case VarTypeChoice:
			if len(v.Choices) == 0 {
				errs.Add(&ValidationError{
					Field:  fmt.Sprintf("variables[%d].choices", i),
					Reason: "choice type requires at least one choice",
				})
			}
		case "":
			// Default to string
			tmpl.Variables[i].Type = VarTypeString
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

	// Validate repos
	repoNames := make(map[string]bool)
	for i, r := range tmpl.Repos {
		if r.Name == "" {
			errs.Add(&ValidationError{
				Field:  fmt.Sprintf("repos[%d].name", i),
				Reason: "is required",
			})
			continue
		}

		if repoNames[r.Name] {
			errs.Add(&ValidationError{
				Field:  fmt.Sprintf("repos[%d].name", i),
				Reason: fmt.Sprintf("duplicate repo name: %s", r.Name),
			})
		}
		repoNames[r.Name] = true

		// Must have either clone_url or init
		if r.CloneURL == "" && !r.Init {
			errs.Add(&ValidationError{
				Field:  fmt.Sprintf("repos[%d]", i),
				Reason: "must have either clone_url or init: true",
			})
		}
	}

	// Validate hook timeouts if specified
	validateHookTimeout := func(name string, spec HookSpec) {
		if spec.Timeout != "" {
			if _, err := parseTimeoutString(spec.Timeout); err != nil {
				errs.Add(&ValidationError{
					Field:  fmt.Sprintf("hooks.%s.timeout", name),
					Reason: fmt.Sprintf("invalid timeout: %v", err),
				})
			}
		}
	}

	validateHookTimeout("pre_create", tmpl.Hooks.PreCreate)
	validateHookTimeout("post_create", tmpl.Hooks.PostCreate)
	validateHookTimeout("post_clone", tmpl.Hooks.PostClone)
	validateHookTimeout("post_complete", tmpl.Hooks.PostComplete)
	validateHookTimeout("post_migrate", tmpl.Hooks.PostMigrate)

	return errs.ErrorOrNil()
}

// ValidateTemplateDir validates a template including its files and hooks.
func ValidateTemplateDir(templatesDir, name string) error {
	tmpl, err := LoadTemplate(templatesDir, name)
	if err != nil {
		return err
	}

	errs := &MultiError{}

	// Check files directory exists if template has file patterns
	filesPath := GetTemplateFilesPath(templatesDir, name)
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		// Files directory is optional, but warn if include patterns are defined
		if len(tmpl.Files.Include) > 0 {
			errs.Add(&ValidationError{
				Field:  "files",
				Reason: fmt.Sprintf("files directory not found: %s", filesPath),
			})
		}
	}

	// Check hook scripts exist
	hooksPath := GetTemplateHooksPath(templatesDir, name)
	validateHookScript := func(hookName string, spec HookSpec) {
		if spec.Script == "" {
			return
		}
		scriptPath := filepath.Join(hooksPath, spec.Script)
		// Also try relative to template root
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			scriptPath = filepath.Join(templatesDir, name, spec.Script)
			if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
				errs.Add(&HookNotFoundError{HookType: hookName, Script: spec.Script})
			}
		}
	}

	validateHookScript("pre_create", tmpl.Hooks.PreCreate)
	validateHookScript("post_create", tmpl.Hooks.PostCreate)
	validateHookScript("post_clone", tmpl.Hooks.PostClone)
	validateHookScript("post_complete", tmpl.Hooks.PostComplete)
	validateHookScript("post_migrate", tmpl.Hooks.PostMigrate)

	return errs.ErrorOrNil()
}

// parseTimeoutString parses a timeout string like "5m" or "30s".
// Returns the number of seconds or an error.
func parseTimeoutString(timeout string) (int, error) {
	if timeout == "" {
		return 300, nil // 5 minutes default
	}

	timeout = strings.TrimSpace(timeout)
	if len(timeout) < 2 {
		return 0, fmt.Errorf("invalid timeout format: %s", timeout)
	}

	unit := timeout[len(timeout)-1]
	valueStr := timeout[:len(timeout)-1]

	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return 0, fmt.Errorf("invalid timeout value: %s", timeout)
	}

	switch unit {
	case 's':
		return value, nil
	case 'm':
		return value * 60, nil
	case 'h':
		return value * 3600, nil
	default:
		return 0, fmt.Errorf("invalid timeout unit: %c (use s, m, or h)", unit)
	}
}

// EnsureTemplatesDir creates the templates directory if it doesn't exist.
func EnsureTemplatesDir(templatesDir string) error {
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("creating templates directory: %w", err)
	}
	return nil
}

// EnsureGlobalDir creates the _global template directory if it doesn't exist.
func EnsureGlobalDir(templatesDir string) error {
	globalPath := GetGlobalFilesPath(templatesDir)
	if err := os.MkdirAll(globalPath, 0755); err != nil {
		return fmt.Errorf("creating global templates directory: %w", err)
	}
	return nil
}
