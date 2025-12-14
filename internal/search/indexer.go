package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tormodhaugland/co/internal/chunker"
	"github.com/tormodhaugland/co/internal/embedder"
	"github.com/tormodhaugland/co/internal/vectordb"
)

// Indexer handles the code indexing pipeline
type Indexer struct {
	db       *vectordb.DB
	embedder embedder.Embedder
	chunker  chunker.Chunker
	config   IndexConfig
}

// IndexConfig holds indexing configuration
type IndexConfig struct {
	// BatchSize is the number of chunks to embed in a single batch
	BatchSize int

	// Workers is the number of concurrent file processing workers
	Workers int

	// ExcludePatterns are glob patterns for files to exclude
	ExcludePatterns []string

	// IncludePatterns are glob patterns for files to include (if empty, include all code files)
	IncludePatterns []string

	// MaxFileSize is the maximum file size in bytes to index (default: 1MB)
	MaxFileSize int64

	// Verbose enables verbose logging
	Verbose bool
}

// DefaultIndexConfig returns sensible defaults
func DefaultIndexConfig() IndexConfig {
	return IndexConfig{
		BatchSize:   50,
		Workers:     4,
		MaxFileSize: 1024 * 1024, // 1MB
		ExcludePatterns: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/.git/**",
			"**/target/**",
			"**/dist/**",
			"**/build/**",
			"**/.next/**",
			"**/.venv/**",
			"**/venv/**",
			"**/__pycache__/**",
			"**/*.min.js",
			"**/*.min.css",
			"**/*.map",
			"**/package-lock.json",
			"**/yarn.lock",
			"**/pnpm-lock.yaml",
			"**/go.sum",
			"**/Cargo.lock",
			"**/*.pb.go",
			"**/*_generated.go",
			"**/*.gen.go",
		},
	}
}

// IndexProgress reports indexing progress
type IndexProgress struct {
	Codebase       string
	Phase          string // "scanning", "chunking", "embedding", "storing"
	FilesTotal     int
	FilesProcessed int
	ChunksTotal    int
	ChunksEmbedded int
	CurrentFile    string
	Error          error
}

// NewIndexer creates a new indexer
func NewIndexer(db *vectordb.DB, emb embedder.Embedder, cfg IndexConfig) *Indexer {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 50
	}
	if cfg.Workers == 0 {
		cfg.Workers = 4
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 1024 * 1024
	}

	chunkerCfg := chunker.DefaultConfig()
	return &Indexer{
		db:       db,
		embedder: emb,
		chunker:  chunker.NewTreeSitter(chunkerCfg),
		config:   cfg,
	}
}

// IndexCodebase indexes all code in a codebase (workspace)
func (idx *Indexer) IndexCodebase(ctx context.Context, codebase, workspacePath string, progress chan<- IndexProgress) error {
	defer close(progress)

	// Phase 1: Scan for files
	progress <- IndexProgress{Codebase: codebase, Phase: "scanning"}

	reposPath := filepath.Join(workspacePath, "repos")
	files, err := idx.scanFiles(codebase, reposPath)
	if err != nil {
		return fmt.Errorf("scanning files: %w", err)
	}

	if len(files) == 0 {
		progress <- IndexProgress{Codebase: codebase, Phase: "complete", FilesTotal: 0}
		return nil
	}

	progress <- IndexProgress{Codebase: codebase, Phase: "chunking", FilesTotal: len(files)}

	// Phase 2: Process files and extract chunks
	type fileChunks struct {
		file   fileInfo
		chunks []chunker.Chunk
		err    error
	}

	fileChan := make(chan fileInfo, len(files))
	resultChan := make(chan fileChunks, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < idx.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				chunks, err := idx.processFile(ctx, file)
				resultChan <- fileChunks{file: file, chunks: chunks, err: err}
			}
		}()
	}

	// Feed files to workers
	go func() {
		for _, f := range files {
			fileChan <- f
		}
		close(fileChan)
	}()

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect chunks
	var allChunks []chunkWithMeta
	filesProcessed := 0

	for result := range resultChan {
		filesProcessed++
		if result.err != nil {
			if idx.config.Verbose {
				progress <- IndexProgress{
					Codebase:       codebase,
					Phase:          "chunking",
					FilesTotal:     len(files),
					FilesProcessed: filesProcessed,
					CurrentFile:    result.file.path,
					Error:          result.err,
				}
			}
			continue
		}

		for _, c := range result.chunks {
			allChunks = append(allChunks, chunkWithMeta{
				chunk: c,
				file:  result.file,
			})
		}

		progress <- IndexProgress{
			Codebase:       codebase,
			Phase:          "chunking",
			FilesTotal:     len(files),
			FilesProcessed: filesProcessed,
			ChunksTotal:    len(allChunks),
			CurrentFile:    result.file.path,
		}
	}

	if len(allChunks) == 0 {
		progress <- IndexProgress{Codebase: codebase, Phase: "complete", FilesTotal: len(files), FilesProcessed: filesProcessed}
		return nil
	}

	// Phase 3: Embed chunks in batches
	progress <- IndexProgress{
		Codebase:    codebase,
		Phase:       "embedding",
		ChunksTotal: len(allChunks),
	}

	embeddings, err := idx.embedChunks(ctx, allChunks, func(embedded int) {
		progress <- IndexProgress{
			Codebase:       codebase,
			Phase:          "embedding",
			FilesTotal:     len(files),
			FilesProcessed: filesProcessed,
			ChunksTotal:    len(allChunks),
			ChunksEmbedded: embedded,
		}
	})
	if err != nil {
		return fmt.Errorf("embedding chunks: %w", err)
	}

	// Phase 4: Store in database
	progress <- IndexProgress{
		Codebase:       codebase,
		Phase:          "storing",
		FilesTotal:     len(files),
		FilesProcessed: filesProcessed,
		ChunksTotal:    len(allChunks),
		ChunksEmbedded: len(embeddings),
	}

	if err := idx.storeChunks(ctx, codebase, allChunks, embeddings); err != nil {
		return fmt.Errorf("storing chunks: %w", err)
	}

	progress <- IndexProgress{
		Codebase:       codebase,
		Phase:          "complete",
		FilesTotal:     len(files),
		FilesProcessed: filesProcessed,
		ChunksTotal:    len(allChunks),
		ChunksEmbedded: len(embeddings),
	}

	return nil
}

// fileInfo holds file metadata
type fileInfo struct {
	path        string // Full filesystem path
	codebase    string // Codebase name
	repo        string // Repository name
	relPath     string // Path relative to repo
	contentHash string
	size        int64
}

// chunkWithMeta pairs a chunk with its file metadata
type chunkWithMeta struct {
	chunk chunker.Chunk
	file  fileInfo
}

// scanFiles finds all indexable files in the repos directory
func (idx *Indexer) scanFiles(codebase, reposPath string) ([]fileInfo, error) {
	var files []fileInfo

	// First, list repos
	repos, err := os.ReadDir(reposPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading repos directory: %w", err)
	}

	for _, repo := range repos {
		if !repo.IsDir() {
			continue
		}
		if strings.HasPrefix(repo.Name(), ".") {
			continue
		}

		repoPath := filepath.Join(reposPath, repo.Name())

		err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // Skip files we can't access
			}

			// Skip directories
			if d.IsDir() {
				base := d.Name()
				// Skip hidden and common non-code directories
				if strings.HasPrefix(base, ".") ||
					base == "node_modules" ||
					base == "vendor" ||
					base == "target" ||
					base == "dist" ||
					base == "build" ||
					base == "__pycache__" ||
					base == ".venv" ||
					base == "venv" {
					return filepath.SkipDir
				}
				return nil
			}

			// Check if file is indexable
			if !chunker.IsIndexableFile(path) {
				return nil
			}

			// Check exclude patterns
			for _, pattern := range idx.config.ExcludePatterns {
				matched, _ := filepath.Match(pattern, path)
				if matched {
					return nil
				}
				// Also check against relative path
				relPath, _ := filepath.Rel(repoPath, path)
				matched, _ = doubleStarMatch(pattern, relPath)
				if matched {
					return nil
				}
			}

			// Check file size
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.Size() > idx.config.MaxFileSize {
				return nil
			}

			relPath, _ := filepath.Rel(repoPath, path)

			files = append(files, fileInfo{
				path:     path,
				codebase: codebase,
				repo:     repo.Name(),
				relPath:  relPath,
				size:     info.Size(),
			})

			return nil
		})

		if err != nil {
			// Log but continue with other repos
			continue
		}
	}

	return files, nil
}

// processFile reads and chunks a single file
func (idx *Indexer) processFile(ctx context.Context, file fileInfo) ([]chunker.Chunk, error) {
	// Read file
	content, err := os.ReadFile(file.path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Compute hash
	hash := sha256.Sum256(content)
	file.contentHash = hex.EncodeToString(hash[:])

	// Check if file changed (for incremental indexing)
	existing, err := idx.db.GetFileByPath(file.codebase, file.repo, file.relPath)
	if err == nil && existing != nil && existing.ContentHash == file.contentHash {
		// File unchanged, skip
		return nil, nil
	}

	// Chunk the file
	chunks, err := idx.chunker.Chunk(content, file.path)
	if err != nil {
		return nil, fmt.Errorf("chunking file: %w", err)
	}

	return chunks, nil
}

// embedChunks generates embeddings for chunks in batches
func (idx *Indexer) embedChunks(ctx context.Context, chunks []chunkWithMeta, progressFn func(int)) ([][]float32, error) {
	embeddings := make([][]float32, len(chunks))

	for i := 0; i < len(chunks); i += idx.config.BatchSize {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		end := i + idx.config.BatchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		texts := make([]string, len(batch))
		for j, c := range batch {
			// Create embedding text with context
			texts[j] = idx.createEmbeddingText(c)
		}

		batchEmbeddings, err := idx.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("embedding batch: %w", err)
		}

		for j, emb := range batchEmbeddings {
			embeddings[i+j] = emb
		}

		if progressFn != nil {
			progressFn(end)
		}
	}

	return embeddings, nil
}

// createEmbeddingText creates the text to embed for a chunk
func (idx *Indexer) createEmbeddingText(c chunkWithMeta) string {
	var sb strings.Builder

	// Add context header
	sb.WriteString(fmt.Sprintf("File: %s\n", c.file.relPath))
	if c.chunk.SymbolName != "" {
		sb.WriteString(fmt.Sprintf("%s: %s\n", c.chunk.ChunkType, c.chunk.SymbolName))
	}
	sb.WriteString(fmt.Sprintf("Language: %s\n\n", c.chunk.Language))
	sb.WriteString(c.chunk.Content)

	return sb.String()
}

// storeChunks stores chunks and their embeddings in the database
func (idx *Indexer) storeChunks(ctx context.Context, codebase string, chunks []chunkWithMeta, embeddings [][]float32) error {
	// Group chunks by file for efficient storage
	fileChunks := make(map[string][]int) // file key -> chunk indices
	for i, c := range chunks {
		key := c.file.repo + "/" + c.file.relPath
		fileChunks[key] = append(fileChunks[key], i)
	}

	for _, indices := range fileChunks {
		if len(indices) == 0 {
			continue
		}

		c := chunks[indices[0]]

		// Compute content hash
		content, err := os.ReadFile(c.file.path)
		if err != nil {
			continue
		}
		hash := sha256.Sum256(content)
		contentHash := hex.EncodeToString(hash[:])

		// Upsert file record
		fileRecord := &vectordb.IndexedFile{
			Codebase:    codebase,
			Repo:        c.file.repo,
			FilePath:    c.file.relPath,
			Language:    c.chunk.Language,
			ContentHash: contentHash,
			FileSize:    c.file.size,
			IndexedAt:   time.Now(),
		}

		fileID, err := idx.db.UpsertFile(fileRecord)
		if err != nil {
			return fmt.Errorf("upserting file %s: %w", c.file.relPath, err)
		}

		// Delete old chunks for this file
		if err := idx.db.DeleteFileChunks(fileID); err != nil {
			return fmt.Errorf("deleting old chunks: %w", err)
		}

		// Insert new chunks
		for _, chunkIdx := range indices {
			chunk := chunks[chunkIdx]
			embedding := embeddings[chunkIdx]

			dbChunk := &vectordb.Chunk{
				FileID:        fileID,
				StartLine:     chunk.chunk.StartLine,
				EndLine:       chunk.chunk.EndLine,
				ChunkType:     chunk.chunk.ChunkType,
				SymbolName:    chunk.chunk.SymbolName,
				Content:       chunk.chunk.Content,
				TokenEstimate: chunk.chunk.TokenEstimate,
				Embedding:     embedding,
			}

			if _, err := idx.db.InsertChunk(dbChunk); err != nil {
				return fmt.Errorf("inserting chunk: %w", err)
			}
		}
	}

	return nil
}

// doubleStarMatch provides basic ** glob matching
func doubleStarMatch(pattern, path string) (bool, error) {
	// Simple implementation - handle common cases
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		// Check if path ends with or contains the suffix pattern
		if strings.HasSuffix(path, suffix) {
			return true, nil
		}
		if strings.Contains(path, strings.TrimPrefix(suffix, "*")) {
			return true, nil
		}
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := pattern[:len(pattern)-3]
		if strings.HasPrefix(path, prefix) {
			return true, nil
		}
	}
	return filepath.Match(pattern, path)
}
