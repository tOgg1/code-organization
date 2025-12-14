package template

import (
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// Exact matches
		{"exact match", "foo.txt", "foo.txt", true},
		{"exact no match", "foo.txt", "bar.txt", false},
		{"exact path match", "dir/file.txt", "dir/file.txt", true},
		{"exact path no match", "dir/file.txt", "other/file.txt", false},

		// Single star (*)
		{"star matches anything", "*.txt", "foo.txt", true},
		{"star matches empty", "*.txt", ".txt", true},
		{"star doesnt match slash", "*.txt", "dir/foo.txt", false},
		{"star in middle", "foo*.txt", "foobar.txt", true},
		{"star at end", "foo*", "foobar", true},
		{"star at end with extension", "foo*", "foo.txt", true},
		{"multiple stars", "*.min.*", "app.min.js", true},

		// Question mark (?)
		{"question mark single char", "?.txt", "a.txt", true},
		{"question mark no match empty", "?.txt", ".txt", false},
		{"question mark no match slash", "?.txt", "/a.txt", false},
		{"multiple question marks", "???.go", "foo.go", true},
		{"question marks no match", "???.go", "fo.go", false},

		// Double star (**)
		{"double star matches everything", "**", "anything/at/all", true},
		{"double star matches empty", "**", "", true},
		{"double star prefix", "**/file.txt", "file.txt", true},
		{"double star prefix deep", "**/file.txt", "a/b/c/file.txt", true},
		{"double star prefix mid", "**/file.txt", "dir/file.txt", true},
		{"double star suffix", "src/**", "src/main.go", true},
		{"double star suffix deep", "src/**", "src/pkg/main.go", true},
		{"double star in middle", "src/**/test.go", "src/test.go", true},
		{"double star in middle deep", "src/**/test.go", "src/a/b/c/test.go", true},

		// Common patterns
		{"node_modules exclude", "**/node_modules/**", "node_modules/pkg/file.js", true},
		{"node_modules deep", "**/node_modules/**", "frontend/node_modules/react/index.js", true},
		{"gitignore pattern", "**/.git/**", ".git/config", true},
		{"gitignore deep", "**/.git/**", "submodule/.git/config", true},
		{"extension pattern", "**/*.js", "src/app.js", true},
		{"extension deep", "**/*.js", "src/components/Button.js", true},
		{"specific directory", "build/**", "build/output.js", true},
		{"backup files", "*.bak", "file.bak", true},
		{"backup not in subdir", "*.bak", "dir/file.bak", false},

		// Edge cases
		{"empty pattern empty path", "", "", true},
		{"empty pattern non-empty path", "", "file.txt", false},
		{"double star only", "**/*", "any/path/file.txt", true},
		{"trailing slash pattern", "dir/", "dir/", true},
		{"leading dot", ".gitignore", ".gitignore", true},
		{"hidden files", ".*", ".hidden", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchGlob(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestPatternMatcher(t *testing.T) {
	tests := []struct {
		name     string
		include  []string
		exclude  []string
		path     string
		want     bool
	}{
		// No patterns - include everything
		{"no patterns includes all", nil, nil, "any/file.txt", true},
		{"empty patterns includes all", []string{}, []string{}, "any/file.txt", true},

		// Include patterns only
		{"include match", []string{"*.go"}, nil, "main.go", true},
		{"include no match", []string{"*.go"}, nil, "main.js", false},
		{"include multiple", []string{"*.go", "*.md"}, nil, "README.md", true},
		{"include all", []string{"**/*"}, nil, "deep/path/file.txt", true},

		// Exclude patterns only
		{"exclude match", nil, []string{"*.bak"}, "file.bak", false},
		{"exclude no match", nil, []string{"*.bak"}, "file.txt", true},
		{"exclude node_modules", nil, []string{"**/node_modules/**"}, "node_modules/pkg/file.js", false},

		// Both include and exclude
		{"include then exclude", []string{"**/*"}, []string{"*.bak"}, "file.bak", false},
		{"include then exclude keeps good", []string{"**/*"}, []string{"*.bak"}, "file.txt", true},
		{"specific include with exclude", []string{"src/**/*.go"}, []string{"**/*_test.go"}, "src/main.go", true},
		{"specific include excluded", []string{"src/**/*.go"}, []string{"**/*_test.go"}, "src/main_test.go", false},

		// Real-world scenarios
		{
			"typical template include",
			[]string{"**/*"},
			[]string{"*.bak", ".DS_Store", "**/node_modules/**"},
			"src/components/Button.tsx",
			true,
		},
		{
			"typical template exclude DS_Store",
			[]string{"**/*"},
			[]string{"*.bak", ".DS_Store", "**/node_modules/**"},
			".DS_Store",
			false,
		},
		{
			"typical template exclude node_modules",
			[]string{"**/*"},
			[]string{"*.bak", ".DS_Store", "**/node_modules/**"},
			"frontend/node_modules/react/index.js",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPatternMatcher(tt.include, tt.exclude)
			got := pm.Match(tt.path)
			if got != tt.want {
				t.Errorf("PatternMatcher.Match(%q) = %v, want %v (include=%v, exclude=%v)",
					tt.path, got, tt.want, tt.include, tt.exclude)
			}
		})
	}
}

func TestShouldProcessFile(t *testing.T) {
	tests := []struct {
		name    string
		files   TemplateFiles
		path    string
		want    bool
	}{
		{
			"default config includes all",
			TemplateFiles{},
			"any/file.txt",
			true,
		},
		{
			"explicit include",
			TemplateFiles{Include: []string{"**/*"}},
			"any/file.txt",
			true,
		},
		{
			"exclude backup",
			TemplateFiles{Exclude: []string{"*.bak"}},
			"file.bak",
			false,
		},
		{
			"typical config",
			TemplateFiles{
				Include: []string{"**/*"},
				Exclude: []string{"*.bak", ".DS_Store"},
			},
			"src/main.go",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldProcessFile(tt.files, tt.path)
			if got != tt.want {
				t.Errorf("ShouldProcessFile(%v, %q) = %v, want %v", tt.files, tt.path, got, tt.want)
			}
		})
	}
}

func TestIsTemplateFile(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       bool
	}{
		{"default tmpl", "README.md.tmpl", nil, true},
		{"default not tmpl", "README.md", nil, false},
		{"custom extension", "file.template", []string{".template"}, true},
		{"multiple extensions match first", "file.tmpl", []string{".tmpl", ".tpl"}, true},
		{"multiple extensions match second", "file.tpl", []string{".tmpl", ".tpl"}, true},
		{"multiple extensions no match", "file.txt", []string{".tmpl", ".tpl"}, false},
		{"nested path", "src/pkg/main.go.tmpl", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTemplateFile(tt.path, tt.extensions)
			if got != tt.want {
				t.Errorf("IsTemplateFile(%q, %v) = %v, want %v", tt.path, tt.extensions, got, tt.want)
			}
		})
	}
}

func TestStripTemplateExtension(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       string
	}{
		{"default strip tmpl", "README.md.tmpl", nil, "README.md"},
		{"default no extension", "README.md", nil, "README.md"},
		{"custom extension", "file.template", []string{".template"}, "file"},
		{"multiple extensions", "file.tpl", []string{".tmpl", ".tpl"}, "file"},
		{"nested path", "src/pkg/main.go.tmpl", nil, "src/pkg/main.go"},
		{"only extension", ".tmpl", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripTemplateExtension(tt.path, tt.extensions)
			if got != tt.want {
				t.Errorf("StripTemplateExtension(%q, %v) = %q, want %q", tt.path, tt.extensions, got, tt.want)
			}
		})
	}
}
