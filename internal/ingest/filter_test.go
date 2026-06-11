package ingest

import (
	"testing"
)

func TestFileFilterFilesOnly(t *testing.T) {
	f := newFileFilter([]string{"README.md", "index.html"}, nil, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"docs/README.md", true},
		{"index.html", true},
		{"sub/README.md", true},
		{"docs/intro.md", true},
		{"style.css", true},
		{"data.bin", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterIncludeOnly(t *testing.T) {
	f := newFileFilter(nil, []string{"*.md", "*.go"}, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", true},
		{"src/util.go", true},
		{"style.css", false},
		{"README.txt", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterIncludeFolder(t *testing.T) {
	f := newFileFilter(nil, []string{"docs/**/*", "tests/**/*", "src/*.go"}, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", false},
		{"src/util.go", true},
		{"src/main.ts", false},
		{"style.css", false},
		{"README.txt", false},
		{"tests/test1.go", true},
		{"tests/auth/test2.go", true},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterExcludeOnly(t *testing.T) {
	f := newFileFilter(nil, nil, []string{"*.txt", "*.log"})
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", true},
		{"notes.txt", false},
		{"error.log", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterFilesAndInclude(t *testing.T) {
	f := newFileFilter([]string{"README.md", "config.yaml"}, []string{"*.go"}, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"config.yaml", true},
		{"other/README.md", true},
		{"src/main.go", true},
		{"docs/intro.md", false},
		{"style.css", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterFilesIgnoresExclude(t *testing.T) {
	f := newFileFilter([]string{"notes.txt"}, []string{"*.go"}, []string{"*.txt", "*.go"})
	tests := []struct {
		path string
		want bool
	}{
		{"notes.txt", true},
		{"main.go", true},
		{"README.md", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterIncludeAndExclude(t *testing.T) {
	f := newFileFilter(nil, []string{"*.md", "*.go"}, []string{"*_test.go", "*_old.md"})
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"main.go", true},
		{"main_test.go", false},
		{"docs/changes_old.md", false},
		{"style.css", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileFilterEmpty(t *testing.T) {
	f := newFileFilter(nil, nil, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"any/file.go", true},
		{"another/file.md", true},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestExcludeOverridesInclude(t *testing.T) {
	f := newFileFilter(nil, []string{"*.md"}, []string{"*_old.md"})
	tests := []struct {
		path string
		want bool
	}{
		{"docs/intro.md", true},
		{"docs/changes_old.md", false},
		{"README.md", true},
		{"archive_old.md", false},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestExcludeDoubleStarPattern(t *testing.T) {
	f := newFileFilter(nil, []string{"**/*"}, []string{"**/node_modules/**"})
	tests := []struct {
		path string
		want bool
	}{
		{"src/main.go", true},
		{"node_modules/pkg/index.js", false},
		{"a/b/node_modules/pkg/index.js", false},
		{"README.md", true},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
