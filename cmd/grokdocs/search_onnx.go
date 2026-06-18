//go:build onnx

package main

import (
	"fmt"
	"unicode/utf8"

	"github.com/minhhh/grokdocs/internal/embed"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
)

func init() {
	semanticSearchFn = searchSemantic
}

func searchSemantic(proj *project.Project, ftsDB *project.FTSDatabase, query, collection string, limit int) ([]*project.FTSResult, error) {
	mf, err := embed.GetOrDownloadModels(proj.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("get models: %w", err)
	}

	embedder, err := embed.NewEmbedder(mf.ModelPath, mf.VocabPath)
	if err != nil {
		return nil, fmt.Errorf("new embedder: %w", err)
	}
	defer embedder.Close()

	vec, err := embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	vdb, err := proj.OpenCollectionVector(collection)
	if err != nil {
		return nil, fmt.Errorf("open vector db: %w", err)
	}

	labels, distances, err := vdb.Search(vec, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	if len(labels) == 0 {
		return nil, nil
	}

	results := make([]*project.FTSResult, 0, len(labels))
	for i, label := range labels {
		chunk, err := ftsDB.GetChunkByID(label)
		if err != nil {
			util.Logger.Warn().Int64("chunk_id", label).Err(err).Msg("skipping FAISS result: chunk not found")
			continue
		}

		results = append(results, &project.FTSResult{
			ID:           chunk.ID,
			DocumentID:   chunk.DocumentID,
			ChunkIndex:   chunk.ChunkIndex,
			LineStart:    chunk.LineStart,
			LineEnd:      chunk.LineEnd,
			SectionTitle: chunk.SectionTitle,
			Slug:         chunk.Slug,
			Snippet:      makeSnippet(chunk.TextContent, 150),
			Rank:         1.0 / (1.0 + float64(distances[i])),
		})
	}

	return results, nil
}

func makeSnippet(text string, maxLen int) string {
	if utf8.RuneCountInString(text) <= maxLen {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxLen]) + "..."
}
