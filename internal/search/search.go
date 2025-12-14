package search

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tormodhaugland/co/internal/embedder"
	"github.com/tormodhaugland/co/internal/vectordb"
)

// Searcher handles semantic code search
type Searcher struct {
	db       *vectordb.DB
	embedder embedder.Embedder
}

// SearchConfig holds search configuration
type SearchConfig struct {
	// Limit is the maximum number of results to return
	Limit int

	// Codebase filters results to a specific codebase
	Codebase string

	// MinScore is the minimum similarity score (0-1) to include in results
	MinScore float64

	// IncludeContent includes the full chunk content in results
	IncludeContent bool
}

// SearchResult represents a search result
type SearchResult struct {
	Codebase   string  `json:"codebase"`
	Repo       string  `json:"repo"`
	FilePath   string  `json:"file_path"`
	FullPath   string  `json:"full_path,omitempty"`
	StartLine  int     `json:"start_line"`
	EndLine    int     `json:"end_line"`
	Score      float64 `json:"score"`
	ChunkType  string  `json:"chunk_type"`
	SymbolName string  `json:"symbol_name,omitempty"`
	Language   string  `json:"language"`
	Content    string  `json:"content,omitempty"`
}

// NewSearcher creates a new searcher
func NewSearcher(db *vectordb.DB, emb embedder.Embedder) *Searcher {
	return &Searcher{
		db:       db,
		embedder: emb,
	}
}

// Search performs a semantic search using a text query
func (s *Searcher) Search(ctx context.Context, query string, cfg SearchConfig) ([]SearchResult, error) {
	if cfg.Limit == 0 {
		cfg.Limit = 10
	}

	// Embed the query
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	// Search the vector database
	// Request more results than limit to allow for filtering
	dbResults, err := s.db.SearchSimilar(queryEmbedding, cfg.Limit*2, cfg.Codebase)
	if err != nil {
		return nil, fmt.Errorf("searching database: %w", err)
	}

	// Convert and filter results
	var results []SearchResult
	for _, r := range dbResults {
		if r.Score < cfg.MinScore {
			continue
		}

		result := SearchResult{
			Codebase:   r.Codebase,
			Repo:       r.Repo,
			FilePath:   r.FilePath,
			StartLine:  r.Chunk.StartLine,
			EndLine:    r.Chunk.EndLine,
			Score:      r.Score,
			ChunkType:  r.Chunk.ChunkType,
			SymbolName: r.SymbolName,
			Language:   r.Language,
		}

		if cfg.IncludeContent {
			result.Content = r.Chunk.Content
		}

		results = append(results, result)

		if len(results) >= cfg.Limit {
			break
		}
	}

	return results, nil
}

// SearchByFile performs a search using a file's content as the query
func (s *Searcher) SearchByFile(ctx context.Context, filePath string, cfg SearchConfig) ([]SearchResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return s.Search(ctx, string(content), cfg)
}

// SearchByCode performs a search using a code snippet as the query
func (s *Searcher) SearchByCode(ctx context.Context, code string, language string, cfg SearchConfig) ([]SearchResult, error) {
	// Add language context to improve embedding quality
	query := fmt.Sprintf("Language: %s\n\n%s", language, code)
	return s.Search(ctx, query, cfg)
}

// SearchSimilarTo finds code similar to a specific location in the codebase
func (s *Searcher) SearchSimilarTo(ctx context.Context, codebase, repo, filePath string, startLine, endLine int, cfg SearchConfig) ([]SearchResult, error) {
	// First, we need to construct the full file path
	// This is a simplified version - in practice you'd use the config to resolve paths

	// Build query from the chunk context
	// For now, just search by the file path pattern
	query := fmt.Sprintf("File: %s/%s lines %d-%d", repo, filePath, startLine, endLine)

	return s.Search(ctx, query, cfg)
}

// FormatResult formats a search result for display
func FormatResult(r SearchResult, showContent bool, codeRoot string) string {
	var sb strings.Builder

	// Build path with line numbers
	fullPath := fmt.Sprintf("%s/repos/%s/%s", r.Codebase, r.Repo, r.FilePath)
	if codeRoot != "" {
		fullPath = fmt.Sprintf("%s/%s", codeRoot, fullPath)
	}

	lineRange := fmt.Sprintf("%d-%d", r.StartLine, r.EndLine)
	if r.StartLine == r.EndLine {
		lineRange = fmt.Sprintf("%d", r.StartLine)
	}

	// Header
	sb.WriteString(fmt.Sprintf("%s:%s\n", fullPath, lineRange))
	sb.WriteString(fmt.Sprintf("  Score: %.2f | %s | %s", r.Score, r.Language, r.ChunkType))
	if r.SymbolName != "" {
		sb.WriteString(fmt.Sprintf(" | %s", r.SymbolName))
	}
	sb.WriteString("\n")

	// Content preview
	if showContent && r.Content != "" {
		lines := strings.Split(r.Content, "\n")
		maxLines := 5
		if len(lines) > maxLines {
			for _, line := range lines[:maxLines] {
				sb.WriteString(fmt.Sprintf("  │ %s\n", truncate(line, 80)))
			}
			sb.WriteString(fmt.Sprintf("  │ ... (%d more lines)\n", len(lines)-maxLines))
		} else {
			for _, line := range lines {
				sb.WriteString(fmt.Sprintf("  │ %s\n", truncate(line, 80)))
			}
		}
	}

	return sb.String()
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
