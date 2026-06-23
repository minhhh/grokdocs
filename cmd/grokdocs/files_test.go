package main

import (
	"os"
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
)

func TestListCollectionNames(t *testing.T) {
	root := t.TempDir()

	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Collections = map[string]config.CollectionConfig{
		"alpha": {Path: "docs"},
		"beta":  {Path: "src"},
	}
	proj.Config = cfg
	if err := cfg.SaveToFile(proj.ConfigDir + "/config.yaml"); err != nil {
		t.Fatal(err)
	}

	projectPath = root
	t.Cleanup(func() { projectPath = "" })

	names := listCollectionNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 collection names, got %d", len(names))
	}
	m := make(map[string]bool)
	for _, n := range names {
		m[n] = true
	}
	if !m["alpha"] {
		t.Error("expected alpha in results")
	}
	if !m["beta"] {
		t.Error("expected beta in results")
	}
}

func TestRunFiles_DefaultCollection(t *testing.T) {
	root := setupTestProject(t)

	err := runFiles(root)
	if err != nil {
		t.Fatalf("runFiles failed: %v", err)
	}
}

func TestRunFiles_CollectionFilter(t *testing.T) {
	root := setupTestProject(t)
	filesCollection = "default"
	t.Cleanup(func() { filesCollection = "" })

	err := runFiles(root)
	if err != nil {
		t.Fatalf("runFiles with collection failed: %v", err)
	}
}

func TestRunFiles_AllCollections(t *testing.T) {
	root := setupTestProject(t)
	filesAll = true
	t.Cleanup(func() { filesAll = false })

	err := runFiles(root)
	if err != nil {
		t.Fatalf("runFiles --all failed: %v", err)
	}
}

func TestRunFiles_WithLimit(t *testing.T) {
	root := setupTestProject(t)
	filesLimit = 1
	filesOffset = 0
	t.Cleanup(func() { filesLimit = 0; filesOffset = 0 })

	err := runFiles(root)
	if err != nil {
		t.Fatalf("runFiles with limit failed: %v", err)
	}
}

func TestRunFiles_NonExistentRoot(t *testing.T) {
	err := runFiles("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func setupTestProject(t *testing.T) string {
	t.Helper()
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
	defer db.Close()

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
	return root
}
