package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/minhhh/grokdocs/internal/config"
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
	proj := &project.Project{
		RootPath: root,
		Config: &config.Config{
			Collections: map[string]config.CollectionConfig{
				"default": {
					Path:    "docs",
					Parsers: []string{"markdown"},
				},
			},
		},
	}

	// Mock DB in memory to avoid actual SQLite writing if not needed, but SyncCollection calls db.OpenFTS.
	// We can use a real FTS database inside proj.ConfigDir!
	proj.ConfigDir = filepath.Join(root, ".grokdocs")
	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	if err := SyncCollection(proj, "default"); err != nil {
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

	chunks, err := chunkContent(docContent, "test.md", 100)
	if err != nil {
		t.Fatalf("chunkContent failed: %v", err)
	}

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

	// Mock project
	proj := &project.Project{
		RootPath:  root,
		ConfigDir: filepath.Join(root, ".grokdocs"),
		Config: &config.Config{
			Collections: map[string]config.CollectionConfig{
				"default": {
					Path:    "docs",
					Parsers: []string{"markdown"},
				},
			},
		},
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	// Initial sync
	if err := SyncCollection(proj, "default"); err != nil {
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
	futureTime := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(introPath, futureTime, futureTime); err != nil {
		t.Fatal(err)
	}

	if err := SyncCollection(proj, "default"); err != nil {
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

	if err := SyncCollection(proj, "default"); err != nil {
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
