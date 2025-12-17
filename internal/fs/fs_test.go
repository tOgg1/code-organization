package fs

import "testing"

func TestIsValidWorkspaceSlug(t *testing.T) {
	tests := []struct {
		name  string
		slug  string
		valid bool
	}{
		{"basic valid slug", "owner--project", true},
		{"with hyphens", "my-owner--my-project", true},
		{"with numbers", "owner123--project456", true},
		{"with suffix poc", "owner--project--poc", true},
		{"with suffix demo", "owner--project--demo", true},
		{"with suffix legacy", "owner--project--legacy", true},
		{"with suffix migration", "owner--project--migration", true},
		{"with suffix infra", "owner--project--infra", true},
		{"complex with suffix", "my-owner--my-project--poc", true},

		{"empty string", "", false},
		{"no separator", "ownerproject", false},
		{"single dash", "owner-project", false},
		{"underscore", "owner__project", false},
		{"uppercase letters", "Owner--Project", false},
		{"spaces", "owner --project", false},
		{"special chars", "owner--proj@ct", false},
		{"only separator", "--", false},
		{"missing owner", "--project", false},
		{"missing project", "owner--", false},
		{"with arbitrary third part", "owner--project--foo", true},
		{"starts with dash accepted", "-owner--project", true},
		{"ends with dash accepted", "owner--project-", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidWorkspaceSlug(tt.slug)
			if got != tt.valid {
				t.Errorf("IsValidWorkspaceSlug(%q) = %v, want %v", tt.slug, got, tt.valid)
			}
		})
	}
}

func TestShouldExcludeDir(t *testing.T) {
	tests := []struct {
		name    string
		exclude bool
	}{
		{"node_modules", true},
		{"vendor", true},
		{"target", true},
		{".next", true},
		{"dist", true},
		{"build", true},
		{"out", true},
		{"bin", true},
		{"obj", true},
		{"coverage", true},
		{"__pycache__", true},
		{".venv", true},
		{".cache", true},
		{".pytest_cache", true},
		{".DS_Store", true},

		{"src", false},
		{"internal", false},
		{"cmd", false},
		{"lib", false},
		{"node_modules_backup", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExcludeDir(tt.name)
			if got != tt.exclude {
				t.Errorf("shouldExcludeDir(%q) = %v, want %v", tt.name, got, tt.exclude)
			}
		})
	}
}

func TestDefaultExcludes(t *testing.T) {
	excludes := DefaultExcludes()
	if len(excludes) == 0 {
		t.Error("DefaultExcludes() returned empty slice")
	}

	excludes[0] = "modified"
	original := DefaultExcludes()
	if original[0] == "modified" {
		t.Error("DefaultExcludes() does not return a copy")
	}
}
