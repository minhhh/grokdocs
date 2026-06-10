package project

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// FTSDatabase encapsulates the SQLite connection and file path.
type FTSDatabase struct {
	Path string
	db   *sql.DB
}

const (
	SQLiteDriverName = "sqlite3"

	createAllTablesSQL = `CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_path TEXT UNIQUE NOT NULL,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL,
			modified_at INTEGER NOT NULL,
			content_hash TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_id INTEGER NOT NULL,
			collection TEXT NOT NULL,
			slug TEXT NOT NULL,
			chunk_count INTEGER NOT NULL,
			total_chars INTEGER NOT NULL,
			metadata TEXT,
			FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
			UNIQUE(file_id, collection)
		);
		CREATE TABLE IF NOT EXISTS chunks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			document_id INTEGER NOT NULL,
			chunk_index INTEGER NOT NULL,
			text_content TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			total_chars INTEGER NOT NULL,
			line_start INTEGER NOT NULL,
			line_end INTEGER NOT NULL,
			section_num INTEGER NOT NULL,
			section_title TEXT NOT NULL,
			metadata TEXT,
			FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
		);
		CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
			text_content,
			content='chunks',
			content_rowid='id'
		);
		CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
			INSERT INTO chunks_fts(rowid, text_content) VALUES (new.id, new.text_content);
		END;
		CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
			INSERT INTO chunks_fts(chunks_fts, rowid, text_content) VALUES('delete', old.id, old.text_content);
		END;
		CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
			INSERT INTO chunks_fts(chunks_fts, rowid, text_content) VALUES('delete', old.id, old.text_content);
			INSERT INTO chunks_fts(rowid, text_content) VALUES (new.id, new.text_content);
		END;`
)

// FileRecord represents a row in the files table.
type FileRecord struct {
	ID          int64
	FilePath    string
	Filename    string
	Size        int64
	ModifiedAt  int64 // Unix timestamp
	ContentHash string
}

// DocumentRecord represents a row in the documents table.
type DocumentRecord struct {
	ID         int64
	FileID     int64
	Collection string
	Slug       string
	ChunkCount int
	TotalChars int64
	Metadata   string // JSON-encoded string
}

// ChunkRecord represents a row in the chunks table.
type ChunkRecord struct {
	ID           int64 // maps 1-to-1 to FAISS index IDs
	DocumentID   int64
	ChunkIndex   int
	TextContent  string
	ContentHash  string
	TotalChars   int64
	LineStart    int
	LineEnd      int
	SectionNum   int
	SectionTitle string
	Metadata     string // JSON-encoded string
}

// FTSResult wraps a matching chunk record and its BM25 rank score from SQLite.
type FTSResult struct {
	Chunk *ChunkRecord
	Rank  float64
}

// OpenFTSDatabase opens (and initializes if necessary) the SQLite database.
func OpenFTSDatabase(dbPath string) (*FTSDatabase, error) {
	db, err := sql.Open(SQLiteDriverName, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Ping to verify the file can be opened/created
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
	}

	return &FTSDatabase{
		Path: dbPath,
		db:   db,
	}, nil
}

// Close closes the SQLite database connection.
func (f *FTSDatabase) Close() error {
	if f.db != nil {
		return f.db.Close()
	}
	return nil
}

// DB returns the underlying sql.DB connection.
func (f *FTSDatabase) DB() *sql.DB {
	return f.db
}

// InitSchema initializes database tables, FTS5 virtual table, and triggers.
func (f *FTSDatabase) InitSchema() error {
	if _, err := f.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := f.db.Exec(createAllTablesSQL); err != nil {
		return fmt.Errorf("failed to execute schema: %w\nQuery: %s", err, createAllTablesSQL)
	}
	return nil
}

// GetFile retrieves file metadata by path. Returns sql.ErrNoRows if not found.
func (f *FTSDatabase) GetFile(filePath string) (*FileRecord, error) {
	row := f.db.QueryRow(`
		SELECT id, file_path, filename, size, modified_at, content_hash
		FROM files WHERE file_path = ?`, filePath)
	var r FileRecord
	if err := row.Scan(&r.ID, &r.FilePath, &r.Filename, &r.Size, &r.ModifiedAt, &r.ContentHash); err != nil {
		return nil, err
	}
	return &r, nil
}

// SaveFile inserts or updates a file.
func (f *FTSDatabase) SaveFile(file *FileRecord) error {
	if file.ID == 0 {
		res, err := f.db.Exec(`
			INSERT INTO files (file_path, filename, size, modified_at, content_hash)
			VALUES (?, ?, ?, ?, ?)`,
			file.FilePath, file.Filename, file.Size, file.ModifiedAt, file.ContentHash)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		file.ID = id
	} else {
		_, err := f.db.Exec(`
			UPDATE files
			SET file_path = ?, filename = ?, size = ?, modified_at = ?, content_hash = ?
			WHERE id = ?`,
			file.FilePath, file.Filename, file.Size, file.ModifiedAt, file.ContentHash, file.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteFile deletes a file by ID (triggers cascading deletes to documents and chunks).
func (f *FTSDatabase) DeleteFile(id int64) error {
	_, err := f.db.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

// GetDocument retrieves document mapping by file ID and collection. Returns sql.ErrNoRows if not found.
func (f *FTSDatabase) GetDocument(fileID int64, collection string) (*DocumentRecord, error) {
	row := f.db.QueryRow(`
		SELECT id, file_id, collection, slug, chunk_count, total_chars, metadata
		FROM documents WHERE file_id = ? AND collection = ?`, fileID, collection)
	var r DocumentRecord
	var metadata sql.NullString
	if err := row.Scan(&r.ID, &r.FileID, &r.Collection, &r.Slug, &r.ChunkCount, &r.TotalChars, &metadata); err != nil {
		return nil, err
	}
	if metadata.Valid {
		r.Metadata = metadata.String
	}
	return &r, nil
}

// SaveDocument inserts or updates a document map.
func (f *FTSDatabase) SaveDocument(doc *DocumentRecord) error {
	var metadata any = nil
	if doc.Metadata != "" {
		metadata = doc.Metadata
	}

	if doc.ID == 0 {
		res, err := f.db.Exec(`
			INSERT INTO documents (file_id, collection, slug, chunk_count, total_chars, metadata)
			VALUES (?, ?, ?, ?, ?, ?)`,
			doc.FileID, doc.Collection, doc.Slug, doc.ChunkCount, doc.TotalChars, metadata)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		doc.ID = id
	} else {
		_, err := f.db.Exec(`
			UPDATE documents
			SET file_id = ?, collection = ?, slug = ?, chunk_count = ?, total_chars = ?, metadata = ?
			WHERE id = ?`,
			doc.FileID, doc.Collection, doc.Slug, doc.ChunkCount, doc.TotalChars, metadata, doc.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetChunksForDocument retrieves all chunks for a document in order.
func (f *FTSDatabase) GetChunksForDocument(docID int64) ([]*ChunkRecord, error) {
	rows, err := f.db.Query(`
		SELECT id, document_id, chunk_index, text_content, content_hash, total_chars, line_start, line_end, section_num, section_title, metadata
		FROM chunks WHERE document_id = ? ORDER BY chunk_index ASC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*ChunkRecord
	for rows.Next() {
		var r ChunkRecord
		var metadata sql.NullString
		err := rows.Scan(
			&r.ID, &r.DocumentID, &r.ChunkIndex, &r.TextContent, &r.ContentHash, &r.TotalChars,
			&r.LineStart, &r.LineEnd, &r.SectionNum, &r.SectionTitle, &metadata,
		)
		if err != nil {
			return nil, err
		}
		if metadata.Valid {
			r.Metadata = metadata.String
		}
		chunks = append(chunks, &r)
	}
	return chunks, nil
}

// SaveChunk inserts or updates a text chunk.
func (f *FTSDatabase) SaveChunk(chunk *ChunkRecord) error {
	var metadata any = nil
	if chunk.Metadata != "" {
		metadata = chunk.Metadata
	}

	if chunk.ID == 0 {
		res, err := f.db.Exec(`
			INSERT INTO chunks (document_id, chunk_index, text_content, content_hash, total_chars, line_start, line_end, section_num, section_title, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chunk.DocumentID, chunk.ChunkIndex, chunk.TextContent, chunk.ContentHash, chunk.TotalChars,
			chunk.LineStart, chunk.LineEnd, chunk.SectionNum, chunk.SectionTitle, metadata,
		)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		chunk.ID = id
	} else {
		_, err := f.db.Exec(`
			UPDATE chunks
			SET document_id = ?, chunk_index = ?, text_content = ?, content_hash = ?, total_chars = ?, line_start = ?, line_end = ?, section_num = ?, section_title = ?, metadata = ?
			WHERE id = ?`,
			chunk.DocumentID, chunk.ChunkIndex, chunk.TextContent, chunk.ContentHash, chunk.TotalChars,
			chunk.LineStart, chunk.LineEnd, chunk.SectionNum, chunk.SectionTitle, metadata, chunk.ID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteChunksForDocument deletes all chunks for a document.
func (f *FTSDatabase) DeleteChunksForDocument(docID int64) error {
	_, err := f.db.Exec("DELETE FROM chunks WHERE document_id = ?", docID)
	return err
}

// SearchFTS queries the FTS5 virtual table for matching text and returns matching chunks + FTS BM25 rank score.
func (f *FTSDatabase) SearchFTS(queryText string, collection string, limit int) ([]*FTSResult, error) {
	sqlQuery := `
		SELECT c.id, c.document_id, c.chunk_index, c.text_content, c.content_hash, c.total_chars, c.line_start, c.line_end, c.section_num, c.section_title, c.metadata, f.rank
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
		JOIN chunks_fts f ON c.id = f.rowid
		WHERE chunks_fts MATCH ?
	`
	args := []any{queryText}

	if collection != "" {
		sqlQuery += " AND d.collection = ?"
		args = append(args, collection)
	}

	sqlQuery += " ORDER BY f.rank LIMIT ?"
	args = append(args, limit)

	rows, err := f.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("FTS query failed: %w", err)
	}
	defer rows.Close()

	var results []*FTSResult
	for rows.Next() {
		var r ChunkRecord
		var rank float64
		var metadata sql.NullString
		err := rows.Scan(
			&r.ID, &r.DocumentID, &r.ChunkIndex, &r.TextContent, &r.ContentHash, &r.TotalChars,
			&r.LineStart, &r.LineEnd, &r.SectionNum, &r.SectionTitle, &metadata, &rank,
		)
		if err != nil {
			return nil, err
		}
		if metadata.Valid {
			r.Metadata = metadata.String
		}
		results = append(results, &FTSResult{
			Chunk: &r,
			Rank:  rank,
		})
	}
	return results, nil
}

// DBStats represents index statistics queried from the database.
type DBStats struct {
	TotalFiles        int64
	TotalChunks       int64
	TotalChars        int64
	CollectionsCount  int64
	DocsPerCollection map[string]int64
}

// GetStats returns summary statistics from the database.
func (f *FTSDatabase) GetStats() (*DBStats, error) {
	stats := &DBStats{
		DocsPerCollection: make(map[string]int64),
	}

	// Total files
	err := f.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&stats.TotalFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	// Total chunks
	err = f.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&stats.TotalChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to count chunks: %w", err)
	}

	// Total characters (sum of total_chars in chunks)
	var totalChars sql.NullInt64
	err = f.db.QueryRow("SELECT SUM(total_chars) FROM chunks").Scan(&totalChars)
	if err != nil {
		return nil, fmt.Errorf("failed to sum total chars: %w", err)
	}
	stats.TotalChars = totalChars.Int64

	// Collections count and documents per collection
	rows, err := f.db.Query("SELECT collection, COUNT(*) FROM documents GROUP BY collection")
	if err != nil {
		return nil, fmt.Errorf("failed to query documents per collection: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var collection string
		var count int64
		if err := rows.Scan(&collection, &count); err != nil {
			return nil, fmt.Errorf("failed to scan collection stats: %w", err)
		}
		stats.DocsPerCollection[collection] = count
		stats.CollectionsCount++
	}

	return stats, nil
}

