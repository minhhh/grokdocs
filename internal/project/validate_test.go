package project

import (
	"testing"
)

func TestAssertCollectionValid_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	proj, err := NewProject(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Init(); err != nil {
		t.Fatal(err)
	}

	AssertCollectionValid(proj, "default")
}
