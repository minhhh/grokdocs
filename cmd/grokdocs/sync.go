package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	syncAll        bool
	syncCollection string
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

		var targets []string
		if syncAll {
			for name := range proj.Config.Collections {
				targets = append(targets, name)
			}
		} else if syncCollection != "" {
			targets = []string{syncCollection}
		} else {
			targets = []string{"default"}
		}

		for _, coll := range targets {
			bar := progressbar.NewOptions(-1,
				progressbar.OptionSetDescription("Processing files"),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionShowCount(),
				progressbar.OptionThrottle(100*time.Millisecond),
			)

			progress := make(chan ingest.SyncProgress)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				var totalFiles int
				for progressUpdate := range progress {
					if progressUpdate.TotalFiles > 0 {
						bar.ChangeMax(progressUpdate.TotalFiles)
					}
					bar.Set(progressUpdate.FilesProcessed)
					totalFiles = progressUpdate.FilesProcessed
				}
				bar.ChangeMax(totalFiles)
				bar.Set(totalFiles)
				bar.Finish()
				fmt.Fprintln(os.Stderr)
			}()

			result, err := ingest.SyncCollection(proj, coll, progress)
			close(progress)
			wg.Wait()

			if err != nil {
				os.Exit(1)
			}

			fmt.Fprintln(os.Stderr, "Sync complete:")
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
	syncCmd.Flags().StringVar(&syncCollection, "collection", "", "Synchronize only the specified collection")
	rootCmd.AddCommand(syncCmd)
}
