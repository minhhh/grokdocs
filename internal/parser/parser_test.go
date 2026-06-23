package parser

import (
	"testing"

	"github.com/minhhh/grokdocs/internal/config"
)

func TestResolveParserName_DefaultMapping(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		path       string
		expected   string
		expectOK   bool
	}{
		{"file.go", "chunkx", true},
		{"file.py", "chunkx", true},
		{"file.md", "markdown", true},
		{"file.rs", "chunkx", true},
		{"file.js", "chunkx", true},
		{"file.ts", "chunkx", true},
		{"file.java", "chunkx", true},
		{"Dockerfile", "chunkx", true},
		{"file.unknown", "", false},
		{"noext", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := ResolveParserName(cfg, "default", tt.path)
			if ok != tt.expectOK {
				t.Errorf("expected ok=%v, got %v", tt.expectOK, ok)
			}
			if got != tt.expected {
				t.Errorf("expected parser %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestResolveParserName_CustomParser(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collections["default"] = config.CollectionConfig{
		Path: ".",
		Parsers: map[string]string{
			"*.md": "custom",
		},
	}

	got, ok := ResolveParserName(cfg, "default", "README.md")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "custom" {
		t.Errorf("expected custom parser, got %q", got)
	}
}

func TestResolveParserName_NilConfig(t *testing.T) {
	got, ok := ResolveParserName(nil, "default", "file.md")
	if ok {
		t.Error("expected ok=false for nil config")
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveParserName_UnknownCollection(t *testing.T) {
	cfg := config.DefaultConfig()
	got, ok := ResolveParserName(cfg, "nonexistent", "file.md")
	if ok {
		t.Error("expected ok=false for unknown collection")
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{"extension match", "file.md", ".md", true},
		{"extension no match", "file.go", ".md", false},
		{"exact match", "README.md", "README.md", true},
		{"exact no match", "other.md", "README.md", false},
		{"wildcard match", "file.go", "*.go", true},
		{"wildcard no match", "file.rs", "*.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPattern(tt.path, tt.pattern); got != tt.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestGetParser(t *testing.T) {
	// Before register
	if _, ok := GetParser("nonexistent"); ok {
		t.Error("expected false for unregistered parser")
	}

	// Register one
	mp := &MarkdownParser{}
	RegisterParser("test-md", mp)
	got, ok := GetParser("test-md")
	if !ok {
		t.Fatal("expected true for registered parser")
	}
	if got != mp {
		t.Errorf("expected same parser instance")
	}
}

func TestParseMarkdownHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantLen int
		wantH1  string
	}{
		{"h1 only", "# Title\n\ncontent", 1, "# Title"},
		{"multiple headers", "# H1\n## H2\n### H3", 3, "# H1"},
		{"no headers", "plain text\nno hashes", 0, ""},
		{"hash in middle", "a # b", 0, ""},
		{"h1 and h2", "# Top\n\n## Sub\n\nmore", 2, "# Top"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMarkdownHeaders(tt.content)
			if len(got) != tt.wantLen {
				t.Fatalf("expected %d headers, got %d", tt.wantLen, len(got))
			}
			if tt.wantLen > 0 && got[0].Title != tt.wantH1 {
				t.Errorf("expected first header %q, got %q", tt.wantH1, got[0].Title)
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	t.Run("markdown file", func(t *testing.T) {
		headers := parseHeaders(".md", "# Title\n\n## Sub")
		if len(headers) != 2 {
			t.Fatalf("expected 2 headers, got %d", len(headers))
		}
		if headers[0].Title != "# Title" {
			t.Errorf("expected '# Title', got %q", headers[0].Title)
		}
	})

	t.Run("markdown extension alternate", func(t *testing.T) {
		headers := parseHeaders(".markdown", "# Hello")
		if len(headers) != 1 {
			t.Fatalf("expected 1 header, got %d", len(headers))
		}
	})

	t.Run("non-markdown file", func(t *testing.T) {
		headers := parseHeaders(".go", "# this is not a header")
		if headers != nil {
			t.Error("expected nil for non-markdown file")
		}
	})
}

func TestGetMatchPriority(t *testing.T) {
	tests := []struct {
		pattern  string
		expected MatchPriority
	}{
		{"README", PriorityExactFile},
		{".md", PriorityExtension},
		{".tar.gz", PriorityComplexExtension},
		{"*.go", PriorityWildcard},
		{"foo.*", PriorityWildcard},
		{"[abc]", PriorityWildcard},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := getMatchPriority(tt.pattern)
			if got != tt.expected {
				t.Errorf("getMatchPriority(%q) = %d, want %d", tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestResolveParserName_CustomPrecedence(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collections["default"] = config.CollectionConfig{
		Path: ".",
		Parsers: map[string]string{
			"*.md":      "md-parser",
			"README.md": "readme-parser",
			"guide*":    "guide-glob",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"exact match beats extension", "README.md", "readme-parser"},
		{"longer wildcard beats shorter", "guide.md", "guide-glob"},
		{"extension fallback", "other.md", "md-parser"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResolveParserName(cfg, "default", tt.path)
			if !ok {
				t.Fatal("expected ok=true")
			}
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
