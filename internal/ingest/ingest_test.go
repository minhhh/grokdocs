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
					Parsers: map[string]string{".md": "markdown", ".markdown": "markdown"},
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

	p, ok := GetParser("markdown")
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

	// Mock project
	proj := &project.Project{
		RootPath:  root,
		ConfigDir: filepath.Join(root, ".grokdocs"),
		Config: &config.Config{
			Collections: map[string]config.CollectionConfig{
				"default": {
					Path:    "docs",
					Parsers: map[string]string{".md": "markdown", ".markdown": "markdown"},
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

func TestFileFilterFilesOnly(t *testing.T) {
	f := newFileFilter([]string{"README.md", "index.html"}, nil, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"docs/README.md", true},
		{"index.html", true},
		{"sub/README.md", true},
		{"docs/intro.md", false},
		{"style.css", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterIncludeOnly(t *testing.T) {
	f := newFileFilter(nil, []string{"*.md", "*.go"}, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", true},
		{"src/util.go", true},
		{"style.css", false},
		{"README.txt", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterExcludeOnly(t *testing.T) {
	// No include => all files pass unless excluded
	f := newFileFilter(nil, nil, []string{"*.txt", "*.log"})
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", true},
		{"notes.txt", false},
		{"error.log", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterFilesAndInclude(t *testing.T) {
	// files and include are merged
	f := newFileFilter([]string{"README.md", "config.yaml"}, []string{"*.go"}, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"config.yaml", true},
		{"src/main.go", true},
		{"docs/intro.md", false},
		{"style.css", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterFilesIgnoresExclude(t *testing.T) {
	// When files is set, exclude is ignored
	f := newFileFilter([]string{"notes.txt"}, []string{"*.go"}, []string{"*.txt", "*.go"})
	tests := []struct {
		path string
		want bool
	}{
		{"notes.txt", true},  // would be excluded but files is set, so ignore exclude
		{"main.go", true},    // matches include *.go, exclude ignored
		{"README.md", false}, // not in files or include
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterIncludeAndExclude(t *testing.T) {
	f := newFileFilter(nil, []string{"*.md", "*.go"}, []string{"*_test.go", "*_old.md"})
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", true},
		{"main_test.go", false},  // excluded
		{"docs/changes_old.md", false}, // excluded
		{"style.css", false},     // not in include
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterEmpty(t *testing.T) {
	f := newFileFilter(nil, nil, nil)
	if !f.Match("any/file.go") {
		t.Error("expected empty filter to match everything")
	}
	if !f.Match("another/file.md") {
		t.Error("expected empty filter to match everything")
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

	proj := &project.Project{
		RootPath:  root,
		ConfigDir: filepath.Join(root, ".grokdocs"),
		Config: &config.Config{
			Collections: map[string]config.CollectionConfig{
				"default": {
					Path:    "docs",
					Parsers: map[string]string{".md": "markdown"},
					Files:   []string{"README.md"},
					Include: []string{"*.md"},
					Exclude: []string{"api.md"},
				},
			},
		},
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	if err := SyncCollection(proj, "default"); err != nil {
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
	pName, ok := ResolveParserName(cfg, "default", "path/to/hello.md")
	if !ok || pName != "hello-parser" {
		t.Errorf("expected hello-parser, got %s (ok=%t)", pName, ok)
	}

	// 2. Complex extension matches next
	pName, ok = ResolveParserName(cfg, "default", "path/to/doc.rfc.md")
	if !ok || pName != "rfc-parser" {
		t.Errorf("expected rfc-parser, got %s (ok=%t)", pName, ok)
	}

	// 3. Simple extension matches next
	pName, ok = ResolveParserName(cfg, "default", "path/to/other.md")
	if !ok || pName != "markdown" {
		t.Errorf("expected markdown, got %s (ok=%t)", pName, ok)
	}

	// 4. Wildcard/glob pattern matches
	pName, ok = ResolveParserName(cfg, "default", "path/to/script.js")
	if !ok || pName != "javascript-parser" {
		t.Errorf("expected javascript-parser, got %s (ok=%t)", pName, ok)
	}

	// 5. Unmatched file
	_, ok = ResolveParserName(cfg, "default", "path/to/style.css")
	if ok {
		t.Errorf("expected no match for style.css")
	}
}
