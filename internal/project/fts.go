package project

import (
	"database/sql"

	"github.com/minhhh/grokdocs/internal/util"
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
			slug TEXT NOT NULL DEFAULT '',
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
	Slug         string
	Metadata     string // JSON-encoded string
}

// FTSResult contains all matching chunk fields plus its BM25 rank score from SQLite.
type FTSResult struct {
	ID           int64 // maps 1-to-1 to FAISS index IDs
	DocumentID   int64
	ChunkIndex   int
	LineStart    int
	LineEnd      int
	SectionTitle string
	Slug         string
	Snippet      string
	Rank         float64
}

// OpenFTSDatabase opens (and initializes if necessary) the SQLite database.
func OpenFTSDatabase(dbPath string) (*FTSDatabase, error) {
	db, err := sql.Open(SQLiteDriverName, dbPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", dbPath).Msg("failed to open sqlite database")
		return nil, err
	}

	// Ping to verify the file can be opened/created
	if err := db.Ping(); err != nil {
		db.Close()
		util.Logger.Error().Err(err).Str("path", dbPath).Msg("failed to connect to sqlite database")
		return nil, err
	}

	return &FTSDatabase{
		Path: dbPath,
		db:   db,
	}, nil
}

// Close closes the SQLite database connection.
func (fts *FTSDatabase) Close() error {
	if fts.db != nil {
		return fts.db.Close()
	}
	return nil
}

// DB returns the underlying sql.DB connection.
func (fts *FTSDatabase) DB() *sql.DB {
	return fts.db
}

// InitSchema initializes database tables, FTS5 virtual table, and triggers.
func (fts *FTSDatabase) InitSchema() error {
	if _, err := fts.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		util.Logger.Error().Err(err).Msg("failed to enable foreign keys")
		return err
	}
	if _, err := fts.db.Exec(createAllTablesSQL); err != nil {
		util.Logger.Error().Err(err).Str("query", createAllTablesSQL).Msg("failed to execute schema")
		return err
	}
	return nil
}

// GetFile retrieves file metadata by path. Returns sql.ErrNoRows if not found.
func (fts *FTSDatabase) GetFile(filePath string) (*FileRecord, error) {
	row := fts.db.QueryRow(`
		SELECT id, file_path, filename, size, modified_at, content_hash
		FROM files WHERE file_path = ?`, filePath)
	var record FileRecord
	if err := row.Scan(&record.ID, &record.FilePath, &record.Filename, &record.Size, &record.ModifiedAt, &record.ContentHash); err != nil {
		return nil, err
	}
	return &record, nil
}

// SaveFile inserts or updates a file.
func (fts *FTSDatabase) SaveFile(file *FileRecord) error {
	if file.ID == 0 {
		result, err := fts.db.Exec(`
			INSERT INTO files (file_path, filename, size, modified_at, content_hash)
			VALUES (?, ?, ?, ?, ?)`,
			file.FilePath, file.Filename, file.Size, file.ModifiedAt, file.ContentHash)
		if err != nil {
			return err
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		file.ID = lastID
	} else {
		_, err := fts.db.Exec(`
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

// CollectionFile represents a file record scoped to a collection.
type CollectionFile struct {
	ID      int64
	Path    string
	Hash    string
}

// ListCollectionFiles returns all files belonging to a collection.
func (fts *FTSDatabase) ListCollectionFiles(collectionName string) ([]CollectionFile, error) {
	rows, err := fts.db.Query(`
		SELECT f.id, f.file_path, f.content_hash
		FROM files f
		JOIN documents d ON f.id = d.file_id
		WHERE d.collection = ?`, collectionName)
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to query collection files")
		return nil, err
	}
	defer rows.Close()

	var files []CollectionFile
	for rows.Next() {
		var collectionFile CollectionFile
		if err := rows.Scan(&collectionFile.ID, &collectionFile.Path, &collectionFile.Hash); err != nil {
			util.Logger.Error().Err(err).Msg("failed to scan collection file row")
			return nil, err
		}
		files = append(files, collectionFile)
	}
	return files, nil
}

// DeleteFile deletes a file by ID (triggers cascading deletes to documents and chunks).
func (fts *FTSDatabase) DeleteFile(id int64) error {
	_, err := fts.db.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

// GetDocument retrieves document mapping by file ID and collection. Returns sql.ErrNoRows if not found.
func (fts *FTSDatabase) GetDocument(fileID int64, collection string) (*DocumentRecord, error) {
	row := fts.db.QueryRow(`
		SELECT id, file_id, collection, slug, chunk_count, total_chars, metadata
		FROM documents WHERE file_id = ? AND collection = ?`, fileID, collection)
	var record DocumentRecord
	var metadata sql.NullString
	if err := row.Scan(&record.ID, &record.FileID, &record.Collection, &record.Slug, &record.ChunkCount, &record.TotalChars, &metadata); err != nil {
		return nil, err
	}
	if metadata.Valid {
		record.Metadata = metadata.String
	}
	return &record, nil
}

// GetFilePathByDocumentID retrieves the file path for a given document ID.
func (fts *FTSDatabase) GetFilePathByDocumentID(documentID int64) (string, error) {
	var filePath string
	err := fts.db.QueryRow(
		"SELECT f.file_path FROM files f JOIN documents d ON f.id = d.file_id WHERE d.id = ?", documentID,
	).Scan(&filePath)
	return filePath, err
}

// SaveDocument inserts or updates a document map.
func (fts *FTSDatabase) SaveDocument(doc *DocumentRecord) error {
	var metadata any = nil
	if doc.Metadata != "" {
		metadata = doc.Metadata
	}

	if doc.ID == 0 {
		result, err := fts.db.Exec(`
			INSERT INTO documents (file_id, collection, slug, chunk_count, total_chars, metadata)
			VALUES (?, ?, ?, ?, ?, ?)`,
			doc.FileID, doc.Collection, doc.Slug, doc.ChunkCount, doc.TotalChars, metadata)
		if err != nil {
			return err
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		doc.ID = lastID
	} else {
		_, err := fts.db.Exec(`
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
func (fts *FTSDatabase) GetChunksForDocument(docID int64) ([]*ChunkRecord, error) {
	rows, err := fts.db.Query(`
		SELECT id, document_id, chunk_index, text_content, content_hash, total_chars, line_start, line_end, section_num, section_title, slug, metadata
		FROM chunks WHERE document_id = ? ORDER BY chunk_index ASC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*ChunkRecord
	for rows.Next() {
		var record ChunkRecord
		var metadata sql.NullString
		err := rows.Scan(
			&record.ID, &record.DocumentID, &record.ChunkIndex, &record.TextContent, &record.ContentHash, &record.TotalChars,
			&record.LineStart, &record.LineEnd, &record.SectionNum, &record.SectionTitle, &record.Slug, &metadata,
		)
		if err != nil {
			return nil, err
		}
		if metadata.Valid {
			record.Metadata = metadata.String
		}
		chunks = append(chunks, &record)
	}
	return chunks, nil
}

// SaveChunk inserts or updates a text chunk.
func (fts *FTSDatabase) SaveChunk(chunk *ChunkRecord) error {
	var metadata any = nil
	if chunk.Metadata != "" {
		metadata = chunk.Metadata
	}

	if chunk.ID == 0 {
		result, err := fts.db.Exec(`
			INSERT INTO chunks (document_id, chunk_index, text_content, content_hash, total_chars, line_start, line_end, section_num, section_title, slug, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chunk.DocumentID, chunk.ChunkIndex, chunk.TextContent, chunk.ContentHash, chunk.TotalChars,
			chunk.LineStart, chunk.LineEnd, chunk.SectionNum, chunk.SectionTitle, chunk.Slug, metadata,
		)
		if err != nil {
			return err
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		chunk.ID = lastID
	} else {
		_, err := fts.db.Exec(`
			UPDATE chunks
			SET document_id = ?, chunk_index = ?, text_content = ?, content_hash = ?, total_chars = ?, line_start = ?, line_end = ?, section_num = ?, section_title = ?, slug = ?, metadata = ?
			WHERE id = ?`,
			chunk.DocumentID, chunk.ChunkIndex, chunk.TextContent, chunk.ContentHash, chunk.TotalChars,
			chunk.LineStart, chunk.LineEnd, chunk.SectionNum, chunk.SectionTitle, chunk.Slug, metadata, chunk.ID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// SaveChunksBatch saves all chunks in a single transaction.
func (fts *FTSDatabase) SaveChunksBatch(chunks []*ChunkRecord) error {
	tx, err := fts.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		var metadata any = nil
		if chunk.Metadata != "" {
			metadata = chunk.Metadata
		}
		_, err := tx.Exec(`
			INSERT INTO chunks (document_id, chunk_index, text_content, content_hash, total_chars, line_start, line_end, section_num, section_title, slug, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chunk.DocumentID, chunk.ChunkIndex, chunk.TextContent, chunk.ContentHash, chunk.TotalChars,
			chunk.LineStart, chunk.LineEnd, chunk.SectionNum, chunk.SectionTitle, chunk.Slug, metadata,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteChunksForDocument deletes all chunks for a document.
func (fts *FTSDatabase) DeleteChunksForDocument(docID int64) error {
	_, err := fts.db.Exec("DELETE FROM chunks WHERE document_id = ?", docID)
	return err
}

// SearchFTS queries the FTS5 virtual table for matching text and returns matching chunks + FTS BM25 rank score.
func (fts *FTSDatabase) SearchFTS(queryText string, collection string, limit int) ([]*FTSResult, error) {
	sqlQuery := `
		SELECT c.id, c.document_id, c.chunk_index, c.line_start, c.line_end, c.section_title, c.slug, snippet(chunks_fts, 0, '', '', '...', 5), -f.rank AS rank
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

	rows, err := fts.db.Query(sqlQuery, args...)
	if err != nil {
		util.Logger.Error().Err(err).Str("query", sqlQuery).Msg("FTS query failed")
		return nil, err
	}
	defer rows.Close()

	var results []*FTSResult
	for rows.Next() {
		var r FTSResult
		var snippet sql.NullString
		err := rows.Scan(
			&r.ID, &r.DocumentID, &r.ChunkIndex,
			&r.LineStart, &r.LineEnd, &r.SectionTitle, &r.Slug, &snippet, &r.Rank,
		)
		if err != nil {
			return nil, err
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		results = append(results, &r)
	}
	return results, nil
}

// DBStats represents index statistics queried from the database.
type DBStats struct {
	TotalFiles          int64
	TotalDocuments      int64
	TotalChunks         int64
	TotalChars          int64
	CollectionsCount    int64
	DocsPerCollection   map[string]int64
	ChunksPerCollection map[string]int64
}

// GetStats returns summary statistics from the database.
func (fts *FTSDatabase) GetStats() (*DBStats, error) {
	stats := &DBStats{
		DocsPerCollection:  make(map[string]int64),
		ChunksPerCollection: make(map[string]int64),
	}

	// Total source files indexed (distinct files on disk)
	err := fts.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&stats.TotalFiles)
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to count files")
		return nil, err
	}

	// Total document mappings (one file can appear in multiple collections)
	err = fts.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&stats.TotalDocuments)
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to count documents")
		return nil, err
	}

	// Total chunks
	err = fts.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&stats.TotalChunks)
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to count chunks")
		return nil, err
	}

	// Total characters (sum of total_chars in chunks)
	var totalChars sql.NullInt64
	err = fts.db.QueryRow("SELECT SUM(total_chars) FROM chunks").Scan(&totalChars)
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to sum total chars")
		return nil, err
	}
	stats.TotalChars = totalChars.Int64

	// Collections count and documents per collection
	rows, err := fts.db.Query("SELECT collection, COUNT(*) FROM documents GROUP BY collection")
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to query documents per collection")
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var collection string
		var count int64
		if err := rows.Scan(&collection, &count); err != nil {
			util.Logger.Error().Err(err).Msg("failed to scan collection stats")
			return nil, err
		}
		stats.DocsPerCollection[collection] = count
		stats.CollectionsCount++
	}
	rows.Close()

	// Chunks per collection
	chunkRows, err := fts.db.Query(`
		SELECT d.collection, COUNT(*)
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
		GROUP BY d.collection`)
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to query chunks per collection")
		return nil, err
	}
	defer chunkRows.Close()

	for chunkRows.Next() {
		var collection string
		var count int64
		if err := chunkRows.Scan(&collection, &count); err != nil {
			util.Logger.Error().Err(err).Msg("failed to scan chunks per collection")
			return nil, err
		}
		stats.ChunksPerCollection[collection] = count
	}

	return stats, nil
}

