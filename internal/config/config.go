package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ServerConfig struct {
	SSH      string `json:"ssh"`
	CodeRoot string `json:"code_root,omitempty"`
}

// EmbeddingsConfig holds configuration for the embedding backend
type EmbeddingsConfig struct {
	// Backend is the embedding backend to use: "ollama" (default) or "openai"
	Backend string `json:"backend,omitempty"`

	// OllamaURL is the URL of the Ollama server (default: http://localhost:11434)
	OllamaURL string `json:"ollama_url,omitempty"`

	// OllamaModel is the Ollama model to use (default: nomic-embed-text)
	OllamaModel string `json:"ollama_model,omitempty"`

	// OpenAIModel is the OpenAI model to use (default: text-embedding-3-small)
	OpenAIModel string `json:"openai_model,omitempty"`

	// OpenAIAPIKeyEnv is the environment variable containing the OpenAI API key
	OpenAIAPIKeyEnv string `json:"openai_api_key_env,omitempty"`
}

// TmpConfig holds configuration for temporary workspaces
type TmpConfig struct {
	// CleanupDays is the number of days of inactivity before a tmp workspace
	// is eligible for cleanup (default: 30)
	CleanupDays int `json:"cleanup_days,omitempty"`
}

// IndexingConfig holds configuration for code indexing
type IndexingConfig struct {
	// ChunkMaxLines is the maximum number of lines per chunk (default: 100)
	ChunkMaxLines int `json:"chunk_max_lines,omitempty"`

	// ChunkMinLines is the minimum number of lines for a chunk (default: 5)
	ChunkMinLines int `json:"chunk_min_lines,omitempty"`

	// ChunkOverlapLines is the number of context lines around chunks (default: 3)
	ChunkOverlapLines int `json:"chunk_overlap_lines,omitempty"`

	// ExcludePatterns are glob patterns for files to exclude from indexing
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// IncludeLanguages limits indexing to specific languages (if empty, all supported)
	IncludeLanguages []string `json:"include_languages,omitempty"`

	// MaxFileSizeBytes is the maximum file size to index (default: 1MB)
	MaxFileSizeBytes int64 `json:"max_file_size_bytes,omitempty"`

	// BatchSize is the number of chunks to embed in a single batch (default: 50)
	BatchSize int `json:"batch_size,omitempty"`

	// Workers is the number of concurrent file processing workers (default: 4)
	Workers int `json:"workers,omitempty"`
}

type Config struct {
	Schema     int                     `json:"schema"`
	CodeRoot   string                  `json:"code_root"`
	Editor     string                  `json:"editor,omitempty"`
	Servers    map[string]ServerConfig `json:"servers,omitempty"`
	Embeddings *EmbeddingsConfig       `json:"embeddings,omitempty"`
	Indexing   *IndexingConfig         `json:"indexing,omitempty"`
	Tmp        *TmpConfig              `json:"tmp,omitempty"`
}

const CurrentConfigSchema = 1

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Schema:   CurrentConfigSchema,
		CodeRoot: filepath.Join(home, "Code"),
		Editor:   "",
		Servers:  map[string]ServerConfig{},
	}
}

func Load(configPath string) (*Config, error) {
	paths := getConfigPaths(configPath)

	for _, path := range paths {
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}

		cfg.expandPaths()
		return &cfg, nil
	}

	return DefaultConfig(), nil
}

func getConfigPaths(explicit string) []string {
	home, _ := os.UserHomeDir()

	var paths []string

	if explicit != "" {
		paths = append(paths, explicit)
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	paths = append(paths, filepath.Join(xdgConfig, "co", "config.json"))

	paths = append(paths, filepath.Join(home, "Code", "_system", "config.json"))

	return paths
}

func (c *Config) expandPaths() {
	home, _ := os.UserHomeDir()

	if len(c.CodeRoot) > 0 && c.CodeRoot[0] == '~' {
		c.CodeRoot = filepath.Join(home, c.CodeRoot[1:])
	}

	for name, server := range c.Servers {
		if server.CodeRoot == "" {
			server.CodeRoot = "~/Code"
		}
		c.Servers[name] = server
	}
}

func (c *Config) GetServer(name string) *ServerConfig {
	if server, ok := c.Servers[name]; ok {
		return &server
	}
	return &ServerConfig{
		SSH:      name,
		CodeRoot: "~/Code",
	}
}

func (c *Config) SystemDir() string {
	return filepath.Join(c.CodeRoot, "_system")
}

func (c *Config) IndexPath() string {
	return filepath.Join(c.SystemDir(), "index.jsonl")
}

func (c *Config) ArchiveDir() string {
	return filepath.Join(c.SystemDir(), "archive")
}

func (c *Config) LogsDir() string {
	return filepath.Join(c.SystemDir(), "logs")
}

func (c *Config) CacheDir() string {
	return filepath.Join(c.SystemDir(), "cache")
}

// TemplatesDir returns the path to the primary templates directory.
func (c *Config) TemplatesDir() string {
	return filepath.Join(c.SystemDir(), "templates")
}

// PartialsDir returns the path to the primary partials directory.
func (c *Config) PartialsDir() string {
	return filepath.Join(c.SystemDir(), "partials")
}

// FallbackTemplatesDir returns the XDG config templates directory for backwards compatibility.
func (c *Config) FallbackTemplatesDir() string {
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		home, _ := os.UserHomeDir()
		xdgConfig = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfig, "co", "templates")
}

// FallbackPartialsDir returns the XDG config partials directory for backwards compatibility.
func (c *Config) FallbackPartialsDir() string {
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		home, _ := os.UserHomeDir()
		xdgConfig = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfig, "co", "partials")
}

// AllTemplatesDirs returns all template directories to search, in priority order.
// Primary (_system/templates) is checked first, then fallback (XDG config).
func (c *Config) AllTemplatesDirs() []string {
	return []string{c.TemplatesDir(), c.FallbackTemplatesDir()}
}

// AllPartialsDirs returns all partials directories to search, in priority order.
// Primary (_system/partials) is checked first, then fallback (XDG config).
func (c *Config) AllPartialsDirs() []string {
	return []string{c.PartialsDir(), c.FallbackPartialsDir()}
}

func (c *Config) WorkspacePath(slug string) string {
	return filepath.Join(c.CodeRoot, slug)
}

// VectorsDBPath returns the path to the vector search database
func (c *Config) VectorsDBPath() string {
	return filepath.Join(c.SystemDir(), "vectors.db")
}

// GetEmbeddingsConfig returns the embeddings config with defaults applied
func (c *Config) GetEmbeddingsConfig() EmbeddingsConfig {
	cfg := EmbeddingsConfig{
		Backend:         "ollama",
		OllamaURL:       "http://localhost:11434",
		OllamaModel:     "nomic-embed-text",
		OpenAIModel:     "text-embedding-3-small",
		OpenAIAPIKeyEnv: "OPENAI_API_KEY",
	}

	if c.Embeddings != nil {
		if c.Embeddings.Backend != "" {
			cfg.Backend = c.Embeddings.Backend
		}
		if c.Embeddings.OllamaURL != "" {
			cfg.OllamaURL = c.Embeddings.OllamaURL
		}
		if c.Embeddings.OllamaModel != "" {
			cfg.OllamaModel = c.Embeddings.OllamaModel
		}
		if c.Embeddings.OpenAIModel != "" {
			cfg.OpenAIModel = c.Embeddings.OpenAIModel
		}
		if c.Embeddings.OpenAIAPIKeyEnv != "" {
			cfg.OpenAIAPIKeyEnv = c.Embeddings.OpenAIAPIKeyEnv
		}
	}

	return cfg
}

// GetIndexingConfig returns the indexing config with defaults applied
func (c *Config) GetIndexingConfig() IndexingConfig {
	cfg := IndexingConfig{
		ChunkMaxLines:     100,
		ChunkMinLines:     5,
		ChunkOverlapLines: 3,
		MaxFileSizeBytes:  1024 * 1024, // 1MB
		BatchSize:         50,
		Workers:           4,
		ExcludePatterns: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/.git/**",
			"**/target/**",
			"**/dist/**",
			"**/build/**",
		},
	}

	if c.Indexing != nil {
		if c.Indexing.ChunkMaxLines > 0 {
			cfg.ChunkMaxLines = c.Indexing.ChunkMaxLines
		}
		if c.Indexing.ChunkMinLines > 0 {
			cfg.ChunkMinLines = c.Indexing.ChunkMinLines
		}
		if c.Indexing.ChunkOverlapLines > 0 {
			cfg.ChunkOverlapLines = c.Indexing.ChunkOverlapLines
		}
		if c.Indexing.MaxFileSizeBytes > 0 {
			cfg.MaxFileSizeBytes = c.Indexing.MaxFileSizeBytes
		}
		if c.Indexing.BatchSize > 0 {
			cfg.BatchSize = c.Indexing.BatchSize
		}
		if c.Indexing.Workers > 0 {
			cfg.Workers = c.Indexing.Workers
		}
		if len(c.Indexing.ExcludePatterns) > 0 {
			cfg.ExcludePatterns = c.Indexing.ExcludePatterns
		}
		if len(c.Indexing.IncludeLanguages) > 0 {
			cfg.IncludeLanguages = c.Indexing.IncludeLanguages
		}
	}

	return cfg
}

// GetTmpConfig returns the tmp config with defaults applied
func (c *Config) GetTmpConfig() TmpConfig {
	cfg := TmpConfig{
		CleanupDays: 30,
	}

	if c.Tmp != nil {
		if c.Tmp.CleanupDays > 0 {
			cfg.CleanupDays = c.Tmp.CleanupDays
		}
	}

	return cfg
}
