package template

import (
	"path/filepath"
	"sort"
)

// DiffType represents the type of difference.
type DiffType string

const (
	DiffAdded   DiffType = "added"
	DiffRemoved DiffType = "removed"
	DiffChanged DiffType = "changed"
)

// VarDiff represents a difference in a variable definition.
type VarDiff struct {
	Name     string   `json:"name"`
	DiffType DiffType `json:"diff_type"`
	ValueA   string   `json:"value_a,omitempty"` // For changed: value in template A
	ValueB   string   `json:"value_b,omitempty"` // For changed: value in template B
}

// RepoDiff represents a difference in a repository definition.
type RepoDiff struct {
	Name     string   `json:"name"`
	DiffType DiffType `json:"diff_type"`
	CloneA   string   `json:"clone_a,omitempty"` // Clone URL in template A
	CloneB   string   `json:"clone_b,omitempty"` // Clone URL in template B
}

// HookDiff represents a difference in hook definitions.
type HookDiff struct {
	Name     string   `json:"name"` // e.g., "pre_create", "post_complete"
	DiffType DiffType `json:"diff_type"`
	ScriptA  string   `json:"script_a,omitempty"`
	ScriptB  string   `json:"script_b,omitempty"`
}

// FileDiff represents a difference in file output.
type FileDiff struct {
	OutputPath string   `json:"output_path"`
	DiffType   DiffType `json:"diff_type"`
	SourceA    string   `json:"source_a,omitempty"` // Source path in template A
	SourceB    string   `json:"source_b,omitempty"` // Source path in template B
}

// CompareResult contains the comparison between two templates.
type CompareResult struct {
	TemplateA string     `json:"template_a"`
	TemplateB string     `json:"template_b"`
	Vars      []VarDiff  `json:"vars"`
	Repos     []RepoDiff `json:"repos"`
	Hooks     []HookDiff `json:"hooks"`
	Files     []FileDiff `json:"files"`
}

// CompareTemplates compares two templates and returns the differences.
func CompareTemplates(tmplA, tmplB *Template, dirA, dirB string) (*CompareResult, error) {
	result := &CompareResult{
		TemplateA: tmplA.Name,
		TemplateB: tmplB.Name,
	}

	// Compare variables
	result.Vars = compareVariables(tmplA.Variables, tmplB.Variables)

	// Compare repos
	result.Repos = compareRepos(tmplA.Repos, tmplB.Repos)

	// Compare hooks
	result.Hooks = compareHooks(&tmplA.Hooks, &tmplB.Hooks)

	// Compare files - dirA and dirB are templates directories, need to add template names
	templatePathA := filepath.Join(dirA, tmplA.Name)
	templatePathB := filepath.Join(dirB, tmplB.Name)
	filesA, _ := ListTemplateFiles(tmplA, templatePathA)
	filesB, _ := ListTemplateFiles(tmplB, templatePathB)
	result.Files = compareFiles(filesA, filesB, templatePathA, templatePathB)

	return result, nil
}

// compareVariables compares variable definitions between two templates.
func compareVariables(varsA, varsB []TemplateVar) []VarDiff {
	var diffs []VarDiff

	// Build maps for lookup
	mapA := make(map[string]TemplateVar)
	mapB := make(map[string]TemplateVar)

	for _, v := range varsA {
		mapA[v.Name] = v
	}
	for _, v := range varsB {
		mapB[v.Name] = v
	}

	// Find added (in B but not A) and changed
	for name, vB := range mapB {
		if vA, ok := mapA[name]; ok {
			// Check if different
			if varsDiffer(vA, vB) {
				diffs = append(diffs, VarDiff{
					Name:     name,
					DiffType: DiffChanged,
					ValueA:   formatVarSummary(vA),
					ValueB:   formatVarSummary(vB),
				})
			}
		} else {
			diffs = append(diffs, VarDiff{
				Name:     name,
				DiffType: DiffAdded,
				ValueB:   formatVarSummary(vB),
			})
		}
	}

	// Find removed (in A but not B)
	for name, vA := range mapA {
		if _, ok := mapB[name]; !ok {
			diffs = append(diffs, VarDiff{
				Name:     name,
				DiffType: DiffRemoved,
				ValueA:   formatVarSummary(vA),
			})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})

	return diffs
}

// varsDiffer checks if two variables are different.
func varsDiffer(a, b TemplateVar) bool {
	if a.Type != b.Type {
		return true
	}
	if a.Required != b.Required {
		return true
	}
	if a.Description != b.Description {
		return true
	}
	// Compare defaults (simplified)
	if formatDefault(a.Default) != formatDefault(b.Default) {
		return true
	}
	return false
}

// formatVarSummary creates a short summary of a variable.
func formatVarSummary(v TemplateVar) string {
	summary := string(v.Type)
	if v.Required {
		summary += ", required"
	}
	if v.Default != nil {
		summary += ", default=" + formatDefault(v.Default)
	}
	return summary
}

// formatDefault formats a default value for display.
func formatDefault(d interface{}) string {
	if d == nil {
		return ""
	}
	switch v := d.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return "<complex>"
	}
}

// compareRepos compares repository definitions.
func compareRepos(reposA, reposB []TemplateRepo) []RepoDiff {
	var diffs []RepoDiff

	mapA := make(map[string]TemplateRepo)
	mapB := make(map[string]TemplateRepo)

	for _, r := range reposA {
		mapA[r.Name] = r
	}
	for _, r := range reposB {
		mapB[r.Name] = r
	}

	// Find added and changed
	for name, rB := range mapB {
		if rA, ok := mapA[name]; ok {
			if rA.CloneURL != rB.CloneURL || rA.Init != rB.Init {
				diffs = append(diffs, RepoDiff{
					Name:     name,
					DiffType: DiffChanged,
					CloneA:   formatRepoSource(rA),
					CloneB:   formatRepoSource(rB),
				})
			}
		} else {
			diffs = append(diffs, RepoDiff{
				Name:     name,
				DiffType: DiffAdded,
				CloneB:   formatRepoSource(rB),
			})
		}
	}

	// Find removed
	for name, rA := range mapA {
		if _, ok := mapB[name]; !ok {
			diffs = append(diffs, RepoDiff{
				Name:     name,
				DiffType: DiffRemoved,
				CloneA:   formatRepoSource(rA),
			})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})

	return diffs
}

// formatRepoSource formats repository source info.
func formatRepoSource(r TemplateRepo) string {
	if r.CloneURL != "" {
		return "clone: " + r.CloneURL
	}
	if r.Init {
		branch := r.DefaultBranch
		if branch == "" {
			branch = "main"
		}
		return "init (branch: " + branch + ")"
	}
	return ""
}

// compareHooks compares hook definitions.
func compareHooks(hooksA, hooksB *TemplateHooks) []HookDiff {
	var diffs []HookDiff

	hooks := []struct {
		name  string
		specA HookSpec
		specB HookSpec
	}{
		{"pre_create", hooksA.PreCreate, hooksB.PreCreate},
		{"post_create", hooksA.PostCreate, hooksB.PostCreate},
		{"post_clone", hooksA.PostClone, hooksB.PostClone},
		{"post_complete", hooksA.PostComplete, hooksB.PostComplete},
		{"post_migrate", hooksA.PostMigrate, hooksB.PostMigrate},
	}

	for _, h := range hooks {
		hasA := h.specA.Script != ""
		hasB := h.specB.Script != ""

		if hasA && hasB {
			if h.specA.Script != h.specB.Script || h.specA.Timeout != h.specB.Timeout {
				diffs = append(diffs, HookDiff{
					Name:     h.name,
					DiffType: DiffChanged,
					ScriptA:  formatHookSpec(h.specA),
					ScriptB:  formatHookSpec(h.specB),
				})
			}
		} else if hasB && !hasA {
			diffs = append(diffs, HookDiff{
				Name:     h.name,
				DiffType: DiffAdded,
				ScriptB:  formatHookSpec(h.specB),
			})
		} else if hasA && !hasB {
			diffs = append(diffs, HookDiff{
				Name:     h.name,
				DiffType: DiffRemoved,
				ScriptA:  formatHookSpec(h.specA),
			})
		}
	}

	return diffs
}

// formatHookSpec formats a hook spec for display.
func formatHookSpec(h HookSpec) string {
	if h.Timeout != "" {
		return h.Script + " (timeout: " + h.Timeout + ")"
	}
	return h.Script
}

// compareFiles compares file lists between templates.
func compareFiles(filesA, filesB []string, dirA, dirB string) []FileDiff {
	var diffs []FileDiff

	setA := make(map[string]bool)
	setB := make(map[string]bool)

	for _, f := range filesA {
		setA[f] = true
	}
	for _, f := range filesB {
		setB[f] = true
	}

	// Find added (in B but not A)
	for _, f := range filesB {
		if !setA[f] {
			diffs = append(diffs, FileDiff{
				OutputPath: f,
				DiffType:   DiffAdded,
				SourceB:    dirB,
			})
		}
	}

	// Find removed (in A but not B)
	for _, f := range filesA {
		if !setB[f] {
			diffs = append(diffs, FileDiff{
				OutputPath: f,
				DiffType:   DiffRemoved,
				SourceA:    dirA,
			})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].OutputPath < diffs[j].OutputPath
	})

	return diffs
}

// Summary returns a summary of the comparison.
func (r *CompareResult) Summary() string {
	return ""
}

// HasDifferences returns true if there are any differences.
func (r *CompareResult) HasDifferences() bool {
	return len(r.Vars) > 0 || len(r.Repos) > 0 || len(r.Hooks) > 0 || len(r.Files) > 0
}

// TotalDiffs returns the total number of differences.
func (r *CompareResult) TotalDiffs() int {
	return len(r.Vars) + len(r.Repos) + len(r.Hooks) + len(r.Files)
}
