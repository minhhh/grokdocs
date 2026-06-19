package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		syncAll        bool
		syncCollection string
		syncPrune      bool
		syncConcurrency int
	)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize files with the database",
	Long:  `Scan folders and synchronize files into SQLite and the FAISS index.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		// Validate mutual exclusivity
		if syncAll && syncCollection != "" {
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

		if syncConcurrency < 1 {
			util.Logger.Error().Int("concurrency", syncConcurrency).Msg("--concurrency must be at least 1")
			os.Exit(1)
		}

		var targets []string
		if syncAll {
			for name := range proj.Config.Collections {
				targets = append(targets, name)
			}
		} else if syncCollection != "" {
			project.AssertCollectionValid(proj, syncCollection)
			targets = []string{syncCollection}
		} else {
			targets = []string{config.DefaultCollectionName}
		}

		initEmbedder()
		defer closeEmbedder()

		for _, coll := range targets {
			bar := progressbar.NewOptions(-1,
				progressbar.OptionSetDescription("Processing files"),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionShowCount(),
				progressbar.OptionThrottle(100*time.Millisecond),
			)

			progress := util.NewGuardedChan[ingest.SyncProgress](0)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				for progressUpdate := range progress.Ch() {
					if progressUpdate.TotalFiles > 0 {
						bar.ChangeMax(progressUpdate.TotalFiles)
						if progressUpdate.FilesProcessed == progressUpdate.TotalFiles {
							bar.Finish()
							break
						}
					}
					bar.Set(progressUpdate.FilesProcessed)
				}
				fmt.Fprintln(os.Stderr)
			}()

			result, err := ingest.SyncCollection(proj, coll, progress, syncPrune, syncConcurrency)
			progress.Close()
			wg.Wait()

			if err != nil {
				os.Exit(1)
			}

			absPath := filepath.Join(proj.RootPath, proj.Config.Collections[coll].Path)
			fmt.Fprintf(os.Stderr, "Synced collection %q (%s):\n", coll, absPath)
			fmt.Fprintf(os.Stderr, "  unchanged: %d\n", result.Unchanged)
			fmt.Fprintf(os.Stderr, "  added:     %d\n", result.Added)
			fmt.Fprintf(os.Stderr, "  modified:  %d\n", result.Modified)
			fmt.Fprintf(os.Stderr, "  moved:     %d\n", result.Moved)
			fmt.Fprintf(os.Stderr, "  deleted:   %d\n", result.Deleted)
		}
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Synchronize all configured collections")
	syncCmd.Flags().StringVarP(&syncCollection, "collection", "c", "", "Synchronize only the specified collection")
	syncCmd.Flags().BoolVar(&syncPrune, "prune", false, "Remove orphaned file records (files deleted from disk since last sync)")
	syncCmd.Flags().IntVar(&syncConcurrency, "concurrency", 1, "Number of files to process concurrently")
	rootCmd.AddCommand(syncCmd)
}
