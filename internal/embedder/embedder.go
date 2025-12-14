package embedder

import (
	"context"
	"fmt"
)

// Embedder is the interface for generating text embeddings
type Embedder interface {
	// Embed generates an embedding vector for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts efficiently
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension
	Dimension() int

	// ModelName returns the name of the embedding model
	ModelName() string
}

// Config holds configuration for the embedder
type Config struct {
	// Backend is the embedding backend to use: "ollama" or "openai"
	Backend string `json:"backend"`

	// OllamaURL is the URL of the Ollama server (default: http://localhost:11434)
	OllamaURL string `json:"ollama_url,omitempty"`

	// OllamaModel is the Ollama model to use (default: nomic-embed-text)
	OllamaModel string `json:"ollama_model,omitempty"`

	// OpenAIModel is the OpenAI model to use (default: text-embedding-3-small)
	OpenAIModel string `json:"openai_model,omitempty"`

	// OpenAIAPIKeyEnv is the environment variable containing the OpenAI API key
	OpenAIAPIKeyEnv string `json:"openai_api_key_env,omitempty"`
}

// DefaultConfig returns the default embedder configuration
func DefaultConfig() Config {
	return Config{
		Backend:         "ollama",
		OllamaURL:       "http://localhost:11434",
		OllamaModel:     "nomic-embed-text",
		OpenAIModel:     "text-embedding-3-small",
		OpenAIAPIKeyEnv: "OPENAI_API_KEY",
	}
}

// New creates a new embedder based on the configuration
func New(cfg Config) (Embedder, error) {
	switch cfg.Backend {
	case "ollama", "":
		return NewOllama(cfg.OllamaURL, cfg.OllamaModel)
	case "openai":
		return nil, fmt.Errorf("openai backend not yet implemented")
	default:
		return nil, fmt.Errorf("unknown embedding backend: %s", cfg.Backend)
	}
}
