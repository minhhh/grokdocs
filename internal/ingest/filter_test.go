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
		{"docs/intro.md", false},
		{"style.css", false},
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

func TestExtractIncludeFolders(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{
			name:     "folder-prefixed pattern",
			patterns: []string{"hello/*.md"},
			want:     []string{"hello"},
		},
		{
			name:     "nested folder",
			patterns: []string{"src/hello/*.go"},
			want:     []string{"src/hello"},
		},
		{
			name:     "multiple folders",
			patterns: []string{"docs/*.md", "tests/*.go"},
			want:     []string{"docs", "tests"},
		},
		{
			name:     "basename-only returns nil",
			patterns: []string{"*.md"},
			want:     nil,
		},
		{
			name:     "mixed basename and folder returns nil",
			patterns: []string{"*.md", "hello/*.go"},
			want:     nil,
		},
		{
			name:     "double-star after dir extracts prefix",
			patterns: []string{"docs/**/*.md"},
			want:     []string{"docs"},
		},
		{
			name:     "baseless double-star returns nil",
			patterns: []string{"**/*.md"},
			want:     nil,
		},
		{
			name:     "glob char in dir extracts prefix before glob",
			patterns: []string{"src/*/file.go"},
			want:     []string{"src"},
		},
		{
			name:     "empty patterns",
			patterns: []string{},
			want:     nil,
		},
		{
			name:     "dedup",
			patterns: []string{"docs/one.md", "docs/two.md"},
			want:     []string{"docs"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIncludeFolders(tc.patterns)
			if tc.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
				return
			}
			set := make(map[string]bool)
			for _, f := range got {
				set[f] = true
			}
			for _, w := range tc.want {
				if !set[w] {
					t.Errorf("missing folder %q in %v", w, got)
				}
			}
		})
	}
}

func TestFileFilterOnlyIncludedFolders(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		include []string
		exclude []string
		want   []string
	}{
		{
			name:    "files set -> onlyIncludedFolders empty",
			files:   []string{"docs/readme.md"},
			include: nil,
			exclude: nil,
			want:    nil,
		},
		{
			name:    "basename-only include -> empty",
			files:   nil,
			include: []string{"*.md"},
			exclude: nil,
			want:    nil,
		},
		{
			name:    "folder include -> populated",
			files:   nil,
			include: []string{"hello/*.md"},
			exclude: nil,
			want:    []string{"hello"},
		},
		{
			name:    "folder include with excluded folder",
			files:   nil,
			include: []string{"docs/*.md", "node_modules/*.js"},
			exclude: []string{"node_modules"},
			want:    []string{"docs"},
		},
		{
			name:    "double-star after prefix extracts folder",
			files:   nil,
			include: []string{"workspace/**/*.md"},
			exclude: nil,
			want:    []string{"workspace"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFileFilter(tc.files, tc.include, tc.exclude)
			if tc.want == nil {
				if f.onlyIncludedFolders != nil {
					t.Errorf("got %v, want nil", f.onlyIncludedFolders)
				}
				return
			}
			if len(f.onlyIncludedFolders) != len(tc.want) {
				t.Errorf("got %v, want %v", f.onlyIncludedFolders, tc.want)
				return
			}
			set := make(map[string]bool)
			for _, folder := range f.onlyIncludedFolders {
				set[folder] = true
			}
			for _, w := range tc.want {
				if !set[w] {
					t.Errorf("missing folder %q in %v", w, f.onlyIncludedFolders)
				}
			}
		})
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

func TestExcludeRootNodeModulesOnly(t *testing.T) {
	f := newFileFilter(nil, []string{"**/*"}, []string{"node_modules/**"})
	tests := []struct {
		path string
		want bool
	}{
		{"src/main.go", true},
		{"node_modules/pkg/index.js", false},
		{"sub/node_modules/pkg/index.js", true},
		{"README.md", true},
		{"a/b/node_modules/pkg/index.js", true},
	}
	for _, tc := range tests {
		got := f.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
