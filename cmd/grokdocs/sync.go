package main

import (
	"os"

	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
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
			if err := ingest.SyncCollection(proj, coll); err != nil {
				os.Exit(1)
			}
		}
		util.Logger.Info().Msg("Sync completed successfully.")
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Synchronize all configured collections")
	syncCmd.Flags().StringVar(&syncCollection, "collection", "", "Synchronize only the specified collection")
	rootCmd.AddCommand(syncCmd)
}
