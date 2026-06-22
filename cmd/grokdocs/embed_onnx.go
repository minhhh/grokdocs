//go:build onnx

package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/minhhh/grokdocs/internal/embed"
	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"golang.org/x/sync/errgroup"
)

const vectorBatchSize = 1000

func init() {
	embedCollectionFn = embedCollectionImpl
	pruneOrphansFn = pruneOrphans
}

type embedResult struct {
	id     int64
	vector []float32
	err    error
}

func embedCollectionImpl(proj *project.Project, db *project.FTSDatabase, collection string, chunkIDs []int64, textContents []string, concurrency int, rebuild bool, progress *util.GuardedChan[ingest.SyncProgress]) (int, error) {
	vdb, err := proj.OpenCollectionVector(collection, embed.Dim())
	if err != nil {
		return 0, fmt.Errorf("open collection vector db: %w", err)
	}

	if rebuild {
		if err := vdb.Reset(); err != nil {
			return 0, fmt.Errorf("reset vector index: %w", err)
		}
	}

	work := make(chan int, concurrency)
	results := make(chan embedResult, concurrency)

	g, ctx := errgroup.WithContext(context.Background())

	for w := 0; w < concurrency; w++ {
		g.Go(func() error {
			for idx := range work {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				text := textContents[idx]
				if len(strings.TrimSpace(text)) < 3 {
					results <- embedResult{id: chunkIDs[idx]}
					continue
				}

				vec, err := embed.Embed(text)
				if err != nil {
					preview := text
					if len([]rune(preview)) > 80 {
						preview = string([]rune(preview)[:80]) + "..."
					}
					util.Logger.Error().Err(err).Int64("chunk_id", chunkIDs[idx]).Str("preview", preview).Msg("skipping chunk: embed failed")
					results <- embedResult{id: chunkIDs[idx], err: err}
					continue
				}
				results <- embedResult{id: chunkIDs[idx], vector: vec}
			}
			return nil
		})
	}

	go func() {
		for i := range chunkIDs {
			work <- i
		}
		close(work)
	}()

	var (
		batchIDs   []int64
		batchVecs  []float32
		embedded   int32
	)

	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		defer func() {
			if len(batchIDs) > 0 {
				flushBatch(vdb, db, collection, batchIDs, batchVecs)
			}
		}()

		dim := embed.Dim()
		batchIDs = make([]int64, 0, vectorBatchSize)
		batchVecs = make([]float32, 0, vectorBatchSize*dim)

		for r := range results {
			if r.err != nil || len(r.vector) == 0 {
				continue
			}
			batchIDs = append(batchIDs, r.id)
			batchVecs = append(batchVecs, r.vector...)
			atomic.AddInt32(&embedded, 1)

			if len(batchIDs) >= vectorBatchSize {
				flushBatch(vdb, db, collection, batchIDs, batchVecs)
				batchIDs = make([]int64, 0, vectorBatchSize)
				batchVecs = make([]float32, 0, vectorBatchSize*dim)
			}

			progress.Send(ingest.SyncProgress{
				FilesProcessed: int(embedded),
				Phase:          "Embedding",
				TotalFiles:     len(chunkIDs),
			})
		}
	}()

	err = g.Wait()
	close(results)
	collectorWg.Wait()

	if err != nil {
		return int(embedded), err
	}
	return int(embedded), nil
}

func flushBatch(vdb *project.VectorDatabase, db *project.FTSDatabase, collection string, ids []int64, vectors []float32) {
	if len(ids) == 0 {
		return
	}

	if err := vdb.RemoveIDs(ids); err != nil {
		util.Logger.Warn().Err(err).Int("batch", len(ids)).Msg("failed to remove existing vectors before batch add")
	}

	if err := vdb.AddVectors(ids, vectors); err != nil {
		util.Logger.Error().Err(err).Int("batch", len(ids)).Msg("failed to add vectors batch")
		return
	}

	if err := vdb.Save(); err != nil {
		util.Logger.Error().Err(err).Int("batch", len(ids)).Msg("failed to save vector index after batch")
		return
	}

	if err := db.MarkVectorized(ids, collection); err != nil {
		util.Logger.Warn().Err(err).Int("batch", len(ids)).Msg("failed to mark chunk batch as vectorized")
	}
}

func pruneOrphans(proj *project.Project, db *project.FTSDatabase, orphans []project.VectorChunkOrphan) error {
	byCollection := make(map[string][]int64)
	for _, o := range orphans {
		byCollection[o.Collection] = append(byCollection[o.Collection], o.ChunkID)
	}

	for coll, ids := range byCollection {
		vdb, err := proj.OpenCollectionVector(coll, embed.Dim())
		if err != nil {
			util.Logger.Warn().Err(err).Str("collection", coll).Msg("failed to open vector db for orphan removal")
			continue
		}

		if err := vdb.RemoveIDs(ids); err != nil {
			util.Logger.Warn().Err(err).Str("collection", coll).Int("count", len(ids)).Msg("failed to remove orphan vectors")
			continue
		}

		if err := vdb.Save(); err != nil {
			util.Logger.Warn().Err(err).Str("collection", coll).Msg("failed to save vector index after orphan removal")
			continue
		}

		if err := db.DeleteVectorizedChunkIDs(ids); err != nil {
			util.Logger.Warn().Err(err).Int("count", len(ids)).Msg("failed to delete orphan vector tracking entries")
		}

		util.Logger.Debug().Str("collection", coll).Int("count", len(ids)).Msg("removed orphan vectors")
	}

	return nil
}
