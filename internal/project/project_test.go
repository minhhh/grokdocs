package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
)

func TestRootDiscovery(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "grokdocs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "nested", "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	// 1. If startDir has no .grokdocs in parents, it should return startDir as root.
	proj, err := FindProject(subDir)
	if err != nil {
		t.Fatalf("FindProject failed: %v", err)
	}
	absSubDir, _ := filepath.Abs(subDir)
	if proj.RootPath != absSubDir {
		t.Errorf("expected fallback root path %q, got %q", absSubDir, proj.RootPath)
	}

	// 2. Create .grokdocs directory in tmpDir
	grokDir := filepath.Join(tmpDir, ConfigDirName)
	if err := os.Mkdir(grokDir, 0755); err != nil {
		t.Fatalf("failed to create .grokdocs: %v", err)
	}

	// 3. Now FindProject from subDir should find the root at tmpDir
	proj2, err := FindProject(subDir)
	if err != nil {
		t.Fatalf("FindProject failed: %v", err)
	}
	absTmpDir, _ := filepath.Abs(tmpDir)
	if proj2.RootPath != absTmpDir {
		t.Errorf("expected root path %q, got %q", absTmpDir, proj2.RootPath)
	}
}

func TestProjectInitializationAndLoading(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grokdocs-init-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	proj, err := NewProject(tmpDir)
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	// Verify project is not initialized yet
	configPath := filepath.Join(proj.ConfigDir, ConfigFileName)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config.yaml should not exist yet")
	}

	// Initialize the project
	if err := proj.Init(); err != nil {
		t.Fatalf("proj.Init() failed: %v", err)
	}

	// Verify .grokdocs and config.yaml are created
	if info, err := os.Stat(proj.ConfigDir); err != nil || !info.IsDir() {
		t.Fatalf(".grokdocs directory was not created: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml was not created: %v", err)
	}

	// Load the project config
	if err := proj.Load(); err != nil {
		t.Fatalf("proj.Load() failed: %v", err)
	}

	if proj.Config == nil {
		t.Fatalf("config was not loaded")
	}

	// Verify default collections config
	defaultColl, exists := proj.Config.Collections[config.DefaultCollectionName]
	if !exists {
		t.Fatalf("expected 'default' collection in config")
	}
	if defaultColl.Path != config.DefaultCollectionPath {
		t.Errorf("expected path '.' for default collection, got %q", defaultColl.Path)
	}
}

func TestDatabasesLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grokdocs-db-lifecycle-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	proj, err := NewProject(tmpDir)
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	if err := proj.Init(); err != nil {
		t.Fatalf("proj.Init() failed: %v", err)
	}

	// 1. Open SQLite FTS database
	fts, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	if fts.db == nil {
		t.Fatalf("expected sql.DB to be initialized")
	}

	// Ping to verify database works
	if err := fts.db.Ping(); err != nil {
		t.Fatalf("sqlite ping failed: %v", err)
	}

	// 2. Open FAISS vector database
	vec, err := proj.OpenVector()
	if err != nil {
		t.Fatalf("OpenVector failed: %v", err)
	}
	if err := vec.Save(); err != nil {
		t.Fatalf("Vector Save failed: %v", err)
	}

	// Verify vector file exists
	if _, err := os.Stat(vec.IndexPath); err != nil {
		t.Fatalf("expected vector index file to exist: %v", err)
	}

	// 3. Close databases
	if err := proj.Close(); err != nil {
		t.Fatalf("proj.Close() failed: %v", err)
	}

	// Verify connections are cleared
	if proj.ftsDB != nil || proj.vectorDB != nil {
		t.Errorf("expected db handles to be nil after Close")
	}
}

func TestSQLiteDatabase(t *testing.T) {
	// Initialize an in-memory database to test FTSDatabase logic cleanly and quickly
	db, err := OpenFTSDatabase(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// 1. Insert file
	file := &FileRecord{
		FilePath:    "docs/intro.md",
		Filename:    "intro.md",
		Size:        1024,
		ModifiedAt:  1234567890,
		ContentHash: "hash-123",
	}
	if err := db.SaveFile(file); err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}
	if file.ID == 0 {
		t.Fatalf("expected ID to be populated")
	}

	// Retrieve file
	retrievedFile, err := db.GetFile("docs/intro.md")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if retrievedFile.ContentHash != "hash-123" {
		t.Errorf("expected hash 'hash-123', got %q", retrievedFile.ContentHash)
	}

	// 2. Insert document mapping
	doc := &DocumentRecord{
		FileID:     file.ID,
		Collection: "default",
		Slug:       "introduction",
		ChunkCount: 2,
		TotalChars: 99,
		Metadata:   `{"author":"Jane Doe","version":1}`,
	}
	if err := db.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}
	if doc.ID == 0 {
		t.Fatalf("expected ID to be populated")
	}

	// Retrieve document
	retrievedDoc, err := db.GetDocument(file.ID, "default")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if retrievedDoc.Slug != "introduction" {
		t.Errorf("expected slug 'introduction', got %q", retrievedDoc.Slug)
	}
	if retrievedDoc.TotalChars != 99 {
		t.Errorf("expected total chars 99, got %d", retrievedDoc.TotalChars)
	}
	if retrievedDoc.Metadata != `{"author":"Jane Doe","version":1}` {
		t.Errorf("expected document metadata %q, got %q", `{"author":"Jane Doe","version":1}`, retrievedDoc.Metadata)
	}

	// 3. Insert chunks
	chunk1 := &ChunkRecord{
		DocumentID:   doc.ID,
		ChunkIndex:   0,
		TextContent:  "Grokdocs is a local-first semantic search engine.",
		ContentHash:  "chunk-hash-1",
		TotalChars:   49,
		LineStart:    1,
		LineEnd:      5,
		SectionNum:   1,
		SectionTitle: "Introduction",
		Metadata:     `{"importance":"high"}`,
	}
	chunk2 := &ChunkRecord{
		DocumentID:   doc.ID,
		ChunkIndex:   1,
		TextContent:  "It uses SQLite FTS5 and FAISS for hybrid retrieval.",
		ContentHash:  "chunk-hash-2",
		TotalChars:   50,
		LineStart:    6,
		LineEnd:      10,
		SectionNum:   2,
		SectionTitle: "Architecture",
		Metadata:     `{"importance":"medium"}`,
	}

	if err := db.SaveChunk(chunk1); err != nil {
		t.Fatalf("SaveChunk 1 failed: %v", err)
	}
	if err := db.SaveChunk(chunk2); err != nil {
		t.Fatalf("SaveChunk 2 failed: %v", err)
	}

	// Retrieve chunks
	chunks, err := db.GetChunksForDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetChunksForDocument failed: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].TextContent != "Grokdocs is a local-first semantic search engine." {
		t.Errorf("unexpected chunk content: %q", chunks[0].TextContent)
	}
	if chunks[0].TotalChars != 49 {
		t.Errorf("expected chunk TotalChars 49, got %d", chunks[0].TotalChars)
	}
	if chunks[0].LineStart != 1 || chunks[0].LineEnd != 5 {
		t.Errorf("expected line range 1-5, got %d-%d", chunks[0].LineStart, chunks[0].LineEnd)
	}
	if chunks[0].SectionNum != 1 || chunks[0].SectionTitle != "Introduction" {
		t.Errorf("expected section 1 'Introduction', got %d %q", chunks[0].SectionNum, chunks[0].SectionTitle)
	}
	if chunks[0].Metadata != `{"importance":"high"}` {
		t.Errorf("expected chunk metadata %q, got %q", `{"importance":"high"}`, chunks[0].Metadata)
	}

	if chunks[1].LineStart != 6 || chunks[1].LineEnd != 10 {
		t.Errorf("expected line range 6-10, got %d-%d", chunks[1].LineStart, chunks[1].LineEnd)
	}
	if chunks[1].SectionNum != 2 || chunks[1].SectionTitle != "Architecture" {
		t.Errorf("expected section 2 'Architecture', got %d %q", chunks[1].SectionNum, chunks[1].SectionTitle)
	}
	if chunks[1].Metadata != `{"importance":"medium"}` {
		t.Errorf("expected chunk metadata %q, got %q", `{"importance":"medium"}`, chunks[1].Metadata)
	}

	// 4. Test Full-Text Search (FTS5)
	results, err := db.SearchFTS("semantic", "default", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result matching 'semantic', got %d", len(results))
	}
	if results[0].Chunk.ID != chunk1.ID {
		t.Errorf("expected chunk1 id %d, got %d", chunk1.ID, results[0].Chunk.ID)
	}
	if results[0].Chunk.TotalChars != 49 {
		t.Errorf("expected chunk TotalChars 49, got %d", results[0].Chunk.TotalChars)
	}
	if results[0].Chunk.LineStart != 1 || results[0].Chunk.LineEnd != 5 {
		t.Errorf("expected line range 1-5, got %d-%d", results[0].Chunk.LineStart, results[0].Chunk.LineEnd)
	}
	if results[0].Chunk.SectionNum != 1 || results[0].Chunk.SectionTitle != "Introduction" {
		t.Errorf("expected section 1 'Introduction', got %d %q", results[0].Chunk.SectionNum, results[0].Chunk.SectionTitle)
	}
	if results[0].Chunk.Metadata != `{"importance":"high"}` {
		t.Errorf("expected metadata %q, got %q", `{"importance":"high"}`, results[0].Chunk.Metadata)
	}

	// Test FTS search with collection filter
	noResults, err := db.SearchFTS("semantic", "nonexistent-collection", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(noResults) != 0 {
		t.Errorf("expected 0 results for nonexistent collection, got %d", len(noResults))
	}

	// 5. Test cascading deletes
	if err := db.DeleteFile(file.ID); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file is deleted
	_, err = db.GetFile("docs/intro.md")
	if err == nil {
		t.Errorf("expected file to be deleted")
	}

	// Verify document is deleted (via cascade)
	_, err = db.GetDocument(file.ID, "default")
	if err == nil {
		t.Errorf("expected document mapping to be deleted via cascade")
	}

	// Verify chunks are deleted (via cascade)
	remainingChunks, err := db.GetChunksForDocument(doc.ID)
	if err != nil {
		t.Fatalf("failed to query remaining chunks: %v", err)
	}
	if len(remainingChunks) != 0 {
		t.Errorf("expected 0 chunks remaining, got %d", len(remainingChunks))
	}

	// Verify FTS table is synchronized (should return 0 matches)
	ftsResults, err := db.SearchFTS("semantic", "default", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(ftsResults) != 0 {
		t.Errorf("expected 0 FTS results after delete, got %d", len(ftsResults))
	}
}

func TestGetStats(t *testing.T) {
	db, err := OpenFTSDatabase(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Stats on empty database
	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalFiles != 0 || stats.TotalChunks != 0 || stats.TotalChars != 0 || stats.CollectionsCount != 0 {
		t.Errorf("expected empty stats, got %+v", stats)
	}

	// Insert test data
	file1 := &FileRecord{
		FilePath:    "docs/a.md",
		Filename:    "a.md",
		Size:        100,
		ModifiedAt:  123,
		ContentHash: "hash-a",
	}
	if err := db.SaveFile(file1); err != nil {
		t.Fatalf("failed to save file: %v", err)
	}

	doc1 := &DocumentRecord{
		FileID:     file1.ID,
		Collection: "notes",
		Slug:       "notes/a",
		ChunkCount: 1,
		TotalChars: 50,
	}
	if err := db.SaveDocument(doc1); err != nil {
		t.Fatalf("failed to save doc: %v", err)
	}

	chunk1 := &ChunkRecord{
		DocumentID:  doc1.ID,
		ChunkIndex:  0,
		TextContent: "hello world notes",
		ContentHash: "hash-c1",
		TotalChars:  17,
	}
	if err := db.SaveChunk(chunk1); err != nil {
		t.Fatalf("failed to save chunk: %v", err)
	}

	file2 := &FileRecord{
		FilePath:    "docs/b.md",
		Filename:    "b.md",
		Size:        200,
		ModifiedAt:  456,
		ContentHash: "hash-b",
	}
	if err := db.SaveFile(file2); err != nil {
		t.Fatalf("failed to save file: %v", err)
	}

	doc2 := &DocumentRecord{
		FileID:     file2.ID,
		Collection: "wiki",
		Slug:       "wiki/b",
		ChunkCount: 2,
		TotalChars: 80,
	}
	if err := db.SaveDocument(doc2); err != nil {
		t.Fatalf("failed to save doc: %v", err)
	}

	chunk2a := &ChunkRecord{
		DocumentID:  doc2.ID,
		ChunkIndex:  0,
		TextContent: "hello wiki first chunk",
		ContentHash: "hash-c2a",
		TotalChars:  22,
	}
	chunk2b := &ChunkRecord{
		DocumentID:  doc2.ID,
		ChunkIndex:  1,
		TextContent: "hello wiki second chunk",
		ContentHash: "hash-c2b",
		TotalChars:  23,
	}
	if err := db.SaveChunk(chunk2a); err != nil {
		t.Fatalf("failed to save chunk: %v", err)
	}
	if err := db.SaveChunk(chunk2b); err != nil {
		t.Fatalf("failed to save chunk: %v", err)
	}

	stats, err = db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", stats.TotalFiles)
	}
	if stats.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", stats.TotalChunks)
	}
	if stats.TotalChars != 62 { // 17 + 22 + 23 = 62
		t.Errorf("expected 62 chars, got %d", stats.TotalChars)
	}
	if stats.CollectionsCount != 2 {
		t.Errorf("expected 2 collections, got %d", stats.CollectionsCount)
	}
	if stats.DocsPerCollection["notes"] != 1 {
		t.Errorf("expected 1 doc in 'notes', got %d", stats.DocsPerCollection["notes"])
	}
	if stats.DocsPerCollection["wiki"] != 1 {
		t.Errorf("expected 1 doc in 'wiki', got %d", stats.DocsPerCollection["wiki"])
	}
}

