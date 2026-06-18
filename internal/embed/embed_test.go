//go:build onnx

package embed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOrDownloadModels(t *testing.T) {
	cacheDir := t.TempDir()

	modelFiles, err := GetOrDownloadModels(cacheDir)
	if err != nil {
		t.Skipf("GetOrDownloadModels failed (SKIP - requires internet): %v", err)
	}

	if _, err := os.Stat(modelFiles.ModelPath); os.IsNotExist(err) {
		t.Errorf("model file should exist at %s", modelFiles.ModelPath)
	}
	if _, err := os.Stat(modelFiles.VocabPath); os.IsNotExist(err) {
		t.Errorf("vocab file should exist at %s", modelFiles.VocabPath)
	}

	// Should return cached files on second call
	cached, err := GetOrDownloadModels(cacheDir)
	if err != nil {
		t.Fatalf("second GetOrDownloadModels failed: %v", err)
	}
	if cached.ModelPath != modelFiles.ModelPath {
		t.Error("expected same model path on cached call")
	}
}

func TestONNXEmbeddings(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "grokdocs-test-embed")
	defer os.RemoveAll(cacheDir)

	modelFiles, err := GetOrDownloadModels(cacheDir)
	if err != nil {
		t.Skipf("GetOrDownloadModels failed (SKIP - requires internet): %v", err)
	}

	if _, err := os.Stat(modelFiles.ModelPath); err != nil {
		t.Skipf("model file not found after download (SKIP): %v", err)
	}
	if _, err := os.Stat(modelFiles.VocabPath); err != nil {
		t.Skipf("vocab file not found after download (SKIP): %v", err)
	}

	embedder, err := NewEmbedder(modelFiles.ModelPath, modelFiles.VocabPath)
	if err != nil {
		t.Fatalf("NewEmbedder failed: %v", err)
	}
	defer embedder.Close()

	vec, err := embedder.Embed("This is a test sentence.")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != embedder.Dim() {
		t.Fatalf("expected embedding dimension %d, got %d", embedder.Dim(), len(vec))
	}

	if embedder.Dim() == 0 {
		t.Fatal("expected non-zero embedding dimension")
	}

	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	if sum < 0.99 || sum > 1.01 {
		t.Fatalf("expected L2-normalized vector (norm ~1.0), got norm=%f", sum)
	}
}
