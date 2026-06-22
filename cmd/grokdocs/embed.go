package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	embedAll         bool
	embedCollection  string
	embedConcurrency int
	embedPrune       bool
	embedRebuild     bool
)

// embedCollectionFn is set by onnx-enabled builds; nil otherwise.
var embedCollectionFn func(proj *project.Project, db *project.FTSDatabase, collection string, chunkIDs []int64, textContents []string, concurrency int, rebuild bool, progress *util.GuardedChan[ingest.SyncProgress]) (int, error)

// pruneOrphansFn is set by onnx-enabled builds; nil otherwise.
var pruneOrphansFn func(proj *project.Project, db *project.FTSDatabase, orphans []project.VectorChunkOrphan) error

var embedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Compute and store vector embeddings",
	Long:  `Embed chunks for a collection that are missing vector embeddings, and optionally remove orphaned vectors.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if embedAll && embedCollection != "" {
			util.Logger.Error().Msg("--all and --collection flags are mutually exclusive")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		startDir := projectPath
		if startDir == "" {
			startDir = DefaultStartDir
		}
		proj, err := project.FindProject(startDir)
		if err != nil {
			os.Exit(1)
		}
		if err := proj.Init(); err != nil {
			os.Exit(1)
		}
		defer proj.Close()

		if embedConcurrency < 1 {
			util.Logger.Error().Int("concurrency", embedConcurrency).Msg("--concurrency must be at least 1")
			os.Exit(1)
		}

		if embedCollectionFn == nil {
			util.Logger.Error().Msg("embedding unavailable (compile with -tags onnx)")
			os.Exit(1)
		}

		if err := initEmbedder(); err != nil {
			util.Logger.Error().Err(err).Msg("failed to initialize embedder")
			os.Exit(1)
		}
		defer closeEmbedder()

		var targets []string
		if embedAll {
			for name := range proj.Config.Collections {
				targets = append(targets, name)
			}
		} else if embedCollection != "" {
			project.AssertCollectionValid(proj, embedCollection)
			targets = []string{embedCollection}
		} else {
			targets = []string{config.DefaultCollectionName}
		}

		db, err := proj.OpenFTS()
		if err != nil {
			util.Logger.Error().Err(err).Msg("failed to open database")
			os.Exit(1)
		}

		for _, coll := range targets {
			rows, err := db.DB().Query(`
				SELECT c.id, c.text_content
				FROM chunks c
				JOIN documents d ON c.document_id = d.id
				WHERE d.collection = ?`, coll)
			if err != nil {
				util.Logger.Error().Err(err).Str("collection", coll).Msg("failed to query chunks")
				os.Exit(1)
			}

			var allIDs []int64
			var allTexts []string
			for rows.Next() {
				var id int64
				var text string
				if err := rows.Scan(&id, &text); err != nil {
					util.Logger.Error().Err(err).Msg("failed to scan chunk row")
					rows.Close()
					os.Exit(1)
				}
				allIDs = append(allIDs, id)
				allTexts = append(allTexts, text)
			}
			rows.Close()

			if len(allIDs) == 0 {
				fmt.Fprintf(os.Stderr, "Collection %q: no chunks to embed\n", coll)
				continue
			}

			vectorizedIDs, err := db.GetVectorizedChunkIDs(coll)
			if err != nil {
				util.Logger.Error().Err(err).Str("collection", coll).Msg("failed to query vectorized chunks")
				os.Exit(1)
			}

			vectorized := make(map[int64]bool, len(vectorizedIDs))
			for _, id := range vectorizedIDs {
				vectorized[id] = true
			}

			var toEmbedIDs []int64
			var toEmbedTexts []string
			for i, id := range allIDs {
				if embedRebuild || !vectorized[id] {
					toEmbedIDs = append(toEmbedIDs, id)
					toEmbedTexts = append(toEmbedTexts, allTexts[i])
				}
			}

			if embedRebuild {
				if err := db.ClearCollectionVectors(coll); err != nil {
					util.Logger.Error().Err(err).Str("collection", coll).Msg("failed to clear vector tracking")
					os.Exit(1)
				}
			}

			if len(toEmbedIDs) == 0 {
				fmt.Fprintf(os.Stderr, "Collection %q: all %d chunks already embedded\n", coll, len(allIDs))
				continue
			}

			fmt.Fprintf(os.Stderr, "Collection %q: embedding %d/%d chunks\n", coll, len(toEmbedIDs), len(allIDs))

			bar := progressbar.NewOptions(-1,
				progressbar.OptionSetDescription("Embedding"),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionShowCount(),
				progressbar.OptionThrottle(100*time.Millisecond),
			)

			progress := util.NewGuardedChan[ingest.SyncProgress](10)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				for update := range progress.Ch() {
					if update.TotalFiles > 0 {
						bar.ChangeMax(update.TotalFiles)
						bar.Set(update.FilesProcessed)
						if update.FilesProcessed == update.TotalFiles {
							bar.Finish()
							fmt.Fprintln(os.Stderr)
						}
					}
				}
			}()

			embedded, err := embedCollectionFn(proj, db, coll, toEmbedIDs, toEmbedTexts, embedConcurrency, embedRebuild, progress)
			progress.Close()
			wg.Wait()

			if err != nil {
				util.Logger.Error().Err(err).Str("collection", coll).Msg("embedding failed")
				os.Exit(1)
			}

			fmt.Fprintf(os.Stderr, "Collection %q: embedded %d chunks\n", coll, embedded)
		}

		if !embedPrune {
			return
		}

		orphans, err := db.GetOrphanedVectorChunks()
		if err != nil {
			util.Logger.Error().Err(err).Msg("failed to query orphaned vectors")
			os.Exit(1)
		}
		if len(orphans) == 0 {
			fmt.Fprintf(os.Stderr, "No orphaned vectors found\n")
			return
		}

		if pruneOrphansFn == nil {
			util.Logger.Warn().Msg("orphan vector removal unavailable (compile with -tags onnx)")
			return
		}

		fmt.Fprintf(os.Stderr, "Removing %d orphaned vectors\n", len(orphans))
		if err := pruneOrphansFn(proj, db, orphans); err != nil {
			util.Logger.Error().Err(err).Msg("orphan removal failed")
			os.Exit(1)
		}
	},
}

func init() {
	embedCmd.Flags().BoolVar(&embedAll, "all", false, "Embed all configured collections")
	embedCmd.Flags().StringVarP(&embedCollection, "collection", "c", "", "Embed only the specified collection")
	embedCmd.Flags().IntVar(&embedConcurrency, "concurrency", 1, "Number of chunks to embed concurrently")
	embedCmd.Flags().BoolVar(&embedPrune, "prune", true, "Remove orphaned vectors after embedding")
	embedCmd.Flags().BoolVar(&embedRebuild, "rebuild", false, "Clear and re-embed all chunks")
	rootCmd.AddCommand(embedCmd)
}
