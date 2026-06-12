package parser

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gomantics/chunkx"
	"github.com/gomantics/chunkx/languages"
	"github.com/minhhh/grokdocs/internal/project"
)

type sectionHeader struct {
	Title      string
	LineNumber int
}

func parseHeaders(content string) []sectionHeader {
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
				title := strings.TrimSpace(trimmed[headingLevel:])
				headers = append(headers, sectionHeader{
					Title:      title,
					LineNumber: lineNum,
				})
			}
		}
	}
	return headers
}

// ChunkxParser wraps the gomantics/chunkx AST-based library.
type ChunkxParser struct {
	DefaultLanguage languages.LanguageName
}

func (cp *ChunkxParser) Parse(relPath string, content string, fileSize int64) (*ParsedDocument, error) {
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
			chunkx.WithMaxSize(DefaultChunkMaxSize),
			chunkx.WithOverlap(DefaultChunkOverlap),
		)
	} else {
		cxChunks, err = chunker.Chunk(
			content,
			chunkx.WithMaxSize(DefaultChunkMaxSize),
			chunkx.WithOverlap(DefaultChunkOverlap),
		)
	}
	if err != nil {
		return nil, err
	}

	headers := parseHeaders(content)

	var chunks []*project.ChunkRecord
	for i, chunk := range cxChunks {
		sectionTitle := ""
		sectionNum := 0
		for idx, header := range headers {
			if header.LineNumber <= chunk.StartLine {
				sectionTitle = header.Title
				sectionNum = idx + 1
			}
		}

		checksum := sha256.Sum256([]byte(chunk.Content))
		chunkHash := fmt.Sprintf("%x", checksum)

		metaMap := map[string]any{
			"filename": filepath.Base(relPath),
		}
		metaBytes, _ := json.Marshal(metaMap)

		chunks = append(chunks, &project.ChunkRecord{
			ChunkIndex:   i,
			TextContent:  chunk.Content,
			ContentHash:  chunkHash,
			TotalChars:   int64(len(chunk.Content)),
			LineStart:    chunk.StartLine,
			LineEnd:      chunk.EndLine,
			SectionNum:   sectionNum,
			SectionTitle: sectionTitle,
			Metadata:     string(metaBytes),
		})
	}

	docMetadata := map[string]any{
		"path": relPath,
		"size": fileSize,
	}
	docMetaBytes, _ := json.Marshal(docMetadata)

	return &ParsedDocument{
		Metadata: string(docMetaBytes),
		Chunks:   chunks,
	}, nil
}

func init() {
	RegisterParser("markdown", &ChunkxParser{DefaultLanguage: languages.Markdown})
	RegisterParser("chunkx", &ChunkxParser{})
}
