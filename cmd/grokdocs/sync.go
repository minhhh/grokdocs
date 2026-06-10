package main

import (
	"fmt"
	"os"

	"github.com/minhhh/grokdocs/internal/ingest"
	"github.com/minhhh/grokdocs/internal/project"
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
			fmt.Fprintln(os.Stderr, "Error: --all and --collection flags are mutually exclusive")
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
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := proj.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing project: %v\n", err)
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
			fmt.Printf("Synchronizing collection %q...\n", coll)
			if err := ingest.SyncCollection(proj, coll); err != nil {
				fmt.Fprintf(os.Stderr, "Error synchronizing collection %q: %v\n", coll, err)
				os.Exit(1)
			}
		}
		fmt.Println("Sync completed successfully.")
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Synchronize all configured collections")
	syncCmd.Flags().StringVar(&syncCollection, "collection", "", "Synchronize only the specified collection")
	rootCmd.AddCommand(syncCmd)
}
