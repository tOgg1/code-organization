package embedder

import (
	"context"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Backend != "ollama" {
		t.Errorf("Backend = %q, want ollama", cfg.Backend)
	}
	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("OllamaURL = %q, want http://localhost:11434", cfg.OllamaURL)
	}
	if cfg.OllamaModel != "nomic-embed-text" {
		t.Errorf("OllamaModel = %q, want nomic-embed-text", cfg.OllamaModel)
	}
}

func TestNewWithUnknownBackend(t *testing.T) {
	cfg := Config{Backend: "unknown"}
	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for unknown backend")
	}
}

// MockEmbedder is a test embedder that returns deterministic embeddings
type MockEmbedder struct {
	dimension int
	model     string
	callCount int
}

func NewMockEmbedder(dimension int) *MockEmbedder {
	return &MockEmbedder{
		dimension: dimension,
		model:     "mock-model",
	}
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	// Generate deterministic embedding based on text hash
	embedding := make([]float32, m.dimension)
	hash := 0
	for _, c := range text {
		hash = hash*31 + int(c)
	}
	for i := range embedding {
		embedding[i] = float32(hash+i) / float32(m.dimension*1000)
	}
	return embedding, nil
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

func (m *MockEmbedder) ModelName() string {
	return m.model
}

func TestMockEmbedder(t *testing.T) {
	mock := NewMockEmbedder(768)

	// Test single embedding
	emb, err := mock.Embed(context.Background(), "test input")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(emb) != 768 {
		t.Errorf("embedding length = %d, want 768", len(emb))
	}

	// Test batch embedding
	texts := []string{"one", "two", "three"}
	embeddings, err := mock.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}
	if len(embeddings) != 3 {
		t.Errorf("batch length = %d, want 3", len(embeddings))
	}

	// Test determinism - same input should give same output
	emb1, _ := mock.Embed(context.Background(), "consistent")
	emb2, _ := mock.Embed(context.Background(), "consistent")
	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Errorf("embeddings not deterministic at index %d", i)
			break
		}
	}

	// Different inputs should give different outputs
	emb3, _ := mock.Embed(context.Background(), "different")
	same := true
	for i := range emb1 {
		if emb1[i] != emb3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different inputs should produce different embeddings")
	}

	// Test dimension and model
	if mock.Dimension() != 768 {
		t.Errorf("Dimension() = %d, want 768", mock.Dimension())
	}
	if mock.ModelName() != "mock-model" {
		t.Errorf("ModelName() = %q, want mock-model", mock.ModelName())
	}
}

func TestOllamaEmbedderCreation(t *testing.T) {
	emb, err := NewOllama("http://localhost:11434", "nomic-embed-text")
	if err != nil {
		t.Fatalf("NewOllama failed: %v", err)
	}

	if emb.Dimension() != 768 {
		t.Errorf("Dimension() = %d, want 768 for nomic-embed-text", emb.Dimension())
	}
	if emb.ModelName() != "nomic-embed-text" {
		t.Errorf("ModelName() = %q, want nomic-embed-text", emb.ModelName())
	}
}

func TestOllamaEmbedderDefaults(t *testing.T) {
	emb, err := NewOllama("", "")
	if err != nil {
		t.Fatalf("NewOllama failed: %v", err)
	}

	if emb.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want http://localhost:11434", emb.baseURL)
	}
	if emb.model != "nomic-embed-text" {
		t.Errorf("model = %q, want nomic-embed-text", emb.model)
	}
}

func TestOllamaEmbedderDimensions(t *testing.T) {
	tests := []struct {
		model     string
		dimension int
	}{
		{"nomic-embed-text", 768},
		{"all-minilm", 384},
		{"mxbai-embed-large", 1024},
		{"unknown-model", 768}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			emb, _ := NewOllama("", tt.model)
			if emb.Dimension() != tt.dimension {
				t.Errorf("Dimension() = %d, want %d for %s", emb.Dimension(), tt.dimension, tt.model)
			}
		})
	}
}
