package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minhhh/grokdocs/internal/project"
)

func TestReadLinesOfFile_Normal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLinesOfFile(path, 2, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "line2\nline3\nline4"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadLinesOfFile_EndBeyondFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLinesOfFile(path, 2, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "line2\nline3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadLinesOfFile_StartOutOfBounds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := readLinesOfFile(path, 0, 2)
	if err == nil {
		t.Fatal("expected error for start=0")
	}
}

func TestReadLinesOfFile_EndBeforeStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := readLinesOfFile(path, 4, 2)
	if err == nil {
		t.Fatal("expected error for end < start")
	}
}

func TestReadLinesOfFile_FileNotFound(t *testing.T) {
	_, err := readLinesOfFile("/nonexistent/path", 1, 1)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadLinesOfFile_SingleLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.txt")
	content := "only line"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLinesOfFile(path, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestMergeHybridResults(t *testing.T) {
	rrfK = 60.0

	fts := []*project.SearchResult{
		{ID: 1, Rank: 0.9},
		{ID: 2, Rank: 0.8},
	}
	semantic := []*project.SearchResult{
		{ID: 3, Rank: 0.7},
		{ID: 4, Rank: 0.6},
	}

	results := mergeHybridResults(fts, semantic, 10)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
}

func TestMergeHybridResults_Dedup(t *testing.T) {
	rrfK = 60.0

	fts := []*project.SearchResult{
		{ID: 1, Rank: 0.9},
		{ID: 2, Rank: 0.8},
	}
	semantic := []*project.SearchResult{
		{ID: 2, Rank: 0.7},
		{ID: 3, Rank: 0.6},
	}

	results := mergeHybridResults(fts, semantic, 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 results (deduped), got %d", len(results))
	}
	if results[0].ID != 2 {
		t.Errorf("expected duplicate ID 2 to be ranked first (combined score), got ID %d", results[0].ID)
	}
}

func TestMergeHybridResults_OnlyFTS(t *testing.T) {
	rrfK = 60.0

	fts := []*project.SearchResult{
		{ID: 1, Rank: 0.9},
	}
	results := mergeHybridResults(fts, nil, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMergeHybridResults_OnlySemantic(t *testing.T) {
	rrfK = 60.0

	semantic := []*project.SearchResult{
		{ID: 1, Rank: 0.9},
	}
	results := mergeHybridResults(nil, semantic, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMergeHybridResults_Limit(t *testing.T) {
	rrfK = 60.0

	fts := []*project.SearchResult{
		{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5},
	}
	semantic := []*project.SearchResult{
		{ID: 6}, {ID: 7}, {ID: 8}, {ID: 9}, {ID: 10},
	}

	results := mergeHybridResults(fts, semantic, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limited), got %d", len(results))
	}
}

func TestMergeHybridResults_Empty(t *testing.T) {
	rrfK = 60.0

	results := mergeHybridResults(nil, nil, 10)
	if results != nil {
		t.Fatal("expected nil for both inputs empty")
	}
}

func TestDisplayResults_FlatFalse(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	f := &project.FileRecord{
		FilePath:    filepath.Join(root, "docs", "a.md"),
		Filename:    "a.md",
		Size:        100,
		ModifiedAt:  1000,
		ContentHash: "abc",
	}
	if err := db.SaveFile(f); err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}
	d := &project.DocumentRecord{
		FileID:     f.ID,
		Collection: "default",
		Slug:       "a",
		ChunkCount: 1,
		TotalChars: 10,
		Metadata:   "{}",
	}
	if err := db.SaveDocument(d); err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}
	c := &project.ChunkRecord{
		DocumentID:   d.ID,
		ChunkIndex:   0,
		TextContent:  "# Title",
		TotalChars:   7,
		LineStart:    1,
		LineEnd:      3,
		SectionNum:   1,
		SectionTitle: "# Title",
	}
	if err := db.SaveChunk(c); err != nil {
		t.Fatalf("SaveChunk failed: %v", err)
	}

	results := []*project.SearchResult{
		{
			ID:           c.ID,
			DocumentID:   d.ID,
			ChunkIndex:   0,
			LineStart:    1,
			LineEnd:      3,
			SectionTitle: "# Title",
			Slug:         "a",
			Snippet:      "# Title",
			Rank:         0.95,
		},
	}

	captured := captureStdout(t, func() {
		displayResults(db, root, results, 5, false)
	})
	if !strings.Contains(captured, "# Title") {
		t.Errorf("expected output to contain 'Title', got:\n%s", captured)
	}
}

func TestDisplayResults_Flat(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	f := &project.FileRecord{
		FilePath:    filepath.Join(root, "docs", "a.md"),
		Filename:    "a.md",
		Size:        100,
		ModifiedAt:  1000,
		ContentHash: "abc",
	}
	if err := db.SaveFile(f); err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}
	d := &project.DocumentRecord{
		FileID:     f.ID,
		Collection: "default",
		Slug:       "a",
		ChunkCount: 2,
		TotalChars: 60,
		Metadata:   "{}",
	}
	if err := db.SaveDocument(d); err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}
	if err := db.SaveChunk(&project.ChunkRecord{
		DocumentID:   d.ID,
		ChunkIndex:   0,
		TextContent:  "# Title",
		TotalChars:   7,
		LineStart:    1,
		LineEnd:      3,
		SectionNum:   1,
		SectionTitle: "# Title",
	}); err != nil {
		t.Fatalf("SaveChunk 1 failed: %v", err)
	}
	c2 := &project.ChunkRecord{
		DocumentID:   d.ID,
		ChunkIndex:   1,
		TextContent:  "# Sub",
		TotalChars:   5,
		LineStart:    4,
		LineEnd:      6,
		SectionNum:   2,
		SectionTitle: "# Sub",
	}
	if err := db.SaveChunk(c2); err != nil {
		t.Fatalf("SaveChunk 2 failed: %v", err)
	}

	results := []*project.SearchResult{
		{
			ID:           c2.ID,
			DocumentID:   d.ID,
			ChunkIndex:   1,
			LineStart:    4,
			LineEnd:      6,
			SectionTitle: "# Sub",
			Slug:         "a",
			Snippet:      "# Sub",
			Rank:         0.85,
		},
	}

	captured := captureStdout(t, func() {
		displayResults(db, root, results, 5, true)
	})
	if !strings.Contains(captured, "# Sub") {
		t.Errorf("expected output to contain 'Sub', got:\n%s", captured)
	}
}

func TestRunSearch_FTSMode(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Init(); err != nil {
		t.Fatal(err)
	}
	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatal(err)
	}
	f := &project.FileRecord{
		FilePath: "docs/a.md", Filename: "a.md",
		Size: 100, ModifiedAt: 1000, ContentHash: "hash-a",
	}
	if err := db.SaveFile(f); err != nil {
		t.Fatal(err)
	}
	d := &project.DocumentRecord{
		FileID: f.ID, Collection: "default", Slug: "a",
		ChunkCount: 1, TotalChars: 10,
	}
	if err := db.SaveDocument(d); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveChunk(&project.ChunkRecord{
		DocumentID: d.ID, ChunkIndex: 0,
		TextContent: "unique search keyword for testing",
		TotalChars:  35,
		LineStart: 1, LineEnd: 1,
		SectionNum: 1, SectionTitle: "# Test",
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	searchMode = "fts"
	searchLimit = 5
	searchCollection = ""
	rrfK = 60
	t.Cleanup(func() { searchMode = "hybrid"; searchLimit = 5; searchCollection = "" })

	err = runSearch("keyword", root)
	if err != nil {
		t.Fatalf("runSearch failed: %v", err)
	}
}

func TestRunSearch_InvalidMode(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Init(); err != nil {
		t.Fatal(err)
	}
	proj.Close()

	searchMode = "invalid"
	t.Cleanup(func() { searchMode = "hybrid" })

	err = runSearch("test", root)
	if err == nil {
		t.Fatal("expected error for invalid search mode")
	}
}

func TestRunSearch_NonExistentRoot(t *testing.T) {
	err := runSearch("test", "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
