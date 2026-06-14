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

func TestWalkFilesFilesOnly(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"docs/intro.md", true, "# Intro"},
		{"src/main.go", true, "package main"},
		{"notes.md", false, "text"},
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(
		[]string{"docs/intro.md", "src/main.go"}, nil, nil,
	))
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

func TestWalkFilesBothFilesAndInclude(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"README.md", true, "# readme"},         // explicit file
		{"docs/intro.md", true, "# Intro"},       // include *.md
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(
		[]string{"README.md"}, []string{"*.md"}, nil,
	))
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

func TestWalkFilesFilesAndIncludeDedup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/intro.md", "# Intro")
	writeFile(t, root, "README.md", "# readme")
	writeFile(t, root, "other.txt", "text")

	results := collectWalkResultsWithFilter(t, root, newFileFilter(
		[]string{"docs/intro.md", "README.md"}, []string{"*.md", "*.txt"}, nil,
	))
	got := map[string]int{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath]++
	}
	if got["docs/intro.md"] != 1 {
		t.Errorf("docs/intro.md emitted %d times, want 1", got["docs/intro.md"])
	}
	if got["README.md"] != 1 {
		t.Errorf("README.md emitted %d times, want 1", got["README.md"])
	}
	if got["other.txt"] != 1 {
		t.Errorf("other.txt emitted %d times, want 1", got["other.txt"])
	}
}

func TestWalkFilesFilesBasenameOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# root readme")
	writeFile(t, root, "a/README.md", "# nested readme")

	results := collectWalkResultsWithFilter(t, root, newFileFilter(
		[]string{"README.md"}, nil, nil,
	))
	got := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		got[r.RelPath] = true
	}
	if !got["README.md"] {
		t.Errorf("expected README.md at root to be emitted, got %v", got)
	}
	if got["a/README.md"] {
		t.Errorf("did not expect a/README.md to be emitted")
	}
}

func TestWalkFilesOnlyIncludedFolders(t *testing.T) {
	tests := []struct {
		path string
		want bool
		cont string
	}{
		{"hello/world.md", true, "# hello"},             // inside target folder
		{"hello/sub/other.md", false, "# sub"},          // */*.md doesn't match subdirs
		{"other/file.md", false, "# other"},             // outside target folder
		{"workspace/src/lib.md", true, "# lib"},         // ** descends into subdirs
	}

	root := t.TempDir()
	for _, tc := range tests {
		writeFile(t, root, tc.path, tc.cont)
	}

	results := collectWalkResultsWithFilter(t, root, newFileFilter(
		nil, []string{"hello/*.md", "workspace/**/*.md"}, nil,
	))
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
