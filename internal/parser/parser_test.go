package parser

import (
	"testing"
)

func TestParseMarkdownHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []sectionHeader
	}{
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "no headers",
			content: "some text\nwithout any\nheaders",
			want:    nil,
		},
		{
			name: "single h1",
			content: "# Title\n\nsome body text",
			want: []sectionHeader{{Title: "# Title", LineNumber: 1}},
		},
		{
			name: "multiple header levels",
			content: "# H1\n\nbody\n\n## H2\n\nmore\n\n### H3",
			want: []sectionHeader{
				{Title: "# H1", LineNumber: 1},
				{Title: "## H2", LineNumber: 5},
				{Title: "### H3", LineNumber: 9},
			},
		},
		{
			name: "header with extra spaces",
			content: "##   Deeply indented",
			want: []sectionHeader{{Title: "##   Deeply indented", LineNumber: 1}},
		},
		{
			name: "not a header (no space after #)",
			content: "#notaheader\n\n#alsonotaheader\n\n# valid header",
			want: []sectionHeader{{Title: "# valid header", LineNumber: 5}},
		},
		{
			name: "headers separated by blank lines",
			content: "# First\n\nsome text\n\n# Second\n\nmore text",
			want: []sectionHeader{
				{Title: "# First", LineNumber: 1},
				{Title: "# Second", LineNumber: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMarkdownHeaders(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("parseMarkdownHeaders() returned %d headers, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Title != tt.want[i].Title || got[i].LineNumber != tt.want[i].LineNumber {
					t.Errorf("header[%d] = {Title: %q, LineNumber: %d}, want {Title: %q, LineNumber: %d}",
						i, got[i].Title, got[i].LineNumber, tt.want[i].Title, tt.want[i].LineNumber)
				}
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name     string
		fileType string
		content  string
		want     []sectionHeader
	}{
		{
			name:     "markdown file returns headers",
			fileType: ".md",
			content:  "# Title\n\nbody",
			want:     []sectionHeader{{Title: "# Title", LineNumber: 1}},
		},
		{
			name:     "markdown extension alias",
			fileType: ".markdown",
			content:  "# Title\n\nbody",
			want:     []sectionHeader{{Title: "# Title", LineNumber: 1}},
		},
		{
			name:     "go file returns no headers",
			fileType: ".go",
			content:  "// Package comment\npackage main",
			want:     nil,
		},
		{
			name:     "unknown extension returns no headers",
			fileType: ".txt",
			content:  "# Not a header\nbody",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHeaders(tt.fileType, tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("parseHeaders() returned %d headers, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Title != tt.want[i].Title || got[i].LineNumber != tt.want[i].LineNumber {
					t.Errorf("header[%d] = {Title: %q, LineNumber: %d}, want {Title: %q, LineNumber: %d}",
						i, got[i].Title, got[i].LineNumber, tt.want[i].Title, tt.want[i].LineNumber)
				}
			}
		})
	}
}
