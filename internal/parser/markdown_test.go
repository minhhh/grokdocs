package parser

import (
	"strings"
	"testing"
)

const mdHeadingBasic = `# Getting Started

Welcome to the documentation.

## Installation

Run the following command:

go install

## Usage

Call the program with:

grokdocs sync

## Configuration

Edit the config file.

### Advanced Options

Tweak these for performance.
`

const mdNoHeaders = `This is a plain document with no markdown headings at all.

It has multiple paragraphs. but nothing that looks like a heading.

Just text content for chunking and a very very very very long long long long
sentence that is longer than the maximum allowed chunk potentially`

const mdTextBeforeHeader = `

# Hello

It has multiple paragraphs but nothing that looks like a heading.

Just text content for chunking.`

const mdHeadingDeep = `# API Reference

## Endpoints

The following endpoints are available:

### GET /api/v1/users

json body with id and name

### POST /api/v1/users

json body with name only

## Authentication

Use bearer tokens.`

const mdLongBody = `# Title

Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident sunt in culpa qui officia deserunt mollit anim id est laborum.

## Section One

Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident sunt in culpa qui officia deserunt mollit anim id est laborum.

## Section Two

Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident sunt in culpa qui officia deserunt mollit anim id est laborum.

Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident sunt in culpa qui officia deserunt mollit anim id est laborum.

## Section Three

Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident sunt in culpa qui officia deserunt mollit anim id est laborum.`

func TestMarkdownParserBasic(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20

	p := &MarkdownParser{}
	doc, err := p.Parse("test.md", mdHeadingBasic)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) != 5 {
		t.Fatal("Should produce 5 chunks")
	}

	for i, c := range doc.Chunks {
		if c.ChunkIndex != i {
			t.Errorf("chunk[%d].ChunkIndex = %d, want %d", i, c.ChunkIndex, i)
		}
		if c.TextContent == "" {
			t.Errorf("chunk[%d].TextContent is empty", i)
		}
		if c.TotalChars <= 0 {
			t.Errorf("chunk[%d].TotalChars = %d, want > 0", i, c.TotalChars)
		}
		if c.LineStart <= 0 {
			t.Errorf("chunk[%d].LineStart = %d, want > 0", i, c.LineStart)
		}
		if c.LineEnd < c.LineStart {
			t.Errorf("chunk[%d].LineEnd (%d) < LineStart (%d)", i, c.LineEnd, c.LineStart)
		}
		if c.Metadata == "" {
			t.Errorf("chunk[%d].Metadata is empty", i)
		}
	}

	if doc.Metadata == "" {
		t.Error("doc.Metadata is empty")
	}
}

func TestMarkdownParserSectionHeaders(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20
	p := &MarkdownParser{}
	filler := strings.Repeat("This is a test line used to generate enough content for heading splitting in the markdown parser tests.\n", 8)
	content := "# Getting Started\n\n" + filler +
		"## Installation\n\n" + filler +
		"## Usage\n\n" + filler +
		"## Configuration\n\n" + filler +
		"### Advanced Options\n\n"
	doc, err := p.Parse("test.md", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []struct {
		SectionNum   int
		SectionTitle string
	}{
		{1, "# Getting Started"},
		{2, "## Installation"},
		{3, "## Usage"},
		{4, "## Configuration"},
		{5, "### Advanced Options"},
	}

	if len(doc.Chunks) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(doc.Chunks), len(want))
	}

	for i, c := range doc.Chunks {
		w := want[i]
		if c.SectionNum != w.SectionNum {
			t.Errorf("chunk[%d].SectionNum = %d, want %d", i, c.SectionNum, w.SectionNum)
		}
		if c.SectionTitle != w.SectionTitle {
			t.Errorf("chunk[%d].SectionTitle = %q, want %q", i, c.SectionTitle, w.SectionTitle)
		}
	}
}

func TestMarkdownParserNoHeaders(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20
	p := &MarkdownParser{}
	doc, err := p.Parse("test.md", mdNoHeaders)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	for _, c := range doc.Chunks {
		if c.SectionNum != 0 {
			t.Errorf("expected SectionNum=0 for no-headers doc, got %d", c.SectionNum)
		}
		if c.SectionTitle != "" {
			t.Errorf("expected empty SectionTitle for no-headers doc, got %q", c.SectionTitle)
		}
	}
}

func TestMarkdownParserTextBeforeHeader(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 30

	p := &MarkdownParser{}
	doc, err := p.Parse("test.md", mdTextBeforeHeader)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []struct {
		SectionTitle string
		LineStart    int
		LineEnd      int
	}{
		{"# Hello", 1, 7},
	}

	if len(doc.Chunks) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(doc.Chunks), len(want))
	}

	for i, c := range doc.Chunks {
		w := want[i]
		if c.SectionTitle != w.SectionTitle {
			t.Errorf("chunk[%d].SectionTitle = %q, want %q", i, c.SectionTitle, w.SectionTitle)
		}
		if c.LineStart != w.LineStart {
			t.Errorf("chunk[%d].LineStart = %d, want %d", i, c.LineStart, w.LineStart)
		}
		if c.LineEnd != w.LineEnd {
			t.Errorf("chunk[%d].LineEnd = %d, want %d", i, c.LineEnd, w.LineEnd)
		}
	}
}

func TestMarkdownParserSplitNoHeader(t *testing.T) {
	DefaultChunkMaxSizeChar = 60
	DefaultChunkMinSizeChar = 30

	p := &MarkdownParser{}
	doc, err := p.Parse("test.md", mdNoHeaders)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []struct {
		SectionTitle string
		LineStart    int
		LineEnd      int
	}{
		{"", 1, 1},
		{"", 2, 3},
		{"", 3, 3},
		{"", 4, 5},
		{"", 5, 6},
		{"", 6, 6},
	}

	if len(doc.Chunks) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(doc.Chunks), len(want))
	}

	for i, c := range doc.Chunks {
		w := want[i]
		t.Logf("%s\n", c.TextContent)
		if c.SectionTitle != w.SectionTitle {
			t.Errorf("chunk[%d].SectionTitle = %q, want %q", i, c.SectionTitle, w.SectionTitle)
		}
		if c.LineStart != w.LineStart {
			t.Errorf("chunk[%d].LineStart = %d, want %d", i, c.LineStart, w.LineStart)
		}
		if c.LineEnd != w.LineEnd {
			t.Errorf("chunk[%d].LineEnd = %d, want %d", i, c.LineEnd, w.LineEnd)
		}
	}
}

func TestMarkdownParserDeepHierarchy(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20

	p := &MarkdownParser{}
	filler := strings.Repeat("This is a test line used to generate enough content for heading splitting in the markdown parser tests.\n", 8)
	content := "# API Reference\n\n" + filler +
		"## Endpoints\n\n" + filler +
		"### GET /api/v1/users\n\n" + filler +
		"### POST /api/v1/users\n\n" + filler +
		"## Authentication\n\n" + filler
	doc, err := p.Parse("test.md", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []struct {
		SectionNum   int
		SectionTitle string
	}{
		{1, "# API Reference"},
		{2, "## Endpoints"},
		{3, "### GET /api/v1/users"},
		{4, "### POST /api/v1/users"},
		{5, "## Authentication"},
	}

	if len(doc.Chunks) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(doc.Chunks), len(want))
	}

	for i, c := range doc.Chunks {
		w := want[i]
		if c.SectionNum != w.SectionNum {
			t.Errorf("chunk[%d].SectionNum = %d, want %d", i, c.SectionNum, w.SectionNum)
		}
		if c.SectionTitle != w.SectionTitle {
			t.Errorf("chunk[%d].SectionTitle = %q, want %q", i, c.SectionTitle, w.SectionTitle)
		}
	}

	if doc.Chunks[2].SectionNum != 3 || doc.Chunks[3].SectionNum != 4 {
		t.Error("H3 chunks should have sequential section numbers")
	}
	if doc.Chunks[4].SectionTitle != "## Authentication" {
		t.Error("After H3, next H2 should be a new top-level section")
	}
}

func TestMarkdownParserLongBody(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20

	p := &MarkdownParser{}
	doc, err := p.Parse("test.md", mdLongBody)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) < 3 {
		t.Fatal("expected at least 3 chunks for long document")
	}

	for i, c := range doc.Chunks {
		if c.ChunkIndex != i {
			t.Errorf("chunk[%d].ChunkIndex = %d, want %d", i, c.ChunkIndex, i)
		}
		if c.TextContent == "" {
			t.Errorf("chunk[%d].TextContent is empty", i)
		}
		if c.TotalChars <= 0 {
			t.Errorf("chunk[%d].TotalChars = %d, want > 0", i, c.TotalChars)
		}
	}
}

func TestMarkdownParserSlugAndIDs(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20

	p := &MarkdownParser{}
	doc, err := p.Parse("docs/guide.md", mdHeadingBasic)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	for _, c := range doc.Chunks {
		if c.Slug != "" {
			t.Errorf("expected empty Slug from Parse, got %q", c.Slug)
		}
		if c.ID != 0 {
			t.Errorf("expected ID=0 from Parse, got %d", c.ID)
		}
		if c.DocumentID != 0 {
			t.Errorf("expected DocumentID=0 from Parse, got %d", c.DocumentID)
		}
	}
}

func TestMarkdownParserEmptyContent(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20

	p := &MarkdownParser{}
	doc, err := p.Parse("test.md", "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) != 1 {
		t.Errorf("expected 1 chunks for empty content, got %d", len(doc.Chunks))
	}

	if doc.Metadata == "" {
		t.Error("Metadata should not be empty even for empty content")
	}
}

func TestMarkdownParserLongBodyExceedsMaxSize(t *testing.T) {
	DefaultChunkMaxSizeChar = 1000
	DefaultChunkMinSizeChar = 20

	p := &MarkdownParser{}

	var longBody strings.Builder
	for i := 0; i < 80; i++ {
		longBody.WriteString("Lorem ipsum dolor sit amet consectetur adipiscing elit. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip.\n")
	}
	content := "# Header\n\n" + longBody.String()

	doc, err := p.Parse("test.md", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) < 2 {
		t.Fatal("expected multiple chunks when body exceeds max size")
	}

	for i, c := range doc.Chunks {
		if c.ChunkIndex != i {
			t.Errorf("chunk[%d].ChunkIndex = %d, want %d", i, c.ChunkIndex, i)
		}
		if c.TextContent == "" {
			t.Errorf("chunk[%d].TextContent is empty", i)
		}
		if c.TotalChars > DefaultChunkMaxSizeChar {
			t.Errorf("chunk[%d].TotalChars = %d, want <= %d", i, c.TotalChars, int64(DefaultChunkMaxSizeChar))
		}
	}
}
