package template

import (
	"reflect"
	"testing"
)

func TestDependencyGraph(t *testing.T) {
	tests := []struct {
		name     string
		vars     []TemplateVar
		expected map[string][]string
	}{
		{
			name: "No dependencies",
			vars: []TemplateVar{
				{Name: "VAR1", Default: "value1"},
				{Name: "VAR2", Default: "value2"},
			},
			expected: map[string][]string{
				"VAR1": {}, 
				"VAR2": {}, 
			},
		},
		{
			name: "Simple dependency",
			vars: []TemplateVar{
				{Name: "VAR1", Default: "value1"},
				{Name: "VAR2", Default: "{{VAR1}}-suffix"},
			},
			expected: map[string][]string{
				"VAR1": {}, 
				"VAR2": {"VAR1"},
			},
		},
		{
			name: "Multi-level dependency",
			vars: []TemplateVar{
				{Name: "A", Default: "a"},
				{Name: "B", Default: "{{A}}b"},
				{Name: "C", Default: "{{B}}c"},
			},
			expected: map[string][]string{
				"A": {}, 
				"B": {"A"}, 
				"C": {"B"}, 
			},
		},
		{
			name: "Multiple dependencies",
			vars: []TemplateVar{
				{Name: "A", Default: "a"},
				{Name: "B", Default: "b"},
				{Name: "C", Default: "{{A}}-{{B}}"},
			},
			expected: map[string][]string{
				"A": {}, 
				"B": {}, 
				"C": {"A", "B"}, 
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildDependencyGraph(tt.vars)
			// Sort slices in expected and result for comparison stability if needed
			// But for these simple cases, order usually matches insertion.
			// However, map iteration is random, but here we are checking the values (slice of strings).
			// The slice order depends on regex match order, which is consistent.

			if len(result) != len(tt.expected) {
				t.Errorf("Expected graph size %d, got %d", len(tt.expected), len(result))
			}

			for k, v := range tt.expected {
				got, ok := result[k]
				if !ok {
					t.Errorf("Expected key %s not found", k)
					continue
				}
				if !reflect.DeepEqual(got, v) {
					// Handle empty slice vs nil slice if necessary, but reflect.DeepEqual handles them differently
					if len(got) == 0 && len(v) == 0 {
						continue
					}
					t.Errorf("For key %s, expected %v, got %v", k, v, got)
				}
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name      string
		graph     map[string][]string
		wantOrder []string // Order can vary, but valid topological sort respects deps
		wantErr   bool
	}{
		{
			name: "Simple chain",
			graph: map[string][]string{
				"A": {}, 
				"B": {"A"}, 
				"C": {"B"}, 
			},
			wantOrder: []string{"A", "B", "C"},
			wantErr:   false,
		},
		{
			name: "Independent nodes",
			graph: map[string][]string{
				"A": {}, 
				"B": {}, 
			},
			wantOrder: []string{"A", "B"}, // or B, A. We'll check validity.
			wantErr:   false,
		},
		{
			name: "Diamond",
			graph: map[string][]string{
				"A": {}, 
				"B": {"A"}, 
				"C": {"A"}, 
				"D": {"B", "C"}, 
			},
			wantErr: false,
		},
		{
			name: "Cycle A->B->A",
			graph: map[string][]string{
				"A": {"B"}, 
				"B": {"A"}, 
			},
			wantErr: true,
		},
		{
			name: "Self cycle",
			graph: map[string][]string{
				"A": {"A"}, 
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TopologicalSort(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Errorf("TopologicalSort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify the order
				if len(got) != len(tt.graph) {
					t.Errorf("TopologicalSort() returned %d items, want %d", len(got), len(tt.graph))
				}
				// Check that for every node, its deps appear earlier
				seen := make(map[string]bool)
				for _, node := range got {
					deps := tt.graph[node]
					for _, dep := range deps {
						if !seen[dep] {
							t.Errorf("TopologicalSort() invalid order: node %s (deps: %v) appears before dep %s. Full order: %v", node, deps, dep, got)
						}
					}
					seen[node] = true
				}
			}
		})
	}
}

func TestSubstituteVariables(t *testing.T) {
	tests := []struct {
		name    string
		content string
		vars    map[string]string
		want    string
	}{
		{
			name:    "Simple substitution",
			content: "Hello {{NAME}}",
			vars:    map[string]string{"NAME": "World"},
			want:    "Hello World",
		},
		{
			name:    "Multiple vars",
			content: "{{GREETING}} {{NAME}}",
			vars:    map[string]string{"GREETING": "Hi", "NAME": "Alice"},
			want:    "Hi Alice",
		},
		{
			name:    "Missing var",
			content: "Hello {{MISSING}}",
			vars:    map[string]string{"NAME": "World"},
			want:    "Hello {{MISSING}}",
		},
		{
			name:    "No vars",
			content: "Hello World",
			vars:    map[string]string{"NAME": "Bob"},
			want:    "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SubstituteVariables(tt.content, tt.vars)
			if err != nil {
				t.Errorf("SubstituteVariables() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("SubstituteVariables() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcessConditionals(t *testing.T) {
	tests := []struct {
		name    string
		content string
		vars    map[string]string
		want    string
	}{
		{
			name:    "Simple if true",
			content: "{{#if ENABLED}}Yes{{/if}}",
			vars:    map[string]string{"ENABLED": "true"},
			want:    "Yes",
		},
		{
			name:    "Simple if false",
			content: "{{#if ENABLED}}Yes{{/if}}",
			vars:    map[string]string{"ENABLED": "false"},
			want:    "",
		},
		{
			name:    "Equality check match",
			content: "{{#if TYPE == \"web\"}}Is Web{{/if}}",
			vars:    map[string]string{"TYPE": "web"},
			want:    "Is Web",
		},
		{
			name:    "Equality check no match",
			content: "{{#if TYPE == \"web\"}}Is Web{{/if}}",
			vars:    map[string]string{"TYPE": "api"},
			want:    "",
		},
		{
			name:    "Inequality check match",
			content: "{{#if TYPE != \"web\"}}Not Web{{/if}}",
			vars:    map[string]string{"TYPE": "api"},
			want:    "Not Web",
		},
		{
			name:    "Inequality check no match",
			content: "{{#if TYPE != \"web\"}}Not Web{{/if}}",
			vars:    map[string]string{"TYPE": "web"},
			want:    "",
		},
		{
			name:    "Nested text",
			content: "Start {{#if SHOW}}Middle{{/if}} End",
			vars:    map[string]string{"SHOW": "true"},
			want:    "Start Middle End",
		},
		{
			name:    "Multiline content",
			content: "{{#if SHOW}}\nLine 1\nLine 2\n{{/if}}",
			vars:    map[string]string{"SHOW": "true"},
			want:    "\nLine 1\nLine 2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProcessConditionals(tt.content, tt.vars)
			if err != nil {
				t.Errorf("ProcessConditionals() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ProcessConditionals() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateVarValue(t *testing.T) {
	tests := []struct {
		name    string
		varDef  TemplateVar
		value   string
		wantErr bool
	}{
		{
			name:    "String valid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeString},
			value:   "anything",
			wantErr: false,
		},
		{
			name:    "Integer valid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeInteger},
			value:   "123",
			wantErr: false,
		},
		{
			name:    "Integer invalid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeInteger},
			value:   "abc",
			wantErr: true,
		},
		{
			name:    "Boolean valid true",
			varDef:  TemplateVar{Name: "V", Type: VarTypeBoolean},
			value:   "true",
			wantErr: false,
		},
		{
			name:    "Boolean valid yes",
			varDef:  TemplateVar{Name: "V", Type: VarTypeBoolean},
			value:   "yes",
			wantErr: false,
		},
		{
			name:    "Boolean valid 1",
			varDef:  TemplateVar{Name: "V", Type: VarTypeBoolean},
			value:   "1",
			wantErr: false,
		},
		{
			name:    "Boolean invalid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeBoolean},
			value:   "maybe",
			wantErr: true,
		},
		{
			name:    "Choice valid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeChoice, Choices: []string{"A", "B"}},
			value:   "A",
			wantErr: false,
		},
		{
			name:    "Choice invalid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeChoice, Choices: []string{"A", "B"}},
			value:   "C",
			wantErr: true,
		},
		{
			name:    "Regex valid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeString, Validation: "^[a-z]+$"},
			value:   "abc",
			wantErr: false,
		},
		{
			name:    "Regex invalid",
			varDef:  TemplateVar{Name: "V", Type: VarTypeString, Validation: "^[a-z]+$"},
			value:   "123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVarValue(tt.varDef, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVarValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveVariables(t *testing.T) {
	tests := []struct {
		name     string
		mpl     *Template
		provided map[string]string
		builtins map[string]string
		want     map[string]string
		wantErr  bool
	}{
		{
			name: "Basic resolution",
			mpl: &Template{
				Variables: []TemplateVar{
					{Name: "VAR1", Type: VarTypeString, Default: "default1"},
					{Name: "VAR2", Type: VarTypeString}, // Optional, no default
				},
			},
			provided: map[string]string{}, 
			builtins: map[string]string{}, 
			want: map[string]string{
				"VAR1": "default1",
				"VAR2": "",
			},
			wantErr: false,
		},
		{
			name: "Provided overrides default",
			mpl: &Template{
				Variables: []TemplateVar{
					{Name: "VAR1", Type: VarTypeString, Default: "default1"},
				},
			},
			provided: map[string]string{"VAR1": "provided"},
			builtins: map[string]string{}, 
			want: map[string]string{
				"VAR1": "provided",
			},
			wantErr: false,
		},
		{
			name: "Dependent default",
			mpl: &Template{
				Variables: []TemplateVar{
					{Name: "BASE", Type: VarTypeString, Default: "base"},
					{Name: "DERIVED", Type: VarTypeString, Default: "{{BASE}}-derived"},
				},
			},
			provided: map[string]string{}, 
			builtins: map[string]string{}, 
			want: map[string]string{
				"BASE":    "base",
				"DERIVED": "base-derived",
			},
			wantErr: false,
		},
		{
			name: "Missing required",
			mpl: &Template{
				Variables: []TemplateVar{
					{Name: "REQ", Type: VarTypeString, Required: true},
				},
			},
			provided: map[string]string{}, 
			builtins: map[string]string{}, 
			want:     nil,
			wantErr:  true,
		},
		{
			name: "Validation error",
			mpl: &Template{
				Variables: []TemplateVar{
					{Name: "INT", Type: VarTypeInteger},
				},
			},
			provided: map[string]string{"INT": "abc"},
			builtins: map[string]string{}, 
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveVariables(tt.mpl, tt.provided, tt.builtins)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ResolveVariables() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGetBuiltinVariables(t *testing.T) {
	owner := "acme"
	project := "webapp"
	path := "/tmp/acme--webapp"
	root := "/tmp"

	vars := GetBuiltinVariables(owner, project, path, root)

	expectedKeys := []string{
		"OWNER", "PROJECT", "SLUG", "CREATED_DATE", "CREATED_DATETIME",
		"YEAR", "CODE_ROOT", "WORKSPACE_PATH",
	}

	for _, key := range expectedKeys {
		if _, ok := vars[key]; !ok {
			t.Errorf("GetBuiltinVariables() missing key %s", key)
		}
	}

	if vars["OWNER"] != owner {
		t.Errorf("Expected OWNER=%s, got %s", owner, vars["OWNER"])
	}
	if vars["SLUG"] != owner+"--"+project {
		t.Errorf("Expected SLUG=%s--%s, got %s", owner, project, vars["SLUG"])
	}
}