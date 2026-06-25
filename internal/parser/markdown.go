package parser

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/minhhh/grokdocs/internal/project"
)

// MarkdownParser splits markdown content line-by-line and section-by-section,
// respecting both min and max chunk size constraints.
type MarkdownParser struct{}

func init() {
	RegisterParser("markdown", &MarkdownParser{})
}

type mdHeading struct {
	Level int
	Title string
}

func parseMdHeading(line string) (mdHeading, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	runes := []rune(trimmed)
	if len(runes) == 0 || (runes[0] != '#' && runes[0] != '▼') {
		return mdHeading{}, false
	}
	marker := runes[0]
	level := 0
	for level < len(runes) && runes[level] == marker {
		level++
	}
	if level >= len(runes) || (runes[level] != ' ' && runes[level] != '\t') {
		return mdHeading{}, false
	}
	title := strings.TrimSpace(trimmed)
	return mdHeading{Level: level, Title: title}, true
}

type chunkBuffer struct {
	lines        []int
	startLine    int
	startPos     int
	sectionNum   int
	sectionTitle string
	charCount    int
}

func (b chunkBuffer) realCharCount() int {
	return b.charCount - 1
}

func (p *MarkdownParser) Parse(relPath string, content string) (*ParsedDocument, error) {
	lines := strings.Split(content, "\n")

	var chunks []*project.ChunkRecord
	chunkIndex := 0

	// reverseEndPos means we split from the end of a line, after position length - 1 - reverseEndPos
	// if reverseEndPos=0, it means we take the whole line
	// otherwise we take part of the line, but never skip the whole line
	// If we want to skip the whole line, then the split should be on the previous line
	cur := chunkBuffer{}
	makeChunk := func(b chunkBuffer, splitLineIdx int, reverseEndPos int) *project.ChunkRecord {
		var sb strings.Builder
		for i := 0; i <= splitLineIdx; i++ {
			li := b.lines[i]
			runes := []rune(lines[li])
			lineLen := len(runes)
			if i == 0 {
				if i == len(b.lines) { 
					sb.WriteString(string(runes[b.startPos:lineLen - reverseEndPos]))
				} else {
					sb.WriteString(string(runes[b.startPos:]))
				}
			} else if i == splitLineIdx {
				sb.WriteByte('\n')
				sb.WriteString(string(runes[:lineLen - reverseEndPos]))
			} else {
				sb.WriteByte('\n')
				sb.WriteString(lines[li])
			}
		}
		text := sb.String()
		c := &project.ChunkRecord{
			TextContent:  text,
			TotalChars:   utf8.RuneCountInString(text),
			ChunkIndex:   chunkIndex,
			LineStart:    b.startLine + 1,
			LineEnd:      b.startLine + splitLineIdx + 1,
			SectionNum:   b.sectionNum,
			SectionTitle: b.sectionTitle,
			Metadata:     "{}",
		}
		chunkIndex++
		return c
	}

	flush := func(splitLineIdx int, reverseEndPos int) {
		chunks = append(chunks, makeChunk(cur, splitLineIdx, reverseEndPos))
		if reverseEndPos == 0 {
			cur.startLine = cur.startLine + splitLineIdx + 1
			if splitLineIdx == len(cur.lines) - 1 {
				cur.lines = []int{}
			} else {
				cur.lines = cur.lines[splitLineIdx + 1:]
			}
			cur.startPos = 0
		} else {
			cur.startLine = cur.startLine + splitLineIdx
			cur.lines = cur.lines[splitLineIdx:]
			cur.startPos = utf8.RuneCountInString(lines[cur.lines[0]]) - reverseEndPos
		}

		cur.charCount = countBufferChars(cur, lines) + 1
	}

	sectionNum := 0
	inCodeBlock := buildFenceMap(lines)

	for lineIdx, rawLine := range lines {
		lineChars := utf8.RuneCountInString(rawLine)

		if heading, ok := parseMdHeading(rawLine); ok && !inCodeBlock[lineIdx] {
			if cur.charCount >= DefaultChunkMinSizeChar {
				flush(len(cur.lines) - 1, 0)
			}
			sectionNum++

			cur.lines = append(cur.lines, lineIdx)
			cur.charCount += lineChars + 1
			cur.sectionNum = sectionNum
			cur.sectionTitle = heading.Title

			continue
		}

		cur.lines = append(cur.lines, lineIdx)
		cur.charCount += lineChars + 1

		for cur.realCharCount() > DefaultChunkMaxSizeChar {
			splitLine, reverseEndPos := findSplit(cur, lines, inCodeBlock, DefaultChunkMaxSizeChar, DefaultChunkMinSizeChar)
			flush(splitLine, reverseEndPos)
		}
	}

	flush(len(cur.lines) - 1, 0)

	docMeta, _ := json.Marshal(map[string]any{"path": relPath})
	return &ParsedDocument{
		Metadata: string(docMeta),
		Chunks:   chunks,
	}, nil
}

func buildFenceMap(lines []string) []bool {
	inCode := false
	result := make([]bool, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			result[i] = true
			inCode = !inCode
		} else {
			result[i] = inCode
		}
	}
	return result
}


func countBufferChars(buf chunkBuffer, lines []string) int {
	count := 0
	for i, li := range buf.lines {
		lineChars := utf8.RuneCountInString(lines[li])
		if i == 0 {
			lineChars -= buf.startPos
		}
		count += lineChars + 1
	}
	return count - 1
}

// Return the place to split the chunk buffer in 2 halves to satisfy the max size condition
func findSplit(cur chunkBuffer, lines []string, inCodeBlock []bool, maxChars int, minChars int) (splitLine int, reverseEndPos int) {
	totalChars := cur.charCount
	// First find a blankline to split (but not inside a fenced code block)
	for i := len(cur.lines) - 1; i >= 0; i-- {
		line := lines[cur.lines[i]]
		lineLen := utf8.RuneCountInString(line)
		totalChars -= lineLen + 1
		if totalChars <= maxChars && totalChars >= minChars && strings.TrimSpace(line) == "" && !inCodeBlock[cur.lines[i]] {
			return i - 1, 0
		}
	}

	// Find a sentence separator to split
	totalChars = cur.charCount
	for i := len(cur.lines) - 1; i >= 0; i-- {
		line := lines[cur.lines[i]]
		runes := []rune(line)
		lineLen := len(runes)

		for j := lineLen - 1; j >=0; j-- {
			if strings.ContainsRune(".!?", runes[j]) {
				tentativeTotalChar := totalChars - lineLen + j + 1
				if tentativeTotalChar <= maxChars && tentativeTotalChar >= minChars {
					return i, lineLen - j - 1
				}
			}
		}
		totalChars -= lineLen + 1
	}

	// Find a word separator to split
	totalChars = cur.charCount
	for i := len(cur.lines) - 1; i >= 0; i-- {
		line := lines[cur.lines[i]]
		runes := []rune(line)
		lineLen := len(runes)

		for j := lineLen - 1; j >=0; j-- {
			if unicode.IsSpace(runes[j]) {
				tentativeTotalChar := totalChars - lineLen + j + 1
				if tentativeTotalChar <= maxChars && tentativeTotalChar >= minChars {
					return i, lineLen - j - 1
				}
			}
		}

		totalChars -= lineLen + 1
	}

	// Just anywhere to split
	totalChars = cur.charCount
	for i := len(cur.lines) - 1; i >= 0; i-- {
		line := lines[cur.lines[i]]
		runes := []rune(line)
		lineLen := len(runes)

		for j := lineLen - 1; j >=0; j-- {
			tentativeTotalChar := totalChars - lineLen + j + 1
			if tentativeTotalChar <= maxChars && tentativeTotalChar >= minChars {
				return i, lineLen - j - 1
			}
		}

		totalChars -= lineLen + 1
	}

	return 0, 0
}
