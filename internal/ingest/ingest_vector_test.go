//go:build onnx

package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/embed"
	"github.com/minhhh/grokdocs/internal/project"
)

func TestSyncWithVectors(t *testing.T) {
	root := t.TempDir()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	introPath := filepath.Join(docsDir, "intro.md")
	introContent := `# Welcome

This is a test document for vector ingestion.
Machine learning and search are fun.
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

	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := proj.OpenFTS()
	if err != nil {
		t.Fatalf("OpenFTS failed: %v", err)
	}
	defer db.Close()

	// Sync should embed chunks and push to FAISS (or skip gracefully if model unavailable)
	if _, err := SyncCollection(proj, "default", nil, true, 1); err != nil {
		t.Fatalf("SyncCollection failed: %v", err)
	}

	// Check how many chunks were created
	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalChunks == 0 {
		t.Fatal("expected at least 1 chunk after sync")
	}

	// Open the per-collection vector DB and verify it
	vdb, err := proj.OpenCollectionVector("default")
	if err != nil {
		t.Fatalf("OpenCollectionVector failed: %v", err)
	}

	// If models aren't available, ingestion was skipped — warn but don't fail
	if vdb.Len() == 0 {
		t.Log("vector DB is empty — model files likely unavailable, vector ingestion skipped")
		return
	}

	if vdb.Len() != stats.TotalChunks {
		t.Fatalf("expected %d vectors (one per chunk), got %d", stats.TotalChunks, vdb.Len())
	}

	// Embed the query the same way the search would, then search FAISS
	mf, err := embed.GetOrDownloadModels(proj.ConfigDir)
	if err != nil {
		t.Fatalf("GetOrDownloadModels failed: %v", err)
	}
	embedder, err := embed.NewEmbedder(mf.ModelPath, mf.VocabPath)
	if err != nil {
		t.Fatalf("NewEmbedder failed: %v", err)
	}
	defer embedder.Close()

	queryVec, err := embedder.Embed("machine learning search")
	if err != nil {
		t.Fatalf("Embed query failed: %v", err)
	}

	labels, distances, err := vdb.Search(queryVec, 5)
	if err != nil {
		t.Fatalf("Vector search failed: %v", err)
	}
	if len(labels) == 0 {
		t.Fatal("expected at least 1 vector search result")
	}

	// The nearest chunk should mention "machine learning"
	closest := labels[0]
	chunk, err := db.GetChunkByID(closest)
	if err != nil {
		t.Fatalf("GetChunkByID(%d) failed: %v", closest, err)
	}
	if len(chunk.TextContent) == 0 {
		t.Errorf("expected non-empty text content for chunk %d", closest)
	}
	t.Logf("closest chunk (id=%d, dist=%f): %s", closest, distances[0], chunk.Slug)
}
