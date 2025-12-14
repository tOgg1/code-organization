package vectordb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database with vector search capabilities
type DB struct {
	conn *sql.DB
	path string
}

// VectorDimension is the embedding dimension (nomic-embed-text uses 768)
const VectorDimension = 768

func init() {
	sqlite_vec.Auto()
}

// Open opens or creates the vector database at the given path
func Open(dbPath string) (*DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	db := &DB{conn: conn, path: dbPath}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// Conn returns the underlying database connection for advanced queries
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// migrate runs database schema migrations
func (db *DB) migrate() error {
	// Create schema version table
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("getting schema version: %w", err)
	}

	// Run migrations
	migrations := []struct {
		version int
		sql     string
	}{
		{1, migrationV1},
	}

	for _, m := range migrations {
		if m.version > currentVersion {
			if _, err := db.conn.Exec(m.sql); err != nil {
				return fmt.Errorf("migration v%d: %w", m.version, err)
			}
			if _, err := db.conn.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
				return fmt.Errorf("recording migration v%d: %w", m.version, err)
			}
		}
	}

	return nil
}

// migrationV1 creates the initial schema
const migrationV1 = `
-- Track indexed files for incremental updates
CREATE TABLE IF NOT EXISTS indexed_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    codebase TEXT NOT NULL,
    repo TEXT NOT NULL,
    file_path TEXT NOT NULL,
    language TEXT,
    content_hash TEXT NOT NULL,
    file_size INTEGER,
    indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(codebase, repo, file_path)
);

CREATE INDEX IF NOT EXISTS idx_files_codebase ON indexed_files(codebase);
CREATE INDEX IF NOT EXISTS idx_files_hash ON indexed_files(content_hash);

-- Store code chunks with metadata
CREATE TABLE IF NOT EXISTS chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL REFERENCES indexed_files(id) ON DELETE CASCADE,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    chunk_type TEXT,
    symbol_name TEXT,
    content TEXT NOT NULL,
    token_estimate INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chunks_file ON chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_chunks_symbol ON chunks(symbol_name);
CREATE INDEX IF NOT EXISTS idx_chunks_type ON chunks(chunk_type);

-- sqlite-vec virtual table for fast similarity search
CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
    chunk_id INTEGER PRIMARY KEY,
    embedding float[768]
);

-- Track indexing jobs
CREATE TABLE IF NOT EXISTS index_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    codebase TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    files_total INTEGER DEFAULT 0,
    files_indexed INTEGER DEFAULT 0,
    chunks_created INTEGER DEFAULT 0,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT
);
`
