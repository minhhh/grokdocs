package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFileWalkingDefaultIncludeList(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"docs/intro.md", true, "# Intro"},
		{"src/main.go", true, "package main"},
		{"README.txt", false, "text"},
		{"styles.css", true, "body {}"},
		{"index.html", true, "<html>"},
		{"src/lib.rs", true, "fn main() {}"},
		{"App.java", true, "class App {}"},
		{"script.exe", false, "binary"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(nil, nil, nil))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}

func TestFileWalkingDefaultExcludeList(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{".git/README.md", false, "# git readme"},
		{"node_modules/pkg/index.js", false, "mod code"},
		{"dist/output.yaml", false, "key: val"},
		{"docs/intro.md", true, "# Intro"},
		{"src/main.go", true, "package main"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(nil, nil, nil))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}

func TestWalkFilesFilesOverrideExclude(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"subdir/doc.md", true},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, "# found")
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter([]string{"subdir/doc.md"}, nil, []string{"subdir"}))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}

func TestWalkFilesIncludeListFiltersFiles(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"docs/intro.md", true, "# Intro"},
		{"docs/main.go", true, "package main"},
		{"README.txt", false, "text"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(nil, []string{"*.md", "*.go"}, nil))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}

func TestWalkFilesDefaultExcludeSkipsHiddenDirs(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{".git/config", false, "git config"},
		{".grokdocs/config.yaml", false, "config"},
		{"docs/intro.md", true, "# Intro"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(nil, nil, nil))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}

func TestWalkFilesExcludeListSkipsDirs(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"node_modules/pkg/index.js", false, "code"},
		{"docs/node_modules/pkg/index.js", false, "code"},
		{"src/main.go", true, "package main"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(nil, nil, []string{"node_modules"}))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}

func TestWalkFilesClosesOnCancel(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, root, fmt.Sprintf("dir/file%d.md", i), "# content")
	}

	ctx, cancel := context.WithCancel(context.Background())

	ch := walkFiles(ctx, root, newFileFilter(nil, nil, nil))

	for i := 0; i < 5; i++ {
		if _, ok := <-ch; !ok {
			t.Fatal("walk closed before producing 5 results")
		}
	}

	cancel()

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("walk did not stop within 5 seconds of cancellation")
	}
}

func TestWalkFilesNonExistentRoot(t *testing.T) {
	ch := walkFiles(context.Background(), "/nonexistent/path", newFileFilter(nil, nil, nil))
	found := false
	for r := range ch {
		if r.Err != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected error for nonexistent root")
	}
}

func TestWalkFilesDoubleStarExclude(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"a/node_modules/pkg/index.js", false, "code"},
		{"a/src/main.go", true, "package main"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(nil, nil, []string{"**/node_modules"}))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	for _, tc := range tests {
		if got[tc.path] != tc.want {
			t.Errorf("RelPath %q: got present=%v, want %v", tc.path, got[tc.path], tc.want)
		}
	}
}
