package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := computeSHA256(filePath)
	if err != nil {
		t.Fatalf("computeSHA256 failed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	// SHA256 of "hello world" is known
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("expected hash %q, got %q", expected, hash)
	}
}

func TestComputeSHA256_FileNotFound(t *testing.T) {
	_, err := computeSHA256("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
