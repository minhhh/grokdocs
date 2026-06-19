package project

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestFAISSIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "test.index")

	vdb, err := OpenVectorDatabase(indexPath, 4)
	if err != nil {
		t.Fatalf("OpenVectorDatabase failed: %v", err)
	}
	defer vdb.Close()

	expectedDim := 4
	if vdb.Dim() != expectedDim {
		t.Errorf("expected dim %d, got %d", expectedDim, vdb.Dim())
	}

	ids := []int64{10, 20, 30}
	vectors := []float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
	}

	if err := vdb.AddVectors(ids, vectors); err != nil {
		t.Fatalf("AddVectors failed: %v", err)
	}

	if vdb.Len() != 3 {
		t.Errorf("expected 3 vectors, got %d", vdb.Len())
	}

	query := []float32{1, 0, 0, 0}
	resultIDs, distances, err := vdb.Search(query, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(resultIDs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resultIDs))
	}
	if resultIDs[0] != 10 {
		t.Errorf("expected nearest neighbor ID 10, got %d", resultIDs[0])
	}
	if math.Abs(float64(distances[0])) > 0.001 {
		t.Errorf("expected distance ~0 for identical vector, got %f", distances[0])
	}

	if err := vdb.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatalf("index file should exist after save: %v", err)
	}

	vdb.Close()

	vdb2, err := OpenVectorDatabase(indexPath, 4)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer vdb2.Close()

	if vdb2.Len() != 3 {
		t.Errorf("expected 3 vectors after reload, got %d", vdb2.Len())
	}

	resultIDs2, distances2, err := vdb2.Search(query, 2)
	if err != nil {
		t.Fatalf("Search after reload failed: %v", err)
	}
	if len(resultIDs2) != 2 {
		t.Fatalf("expected 2 results after reload, got %d", len(resultIDs2))
	}
	if resultIDs2[0] != 10 {
		t.Errorf("expected nearest neighbor ID 10 after reload, got %d", resultIDs2[0])
	}
	if math.Abs(float64(distances2[0])) > 0.001 {
		t.Errorf("expected distance ~0 after reload, got %f", distances2[0])
	}
}

func TestFAISSIndexEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "empty.index")

	vdb, err := OpenVectorDatabase(indexPath, 4)
	if err != nil {
		t.Fatalf("OpenVectorDatabase failed: %v", err)
	}

	if vdb.Len() != 0 {
		t.Errorf("expected 0 vectors in new index, got %d", vdb.Len())
	}

	if err := vdb.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	vdb.Close()

	vdb2, err := OpenVectorDatabase(indexPath, 4)
	if err != nil {
		t.Fatalf("reopen empty index failed: %v", err)
	}
	defer vdb2.Close()

	if vdb2.Len() != 0 {
		t.Errorf("expected 0 vectors after reload, got %d", vdb2.Len())
	}
}

func TestFAISSRemoveIDs(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "remove.index")

	vdb, err := OpenVectorDatabase(indexPath, 4)
	if err != nil {
		t.Fatalf("OpenVectorDatabase failed: %v", err)
	}
	defer vdb.Close()

	ids := []int64{10, 20, 30, 40}
	vectors := []float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	if err := vdb.AddVectors(ids, vectors); err != nil {
		t.Fatalf("AddVectors failed: %v", err)
	}
	if vdb.Len() != 4 {
		t.Fatalf("expected 4 vectors, got %d", vdb.Len())
	}

	// Remove IDs 20 and 30
	if err := vdb.RemoveIDs([]int64{20, 30}); err != nil {
		t.Fatalf("RemoveIDs failed: %v", err)
	}
	if vdb.Len() != 2 {
		t.Fatalf("expected 2 vectors after removal, got %d", vdb.Len())
	}

	// Search should only return remaining IDs
	query := []float32{1, 0, 0, 0}
	resultIDs, _, err := vdb.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	for _, id := range resultIDs {
		if id == 20 || id == 30 {
			t.Errorf("removed ID %d should not appear in search results", id)
		}
	}

	// Removing non-existent IDs should be a no-op
	if err := vdb.RemoveIDs([]int64{999}); err != nil {
		t.Fatalf("RemoveIDs for non-existent IDs failed: %v", err)
	}
	if vdb.Len() != 2 {
		t.Fatalf("expected still 2 vectors after no-op, got %d", vdb.Len())
	}
}

func TestFAISSIndexWithDim(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "custom.index")

	vdb, err := OpenVectorDatabase(indexPath, 8)
	if err != nil {
		t.Fatalf("OpenVectorDatabase failed: %v", err)
	}
	defer vdb.Close()

	if vdb.Dim() != 8 {
		t.Errorf("expected dim 8, got %d", vdb.Dim())
	}
}
