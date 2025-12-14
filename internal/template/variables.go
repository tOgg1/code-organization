package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GetBuiltinVariables returns the built-in variables available to all templates.
func GetBuiltinVariables(owner, project, workspacePath, codeRoot string) map[string]string {
	now := time.Now()

	vars := map[string]string{
		"OWNER":            owner,
		"PROJECT":          project,
		"SLUG":             owner + "--" + project,
		"CREATED_DATE":     now.Format("2006-01-02"),
		"CREATED_DATETIME": now.Format(time.RFC3339),
		"YEAR":             now.Format("2006"),
		"CODE_ROOT":        codeRoot,
		"WORKSPACE_PATH":   workspacePath,
	}

	// Get home directory
	if home, err := os.UserHomeDir(); err == nil {
		vars["HOME"] = home
	}

	// Get git user info
	if name := getGitConfig("user.name"); name != "" {
		vars["GIT_USER_NAME"] = name
	}
	if email := getGitConfig("user.email"); email != "" {
		vars["GIT_USER_EMAIL"] = email
	}

	return vars
}

// getGitConfig retrieves a git config value.
func getGitConfig(key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// variableRefPattern matches {{VAR}} placeholders.
var variableRefPattern = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)

// BuildDependencyGraph builds a dependency graph from variable defaults.
// Returns a map where key is variable name and value is list of variables it depends on.
func BuildDependencyGraph(vars []TemplateVar) map[string][]string {
	graph := make(map[string][]string)

	for _, v := range vars {
		graph[v.Name] = []string{}

		// Check if default contains variable references
		if v.Default == nil {
			continue
		}

		defaultStr, ok := v.Default.(string)
		if !ok {
			continue
		}

		// Find all variable references in default
		matches := variableRefPattern.FindAllStringSubmatch(defaultStr, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				refName := match[1]
				// Only add dependency if it references another template variable
				for _, otherVar := range vars {
					if otherVar.Name == refName {
						graph[v.Name] = append(graph[v.Name], refName)
						break
					}
				}
			}
		}
	}

	return graph
}

// TopologicalSort returns variables in dependency order (dependencies first).
// Returns an error if a cycle is detected.
func TopologicalSort(graph map[string][]string) ([]string, error) {
	// Kahn's algorithm for topological sorting

	// Calculate in-degrees
	inDegree := make(map[string]int)
	for node := range graph {
		if _, exists := inDegree[node]; !exists {
			inDegree[node] = 0
		}
		for _, dep := range graph[node] {
			inDegree[dep]++ // dep has an edge coming to it
		}
	}

	// Wait, that's backwards. In our graph, if A depends on B, then graph[A] contains B.
	// So B must come before A. Let me recalculate.

	// Reset
	inDegree = make(map[string]int)
	for node := range graph {
		inDegree[node] = 0
	}

	// For each dependency, the dependent node has higher in-degree
	// If A depends on B (graph[A] = [B]), then A has in-degree 1 (from B)
	for node, deps := range graph {
		inDegree[node] += len(deps)
		// Make sure all deps are in the map
		for _, dep := range deps {
			if _, exists := inDegree[dep]; !exists {
				inDegree[dep] = 0
			}
		}
	}

	// Find all nodes with in-degree 0 (no dependencies)
	queue := []string{}
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	result := []string{}
	for len(queue) > 0 {
		// Pop from queue
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// For each node that depends on this node, decrease its in-degree
		for other, deps := range graph {
			for _, dep := range deps {
				if dep == node {
					inDegree[other]--
					if inDegree[other] == 0 {
						queue = append(queue, other)
					}
				}
			}
		}
	}

	// If result doesn't contain all nodes, there's a cycle
	if len(result) != len(inDegree) {
		cycle := detectCycle(graph)
		return nil, &CyclicVariableError{Cycle: cycle}
	}

	return result, nil
}

// detectCycle finds and returns a cycle in the graph using DFS.
func detectCycle(graph map[string][]string) []string {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	parent := make(map[string]string)

	var dfs func(node string) []string
	dfs = func(node string) []string {
		visited[node] = true
		recStack[node] = true

		for _, dep := range graph[node] {
			if !visited[dep] {
				parent[dep] = node
				if cycle := dfs(dep); cycle != nil {
					return cycle
				}
			} else if recStack[dep] {
				// Found cycle, reconstruct it
				cycle := []string{dep}
				curr := node
				for curr != dep {
					cycle = append([]string{curr}, cycle...)
					curr = parent[curr]
				}
				cycle = append([]string{dep}, cycle...)
				return cycle
			}
		}

		recStack[node] = false
		return nil
	}

	for node := range graph {
		if !visited[node] {
			if cycle := dfs(node); cycle != nil {
				return cycle
			}
		}
	}

	return nil
}

// SubstituteVariables replaces {{VAR}} placeholders in content with values from vars.
func SubstituteVariables(content string, vars map[string]string) (string, error) {
	result := variableRefPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name from {{NAME}}
		name := match[2 : len(match)-2]
		if value, ok := vars[name]; ok {
			return value
		}
		// Leave unmatched variables as-is (could also error)
		return match
	})
	return result, nil
}

// ProcessConditionals handles {{#if VAR}}...{{/if}} blocks.
func ProcessConditionals(content string, vars map[string]string) (string, error) {
	// Pattern for simple if blocks: {{#if VAR}}...{{/if}}
	simpleIfPattern := regexp.MustCompile(`(?s)\{\{#if\s+([A-Za-z_][A-Za-z0-9_]*)\s*\}\}(.*?)\{\{/if\}\}`)

	// Pattern for equality check: {{#if VAR == "value"}}...{{/if}}
	eqPattern := regexp.MustCompile(`(?s)\{\{#if\s+([A-Za-z_][A-Za-z0-9_]*)\s*==\s*"([^"]*)"\s*\}\}(.*?)\{\{/if\}\}`)

	// Pattern for inequality check: {{#if VAR != "value"}}...{{/if}}
	neqPattern := regexp.MustCompile(`(?s)\{\{#if\s+([A-Za-z_][A-Za-z0-9_]*)\s*!=\s*"([^"]*)"\s*\}\}(.*?)\{\{/if\}\}`)

	// Process inequality checks first (more specific)
	result := neqPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := neqPattern.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		varName := submatches[1]
		compareValue := submatches[2]
		blockContent := submatches[3]

		value, exists := vars[varName]
		if exists && value != compareValue {
			return blockContent
		}
		return ""
	})

	// Process equality checks
	result = eqPattern.ReplaceAllStringFunc(result, func(match string) string {
		submatches := eqPattern.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		varName := submatches[1]
		compareValue := submatches[2]
		blockContent := submatches[3]

		value, exists := vars[varName]
		if exists && value == compareValue {
			return blockContent
		}
		return ""
	})

	// Process simple if blocks (truthy check)
	result = simpleIfPattern.ReplaceAllStringFunc(result, func(match string) string {
		submatches := simpleIfPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		varName := submatches[1]
		blockContent := submatches[2]

		value, exists := vars[varName]
		if exists && isTruthy(value) {
			return blockContent
		}
		return ""
	})

	return result, nil
}

// isTruthy returns true if a string value is considered truthy.
func isTruthy(value string) bool {
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if lower == "false" || lower == "0" || lower == "no" || lower == "none" {
		return false
	}
	return true
}

// ResolveVariables resolves all variables with provided values, defaults, and built-ins.
func ResolveVariables(tmpl *Template, provided map[string]string, builtins map[string]string) (map[string]string, error) {
	if provided == nil {
		provided = make(map[string]string)
	}

	// Build dependency graph for cycle detection
	graph := BuildDependencyGraph(tmpl.Variables)

	// Get topological order
	order, err := TopologicalSort(graph)
	if err != nil {
		return nil, err
	}

	// Start with builtins
	resolved := make(map[string]string)
	for k, v := range builtins {
		resolved[k] = v
	}

	// Create a map of variable definitions for quick lookup
	varDefs := make(map[string]TemplateVar)
	for _, v := range tmpl.Variables {
		varDefs[v.Name] = v
	}

	// Resolve variables in topological order
	for _, varName := range order {
		varDef, exists := varDefs[varName]
		if !exists {
			continue
		}

		// Check if value was provided
		if value, ok := provided[varName]; ok {
			// Validate provided value
			if err := ValidateVarValue(varDef, value); err != nil {
				return nil, err
			}
			resolved[varName] = value
			continue
		}

		// Try to use default
		if varDef.Default != nil {
			defaultStr := fmt.Sprintf("%v", varDef.Default)

			// Substitute any variable references in the default
			substituted, err := SubstituteVariables(defaultStr, resolved)
			if err != nil {
				return nil, err
			}

			resolved[varName] = substituted
			continue
		}

		// Check if required
		if varDef.Required {
			return nil, &MissingRequiredVarError{
				VarName:     varName,
				Description: varDef.Description,
			}
		}

		// Optional with no default - leave empty
		resolved[varName] = ""
	}

	return resolved, nil
}

// ValidateVarValue validates a value against a variable definition.
func ValidateVarValue(varDef TemplateVar, value string) error {
	switch varDef.Type {
	case VarTypeBoolean:
		lower := strings.ToLower(value)
		if lower != "true" && lower != "false" && lower != "yes" && lower != "no" && lower != "1" && lower != "0" {
			return &InvalidVarValueError{
				VarName: varDef.Name,
				Value:   value,
				Reason:  "must be true/false, yes/no, or 1/0",
			}
		}

	case VarTypeInteger:
		if _, err := strconv.Atoi(value); err != nil {
			return &InvalidVarValueError{
				VarName: varDef.Name,
				Value:   value,
				Reason:  "must be an integer",
			}
		}

	case VarTypeChoice:
		found := false
		for _, choice := range varDef.Choices {
			if value == choice {
				found = true
				break
			}
		}
		if !found {
			return &InvalidVarValueError{
				VarName: varDef.Name,
				Value:   value,
				Reason:  fmt.Sprintf("must be one of: %s", strings.Join(varDef.Choices, ", ")),
			}
		}
	}

	// Check regex validation if provided
	if varDef.Validation != "" {
		re, err := regexp.Compile(varDef.Validation)
		if err != nil {
			return &InvalidVarValueError{
				VarName:    varDef.Name,
				Value:      value,
				Validation: varDef.Validation,
				Reason:     fmt.Sprintf("invalid validation pattern: %v", err),
			}
		}
		if !re.MatchString(value) {
			return &InvalidVarValueError{
				VarName:    varDef.Name,
				Value:      value,
				Validation: varDef.Validation,
			}
		}
	}

	return nil
}

// ProcessTemplateContent processes a template file content with variable substitution and conditionals.
func ProcessTemplateContent(content string, vars map[string]string) (string, error) {
	// First process conditionals
	result, err := ProcessConditionals(content, vars)
	if err != nil {
		return "", err
	}

	// Then substitute remaining variables
	result, err = SubstituteVariables(result, vars)
	if err != nil {
		return "", err
	}

	return result, nil
}

// NormalizeBoolValue converts various boolean representations to "true" or "false".
func NormalizeBoolValue(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "true", "yes", "1", "on":
		return "true"
	case "false", "no", "0", "off", "":
		return "false"
	default:
		return value
	}
}

// GetMissingRequiredVars returns a list of required variables that are not provided.
func GetMissingRequiredVars(tmpl *Template, provided map[string]string, builtins map[string]string) []TemplateVar {
	var missing []TemplateVar

	for _, v := range tmpl.Variables {
		if !v.Required {
			continue
		}

		// Check if provided
		if _, ok := provided[v.Name]; ok {
			continue
		}

		// Check if has default
		if v.Default != nil {
			continue
		}

		// Check if it's a builtin
		if _, ok := builtins[v.Name]; ok {
			continue
		}

		missing = append(missing, v)
	}

	return missing
}

// ExpandPath expands ~ to home directory in a path.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
