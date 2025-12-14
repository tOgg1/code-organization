package chunker

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.tsx", "typescript"},
		{"lib.rs", "rust"},
		{"Server.java", "java"},
		{"helper.rb", "ruby"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"Program.cs", "csharp"},
		{"script.sh", "bash"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "make"},
		{"unknown.xyz", ""},
		{"README.md", ""},
		{"data.json", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := DetectLanguage(tt.filename)
			if got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsIndexableFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"main.go", true},
		{"script.py", true},
		{"app.ts", true},
		{"README.md", false},
		{"data.json", false},
		{"style.css", false},
		{".hidden.go", false},
		{"package-lock.json", false},
		{"file.min.js", false},
		{"types.d.ts", false},
		{"image.png", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := IsIndexableFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsIndexableFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 1},
		{"hello", 2},       // 5 chars / 4 = 1.25 -> 2
		{"hello world", 3}, // 11 chars / 4 = 2.75 -> 3
		{"func main() { fmt.Println(\"Hello\") }", 9}, // 36 chars: (36+3)/4 = 9
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxChunkLines != 100 {
		t.Errorf("MaxChunkLines = %d, want 100", cfg.MaxChunkLines)
	}
	if cfg.MinChunkLines != 5 {
		t.Errorf("MinChunkLines = %d, want 5", cfg.MinChunkLines)
	}
	if cfg.OverlapLines != 3 {
		t.Errorf("OverlapLines = %d, want 3", cfg.OverlapLines)
	}
}
