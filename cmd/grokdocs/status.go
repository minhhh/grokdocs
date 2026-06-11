package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show indexing status and statistics",
	Long:  `Displays indexed statistics (collections count, documents per collection, total chunks, total chars).`,
	Run: func(cmd *cobra.Command, args []string) {
		startDir := projectPath
		if startDir == "" {
			startDir = DefaultStartDir
		}
		proj, err := project.FindProject(startDir)
		if err != nil {
			util.Logger.Error().Err(err).Msg("project not found")
			os.Exit(1)
		}
		if err := proj.Init(); err != nil {
			util.Logger.Error().Err(err).Msg("initializing project")
			os.Exit(1)
		}
		defer proj.Close()

		db, err := proj.OpenFTS()
		if err != nil {
			util.Logger.Error().Err(err).Msg("opening database")
			os.Exit(1)
		}

		stats, err := db.GetStats()
		if err != nil {
			util.Logger.Error().Err(err).Msg("querying statistics")
			os.Exit(1)
		}

		fmt.Printf("Project Root:      %s\n", proj.RootPath)
		fmt.Printf("Config Directory:  %s\n", proj.ConfigDir)
		fmt.Printf("Database File:     %s\n", filepath.Join(proj.ConfigDir, "grokdocs.db"))
		fmt.Println()
		fmt.Println("Database Statistics:")
		fmt.Printf("  Total Files Indexed: %d\n", stats.TotalFiles)
		fmt.Printf("  Total Chunks:        %d\n", stats.TotalChunks)
		fmt.Printf("  Total Characters:    %d\n", stats.TotalChars)
		fmt.Println()
		fmt.Printf("Collections status (%d configured):\n", len(proj.Config.Collections))
		for name := range proj.Config.Collections {
			count := stats.DocsPerCollection[name]
			fmt.Printf("  - %s: %d documents\n", name, count)
		}

		// Show any collections in DB that are no longer in config
		firstExtra := true
		for name, count := range stats.DocsPerCollection {
			if _, ok := proj.Config.Collections[name]; !ok {
				if firstExtra {
					fmt.Println()
					fmt.Println("Other collections in database (not in config.yaml):")
					firstExtra = false
				}
				fmt.Printf("  - %s: %d documents\n", name, count)
			}
		}
	},
}

var statusRootCmd = &cobra.Command{
	Use:   "root",
	Short: "Show the path to the active .grokdocs directory",
	Long:  `Prints the absolute path of the discovered .grokdocs directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		startDir := projectPath
		if startDir == "" {
			startDir = DefaultStartDir
		}
		proj, err := project.FindProject(startDir)
		if err != nil {
			util.Logger.Error().Err(err).Msg("project not found")
			os.Exit(1)
		}
		fmt.Println(proj.ConfigDir)
	},
}

func init() {
	statusCmd.AddCommand(statusRootCmd)
	rootCmd.AddCommand(statusCmd)
}
