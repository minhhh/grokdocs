package main

import (
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
			os.Exit(1)
		}
		if err := proj.Init(); err != nil {
			os.Exit(1)
		}
		defer proj.Close()

		db, err := proj.OpenFTS()
		if err != nil {
			os.Exit(1)
		}

		stats, err := db.GetStats()
		if err != nil {
			os.Exit(1)
		}

		util.Logger.Info().Str("path", proj.RootPath).Msg("Project Root")
		util.Logger.Info().Str("path", proj.ConfigDir).Msg("Config Directory")
		util.Logger.Info().Str("path", filepath.Join(proj.ConfigDir, "grokdocs.db")).Msg("Database File")
		util.Logger.Info().Int64("files", stats.TotalFiles).Int64("chunks", stats.TotalChunks).Int64("chars", stats.TotalChars).Msg("Database Statistics")

		for name := range proj.Config.Collections {
			count := stats.DocsPerCollection[name]
			util.Logger.Info().Str("collection", name).Int64("documents", count).Msg("collection")
		}

		// Show any collections in DB that are no longer in config
		for name, count := range stats.DocsPerCollection {
			if _, ok := proj.Config.Collections[name]; !ok {
				util.Logger.Warn().Str("collection", name).Int64("documents", count).Msg("collection in database but not in config.yaml")
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
			os.Exit(1)
		}
		util.Logger.Info().Msg(proj.ConfigDir)
	},
}

func init() {
	statusCmd.AddCommand(statusRootCmd)
	rootCmd.AddCommand(statusCmd)
}
