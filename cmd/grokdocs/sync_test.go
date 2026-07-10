package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
)

func TestRunSyncPreRun_MutuallyExclusive(t *testing.T) {
	syncAll = true
	syncCollection = "default"
	t.Cleanup(func() { syncAll = false; syncCollection = "" })

	err := runSyncPreRun()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
}

func TestRunSyncPreRun_Valid(t *testing.T) {
	syncAll = false
	syncCollection = ""
	t.Cleanup(func() { syncAll = false; syncCollection = "" })

	err := runSyncPreRun()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSync_DefaultCollection(t *testing.T) {
	root := t.TempDir()
	createSyncableFile(t, root)

	// Reset flags to defaults
	syncCollection = ""
	syncAll = false
	syncConcurrency = 1
	syncPrune = true
	t.Cleanup(func() { syncCollection = ""; syncAll = false })

	err := runSync(root)
	if err != nil {
		t.Fatalf("runSync failed: %v", err)
	}
}

func TestRunSync_NamedCollection(t *testing.T) {
	root := t.TempDir()
	createSyncableFile(t, root)

	syncCollection = "default"
	syncAll = false
	syncConcurrency = 1
	syncPrune = true
	t.Cleanup(func() { syncCollection = "" })

	err := runSync(root)
	if err != nil {
		t.Fatalf("runSync with collection failed: %v", err)
	}
}

func TestRunSync_AllCollections(t *testing.T) {
	root := t.TempDir()
	createSyncableFile(t, root)

	syncCollection = ""
	syncAll = true
	syncConcurrency = 1
	syncPrune = true
	t.Cleanup(func() { syncAll = false })

	err := runSync(root)
	if err != nil {
		t.Fatalf("runSync --all failed: %v", err)
	}
}

func TestRunSync_InvalidConcurrency(t *testing.T) {
	root := t.TempDir()

	syncConcurrency = 0
	syncCollection = ""
	syncAll = false
	syncPrune = false
	t.Cleanup(func() { syncConcurrency = 1 })

	err := runSync(root)
	if err == nil {
		t.Fatal("expected error for concurrency < 1")
	}
}

func TestRunSync_NonExistentRoot(t *testing.T) {
	syncCollection = ""
	syncAll = false
	syncConcurrency = 1

	err := runSync("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func createSyncableFile(t *testing.T, root string) {
	t.Helper()
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
	cfgPath := filepath.Join(proj.ConfigDir, project.ConfigFileName)
	if err := os.MkdirAll(proj.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := proj.Config.SaveToFile(cfgPath); err != nil {
		t.Fatal(err)
	}
	proj.Close()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "intro.md"), []byte("# Hello\nWorld."), 0644); err != nil {
		t.Fatal(err)
	}
}
