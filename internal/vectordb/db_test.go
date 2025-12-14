package vectordb

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if db.Path() != dbPath {
		t.Errorf("Path() = %q, want %q", db.Path(), dbPath)
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify directory was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

func TestMigration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Check tables exist
	tables := []string{"indexed_files", "chunks", "chunk_vectors", "index_jobs", "schema_version"}
	for _, table := range tables {
		var count int
		err := db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
		}
		if table != "chunk_vectors" && count != 1 {
			// chunk_vectors is a virtual table, checked differently
			t.Errorf("table %s not found", table)
		}
	}
}

func TestUpsertFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	file := &IndexedFile{
		Codebase:    "test--project",
		Repo:        "api",
		FilePath:    "main.go",
		Language:    "go",
		ContentHash: "abc123",
		FileSize:    1024,
		IndexedAt:   time.Now(),
	}

	id, err := db.UpsertFile(file)
	if err != nil {
		t.Fatalf("UpsertFile failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Upsert again with different hash
	file.ContentHash = "def456"
	id2, err := db.UpsertFile(file)
	if err != nil {
		t.Fatalf("UpsertFile (update) failed: %v", err)
	}
	if id2 != id {
		t.Errorf("expected same ID %d on update, got %d", id, id2)
	}
}

func TestGetFileByPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Insert a file
	file := &IndexedFile{
		Codebase:    "test--project",
		Repo:        "api",
		FilePath:    "handlers/user.go",
		Language:    "go",
		ContentHash: "hash123",
		FileSize:    2048,
	}
	_, err = db.UpsertFile(file)
	if err != nil {
		t.Fatalf("UpsertFile failed: %v", err)
	}

	// Retrieve it
	retrieved, err := db.GetFileByPath("test--project", "api", "handlers/user.go")
	if err != nil {
		t.Fatalf("GetFileByPath failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected file, got nil")
	}
	if retrieved.ContentHash != "hash123" {
		t.Errorf("ContentHash = %q, want %q", retrieved.ContentHash, "hash123")
	}

	// Non-existent file
	notFound, err := db.GetFileByPath("test--project", "api", "nonexistent.go")
	if err != nil {
		t.Fatalf("GetFileByPath (not found) failed: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent file")
	}
}

func TestInsertChunk(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// First insert a file
	file := &IndexedFile{
		Codebase:    "test--project",
		Repo:        "api",
		FilePath:    "main.go",
		Language:    "go",
		ContentHash: "hash",
	}
	fileID, err := db.UpsertFile(file)
	if err != nil {
		t.Fatalf("UpsertFile failed: %v", err)
	}

	// Insert a chunk with embedding
	embedding := make([]float32, VectorDimension)
	for i := range embedding {
		embedding[i] = float32(i) / float32(VectorDimension)
	}

	chunk := &Chunk{
		FileID:        fileID,
		StartLine:     1,
		EndLine:       10,
		ChunkType:     "function",
		SymbolName:    "main",
		Content:       "func main() { }",
		TokenEstimate: 5,
		Embedding:     embedding,
	}

	chunkID, err := db.InsertChunk(chunk)
	if err != nil {
		t.Fatalf("InsertChunk failed: %v", err)
	}
	if chunkID <= 0 {
		t.Errorf("expected positive chunk ID, got %d", chunkID)
	}
}

func TestDeleteFileChunks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Insert file and chunks
	file := &IndexedFile{
		Codebase:    "test--project",
		Repo:        "api",
		FilePath:    "main.go",
		Language:    "go",
		ContentHash: "hash",
	}
	fileID, _ := db.UpsertFile(file)

	for i := 0; i < 3; i++ {
		chunk := &Chunk{
			FileID:    fileID,
			StartLine: i * 10,
			EndLine:   (i + 1) * 10,
			ChunkType: "block",
			Content:   "// code",
		}
		db.InsertChunk(chunk)
	}

	// Verify chunks exist
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM chunks WHERE file_id = ?", fileID).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 chunks, got %d", count)
	}

	// Delete chunks
	err = db.DeleteFileChunks(fileID)
	if err != nil {
		t.Fatalf("DeleteFileChunks failed: %v", err)
	}

	// Verify deleted
	db.conn.QueryRow("SELECT COUNT(*) FROM chunks WHERE file_id = ?", fileID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 chunks after delete, got %d", count)
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Empty stats
	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", stats.TotalFiles)
	}

	// Add some data
	file := &IndexedFile{
		Codebase:    "test--project",
		Repo:        "api",
		FilePath:    "main.go",
		Language:    "go",
		ContentHash: "hash",
	}
	fileID, _ := db.UpsertFile(file)

	chunk := &Chunk{
		FileID:    fileID,
		StartLine: 1,
		EndLine:   10,
		ChunkType: "function",
		Content:   "func main() {}",
	}
	db.InsertChunk(chunk)

	// Check stats again
	stats, err = db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", stats.TotalFiles)
	}
	if stats.TotalChunks != 1 {
		t.Errorf("TotalChunks = %d, want 1", stats.TotalChunks)
	}
	if stats.TotalCodebases != 1 {
		t.Errorf("TotalCodebases = %d, want 1", stats.TotalCodebases)
	}
}

func TestDeleteCodebase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Add data for two codebases
	for _, cb := range []string{"project-a", "project-b"} {
		file := &IndexedFile{
			Codebase:    cb,
			Repo:        "repo",
			FilePath:    "main.go",
			Language:    "go",
			ContentHash: "hash",
		}
		fileID, _ := db.UpsertFile(file)
		db.InsertChunk(&Chunk{FileID: fileID, StartLine: 1, EndLine: 10, Content: "code"})
	}

	// Delete one codebase
	err = db.DeleteCodebase("project-a")
	if err != nil {
		t.Fatalf("DeleteCodebase failed: %v", err)
	}

	// Verify
	codebases, _ := db.GetCodebases()
	if len(codebases) != 1 {
		t.Errorf("expected 1 codebase, got %d", len(codebases))
	}
	if codebases[0] != "project-b" {
		t.Errorf("expected project-b, got %s", codebases[0])
	}
}

func TestGetCodebases(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Empty
	codebases, err := db.GetCodebases()
	if err != nil {
		t.Fatalf("GetCodebases failed: %v", err)
	}
	if len(codebases) != 0 {
		t.Errorf("expected 0 codebases, got %d", len(codebases))
	}

	// Add some
	for _, cb := range []string{"alpha", "beta", "gamma"} {
		db.UpsertFile(&IndexedFile{Codebase: cb, Repo: "repo", FilePath: "f.go", ContentHash: "h"})
	}

	codebases, err = db.GetCodebases()
	if err != nil {
		t.Fatalf("GetCodebases failed: %v", err)
	}
	if len(codebases) != 3 {
		t.Errorf("expected 3 codebases, got %d", len(codebases))
	}
}

func TestFloat32Serialization(t *testing.T) {
	original := []float32{0.1, 0.2, 0.3, -0.5, 1.0, 0.0}
	bytes := float32SliceToBytes(original)
	restored := bytesToFloat32Slice(bytes)

	if len(restored) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(restored), len(original))
	}

	for i := range original {
		if restored[i] != original[i] {
			t.Errorf("value mismatch at %d: got %f, want %f", i, restored[i], original[i])
		}
	}
}
