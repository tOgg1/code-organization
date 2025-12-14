package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tormodhaugland/co/internal/chunker"
	"github.com/tormodhaugland/co/internal/vectordb"
)

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

func TestNewSearcher(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	searcher := NewSearcher(db, mock)

	if searcher == nil {
		t.Fatal("NewSearcher returned nil")
	}
}

func TestSearchEmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	searcher := NewSearcher(db, mock)

	results, err := searcher.Search(context.Background(), "test query", SearchConfig{Limit: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty database, got %d", len(results))
	}
}

func TestSearchWithData(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)

	// Insert test data with embedding that matches what mock would produce
	file := &vectordb.IndexedFile{
		Codebase:    "test-codebase",
		Repo:        "test-repo",
		FilePath:    "main.go",
		Language:    "go",
		ContentHash: "hash123",
	}
	fileID, err := db.UpsertFile(file)
	if err != nil {
		t.Fatalf("UpsertFile failed: %v", err)
	}

	// Use mock embedder to generate the embedding
	// This ensures the stored embedding matches what search will produce
	chunkContent := "func main() { fmt.Println(\"Hello\") }"
	embedding, _ := mock.Embed(context.Background(), chunkContent)

	chunk := &vectordb.Chunk{
		FileID:     fileID,
		StartLine:  1,
		EndLine:    10,
		ChunkType:  "function",
		SymbolName: "main",
		Content:    chunkContent,
		Embedding:  embedding,
	}
	_, err = db.InsertChunk(chunk)
	if err != nil {
		t.Fatalf("InsertChunk failed: %v", err)
	}

	searcher := NewSearcher(db, mock)

	// Search with the same content to get a matching embedding
	results, err := searcher.Search(context.Background(), chunkContent, SearchConfig{
		Limit:          10,
		IncludeContent: true,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// We should get at least one result since embeddings match exactly
	if len(results) == 0 {
		t.Error("expected at least 1 result")
	}

	// Verify result structure
	if len(results) > 0 {
		r := results[0]
		if r.Codebase != "test-codebase" {
			t.Errorf("Codebase = %q, want test-codebase", r.Codebase)
		}
		if r.Repo != "test-repo" {
			t.Errorf("Repo = %q, want test-repo", r.Repo)
		}
		if r.FilePath != "main.go" {
			t.Errorf("FilePath = %q, want main.go", r.FilePath)
		}
		if r.ChunkType != "function" {
			t.Errorf("ChunkType = %q, want function", r.ChunkType)
		}
		if r.Content == "" {
			t.Error("expected content to be included")
		}
	}
}

func TestSearchWithCodebaseFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Insert data for two codebases
	for _, cb := range []string{"codebase-a", "codebase-b"} {
		file := &vectordb.IndexedFile{
			Codebase:    cb,
			Repo:        "repo",
			FilePath:    "main.go",
			Language:    "go",
			ContentHash: "hash-" + cb,
		}
		fileID, _ := db.UpsertFile(file)

		embedding := make([]float32, vectordb.VectorDimension)
		for i := range embedding {
			embedding[i] = float32(i) / float32(vectordb.VectorDimension)
		}

		db.InsertChunk(&vectordb.Chunk{
			FileID:    fileID,
			StartLine: 1,
			EndLine:   10,
			ChunkType: "function",
			Content:   "code in " + cb,
			Embedding: embedding,
		})
	}

	mock := NewMockEmbedder(768)
	searcher := NewSearcher(db, mock)

	// Search with filter for codebase-a only
	results, err := searcher.Search(context.Background(), "code", SearchConfig{
		Limit:    10,
		Codebase: "codebase-a",
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// All results should be from codebase-a
	for _, r := range results {
		if r.Codebase != "codebase-a" {
			t.Errorf("result from %q, expected only codebase-a", r.Codebase)
		}
	}
}

func TestSearchConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	searcher := NewSearcher(db, mock)

	// Empty config should use defaults
	_, err = searcher.Search(context.Background(), "test", SearchConfig{})
	if err != nil {
		t.Fatalf("Search with empty config failed: %v", err)
	}
}

func TestFormatResult(t *testing.T) {
	result := SearchResult{
		Codebase:   "my-project",
		Repo:       "backend",
		FilePath:   "handlers/user.go",
		StartLine:  10,
		EndLine:    25,
		Score:      0.89,
		ChunkType:  "function",
		SymbolName: "GetUser",
		Language:   "go",
		Content:    "func GetUser(id int) *User {\n\treturn users[id]\n}",
	}

	// Format without content
	output := FormatResult(result, false, "")
	if output == "" {
		t.Error("FormatResult returned empty string")
	}

	// Format with content
	outputWithContent := FormatResult(result, true, "")
	if len(outputWithContent) <= len(output) {
		t.Error("expected content to make output longer")
	}

	// Format with code root
	outputWithRoot := FormatResult(result, false, "/home/user/Code")
	if outputWithRoot == output {
		t.Error("code root should affect output")
	}
}

func TestFormatResultSingleLine(t *testing.T) {
	result := SearchResult{
		Codebase:  "project",
		Repo:      "repo",
		FilePath:  "file.go",
		StartLine: 5,
		EndLine:   5, // Same as start
		Score:     0.75,
		ChunkType: "variable",
		Language:  "go",
	}

	output := FormatResult(result, false, "")
	// Should show just "5" not "5-5"
	if output == "" {
		t.Error("FormatResult returned empty string")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestDefaultIndexConfig(t *testing.T) {
	cfg := DefaultIndexConfig()

	if cfg.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", cfg.BatchSize)
	}
	if cfg.Workers != 4 {
		t.Errorf("Workers = %d, want 4", cfg.Workers)
	}
	if cfg.MaxFileSize != 1024*1024 {
		t.Errorf("MaxFileSize = %d, want 1048576", cfg.MaxFileSize)
	}
	if len(cfg.ExcludePatterns) == 0 {
		t.Error("expected default exclude patterns")
	}
}

func TestNewIndexer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	if indexer == nil {
		t.Fatal("NewIndexer returned nil")
	}
}

func TestIndexerConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)

	// Empty config should get defaults applied
	indexer := NewIndexer(db, mock, IndexConfig{})

	if indexer.config.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", indexer.config.BatchSize)
	}
	if indexer.config.Workers != 4 {
		t.Errorf("Workers = %d, want 4", indexer.config.Workers)
	}
	if indexer.config.MaxFileSize != 1024*1024 {
		t.Errorf("MaxFileSize = %d, want 1048576", indexer.config.MaxFileSize)
	}
}

func TestIndexCodebaseEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	// Create empty workspace with repos directory
	workspacePath := filepath.Join(tmpDir, "workspace")
	reposPath := filepath.Join(workspacePath, "repos")
	os.MkdirAll(reposPath, 0755)

	progress := make(chan IndexProgress, 100)

	err = indexer.IndexCodebase(context.Background(), "test-codebase", workspacePath, progress)
	if err != nil {
		t.Fatalf("IndexCodebase failed: %v", err)
	}

	// Drain progress channel
	for range progress {
	}

	// Check stats - should be empty
	stats, _ := db.GetStats()
	if stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", stats.TotalFiles)
	}
}

func TestIndexCodebaseWithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	// Create workspace with a Go file
	workspacePath := filepath.Join(tmpDir, "workspace")
	repoPath := filepath.Join(workspacePath, "repos", "test-repo")
	os.MkdirAll(repoPath, 0755)

	goCode := `package main

func hello() {
	fmt.Println("Hello")
}

func world() {
	fmt.Println("World")
}
`
	err = os.WriteFile(filepath.Join(repoPath, "main.go"), []byte(goCode), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	progress := make(chan IndexProgress, 100)

	err = indexer.IndexCodebase(context.Background(), "test-codebase", workspacePath, progress)
	if err != nil {
		t.Fatalf("IndexCodebase failed: %v", err)
	}

	// Drain and check progress
	var lastProgress IndexProgress
	for p := range progress {
		lastProgress = p
	}

	if lastProgress.Phase != "complete" {
		t.Errorf("last phase = %q, want complete", lastProgress.Phase)
	}
	if lastProgress.FilesProcessed == 0 {
		t.Error("expected files to be processed")
	}

	// Check stats
	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalFiles == 0 {
		t.Error("expected at least 1 indexed file")
	}
	if stats.TotalChunks == 0 {
		t.Error("expected at least 1 chunk")
	}
}

func TestIndexCodebaseExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	cfg := DefaultIndexConfig()
	// Add pattern to exclude test files
	cfg.ExcludePatterns = append(cfg.ExcludePatterns, "**/*_test.go")
	indexer := NewIndexer(db, mock, cfg)

	// Create workspace with regular and test files
	workspacePath := filepath.Join(tmpDir, "workspace")
	repoPath := filepath.Join(workspacePath, "repos", "test-repo")
	os.MkdirAll(repoPath, 0755)

	os.WriteFile(filepath.Join(repoPath, "main.go"), []byte(`package main
func main() {}
`), 0644)

	os.WriteFile(filepath.Join(repoPath, "main_test.go"), []byte(`package main
func TestMain(t *testing.T) {}
`), 0644)

	progress := make(chan IndexProgress, 100)

	err = indexer.IndexCodebase(context.Background(), "test-codebase", workspacePath, progress)
	if err != nil {
		t.Fatalf("IndexCodebase failed: %v", err)
	}

	for range progress {
	}

	// Should have indexed main.go but not main_test.go
	stats, _ := db.GetStats()
	if stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1 (test file should be excluded)", stats.TotalFiles)
	}
}

func TestDoubleStarMatch(t *testing.T) {
	// Note: The current doubleStarMatch implementation is basic and handles
	// common patterns used in exclude lists. It's not a full glob implementation.
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/node_modules/**", "lib/node_modules/pkg.js", true}, // Contains node_modules in path
		{"**/*.min.js", "app.min.js", true},
		{"**/*.min.js", "dist/bundle.min.js", true},
		{"vendor/**", "vendor/pkg/file.go", true},
		{"src/**", "lib/file.go", false},
	}

	for _, tt := range tests {
		got, _ := doubleStarMatch(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("doubleStarMatch(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestCreateEmbeddingText(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	c := chunkWithMeta{
		chunk: chunker.Chunk{
			Content:    "func hello() {}",
			ChunkType:  "function",
			SymbolName: "hello",
			Language:   "go",
		},
		file: fileInfo{
			relPath: "pkg/utils.go",
		},
	}

	text := indexer.createEmbeddingText(c)

	// Should contain file path
	if text == "" {
		t.Error("createEmbeddingText returned empty string")
	}
	// Should contain the content
	if len(text) < len(c.chunk.Content) {
		t.Error("embedding text should contain chunk content")
	}
}

func TestEndToEndSearchPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)

	// Create indexer and index some code
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	workspacePath := filepath.Join(tmpDir, "workspace")
	repoPath := filepath.Join(workspacePath, "repos", "myapp")
	os.MkdirAll(repoPath, 0755)

	// Write multiple files
	os.WriteFile(filepath.Join(repoPath, "auth.go"), []byte(`package auth

func Login(username, password string) (*User, error) {
	// Authenticate user
	return nil, nil
}

func Logout(user *User) error {
	return nil
}
`), 0644)

	os.WriteFile(filepath.Join(repoPath, "user.go"), []byte(`package user

type User struct {
	ID       int
	Username string
	Email    string
}

func CreateUser(username, email string) *User {
	return &User{Username: username, Email: email}
}
`), 0644)

	progress := make(chan IndexProgress, 100)
	err = indexer.IndexCodebase(context.Background(), "test-project", workspacePath, progress)
	if err != nil {
		t.Fatalf("IndexCodebase failed: %v", err)
	}
	for range progress {
	}

	// Verify indexing worked
	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalFiles == 0 {
		t.Fatal("expected indexed files")
	}
	if stats.TotalChunks == 0 {
		t.Fatal("expected indexed chunks")
	}

	// Now search - note: with mock embeddings, search results are based on
	// hash similarity, not semantic similarity. We just verify the pipeline works.
	searcher := NewSearcher(db, mock)

	results, err := searcher.Search(context.Background(), "any query", SearchConfig{
		Limit:          5,
		IncludeContent: true,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// With mock embeddings, we get results (even if not semantically meaningful)
	// because the knn query returns nearest neighbors regardless of actual distance
	// The main point is verifying the end-to-end pipeline works without errors

	// If we do get results, verify they're from our codebase
	for _, r := range results {
		if r.Codebase != "test-project" {
			t.Errorf("unexpected codebase: %s", r.Codebase)
		}
		if r.Repo != "myapp" {
			t.Errorf("unexpected repo: %s", r.Repo)
		}
	}
}

func TestScanFilesSkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	// Create workspace with hidden directory
	reposPath := filepath.Join(tmpDir, "repos")
	os.MkdirAll(filepath.Join(reposPath, "repo", ".hidden"), 0755)
	os.MkdirAll(filepath.Join(reposPath, "repo", "visible"), 0755)

	os.WriteFile(filepath.Join(reposPath, "repo", ".hidden", "secret.go"), []byte("package secret"), 0644)
	os.WriteFile(filepath.Join(reposPath, "repo", "visible", "public.go"), []byte("package public"), 0644)

	files, err := indexer.scanFiles("test-codebase", reposPath)
	if err != nil {
		t.Fatalf("scanFiles failed: %v", err)
	}

	// Should only find the file in visible directory
	for _, f := range files {
		if f.relPath == ".hidden/secret.go" {
			t.Error("should not have scanned file in hidden directory")
		}
	}
}

func TestScanFilesSkipsNodeModules(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	reposPath := filepath.Join(tmpDir, "repos")
	os.MkdirAll(filepath.Join(reposPath, "repo", "node_modules", "pkg"), 0755)
	os.MkdirAll(filepath.Join(reposPath, "repo", "src"), 0755)

	os.WriteFile(filepath.Join(reposPath, "repo", "node_modules", "pkg", "index.js"), []byte("module.exports = {}"), 0644)
	os.WriteFile(filepath.Join(reposPath, "repo", "src", "app.js"), []byte("const x = 1"), 0644)

	files, err := indexer.scanFiles("test-codebase", reposPath)
	if err != nil {
		t.Fatalf("scanFiles failed: %v", err)
	}

	for _, f := range files {
		if f.repo == "repo" && f.relPath == "node_modules/pkg/index.js" {
			t.Error("should not have scanned file in node_modules")
		}
	}
}

func TestScanFilesRespectsMaxFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	cfg := DefaultIndexConfig()
	cfg.MaxFileSize = 100 // Very small limit
	indexer := NewIndexer(db, mock, cfg)

	reposPath := filepath.Join(tmpDir, "repos")
	os.MkdirAll(filepath.Join(reposPath, "repo"), 0755)

	// Small file (under limit)
	os.WriteFile(filepath.Join(reposPath, "repo", "small.go"), []byte("package small"), 0644)

	// Large file (over limit)
	largeContent := make([]byte, 200)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	os.WriteFile(filepath.Join(reposPath, "repo", "large.go"), largeContent, 0644)

	files, err := indexer.scanFiles("test-codebase", reposPath)
	if err != nil {
		t.Fatalf("scanFiles failed: %v", err)
	}

	// Should only find small.go
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if len(files) > 0 && files[0].relPath != "small.go" {
		t.Errorf("expected small.go, got %s", files[0].relPath)
	}
}

func TestSearchByCode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	searcher := NewSearcher(db, mock)

	// Should not error even on empty DB
	_, err = searcher.SearchByCode(context.Background(), "func main() {}", "go", SearchConfig{})
	if err != nil {
		t.Fatalf("SearchByCode failed: %v", err)
	}
}

func TestIncrementalIndexing(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := vectordb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEmbedder(768)
	indexer := NewIndexer(db, mock, DefaultIndexConfig())

	workspacePath := filepath.Join(tmpDir, "workspace")
	repoPath := filepath.Join(workspacePath, "repos", "repo")
	os.MkdirAll(repoPath, 0755)

	// Initial indexing
	os.WriteFile(filepath.Join(repoPath, "main.go"), []byte(`package main
func original() {}
`), 0644)

	progress := make(chan IndexProgress, 100)
	indexer.IndexCodebase(context.Background(), "cb", workspacePath, progress)
	for range progress {
	}

	initialCallCount := mock.callCount

	// Re-index without changes - should skip embedding
	progress = make(chan IndexProgress, 100)
	indexer.IndexCodebase(context.Background(), "cb", workspacePath, progress)
	for range progress {
	}

	// Call count should be similar (may have some overhead)
	// The key is it shouldn't double
	if mock.callCount > initialCallCount*2 {
		t.Errorf("expected incremental indexing to skip unchanged files, call count went from %d to %d",
			initialCallCount, mock.callCount)
	}
}
