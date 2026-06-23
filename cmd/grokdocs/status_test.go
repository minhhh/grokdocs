package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
)

func TestRunStatus_HappyPath(t *testing.T) {
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
		FilePath:    "docs/a.md", Filename: "a.md",
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
		TextContent: "# Title", TotalChars: 7,
		LineStart: 1, LineEnd: 3,
		SectionNum: 1, SectionTitle: "# Title",
	}); err != nil {
		t.Fatal(err)
	}

	if err := runStatus(root); err != nil {
		t.Fatalf("runStatus failed: %v", err)
	}
}

func TestRunStatus_EmptyProject(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Init(); err != nil {
		t.Fatal(err)
	}

	if err := runStatus(root); err != nil {
		t.Fatalf("runStatus on empty project failed: %v", err)
	}
}

func TestRunStatus_NonExistentRoot(t *testing.T) {
	err := runStatus("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func TestRunStatus_RootIsFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "blocker.txt")
	if err := os.WriteFile(filePath, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	err := runStatus(filePath)
	if err == nil {
		t.Fatal("expected error when root is a file")
	}
}

func TestRunStatus_InvalidConfig(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj.ConfigDir, project.ConfigFileName), []byte("invalid: [[["), 0644); err != nil {
		t.Fatal(err)
	}

	err = runStatus(root)
	if err == nil {
		t.Fatal("expected error for invalid config.yaml")
	}
}

func TestRunStatus_OrphanWarnings(t *testing.T) {
	root := t.TempDir()
	proj, err := project.NewProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Init(); err != nil {
		t.Fatal(err)
	}
	proj.Config = &config.Config{
		Collections: map[string]config.CollectionConfig{
			"active": {Path: "docs"},
		},
	}
	cfgPath := filepath.Join(proj.ConfigDir, project.ConfigFileName)
	if err := proj.Config.SaveToFile(cfgPath); err != nil {
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
		FilePath: "docs/orphan.md", Filename: "orphan.md",
		Size: 100, ModifiedAt: 1000, ContentHash: "hash-o",
	}
	if err := db.SaveFile(f); err != nil {
		t.Fatal(err)
	}
	d := &project.DocumentRecord{
		FileID: f.ID, Collection: "orphan-collection", Slug: "o",
		ChunkCount: 1, TotalChars: 10,
	}
	if err := db.SaveDocument(d); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := runStatus(root); err != nil {
		t.Fatalf("runStatus with orphans failed: %v", err)
	}
}
