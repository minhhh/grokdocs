package parser

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gomantics/chunkx"
	"github.com/gomantics/chunkx/languages"
	"github.com/minhhh/grokdocs/internal/project"
)

// CharTokenizer implements chunkx.TokenCounter counting Unicode code points.
type CharTokenizer struct{}

func (CharTokenizer) CountTokens(text string) (int, error) {
	return utf8.RuneCountInString(text), nil
}

var charTok = &CharTokenizer{}

type sectionHeader struct {
	Title      string
	LineNumber int
}

func parseMarkdownHeaders(content string) []sectionHeader {
	lines := strings.Split(content, "\n")
	var headers []sectionHeader
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			headingLevel := 0
			for headingLevel < len(trimmed) && trimmed[headingLevel] == '#' {
				headingLevel++
			}
			if headingLevel < len(trimmed) && (trimmed[headingLevel] == ' ' || trimmed[headingLevel] == '\t') {
				headers = append(headers, sectionHeader{
					Title:      trimmed,
					LineNumber: lineNum,
				})
			}
		}
	}
	return headers
}

func parseHeaders(fileType string, content string) []sectionHeader {
	switch fileType {
	case ".md", ".markdown":
		return parseMarkdownHeaders(content)
	default:
		return nil
	}
}

// ChunkxParser wraps the gomantics/chunkx AST-based library.
type ChunkxParser struct {
	DefaultLanguage languages.LanguageName
}

func (cp *ChunkxParser) Parse(relPath string, content string) (*ParsedDocument, error) {
	var lang languages.LanguageName
	if detectedLang, ok := languages.DetectLanguage(relPath); ok {
		lang = detectedLang.Name
	} else {
		lang = cp.DefaultLanguage
	}

	chunker := chunkx.NewChunker()
	var cxChunks []chunkx.Chunk
	var err error

	if lang != "" {
		cxChunks, err = chunker.Chunk(
			content,
			chunkx.WithLanguage(lang),
			chunkx.WithMaxSize(DefaultChunkMaxSizeChar),
			chunkx.WithOverlap(DefaultChunkOverlap),
			chunkx.WithTokenCounter(charTok),
		)
	} else {
		cxChunks, err = chunker.Chunk(
			content,
			chunkx.WithMaxSize(DefaultChunkMaxSizeChar),
			chunkx.WithOverlap(DefaultChunkOverlap),
			chunkx.WithTokenCounter(charTok),
		)
	}
	if err != nil {
		return nil, err
	}

	headers := parseHeaders(filepath.Ext(relPath), content)

	var chunks []*project.ChunkRecord
	chunkIndex := 0
	for _, chunk := range cxChunks {
		sectionTitle := ""
		sectionNum := 0
		for idx, header := range headers {
			if header.LineNumber <= chunk.StartLine {
				sectionTitle = header.Title
				sectionNum = idx + 1
			}
		}

		metaMap := map[string]any{}
		metaBytes, _ := json.Marshal(metaMap)

		subRecords := splitChunk(chunk, sectionTitle, sectionNum, string(metaBytes))
		for _, r := range subRecords {
			r.ChunkIndex = chunkIndex
			chunkIndex++
		}
		chunks = append(chunks, subRecords...)
	}

	docMetadata := map[string]any{
		"path": relPath,
	}
	docMetaBytes, _ := json.Marshal(docMetadata)

	return &ParsedDocument{
		Metadata: string(docMetaBytes),
		Chunks:   chunks,
	}, nil
}

// splitChunk splits a chunk's content into sub-chunks when it exceeds DefaultChunkMaxSizeChar.
// chunkx's AST-based splitting can produce chunks larger than the max size, so we divide
// by rune count into roughly equal parts.
//
// Line numbers from chunkx have a known off-by-one issue: when chunkx merges an overlapping
// portion back into the main portion, the resulting StartLine/EndLine can be +1 off from
// actual source lines. This is a minor cosmetic issue and does not affect correctness.
func splitChunk(cx chunkx.Chunk, sectionTitle string, sectionNum int, meta string) []*project.ChunkRecord {
	numChars, _ := charTok.CountTokens(cx.Content)
	if numChars <= DefaultChunkMaxSizeChar {
		return []*project.ChunkRecord{{
			TextContent:  cx.Content,
			TotalChars:   numChars,
			LineStart:    cx.StartLine,
			LineEnd:      cx.EndLine,
			SectionNum:   sectionNum,
			SectionTitle: sectionTitle,
			Metadata:     meta,
		}}
	}

	n := (numChars + DefaultChunkMaxSizeChar - 1) / DefaultChunkMaxSizeChar
	runes := []rune(cx.Content)
	partSize := (len(runes) + n - 1) / n
	out := make([]*project.ChunkRecord, 0, n)
	lineOffset := 0
	for j := 0; j < n; j++ {
		start := j * partSize
		end := start + partSize
		if end > len(runes) {
			end = len(runes)
		}
		subContent := string(runes[start:end])
		subChars, _ := charTok.CountTokens(subContent)
		newlines := strings.Count(subContent, "\n")
		lineStart := cx.StartLine + lineOffset
		lineEnd := lineStart + newlines
		if lineEnd > cx.EndLine {
			lineEnd = cx.EndLine
		}
		out = append(out, &project.ChunkRecord{
			TextContent:  subContent,
			TotalChars:   subChars,
			LineStart:    lineStart,
			LineEnd:      lineEnd,
			SectionNum:   sectionNum,
			SectionTitle: sectionTitle,
			Metadata:     meta,
		})
		lineOffset += newlines
	}
	return out
}

func init() {
	RegisterParser("chunkx", &ChunkxParser{})
}
