package project

import "testing"

func setupTestDB(t *testing.T) *FTSDatabase {
	t.Helper()
	db := openTestDB(t)
	return db
}

func openTestDB(t *testing.T) *FTSDatabase {
	t.Helper()
	db, err := OpenFTSDatabase("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("OpenFTSDatabase failed: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func insertTestFile(t *testing.T, db *FTSDatabase, path string) *FileRecord {
	t.Helper()
	f := &FileRecord{
		FilePath:    path,
		Filename:    "test.md",
		Size:        100,
		ModifiedAt:  1000,
		ContentHash: "hash-" + path,
	}
	if err := db.SaveFile(f); err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}
	return f
}

func insertTestDocument(t *testing.T, db *FTSDatabase, fileID int64, collection, slug string) *DocumentRecord {
	t.Helper()
	d := &DocumentRecord{
		FileID:     fileID,
		Collection: collection,
		Slug:       slug,
		ChunkCount: 2,
		TotalChars: 50,
	}
	if err := db.SaveDocument(d); err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}
	return d
}

func insertTestChunk(t *testing.T, db *FTSDatabase, docID int64, index int, text string) *ChunkRecord {
	t.Helper()
	c := &ChunkRecord{
		DocumentID: docID,
		ChunkIndex: index,
		TextContent: text,
		TotalChars: len(text),
		LineStart: index + 1,
		LineEnd: index + 5,
		SectionNum: index + 1,
		SectionTitle: "Section",
		Slug: "test-slug",
	}
	if err := db.SaveChunk(c); err != nil {
		t.Fatalf("SaveChunk failed: %v", err)
	}
	return c
}

func TestListCollectionFiles(t *testing.T) {
	db := setupTestDB(t)
	f1 := insertTestFile(t, db, "docs/a.md")
	f2 := insertTestFile(t, db, "docs/b.md")
	insertTestDocument(t, db, f1.ID, "default", "a")
	insertTestDocument(t, db, f2.ID, "default", "b")

	files, err := db.ListCollectionFiles("default")
	if err != nil {
		t.Fatalf("ListCollectionFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	files, err = db.ListCollectionFiles("nonexistent")
	if err != nil {
		t.Fatalf("ListCollectionFiles failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for nonexistent collection, got %d", len(files))
	}
}

func TestDeleteOrphanedDocuments(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")

	if err := db.DeleteFile(f.ID); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	if err := db.DeleteOrphanedDocuments("default"); err != nil {
		t.Fatalf("DeleteOrphanedDocuments failed: %v", err)
	}

	_, err := db.GetDocumentByID(doc.ID)
	if err == nil {
		t.Error("expected orphaned document to be deleted")
	}
}

func TestDeleteFilesBatch(t *testing.T) {
	db := setupTestDB(t)
	f1 := insertTestFile(t, db, "docs/a.md")
	f2 := insertTestFile(t, db, "docs/b.md")
	f3 := insertTestFile(t, db, "docs/c.md")

	if err := db.DeleteFilesBatch([]int64{f1.ID, f2.ID, f3.ID}); err != nil {
		t.Fatalf("DeleteFilesBatch failed: %v", err)
	}

	if _, err := db.GetFile("docs/a.md"); err == nil {
		t.Error("expected file a to be deleted")
	}
	if _, err := db.GetFile("docs/b.md"); err == nil {
		t.Error("expected file b to be deleted")
	}
	if _, err := db.GetFile("docs/c.md"); err == nil {
		t.Error("expected file c to be deleted")
	}
}

func TestDeleteFilesBatch_Empty(t *testing.T) {
	db := setupTestDB(t)
	if err := db.DeleteFilesBatch(nil); err != nil {
		t.Errorf("expected nil for empty batch, got %v", err)
	}
}

func TestGetFilePathByDocumentID(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")

	path, err := db.GetFilePathByDocumentID(doc.ID)
	if err != nil {
		t.Fatalf("GetFilePathByDocumentID failed: %v", err)
	}
	if path != "docs/a.md" {
		t.Errorf("expected path docs/a.md, got %q", path)
	}
}

func TestGetFilePathByDocumentID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetFilePathByDocumentID(999)
	if err == nil {
		t.Error("expected error for nonexistent document")
	}
}

func TestGetChunkIDsByFileID(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	c1 := insertTestChunk(t, db, doc.ID, 0, "chunk one")
	insertTestChunk(t, db, doc.ID, 1, "chunk two")

	ids, err := db.GetChunkIDsByFileID(f.ID)
	if err != nil {
		t.Fatalf("GetChunkIDsByFileID failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 chunk IDs, got %d", len(ids))
	}
	found := false
	for _, id := range ids {
		if id == c1.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected c1.ID in results")
	}
}

func TestGetChunkIDsByFileID_Empty(t *testing.T) {
	db := setupTestDB(t)
	ids, err := db.GetChunkIDsByFileID(999)
	if err != nil {
		t.Fatalf("GetChunkIDsByFileID failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 ids, got %d", len(ids))
	}
}

func TestSaveChunksBatch(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")

	chunks := []*ChunkRecord{
		{
			DocumentID:   doc.ID,
			ChunkIndex:   0,
			TextContent:  "batch chunk 0",
			TotalChars:   13,
			LineStart:    1, LineEnd: 2,
			SectionNum: 1, SectionTitle: "Intro", Slug: "s0",
		},
		{
			DocumentID:   doc.ID,
			ChunkIndex:   1,
			TextContent:  "batch chunk 1",
			TotalChars:   13,
			LineStart:    3, LineEnd: 4,
			SectionNum: 2, SectionTitle: "Body", Slug: "s1",
		},
	}

	if err := db.SaveChunksBatch(chunks); err != nil {
		t.Fatalf("SaveChunksBatch failed: %v", err)
	}
	if chunks[0].ID == 0 || chunks[1].ID == 0 {
		t.Error("expected chunk IDs to be populated after batch save")
	}

	results, err := db.GetChunksForDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetChunksForDocument failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(results))
	}
}

func TestDeleteChunksForDocument(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	insertTestChunk(t, db, doc.ID, 0, "delete me")
	insertTestChunk(t, db, doc.ID, 1, "delete me too")

	if err := db.DeleteChunksForDocument(doc.ID); err != nil {
		t.Fatalf("DeleteChunksForDocument failed: %v", err)
	}

	results, err := db.GetChunksForDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetChunksForDocument failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 chunks after delete, got %d", len(results))
	}
}

func TestGetChunkByID(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	c := insertTestChunk(t, db, doc.ID, 0, "find me")

	retrieved, err := db.GetChunkByID(c.ID)
	if err != nil {
		t.Fatalf("GetChunkByID failed: %v", err)
	}
	if retrieved.TextContent != "find me" {
		t.Errorf("expected 'find me', got %q", retrieved.TextContent)
	}
}

func TestGetChunkByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetChunkByID(999)
	if err == nil {
		t.Error("expected error for nonexistent chunk")
	}
}

func TestGetDocumentByID(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")

	retrieved, err := db.GetDocumentByID(doc.ID)
	if err != nil {
		t.Fatalf("GetDocumentByID failed: %v", err)
	}
	if retrieved.Slug != "a" {
		t.Errorf("expected slug 'a', got %q", retrieved.Slug)
	}
}

func TestGetDocumentByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetDocumentByID(999)
	if err == nil {
		t.Error("expected error for nonexistent document")
	}
}

func TestVectorizedChunks(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	c1 := insertTestChunk(t, db, doc.ID, 0, "alpha")
	c2 := insertTestChunk(t, db, doc.ID, 1, "beta")

	if err := db.MarkVectorized([]int64{c1.ID, c2.ID}, "default"); err != nil {
		t.Fatalf("MarkVectorized failed: %v", err)
	}

	ids, err := db.GetVectorizedChunkIDs("default")
	if err != nil {
		t.Fatalf("GetVectorizedChunkIDs failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 vectorized IDs, got %d", len(ids))
	}

	if err := db.DeleteVectorizedChunkIDs([]int64{c1.ID}); err != nil {
		t.Fatalf("DeleteVectorizedChunkIDs failed: %v", err)
	}
	ids, _ = db.GetVectorizedChunkIDs("default")
	if len(ids) != 1 {
		t.Errorf("expected 1 remaining vectorized ID, got %d", len(ids))
	}

	if err := db.ClearCollectionVectors("default"); err != nil {
		t.Fatalf("ClearCollectionVectors failed: %v", err)
	}
	ids, _ = db.GetVectorizedChunkIDs("default")
	if len(ids) != 0 {
		t.Errorf("expected 0 vectorized IDs after clear, got %d", len(ids))
	}
}

func TestMarkVectorized_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	c := insertTestChunk(t, db, doc.ID, 0, "alpha")

	if err := db.MarkVectorized([]int64{c.ID}, "default"); err != nil {
		t.Fatalf("first MarkVectorized failed: %v", err)
	}
	if err := db.MarkVectorized([]int64{c.ID}, "default"); err != nil {
		t.Fatalf("second MarkVectorized (duplicate) failed: %v", err)
	}

	ids, _ := db.GetVectorizedChunkIDs("default")
	if len(ids) != 1 {
		t.Errorf("expected 1 ID after duplicate insert, got %d", len(ids))
	}
}

func TestDeleteVectorizedChunkIDs_Empty(t *testing.T) {
	db := setupTestDB(t)
	if err := db.DeleteVectorizedChunkIDs(nil); err != nil {
		t.Errorf("expected nil for empty input, got %v", err)
	}
}

func TestGetOrphanedVectorChunks(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	c := insertTestChunk(t, db, doc.ID, 0, "alpha")

	if err := db.MarkVectorized([]int64{c.ID}, "default"); err != nil {
		t.Fatalf("MarkVectorized failed: %v", err)
	}

	orphans, err := db.GetOrphanedVectorChunks()
	if err != nil {
		t.Fatalf("GetOrphanedVectorChunks failed: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans initially, got %d", len(orphans))
	}

	if err := db.DeleteChunksForDocument(doc.ID); err != nil {
		t.Fatalf("DeleteChunksForDocument failed: %v", err)
	}

	orphans, err = db.GetOrphanedVectorChunks()
	if err != nil {
		t.Fatalf("GetOrphanedVectorChunks failed: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].ChunkID != c.ID {
		t.Errorf("expected orphan chunk ID %d, got %d", c.ID, orphans[0].ChunkID)
	}
	if orphans[0].Collection != "default" {
		t.Errorf("expected orphan collection 'default', got %q", orphans[0].Collection)
	}
}

func TestListCollectionFilesPaginated(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	insertTestDocument(t, db, f.ID, "default", "a")

	results, total, err := db.ListCollectionFilesPaginated("default", 10, 0)
	if err != nil {
		t.Fatalf("ListCollectionFilesPaginated failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].FilePath != "docs/a.md" {
		t.Errorf("expected file path docs/a.md, got %q", results[0].FilePath)
	}

	results, total, err = db.ListCollectionFilesPaginated("default", 10, 10)
	if err != nil {
		t.Fatalf("ListCollectionFilesPaginated with offset failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results with offset past end, got %d", len(results))
	}
}

func TestListCollectionFilesPaginated_EmptyCollection(t *testing.T) {
	db := setupTestDB(t)
	results, total, err := db.ListCollectionFilesPaginated("nonexistent", 10, 0)
	if err != nil {
		t.Fatalf("ListCollectionFilesPaginated failed: %v", err)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestOpenFTSDatabase_InvalidPath(t *testing.T) {
	_, err := OpenFTSDatabase("/nonexistent/dir/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestDB(t *testing.T) {
	db := setupTestDB(t)
	if db.DB() == nil {
		t.Error("expected non-nil *sql.DB")
	}
}

func TestSaveFile_Update(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/test.md")
	originalID := f.ID
	f.Filename = "renamed.md"
	if err := db.SaveFile(f); err != nil {
		t.Fatalf("SaveFile (update) failed: %v", err)
	}
	if f.ID != originalID {
		t.Error("ID should remain unchanged on update")
	}
	got, err := db.GetFile("docs/test.md")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if got.Filename != "renamed.md" {
		t.Errorf("expected Filename 'renamed.md', got %q", got.Filename)
	}
}

func TestSaveDocument_Update(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/test.md")
	doc := insertTestDocument(t, db, f.ID, "default", "original")
	doc.Slug = "updated"
	if err := db.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument (update) failed: %v", err)
	}
	got, err := db.GetDocument(f.ID, "default")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if got.Slug != "updated" {
		t.Errorf("expected slug 'updated', got %q", got.Slug)
	}
}

func TestSaveChunk_Update(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/test.md")
	doc := insertTestDocument(t, db, f.ID, "default", "test")
	c := insertTestChunk(t, db, doc.ID, 0, "original content")
	c.TextContent = "updated content"
	if err := db.SaveChunk(c); err != nil {
		t.Fatalf("SaveChunk (update) failed: %v", err)
	}
	got, err := db.GetChunkByID(c.ID)
	if err != nil {
		t.Fatalf("GetChunkByID failed: %v", err)
	}
	if got.TextContent != "updated content" {
		t.Errorf("expected 'updated content', got %q", got.TextContent)
	}
}

func TestSaveChunk_WithMetadata(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/test.md")
	doc := insertTestDocument(t, db, f.ID, "default", "test")
	c := &ChunkRecord{
		DocumentID:   doc.ID,
		ChunkIndex:   0,
		TextContent:  "content with meta",
		TotalChars:   17,
		LineStart:    1, LineEnd: 1,
		SectionNum: 1, SectionTitle: "Test",
		Slug:     "meta-test",
		Metadata: `{"key":"value"}`,
	}
	if err := db.SaveChunk(c); err != nil {
		t.Fatalf("SaveChunk failed: %v", err)
	}
	got, err := db.GetChunkByID(c.ID)
	if err != nil {
		t.Fatalf("GetChunkByID failed: %v", err)
	}
	if got.Metadata != `{"key":"value"}` {
		t.Errorf("expected metadata %q, got %q", `{"key":"value"}`, got.Metadata)
	}
}

func TestMarkVectorized_Empty(t *testing.T) {
	db := setupTestDB(t)
	if err := db.MarkVectorized(nil, "default"); err != nil {
		t.Errorf("expected nil for empty IDs, got %v", err)
	}
}

func TestSearchFTS_EmptyCollectionFilter(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")
	insertTestChunk(t, db, doc.ID, 0, "unique search term xyz123")

	results, err := db.SearchFTS("xyz123", "", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSaveChunksBatch_WithMetadata(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	doc := insertTestDocument(t, db, f.ID, "default", "a")

	chunks := []*ChunkRecord{
		{
			DocumentID:   doc.ID,
			ChunkIndex:   0,
			TextContent:  "batch with meta",
			TotalChars:   14,
			LineStart:    1, LineEnd: 2,
			SectionNum: 1, SectionTitle: "Intro", Slug: "s0",
			Metadata: `{"batch":1}`,
		},
		{
			DocumentID:   doc.ID,
			ChunkIndex:   1,
			TextContent:  "batch without meta",
			TotalChars:   17,
			LineStart:    3, LineEnd: 4,
			SectionNum: 2, SectionTitle: "Body", Slug: "s1",
		},
	}

	if err := db.SaveChunksBatch(chunks); err != nil {
		t.Fatalf("SaveChunksBatch failed: %v", err)
	}

	got, err := db.GetChunkByID(chunks[0].ID)
	if err != nil {
		t.Fatalf("GetChunkByID failed: %v", err)
	}
	if got.Metadata != `{"batch":1}` {
		t.Errorf("expected metadata %q, got %q", `{"batch":1}`, got.Metadata)
	}

	got2, err := db.GetChunkByID(chunks[1].ID)
	if err != nil {
		t.Fatalf("GetChunkByID failed: %v", err)
	}
	if got2.Metadata != "" {
		t.Errorf("expected empty metadata, got %q", got2.Metadata)
	}
}

func TestListCollectionFilesPaginated_ZeroLimit(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/a.md")
	insertTestDocument(t, db, f.ID, "default", "a")

	results, total, err := db.ListCollectionFilesPaginated("default", 0, 0)
	if err != nil {
		t.Fatalf("ListCollectionFilesPaginated failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit=0, got %d", len(results))
	}
}

func TestGetDocument_WithNilMetadata(t *testing.T) {
	db := setupTestDB(t)
	f := insertTestFile(t, db, "docs/test.md")
	d := &DocumentRecord{
		FileID:     f.ID,
		Collection: "default",
		Slug:       "no-meta",
		ChunkCount: 1,
		TotalChars: 10,
	}
	if err := db.SaveDocument(d); err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}
	got, err := db.GetDocument(f.ID, "default")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if got.Metadata != "" {
		t.Errorf("expected empty metadata, got %q", got.Metadata)
	}
}
