package vectordb

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// IndexedFile represents a file that has been indexed
type IndexedFile struct {
	ID          int64
	Codebase    string
	Repo        string
	FilePath    string
	Language    string
	ContentHash string
	FileSize    int64
	IndexedAt   time.Time
}

// Chunk represents a code chunk with its embedding
type Chunk struct {
	ID            int64
	FileID        int64
	StartLine     int
	EndLine       int
	ChunkType     string // "function", "class", "method", "block", etc.
	SymbolName    string // Name of the function/class/method if applicable
	Content       string
	TokenEstimate int
	Embedding     []float32
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Chunk      Chunk
	File       IndexedFile
	Score      float64
	Distance   float64
	Codebase   string
	Repo       string
	FilePath   string
	Language   string
	SymbolName string
}

// Stats contains database statistics
type Stats struct {
	TotalFiles       int64
	TotalChunks      int64
	TotalCodebases   int64
	CodebaseStats    []CodebaseStats
	OldestIndexed    time.Time
	NewestIndexed    time.Time
	DatabaseSizeBytes int64
}

// CodebaseStats contains stats for a single codebase
type CodebaseStats struct {
	Codebase    string
	FileCount   int64
	ChunkCount  int64
	RepoCount   int64
	Languages   []string
	LastIndexed time.Time
}

// UpsertFile inserts or updates an indexed file record
func (db *DB) UpsertFile(f *IndexedFile) (int64, error) {
	_, err := db.conn.Exec(`
		INSERT INTO indexed_files (codebase, repo, file_path, language, content_hash, file_size, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(codebase, repo, file_path) DO UPDATE SET
			language = excluded.language,
			content_hash = excluded.content_hash,
			file_size = excluded.file_size,
			indexed_at = excluded.indexed_at
	`, f.Codebase, f.Repo, f.FilePath, f.Language, f.ContentHash, f.FileSize, time.Now())
	if err != nil {
		return 0, fmt.Errorf("upserting file: %w", err)
	}

	// Always query for the ID since LastInsertId() is unreliable with ON CONFLICT UPDATE
	var id int64
	row := db.conn.QueryRow(
		"SELECT id FROM indexed_files WHERE codebase = ? AND repo = ? AND file_path = ?",
		f.Codebase, f.Repo, f.FilePath,
	)
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("getting file id: %w", err)
	}
	return id, nil
}

// GetFileByPath retrieves a file by its path within a codebase/repo
func (db *DB) GetFileByPath(codebase, repo, filePath string) (*IndexedFile, error) {
	var f IndexedFile
	err := db.conn.QueryRow(`
		SELECT id, codebase, repo, file_path, language, content_hash, file_size, indexed_at
		FROM indexed_files
		WHERE codebase = ? AND repo = ? AND file_path = ?
	`, codebase, repo, filePath).Scan(
		&f.ID, &f.Codebase, &f.Repo, &f.FilePath, &f.Language, &f.ContentHash, &f.FileSize, &f.IndexedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting file: %w", err)
	}
	return &f, nil
}

// DeleteFileChunks deletes all chunks for a file
func (db *DB) DeleteFileChunks(fileID int64) error {
	// First delete from vector table
	_, err := db.conn.Exec(`
		DELETE FROM chunk_vectors
		WHERE chunk_id IN (SELECT id FROM chunks WHERE file_id = ?)
	`, fileID)
	if err != nil {
		return fmt.Errorf("deleting chunk vectors: %w", err)
	}

	// Then delete chunks
	_, err = db.conn.Exec("DELETE FROM chunks WHERE file_id = ?", fileID)
	if err != nil {
		return fmt.Errorf("deleting chunks: %w", err)
	}
	return nil
}

// InsertChunk inserts a chunk and its embedding
func (db *DB) InsertChunk(c *Chunk) (int64, error) {
	// Insert chunk metadata
	result, err := db.conn.Exec(`
		INSERT INTO chunks (file_id, start_line, end_line, chunk_type, symbol_name, content, token_estimate)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, c.FileID, c.StartLine, c.EndLine, c.ChunkType, c.SymbolName, c.Content, c.TokenEstimate)
	if err != nil {
		return 0, fmt.Errorf("inserting chunk: %w", err)
	}

	chunkID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting chunk id: %w", err)
	}

	// Insert embedding if present
	if len(c.Embedding) > 0 {
		embeddingBlob := float32SliceToBytes(c.Embedding)
		_, err = db.conn.Exec(`
			INSERT INTO chunk_vectors (chunk_id, embedding)
			VALUES (?, ?)
		`, chunkID, embeddingBlob)
		if err != nil {
			return 0, fmt.Errorf("inserting embedding: %w", err)
		}
	}

	return chunkID, nil
}

// SearchSimilar finds chunks similar to the given embedding
func (db *DB) SearchSimilar(embedding []float32, limit int, codebase string) ([]SearchResult, error) {
	embeddingBlob := float32SliceToBytes(embedding)

	// sqlite-vec requires k = ? in WHERE clause for knn queries
	query := `
		SELECT
			c.id, c.file_id, c.start_line, c.end_line, c.chunk_type,
			c.symbol_name, c.content, c.token_estimate,
			f.codebase, f.repo, f.file_path, f.language,
			distance
		FROM chunk_vectors v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN indexed_files f ON f.id = c.file_id
		WHERE v.embedding MATCH ? AND k = ?
	`
	args := []interface{}{embeddingBlob, limit}

	if codebase != "" {
		query += " AND f.codebase = ?"
		args = append(args, codebase)
	}

	query += `
		ORDER BY distance
	`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var symbolName sql.NullString
		err := rows.Scan(
			&r.Chunk.ID, &r.Chunk.FileID, &r.Chunk.StartLine, &r.Chunk.EndLine,
			&r.Chunk.ChunkType, &symbolName, &r.Chunk.Content, &r.Chunk.TokenEstimate,
			&r.Codebase, &r.Repo, &r.FilePath, &r.Language,
			&r.Distance,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		if symbolName.Valid {
			r.SymbolName = symbolName.String
			r.Chunk.SymbolName = symbolName.String
		}
		// Convert distance to similarity score (1 - distance for cosine)
		r.Score = 1.0 - r.Distance
		results = append(results, r)
	}

	return results, rows.Err()
}

// GetStats returns database statistics
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{}

	// Total counts
	row := db.conn.QueryRow("SELECT COUNT(*) FROM indexed_files")
	if err := row.Scan(&stats.TotalFiles); err != nil {
		return nil, fmt.Errorf("counting files: %w", err)
	}

	row = db.conn.QueryRow("SELECT COUNT(*) FROM chunks")
	if err := row.Scan(&stats.TotalChunks); err != nil {
		return nil, fmt.Errorf("counting chunks: %w", err)
	}

	row = db.conn.QueryRow("SELECT COUNT(DISTINCT codebase) FROM indexed_files")
	if err := row.Scan(&stats.TotalCodebases); err != nil {
		return nil, fmt.Errorf("counting codebases: %w", err)
	}

	// Date range - SQLite stores datetimes as strings
	row = db.conn.QueryRow("SELECT MIN(indexed_at), MAX(indexed_at) FROM indexed_files")
	var oldestStr, newestStr sql.NullString
	if err := row.Scan(&oldestStr, &newestStr); err != nil {
		return nil, fmt.Errorf("getting date range: %w", err)
	}
	if oldestStr.Valid && oldestStr.String != "" {
		if t, err := time.Parse(time.RFC3339, oldestStr.String); err == nil {
			stats.OldestIndexed = t
		}
	}
	if newestStr.Valid && newestStr.String != "" {
		if t, err := time.Parse(time.RFC3339, newestStr.String); err == nil {
			stats.NewestIndexed = t
		}
	}

	// Per-codebase stats
	rows, err := db.conn.Query(`
		SELECT
			f.codebase,
			COUNT(DISTINCT f.id) as file_count,
			COUNT(c.id) as chunk_count,
			COUNT(DISTINCT f.repo) as repo_count,
			MAX(f.indexed_at) as last_indexed
		FROM indexed_files f
		LEFT JOIN chunks c ON c.file_id = f.id
		GROUP BY f.codebase
		ORDER BY file_count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("getting codebase stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cs CodebaseStats
		var lastIndexedStr sql.NullString
		if err := rows.Scan(&cs.Codebase, &cs.FileCount, &cs.ChunkCount, &cs.RepoCount, &lastIndexedStr); err != nil {
			return nil, fmt.Errorf("scanning codebase stats: %w", err)
		}
		if lastIndexedStr.Valid && lastIndexedStr.String != "" {
			if t, err := time.Parse(time.RFC3339, lastIndexedStr.String); err == nil {
				cs.LastIndexed = t
			}
		}

		// Get languages for this codebase
		langRows, err := db.conn.Query(`
			SELECT DISTINCT language FROM indexed_files
			WHERE codebase = ? AND language IS NOT NULL AND language != ''
		`, cs.Codebase)
		if err != nil {
			return nil, fmt.Errorf("getting languages: %w", err)
		}
		for langRows.Next() {
			var lang string
			if err := langRows.Scan(&lang); err != nil {
				langRows.Close()
				return nil, fmt.Errorf("scanning language: %w", err)
			}
			cs.Languages = append(cs.Languages, lang)
		}
		langRows.Close()

		stats.CodebaseStats = append(stats.CodebaseStats, cs)
	}

	return stats, rows.Err()
}

// DeleteCodebase removes all data for a codebase
func (db *DB) DeleteCodebase(codebase string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete vectors first (references chunks)
	_, err = tx.Exec(`
		DELETE FROM chunk_vectors
		WHERE chunk_id IN (
			SELECT c.id FROM chunks c
			JOIN indexed_files f ON f.id = c.file_id
			WHERE f.codebase = ?
		)
	`, codebase)
	if err != nil {
		return fmt.Errorf("deleting vectors: %w", err)
	}

	// Delete chunks (references files)
	_, err = tx.Exec(`
		DELETE FROM chunks
		WHERE file_id IN (SELECT id FROM indexed_files WHERE codebase = ?)
	`, codebase)
	if err != nil {
		return fmt.Errorf("deleting chunks: %w", err)
	}

	// Delete files
	_, err = tx.Exec("DELETE FROM indexed_files WHERE codebase = ?", codebase)
	if err != nil {
		return fmt.Errorf("deleting files: %w", err)
	}

	return tx.Commit()
}

// GetCodebases returns all indexed codebases
func (db *DB) GetCodebases() ([]string, error) {
	rows, err := db.conn.Query("SELECT DISTINCT codebase FROM indexed_files ORDER BY codebase")
	if err != nil {
		return nil, fmt.Errorf("querying codebases: %w", err)
	}
	defer rows.Close()

	var codebases []string
	for rows.Next() {
		var cb string
		if err := rows.Scan(&cb); err != nil {
			return nil, fmt.Errorf("scanning codebase: %w", err)
		}
		codebases = append(codebases, cb)
	}
	return codebases, rows.Err()
}

// Helper functions for embedding serialization

func float32SliceToBytes(floats []float32) []byte {
	bytes := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(bytes[i*4:], bits)
	}
	return bytes
}

func bytesToFloat32Slice(bytes []byte) []float32 {
	floats := make([]float32, len(bytes)/4)
	for i := range floats {
		bits := binary.LittleEndian.Uint32(bytes[i*4:])
		floats[i] = math.Float32frombits(bits)
	}
	return floats
}
