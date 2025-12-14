package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaEmbedder implements the Embedder interface using Ollama
type OllamaEmbedder struct {
	baseURL   string
	model     string
	client    *http.Client
	dimension int
}

// ollamaEmbedRequest is the request format for Ollama embeddings API
type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string or []string
}

// ollamaEmbedResponse is the response format for Ollama embeddings API
type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// ollamaErrorResponse is the error response format from Ollama
type ollamaErrorResponse struct {
	Error string `json:"error"`
}

// Model dimensions (common embedding models)
var modelDimensions = map[string]int{
	"nomic-embed-text": 768,
	"all-minilm":       384,
	"mxbai-embed-large": 1024,
	"snowflake-arctic-embed": 1024,
}

// NewOllama creates a new Ollama embedder
func NewOllama(baseURL, model string) (*OllamaEmbedder, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	// Remove trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Determine dimension
	dimension := 768 // default for nomic-embed-text
	if d, ok := modelDimensions[model]; ok {
		dimension = d
	}

	e := &OllamaEmbedder{
		baseURL:   baseURL,
		model:     model,
		dimension: dimension,
		client: &http.Client{
			Timeout: 5 * time.Minute, // Embeddings can take a while for large batches
		},
	}

	return e, nil
}

// Embed generates an embedding for a single text
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Prepare request
	reqBody := ollamaEmbedRequest{
		Model: e.model,
		Input: texts,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embed", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp ollamaErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("ollama error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embedResp ollamaEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(embedResp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(embedResp.Embeddings))
	}

	return embedResp.Embeddings, nil
}

// Dimension returns the embedding dimension
func (e *OllamaEmbedder) Dimension() int {
	return e.dimension
}

// ModelName returns the model name
func (e *OllamaEmbedder) ModelName() string {
	return e.model
}

// Ping checks if Ollama is available and the model is loaded
func (e *OllamaEmbedder) Ping(ctx context.Context) error {
	// Try to generate a simple embedding to verify everything works
	_, err := e.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("ollama not available or model not loaded: %w", err)
	}
	return nil
}

// PullModel attempts to pull the embedding model if not already available
func (e *OllamaEmbedder) PullModel(ctx context.Context) error {
	reqBody := map[string]string{
		"name": e.model,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/pull", bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Stream response until done (Ollama returns streaming JSON)
	decoder := json.NewDecoder(resp.Body)
	for {
		var status map[string]any
		if err := decoder.Decode(&status); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading pull status: %w", err)
		}
		// Check for completion
		if status["status"] == "success" {
			break
		}
	}

	return nil
}
