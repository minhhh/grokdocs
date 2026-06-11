package ingest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/parser"
	"github.com/minhhh/grokdocs/internal/project"
)

func TestFileWalking(t *testing.T) {
	// Create temp directory representing project root
	root := t.TempDir()

	// Create directories to ignore and scan
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grokdocs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create dummy files
	filesToCreate := []string{
		filepath.Join(root, ".git", "config"),
		filepath.Join(root, ".grokdocs", "config.yaml"),
		filepath.Join(root, "docs", "intro.md"),
		filepath.Join(root, "docs", "subdir", "guide.markdown"),
		filepath.Join(root, "docs", "extra.txt"), // unsupported extension
	}

	for _, path := range filesToCreate {
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Mock project
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {
				Path:    "docs",
				Parsers: map[string]string{".md": "markdown", ".markdown": "markdown"},
			},
		},
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// Query DB to see which files were synced
	file1, err := db.GetFile("docs/intro.md")
	if err != nil {
		t.Errorf("expected docs/intro.md to be synced, got error: %v", err)
	} else if file1.Filename != "intro.md" {
		t.Errorf("expected filename intro.md, got %q", file1.Filename)
	}

	file2, err := db.GetFile("docs/subdir/guide.markdown")
	if err != nil {
		t.Errorf("expected docs/subdir/guide.markdown to be synced, got error: %v", err)
	} else if file2.Filename != "guide.markdown" {
		t.Errorf("expected filename guide.markdown, got %q", file2.Filename)
	}

	// Verify ignored files are not in DB
	if _, err := db.GetFile("docs/extra.txt"); err == nil {
		t.Error("docs/extra.txt should not be synced")
	}
	if _, err := db.GetFile(".git/config"); err == nil {
		t.Error(".git/config should not be synced")
	}
	if _, err := db.GetFile(".grokdocs/config.yaml"); err == nil {
		t.Error(".grokdocs/config.yaml should not be synced")
	}
}

func TestMarkdownChunking(t *testing.T) {
	docContent := `# Ingestion Setup

This is the introduction section.
It spans multiple lines.

## Technical Configuration

This is the configuration section.
Here is some config text.
`

	p, ok := parser.GetParser("markdown")
	if !ok {
		t.Fatal("markdown parser not registered")
	}
	parsed, err := p.Parse("test.md", docContent, 100)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	chunks := parsed.Chunks

	if len(chunks) == 0 {
		t.Fatalf("expected some chunks to be generated, got 0")
	}

	// Verify sequential section increment and line mapping
	for _, chunk := range chunks {
		if chunk.TextContent == "" {
			t.Error("expected chunk text content to be populated")
		}
		if chunk.LineStart <= 0 || chunk.LineEnd <= 0 || chunk.LineStart > chunk.LineEnd {
			t.Errorf("invalid line range: %d-%d", chunk.LineStart, chunk.LineEnd)
		}

		// Ensure text lines match original content lines
		origLines := strings.Split(docContent, "\n")
		reconstructed := strings.Join(origLines[chunk.LineStart-1:chunk.LineEnd], "\n")
		// The AST chunk might trim some leading/trailing spaces/newlines, but it should be a substring
		if !strings.Contains(reconstructed, strings.TrimSpace(chunk.TextContent)) {
			t.Errorf("reconstructed chunk text is not in original lines: %q (lines %d-%d: %q)", chunk.TextContent, chunk.LineStart, chunk.LineEnd, reconstructed)
		}

		// Check metadata JSON structure
		var meta map[string]any
		if err := json.Unmarshal([]byte(chunk.Metadata), &meta); err != nil {
			t.Errorf("failed to unmarshal chunk metadata: %v", err)
		}

		if meta["filename"] != "test.md" {
			t.Errorf("expected filename test.md, got %v", meta["filename"])
		}

		secNum, ok := meta["section_num"].(float64)
		if !ok {
			t.Error("section_num must be present in metadata")
		}

		secTitle, ok := meta["section_title"].(string)
		if !ok {
			t.Error("section_title must be present in metadata")
		}

		// Verify section mapping matching
		if chunk.LineStart < 6 {
			// Belongs to section 1
			if int(secNum) != 1 || secTitle != "Ingestion Setup" {
				t.Errorf("expected section 1 'Ingestion Setup', got %d %q for lines %d-%d", int(secNum), secTitle, chunk.LineStart, chunk.LineEnd)
			}
		} else {
			// Belongs to section 2
			if int(secNum) != 2 || secTitle != "Technical Configuration" {
				t.Errorf("expected section 2 'Technical Configuration', got %d %q for lines %d-%d", int(secNum), secTitle, chunk.LineStart, chunk.LineEnd)
			}
		}
	}
}

func TestSyncAndSearchFTS(t *testing.T) {
	root := t.TempDir()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grokdocs"), 0755); err != nil {
		t.Fatal(err)
	}

	introPath := filepath.Join(docsDir, "intro.md")
	introContent := `# Welcome to Grokdocs

This is the introductory documentation page for grokdocs.
We hope you enjoy searching locally and offline.
`
	if err := os.WriteFile(introPath, []byte(introContent), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {
				Path:    "docs",
				Parsers: map[string]string{".md": "markdown", ".markdown": "markdown"},
			},
		},
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	// Initial sync
	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// Search verification
	results, err := db.SearchFTS("offline", "default", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 search match, got %d", len(results))
	}
	if !strings.Contains(results[0].Chunk.TextContent, "offline") {
		t.Errorf("expected chunk to contain 'offline', got: %q", results[0].Chunk.TextContent)
	}

	// Verification of file update and incremental sync
	updatedIntroContent := `# Welcome to Grokdocs

This is the updated introductory page.
We have replaced the word and added remote.
`
	if err := os.WriteFile(introPath, []byte(updatedIntroContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Artificially advance modification time to ensure change detection registers the update
	futureTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(introPath, futureTime, futureTime); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// Search for old keyword should return 0 results
	oldResults, err := db.SearchFTS("offline", "default", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(oldResults) != 0 {
		t.Errorf("expected 0 matches for 'offline' after update, got %d", len(oldResults))
	}

	// Search for new keyword should succeed
	newResults, err := db.SearchFTS("remote", "default", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(newResults) != 1 {
		t.Errorf("expected 1 match for 'remote' after update, got %d", len(newResults))
	}

	// Verification of file deletion
	if err := os.Remove(introPath); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// Search for any keyword should return 0 results
	deletedResults, err := db.SearchFTS("remote", "default", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(deletedResults) != 0 {
		t.Errorf("expected 0 matches after document deletion, got %d", len(deletedResults))
	}
}

func TestSyncCollectionWithFileFiltering(t *testing.T) {
	root := t.TempDir()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grokdocs"), 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		filepath.Join(docsDir, "intro.md"):    "# Intro\nWelcome.",
		filepath.Join(docsDir, "guide.md"):    "# Guide\nFollow along.",
		filepath.Join(docsDir, "api.md"):      "# API\nReference docs.",
		filepath.Join(docsDir, "notes.txt"):   "Just some notes.",
		filepath.Join(docsDir, "README.md"):   "# Readme\nTop-level.",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {
				Path:    "docs",
				Parsers: map[string]string{".md": "markdown"},
				Files:   []string{"README.md"},
				Include: []string{"*.md"},
				Exclude: []string{"api.md"},
			},
		},
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// files is set, so files+include are merged, exclude is ignored
	// Expected: README.md (in files), intro.md, guide.md, api.md (all *.md via include)
	// notes.txt excluded (not .md)
	for name, shouldExist := range map[string]bool{
		"docs/README.md": true,
		"docs/intro.md":  true,
		"docs/guide.md":  true,
		"docs/api.md":    true,  // exclude is ignored because files is set
		"docs/notes.txt": false,
	} {
		_, err := db.GetFile(name)
		if shouldExist && err != nil {
			t.Errorf("expected %s to be synced, got error: %v", name, err)
		}
		if !shouldExist && err == nil {
			t.Errorf("expected %s to NOT be synced", name)
		}
	}
}

func TestParserResolutionAndPrecedence(t *testing.T) {
	cfg := &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {
				Path: ".",
				Parsers: map[string]string{
					"hello.md": "hello-parser",
					".md":      "markdown",
					".rfc.md":  "rfc-parser",
					"*.js":     "javascript-parser",
				},
			},
		},
	}

	// 1. Specific filename takes highest priority
	pName, ok := parser.ResolveParserName(cfg, "default", "path/to/hello.md")
	if !ok || pName != "hello-parser" {
		t.Errorf("expected hello-parser, got %s (ok=%t)", pName, ok)
	}

	// 2. Complex extension matches next
	pName, ok = parser.ResolveParserName(cfg, "default", "path/to/doc.rfc.md")
	if !ok || pName != "rfc-parser" {
		t.Errorf("expected rfc-parser, got %s (ok=%t)", pName, ok)
	}

	// 3. Simple extension matches next
	pName, ok = parser.ResolveParserName(cfg, "default", "path/to/other.md")
	if !ok || pName != "markdown" {
		t.Errorf("expected markdown, got %s (ok=%t)", pName, ok)
	}

	// 4. Wildcard/glob pattern matches
	pName, ok = parser.ResolveParserName(cfg, "default", "path/to/script.js")
	if !ok || pName != "javascript-parser" {
		t.Errorf("expected javascript-parser, got %s (ok=%t)", pName, ok)
	}

	// 5. Unmatched file falls back to default parser mapping
	pName, ok = parser.ResolveParserName(cfg, "default", "path/to/style.css")
	if !ok || pName != "chunkx" {
		t.Errorf("expected chunkx, got %s (ok=%t)", pName, ok)
	}
}

func TestSyncSkipsUnchangedFile(t *testing.T) {
	root := t.TempDir()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grokdocs"), 0755); err != nil {
		t.Fatal(err)
	}

	introPath := filepath.Join(docsDir, "intro.md")
	introContent := "# Welcome\nContent here."
	if err := os.WriteFile(introPath, []byte(introContent), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {
				Path:    "docs",
				Parsers: map[string]string{".md": "markdown"},
			},
		},
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	// First sync — ingests the file
	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("first SyncCollection failed: %v", err)
	}

	fileBefore, err := db.GetFile("docs/intro.md")
	if err != nil {
		t.Fatalf("expected file after first sync: %v", err)
	}

	// Second sync — mtime matches, should hit mtime-skip branch
	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("second SyncCollection failed: %v", err)
	}

	fileAfter, err := db.GetFile("docs/intro.md")
	if err != nil {
		t.Fatalf("expected file after second sync: %v", err)
	}

	// mtime must be unchanged (skip path doesn't touch the record)
	if fileBefore.ModifiedAt != fileAfter.ModifiedAt {
		t.Errorf("mtime changed after skip-sync: before=%d after=%d", fileBefore.ModifiedAt, fileAfter.ModifiedAt)
	}

	// Exactly one file record
	var count int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM files").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 file record, got %d", count)
	}
}

func TestSyncCollectionResult(t *testing.T) {
	root := t.TempDir()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grokdocs"), 0755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, root, "docs/intro.md", "# Intro\nSame content")
	writeFile(t, root, "docs/guide.md", "# Guide\nOld content")
	writeFile(t, root, "docs/old-moved.md", "# Moved\nContent that will move")
	writeFile(t, root, "docs/config.md", "# Config\nWill be deleted")

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {
				Path:    "docs",
				Parsers: map[string]string{".md": "markdown"},
			},
		},
	}

	// First sync — ingest all 4 files
	if _, err := SyncCollection(proj, "default", nil); err != nil {
		t.Fatalf("first SyncCollection failed: %v", err)
	}

	// Modify guide.md
	if err := os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte("# Guide\nUpdated content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Advance mtime for the modified file to ensure change detection
	futureTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(docsDir, "guide.md"), futureTime, futureTime); err != nil {
		t.Fatal(err)
	}

	// Delete old-moved.md and config.md
	if err := os.Remove(filepath.Join(docsDir, "old-moved.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(docsDir, "config.md")); err != nil {
		t.Fatal(err)
	}

	// Create moved.md with identical content to old-moved.md (same hash — counted as moved, not added)
	writeFile(t, root, "docs/moved.md", "# Moved\nContent that will move")

	// Create newfile.md with truly new content (counted as added)
	writeFile(t, root, "docs/newfile.md", "# New file\nBrand new content")

	// intro.md is untouched — unchanged

	// Second sync
	result, err := SyncCollection(proj, "default", nil)
	if err != nil {
		t.Fatalf("second SyncCollection failed: %v", err)
	}

	if result.Unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", result.Unchanged)
	}
	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
	if result.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", result.Modified)
	}
	if result.Moved != 1 {
		t.Errorf("expected 1 moved, got %d", result.Moved)
	}
	if result.Deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", result.Deleted)
	}
}

func TestSyncCollectionCollectionNotFound(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {Path: "docs", Parsers: map[string]string{".md": "markdown"}},
		},
	}

	_, err = SyncCollection(proj, "nonexistent-collection", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent collection")
	}
	if err.Error() != "collection not found" {
		t.Errorf("expected 'collection not found', got %v", err)
	}
}

func TestSyncCollectionPathIsFile(t *testing.T) {
	root := t.TempDir()

	filePath := filepath.Join(root, "blocker.txt")
	if err := os.WriteFile(filePath, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {Path: "blocker.txt", Parsers: map[string]string{".txt": "chunkx"}},
		},
	}

	_, err = SyncCollection(proj, "default", nil)
	if err == nil {
		t.Fatal("expected error when collection path is a file")
	}
	if err.Error() != "collection path is not a directory" {
		t.Errorf("expected 'collection path is not a directory', got %v", err)
	}
}

func TestSyncCollectionPathDoesNotExist(t *testing.T) {
	root := t.TempDir()

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"default": {Path: "nonexistent-dir", Parsers: map[string]string{".md": "markdown"}},
		},
	}

	_, err = SyncCollection(proj, "default", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent collection path")
	}
}

// helpers

func collectWalkResultsWithFilter(t *testing.T, root string, filter *fileFilter) []WalkResult {
	t.Helper()
	var results []WalkResult
	for r := range walkFiles(context.Background(), root, filter) {
		results = append(results, r)
	}
	return results
}

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
