//go:build onnx

package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minhhh/grokdocs/internal/embed"
	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"golang.org/x/sync/errgroup"
)

const vectorBatchSize = 100

func init() {
	embedCollectionFn = embedCollectionImpl
	pruneOrphansFn = pruneOrphans
}

type embedResult struct {
	id     int64
	vector []float32
	err    error
}

func embedCollectionImpl(ctx context.Context, proj *project.Project, db *project.FTSDatabase, collection string, chunkIDs []int64, textContents []string, concurrency int, rebuild bool, progress *util.GuardedChan[ingest.SyncProgress]) (int, error) {
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

	g, ctx := errgroup.WithContext(ctx)

	var chunkCount int32
	for w := 0; w < concurrency; w++ {
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("worker panicked: %v", r)
				}
			}()
			for idx := range work {
				if n := atomic.AddInt32(&chunkCount, 1); n%100 == 0 {
					util.Logger.Debug().Int32("processed", n).Int("total", len(chunkIDs)).Msg("embed progress")
				}
				text := textContents[idx]
				if len(strings.TrimSpace(text)) < 3 {
					select {
					case results <- embedResult{id: chunkIDs[idx]}:
					case <-ctx.Done():
						return ctx.Err()
					}
					continue
				}

				tStart := time.Now()
				vec, derr := embed.Embed(text)
				if elapsed := time.Since(tStart); elapsed > 500*time.Millisecond {
					util.Logger.Debug().Int64("chunk_id", chunkIDs[idx]).Dur("elapsed", elapsed).Msg("slow embed")
				}
				if derr != nil {
					preview := text
					if len([]rune(preview)) > 80 {
						preview = string([]rune(preview)[:80]) + "..."
					}
					util.Logger.Error().Err(derr).Int64("chunk_id", chunkIDs[idx]).Str("preview", preview).Msg("skipping chunk: embed failed")
					select {
					case results <- embedResult{id: chunkIDs[idx], err: derr}:
					case <-ctx.Done():
						return ctx.Err()
					}
					continue
				}
				select {
				case results <- embedResult{id: chunkIDs[idx], vector: vec}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	go func() {
		defer close(work)
		for i := range chunkIDs {
			select {
			case work <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	var (
		batchIDs  []int64
		batchVecs []float32
		embedded  int32
	)

	var flushPartial atomic.Bool
	flushPartial.Store(true)

	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()

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
				util.Logger.Debug().Str("collection", collection).Msg("Flush batch")
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

		if flushPartial.Load() && len(batchIDs) > 0 {
			util.Logger.Debug().Str("collection", collection).Msg("Flush last time")
			flushBatch(vdb, db, collection, batchIDs, batchVecs)
		}
	}()

	err = g.Wait()
	if err == nil {
		err = ctx.Err()
	}
	util.Logger.Debug().Err(err).Str("collection", collection).Msg("Finish Embedding")
	if err != nil {
		flushPartial.Store(false)
	}
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

	tStart := time.Now()
	util.Logger.Debug().Str("collection", collection).Int("batch_size", len(ids)).Msg("flushBatch: start")

	if err := vdb.RemoveIDs(ids); err != nil {
		util.Logger.Warn().Err(err).Int("batch", len(ids)).Msg("failed to remove existing vectors before batch add")
	}
	tAfterRemove := time.Now()

	if err := vdb.AddVectors(ids, vectors); err != nil {
		util.Logger.Error().Err(err).Int("batch", len(ids)).Msg("failed to add vectors batch")
		return
	}
	tAfterAdd := time.Now()

	if err := vdb.Save(); err != nil {
		util.Logger.Error().Err(err).Int("batch", len(ids)).Msg("failed to save vector index after batch")
		return
	}
	tAfterSave := time.Now()

	if err := db.MarkVectorized(ids, collection); err != nil {
		util.Logger.Warn().Err(err).Int("batch", len(ids)).Msg("failed to mark chunk batch as vectorized")
	}

	util.Logger.Debug().Str("collection", collection).Int("batch_size", len(ids)).
		Dur("remove_ms", tAfterRemove.Sub(tStart)).
		Dur("add_ms", tAfterAdd.Sub(tAfterRemove)).
		Dur("save_ms", tAfterSave.Sub(tAfterAdd)).
		Dur("total_ms", time.Since(tStart)).
		Msg("flushBatch: done")
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
