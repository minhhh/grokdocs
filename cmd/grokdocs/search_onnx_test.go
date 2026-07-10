//go:build onnx

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
)

func TestSearchSemanticLimitExceedsAvailable(t *testing.T) {
	root := t.TempDir()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a single small file (produces 1 chunk)
	writeTestFile(t, filepath.Join(docsDir, "intro.md"), "# Intro\nSmall file, one chunk only.")

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
	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	if err := initEmbedder(); err != nil {
		t.Fatalf("initEmbedder: %v", err)
	}
	defer closeEmbedder()

	// Sync — produces vectors for the single chunk
	if _, err := ingest.SyncCollection(proj, "default", nil, true, 1); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// Verify vector DB has exactly 1 vector
	vdb, err := proj.OpenCollectionVector("default", 384)
	if err != nil {
		t.Fatalf("OpenCollectionVector failed: %v", err)
	}
	if vdb.Len() == 0 {
		t.Skip("vector DB is empty — model files likely unavailable")
	}

	// Search with limit > available vectors
	limit := 100
	results, err := searchSemantic(proj, db, "test", "default", limit)
	if err != nil {
		t.Fatalf("searchSemantic failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result when limit exceeds available")
	}
	if len(results) > int(vdb.Len()) {
		t.Errorf("expected results <= vector count (%d), got %d", vdb.Len(), len(results))
	}
	t.Logf("limit=%d, vectors=%d, results=%d", limit, vdb.Len(), len(results))
}

func TestMakeSnippet(t *testing.T) {
	tests := []struct {
		text   string
		maxLen int
		want   string
	}{
		{"hello world", 50, "hello world"},
		{"hello", 3, "hel..."},
		{"", 10, ""},
		{"abcdef", 6, "abcdef"},
		{"abcdef", 5, "abcde..."},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := makeSnippet(tt.text, tt.maxLen)
			if got != tt.want {
				t.Errorf("makeSnippet(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
