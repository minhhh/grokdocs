package main

import (
	"testing"
)

func TestRunEmbed_NonExistentRoot(t *testing.T) {
	err := runEmbed("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func TestRunEmbed_InvalidConcurrency(t *testing.T) {
	root := setupTestProject(t)
	embedConcurrency = 0
	t.Cleanup(func() { embedConcurrency = 1 })

	err := runEmbed(root)
	if err == nil {
		t.Fatal("expected error for concurrency < 1")
	}
}

func TestRunEmbed_OnnxUnavailable(t *testing.T) {
	root := setupTestProject(t)
	embedConcurrency = 1
	embedAll = false
	embedCollection = ""
	embedPrune = false
	embedRebuild = false
	embedCollectionFn = nil

	err := runEmbed(root)
	if err == nil {
		t.Fatal("expected error when embedCollectionFn is nil")
	}
}

func TestRunEmbed_WithCollection(t *testing.T) {
	root := setupTestProject(t)
	embedConcurrency = 1
	embedAll = false
	embedCollection = "default"
	embedPrune = false
	embedRebuild = false
	embedCollectionFn = nil
	t.Cleanup(func() { embedCollection = "" })

	err := runEmbed(root)
	if err == nil {
		t.Fatal("expected error (embedCollectionFn nil) with collection flag")
	}
}
