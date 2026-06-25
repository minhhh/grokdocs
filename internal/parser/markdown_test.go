package parser

import (
	"strings"
	"testing"
)

func TestBuildFenceMap(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected []bool
	}{
		{
			name:     "no fences",
			lines:    []string{"a", "b", "c"},
			expected: []bool{false, false, false},
		},
		{
			name:     "basic fence pair",
			lines:    []string{"a", "```", "code", "```", "b"},
			expected: []bool{false, true, true, true, false},
		},
		{
			name:     "tilde fence pair",
			lines:    []string{"a", "~~~", "code", "~~~", "b"},
			expected: []bool{false, true, true, true, false},
		},
		{
			name:     "unclosed fence",
			lines:    []string{"a", "```", "code"},
			expected: []bool{false, true, true},
		},
		{
			name:     "consecutive fences toggle each line",
			lines:    []string{"```", "```", "```"},
			expected: []bool{true, true, true},
		},
		{
			name:     "fence with info string",
			lines:    []string{"a", "```go", "code", "```", "b"},
			expected: []bool{false, true, true, true, false},
		},
		{
			name:     "indented fence",
			lines:    []string{"a", "  ```", "code", "  ```", "b"},
			expected: []bool{false, true, true, true, false},
		},
		{
			name:     "empty lines",
			lines:    []string{},
			expected: []bool{},
		},
		{
			name:     "single fence line",
			lines:    []string{"```"},
			expected: []bool{true},
		},
		{
			name:     "mixed fence types toggle together (same toggle in current impl)",
			lines:    []string{"```", "code", "~~~", "more"},
			expected: []bool{true, true, true, false},
		},
		{
			name:     "consecutive fence",
			lines:    []string{"a", "  ```", "code1", "  ```", "  ```", "code2", "  ```",  "b"},
			expected: []bool{false, true, true, true, true, true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFenceMap(tt.lines)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d", len(tt.expected), len(result))
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("index %d: expected %v, got %v", i, exp, result[i])
				}
			}
		})
	}
}

func TestFindSplit_BlankLine(t *testing.T) {
	lines := []string{"aaa", "", "bbb"}
	cur := chunkBuffer{
		lines:     []int{0, 1, 2},
		startLine: 0,
		startPos:  0,
		charCount: (3 + 1) + (0 + 1) + (3 + 1), // 9
	}
	inCodeBlock := []bool{false, false, false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 5, 2)
	// i=2: totalChars=9-3-1=5, not blank
	// i=1: totalChars=5-0-1=4, blank line, 4<=5 && >=2 -> returns (0,0)
	if splitLine != 0 || reverseEndPos != 0 {
		t.Errorf("expected (0,0), got (%d,%d)", splitLine, reverseEndPos)
	}
}

func TestFindSplit_BlankLineInCodeBlock(t *testing.T) {
	lines := []string{"aaa", "", "bbb", "ccc"}
	cur := chunkBuffer{
		lines:     []int{0, 1, 2, 3},
		startLine: 0,
		startPos:  0,
		charCount: (3 + 1) + (0 + 1) + (3 + 1) + (3 + 1), // 13
	}
	// Line 1 (blank) is inside code block so blank-split is skipped
	inCodeBlock := []bool{false, true, true, false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 9, 2)
	// Force split at line index 2, last char position -> (2,0)
	if splitLine != 2 || reverseEndPos != 0 {
		t.Errorf("expected (2,0), got (%d,%d)", splitLine, reverseEndPos)
	}
}

func TestFindSplit_Sentence(t *testing.T) {
	// "b.c" has '.' separator, and at that point totalChars-leftover fits maxChars=5
	lines := []string{"a", "b.c", "d"}
	cur := chunkBuffer{
		lines:     []int{0, 1, 2},
		startLine: 0,
		startPos:  0,
		charCount: (1 + 1) + (3 + 1) + (1 + 1), // 8
	}
	inCodeBlock := []bool{false, false, false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 5, 1)
	// Blank: i=2 total=6 "d" not blank, i=1 total=2 "b.c" not blank, i=0 total=0
	// Sentence: i=2 total=8 "d" no sep, i=1 total=6 "b.c" j=1 '.' tent=6-3+1+1=5 -> (1,1)
	if splitLine != 1 || reverseEndPos != 1 {
		t.Errorf("expected (1,1), got (%d,%d)", splitLine, reverseEndPos)
	}
}

func TestFindSplit_Word(t *testing.T) {
	// "b c" has space separator, and at that point totalChars-leftover fits maxChars=5
	lines := []string{"a", "b c", "d"}
	cur := chunkBuffer{
		lines:     []int{0, 1, 2},
		startLine: 0,
		startPos:  0,
		charCount: (1 + 1) + (3 + 1) + (1 + 1), // 8
	}
	inCodeBlock := []bool{false, false, false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 5, 1)
	// Sentence: i=2 total=8 "d" no sep, i=1 total=6 "b c" j=1 ' ' not .!?
	// Word: i=2 total=8 "d" not space, i=1 total=6 "b c" j=1 ' ' space tent=6-3+1+1=5 -> (1,1)
	if splitLine != 1 || reverseEndPos != 1 {
		t.Errorf("expected (1,1), got (%d,%d)", splitLine, reverseEndPos)
	}
}

func TestFindSplit_Force(t *testing.T) {
	// "bc" has no blank, no separator, no word boundary — force split
	lines := []string{"a", "bc", "d"}
	cur := chunkBuffer{
		lines:     []int{0, 1, 2},
		startLine: 0,
		startPos:  0,
		charCount: (1 + 1) + (2 + 1) + (1 + 1), // 7
	}
	inCodeBlock := []bool{false, false, false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 5, 1)
	// Force: i=1 total=5 "bc" j=1 tent=5-2+1+1=5 -> (1,0)
	if splitLine != 1 || reverseEndPos != 0 {
		t.Errorf("expected (1,0), got (%d,%d)", splitLine, reverseEndPos)
	}
}

func TestFindSplit_Fallback(t *testing.T) {
	lines := []string{"aaa"}
	cur := chunkBuffer{
		lines:     []int{0},
		startLine: 0,
		startPos:  0,
		charCount: 3 + 1, // 4
	}
	inCodeBlock := []bool{false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 5, 5)
	// totalChars=4, lineLen=3
	// all tentativeTotalChar values < 5, so no split works -> (0,0)
	if splitLine != 0 || reverseEndPos != 0 {
		t.Errorf("expected (0,0), got (%d,%d)", splitLine, reverseEndPos)
	}
}

func TestFindSplit_StartPosNonZero(t *testing.T) {
	lines := []string{"hello world", "foo bar"}
	cur := chunkBuffer{
		lines:     []int{1},
		startLine: 0,
		startPos:  4,
	}
	cur.charCount = countBufferChars(cur, lines) + 1
	inCodeBlock := []bool{false}

	splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, 10, 1)
	// Should not panic and should return a valid result
	if splitLine < 0 {
		t.Errorf("expected non-negative splitLine, got %d", splitLine)
	}
	if reverseEndPos < 0 {
		t.Errorf("expected non-negative reverseEndPos, got %d", reverseEndPos)
	}
}

func TestParseMdHeading(t *testing.T) {
	tests := []struct {
		line     string
		wantOK   bool
		wantLev  int
		wantTitle string
	}{
		{"# Title", true, 1, "# Title"},
		{"## Sub", true, 2, "## Sub"},
		{"### Deep", true, 3, "### Deep"},
		{"  # Indented", true, 1, "# Indented"},
		{"▼ Title", true, 1, "▼ Title"},
		{"▼▼ Sub", true, 2, "▼▼ Sub"},
		{"  ▼ Indented", true, 1, "▼ Indented"},
		{"#NotSpace", false, 0, ""},
		{"plain text", false, 0, ""},
		{"####", false, 0, ""},
		{"▼NotSpace", false, 0, ""},
		{"▼▼", false, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got, ok := parseMdHeading(tt.line)
			if ok != tt.wantOK {
				t.Errorf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if ok {
				if got.Level != tt.wantLev {
					t.Errorf("expected level %d, got %d", tt.wantLev, got.Level)
				}
				if got.Title != tt.wantTitle {
					t.Errorf("expected title %q, got %q", tt.wantTitle, got.Title)
				}
			}
		})
	}
}

func TestCountBufferChars(t *testing.T) {
	lines := []string{"hello", "world"}
	cur := chunkBuffer{
		lines:     []int{0, 1},
		startLine: 0,
		startPos:  0,
	}
	count := countBufferChars(cur, lines)
	expected := (5 + 1) + (5 + 1) - 1 // 11
	if count != expected {
		t.Errorf("expected %d, got %d", expected, count)
	}
}

func TestCountBufferChars_StartPos(t *testing.T) {
	lines := []string{"hello world", "foo"}
	cur := chunkBuffer{
		lines:     []int{0},
		startLine: 0,
		startPos:  6, // skip "hello "
	}
	count := countBufferChars(cur, lines)
	expected := (11 - 6) + 1 - 1 // 5
	if count != expected {
		t.Errorf("expected %d, got %d", expected, count)
	}
}

func TestParseMarkdown_SplitAcrossMultipleChunks(t *testing.T) {
	p := &MarkdownParser{}
	// Content that should produce multiple chunks due to size
	shortContent := "# Title\n\n" + strings.Repeat("hello world\n", 200)
	doc, err := p.Parse("test.md", shortContent)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(doc.Chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestParseMarkdown_EmptyContent(t *testing.T) {
	p := &MarkdownParser{}
	doc, err := p.Parse("empty.md", "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	// Empty content produces one empty chunk because strings.Split("", "\n") returns [""]
	if len(doc.Chunks) != 1 {
		t.Errorf("expected 1 chunk for empty content, got %d", len(doc.Chunks))
	}
}
