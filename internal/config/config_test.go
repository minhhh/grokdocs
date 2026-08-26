package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	col, ok := cfg.Collections[DefaultCollectionName]
	if !ok {
		t.Fatal("expected default collection")
	}
	if col.Path != DefaultCollectionPath {
		t.Errorf("expected path %q, got %q", DefaultCollectionPath, col.Path)
	}
}

func TestLoadConfig(t *testing.T) {
	yaml := `collections:
  custom:
    path: docs
`
	r := strings.NewReader(yaml)
	cfg, err := LoadConfig(r)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	col, ok := cfg.Collections["custom"]
	if !ok {
		t.Fatal("expected custom collection")
	}
	if col.Path != "docs" {
		t.Errorf("expected path docs, got %q", col.Path)
	}
}

func TestLoadConfig_DefaultsEmptyPath(t *testing.T) {
	yaml := `collections:
  custom:
    include:
      - "*.md"
`
	cfg, err := LoadConfig(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	col, ok := cfg.Collections["custom"]
	if !ok {
		t.Fatal("expected custom collection")
	}
	if col.Path != DefaultCollectionPath {
		t.Errorf("expected path %q, got %q", DefaultCollectionPath, col.Path)
	}
}

func TestLoadConfig_DefaultCollectionPathForced(t *testing.T) {
	yaml := `collections:
  default:
    path: docs
`
	r := strings.NewReader(yaml)
	cfg, err := LoadConfig(r)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	col, ok := cfg.Collections["default"]
	if !ok {
		t.Fatal("expected default collection")
	}
	if col.Path != DefaultCollectionPath {
		t.Errorf("expected default collection path to be %q, got %q", DefaultCollectionPath, col.Path)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	_, err := LoadConfig(strings.NewReader("{{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestSaveConfig(t *testing.T) {
	cfg := DefaultConfig()
	var buf bytes.Buffer
	if err := cfg.SaveConfig(&buf); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	if !strings.Contains(buf.String(), "default") {
		t.Errorf("expected output to contain 'default', got %q", buf.String())
	}
	if strings.Contains(buf.String(), "path:") {
		t.Errorf("default collection should not have path field, got %q", buf.String())
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collections["docs"] = CollectionConfig{Path: "docs", Include: []string{"*.md"}}

	var buf bytes.Buffer
	if err := cfg.SaveConfig(&buf); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(&buf)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(loaded.Collections) != 2 {
		t.Errorf("expected 2 collections, got %d", len(loaded.Collections))
	}
	col, ok := loaded.Collections["docs"]
	if !ok {
		t.Fatal("expected docs collection")
	}
	if col.Path != "docs" {
		t.Errorf("expected path docs, got %q", col.Path)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "collections:\n  test:\n    path: data\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if _, ok := cfg.Collections["test"]; !ok {
		t.Error("expected test collection")
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSaveToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := DefaultConfig()
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "default") {
		t.Errorf("expected file to contain 'default', got %q", string(data))
	}
}

func TestSaveToFile_ErrorPath(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.SaveToFile("/nonexistent/dir/config.yaml")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
