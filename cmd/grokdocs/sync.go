package main

import (
	"fmt"
	"os"

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
		if syncAll {
			fmt.Printf("grokdocs sync called with --all (projectPath: '%s'): synchronizing all collections...\n", projectPath)
		} else if syncCollection != "" {
			fmt.Printf("grokdocs sync called for collection '%s' (projectPath: '%s'): synchronizing...\n", syncCollection, projectPath)
		} else {
			fmt.Printf("grokdocs sync called with no flags (projectPath: '%s'): synchronizing the 'default' collection...\n", projectPath)
		}
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Synchronize all configured collections")
	syncCmd.Flags().StringVar(&syncCollection, "collection", "", "Synchronize only the specified collection")
	rootCmd.AddCommand(syncCmd)
}
