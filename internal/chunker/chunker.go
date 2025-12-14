package chunker

import (
	"path/filepath"
	"strings"
)

// Chunk represents a code chunk extracted from a source file
type Chunk struct {
	StartLine     int
	EndLine       int
	Content       string
	ChunkType     string // "function", "method", "class", "struct", "interface", "block", etc.
	SymbolName    string // Name of the symbol (function name, class name, etc.)
	Language      string
	TokenEstimate int // Rough estimate of tokens (~4 chars per token)
}

// Chunker is the interface for extracting code chunks from source files
type Chunker interface {
	// Chunk extracts semantic code chunks from the given source code
	Chunk(source []byte, filename string) ([]Chunk, error)

	// SupportedLanguages returns the list of languages this chunker supports
	SupportedLanguages() []string
}

// Config holds configuration for the chunker
type Config struct {
	// MaxChunkLines is the maximum number of lines per chunk (default: 100)
	MaxChunkLines int `json:"max_chunk_lines,omitempty"`

	// MinChunkLines is the minimum number of lines for a chunk (default: 5)
	MinChunkLines int `json:"min_chunk_lines,omitempty"`

	// OverlapLines is the number of context lines to include around chunks (default: 3)
	OverlapLines int `json:"overlap_lines,omitempty"`

	// IncludeImports includes import/include statements in chunks (default: false)
	IncludeImports bool `json:"include_imports,omitempty"`
}

// DefaultConfig returns the default chunker configuration
func DefaultConfig() Config {
	return Config{
		MaxChunkLines:  100,
		MinChunkLines:  5,
		OverlapLines:   3,
		IncludeImports: false,
	}
}

// Language detection based on file extension
var extensionToLanguage = map[string]string{
	".go":    "go",
	".py":    "python",
	".pyw":   "python",
	".js":    "javascript",
	".mjs":   "javascript",
	".cjs":   "javascript",
	".jsx":   "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".mts":   "typescript",
	".cts":   "typescript",
	".rs":    "rust",
	".rb":    "ruby",
	".java":  "java",
	".kt":    "kotlin",
	".kts":   "kotlin",
	".c":     "c",
	".h":     "c",
	".cpp":   "cpp",
	".cc":    "cpp",
	".cxx":   "cpp",
	".hpp":   "cpp",
	".hxx":   "cpp",
	".cs":    "csharp",
	".swift": "swift",
	".php":   "php",
	".scala": "scala",
	".lua":   "lua",
	".sh":    "bash",
	".bash":  "bash",
	".zsh":   "bash",
	".sql":   "sql",
	".zig":   "zig",
	".nim":   "nim",
	".ex":    "elixir",
	".exs":   "elixir",
	".erl":   "erlang",
	".hrl":   "erlang",
	".hs":    "haskell",
	".ml":    "ocaml",
	".mli":   "ocaml",
	".vue":   "vue",
	".svelte": "svelte",
}

// DetectLanguage detects the programming language from a filename
func DetectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if lang, ok := extensionToLanguage[ext]; ok {
		return lang
	}

	// Check for special filenames
	base := strings.ToLower(filepath.Base(filename))
	switch base {
	case "dockerfile":
		return "dockerfile"
	case "makefile", "gnumakefile":
		return "make"
	case "cmakelists.txt":
		return "cmake"
	}

	return ""
}

// IsIndexableFile returns true if the file should be indexed
func IsIndexableFile(filename string) bool {
	// Skip hidden files and directories
	base := filepath.Base(filename)
	if strings.HasPrefix(base, ".") {
		return false
	}

	// Check for compound extensions first (e.g., .min.js, .d.ts)
	lowerBase := strings.ToLower(base)
	if strings.HasSuffix(lowerBase, ".min.js") ||
		strings.HasSuffix(lowerBase, ".min.css") ||
		strings.HasSuffix(lowerBase, ".d.ts") ||
		strings.HasSuffix(lowerBase, ".d.mts") ||
		strings.HasSuffix(lowerBase, ".d.cts") {
		return false
	}

	// Skip common non-code files
	ext := strings.ToLower(filepath.Ext(filename))
	skipExtensions := map[string]bool{
		".md":    true,
		".txt":   true,
		".json":  true,
		".yaml":  true,
		".yml":   true,
		".toml":  true,
		".xml":   true,
		".html":  true,
		".css":   true,
		".scss":  true,
		".less":  true,
		".svg":   true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".ico":   true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".eot":   true,
		".map":   true,
		".lock":  true,
		".sum":   true,
		".mod":   true, // go.mod is metadata
		".log":   true,
		".env":   true,
	}
	if skipExtensions[ext] {
		return false
	}

	// Skip if no recognized language
	return DetectLanguage(filename) != ""
}

// EstimateTokens estimates the number of tokens in a string
// Using a rough estimate of 4 characters per token
func EstimateTokens(s string) int {
	return (len(s) + 3) / 4
}
