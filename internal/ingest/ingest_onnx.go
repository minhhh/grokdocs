//go:build onnx

package ingest

import (
	"fmt"
	"strings"

	"github.com/minhhh/grokdocs/internal/embed"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
)

func init() {
	vectorIngestFn = ingestVectors
}

func ingestVectors(proj *project.Project, collection string, chunks []*project.ChunkRecord) error {
	mf, err := embed.GetOrDownloadModels(proj.ConfigDir)
	if err != nil {
		return fmt.Errorf("get models: %w", err)
	}

	embedder, err := embed.NewEmbedder(mf.ModelPath, mf.VocabPath)
	if err != nil {
		return fmt.Errorf("new embedder: %w", err)
	}
	defer embedder.Close()

	vdb, err := proj.OpenCollectionVector(collection)
	if err != nil {
		return fmt.Errorf("open collection vector db: %w", err)
	}

	var ids []int64
	vectors := make([]float32, 0, len(chunks)*embedder.Dim())

	for _, chunk := range chunks {
		if len(strings.TrimSpace(chunk.TextContent)) < 3 {
			continue
		}
		vec, err := embedder.Embed(chunk.TextContent)
		if err != nil {
			preview := chunk.TextContent
			if len([]rune(preview)) > 80 {
				preview = string([]rune(preview)[:80]) + "..."
			}
			util.Logger.Error().Err(err).Int64("chunk_id", chunk.ID).Str("preview", preview).Msg("skipping chunk: embed failed")
			continue
		}
		ids = append(ids, chunk.ID)
		vectors = append(vectors, vec...)
	}

	if len(ids) == 0 {
		return nil
	}

	if err := vdb.AddVectors(ids, vectors); err != nil {
		return fmt.Errorf("add vectors: %w", err)
	}

	if err := vdb.Save(); err != nil {
		return fmt.Errorf("save vector index: %w", err)
	}

	return nil
}
