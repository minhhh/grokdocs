package main

import (
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

func runStatus(startDir string) error {
	if startDir == "" {
		startDir = DefaultStartDir
	}
	proj, err := project.FindProject(startDir)
	if err != nil {
		return err
	}
	if err := proj.Init(); err != nil {
		return err
	}
	defer proj.Close()

	db, err := proj.OpenFTS()
	if err != nil {
		return err
	}

	stats, err := db.GetStats()
	if err != nil {
		return err
	}

	util.Logger.Info().Msgf("Project root:  %s", proj.RootPath)
	util.Logger.Info().Msgf("Config dir:    %s", proj.ConfigDir)
	util.Logger.Info().Msgf("FTS Database:  %s", filepath.Join(proj.ConfigDir, "grokdocs.db"))
	util.Logger.Info().Msg("")
	util.Logger.Info().Msgf("Files:         %d", stats.TotalFiles)
	util.Logger.Info().Msgf("Documents:     %d", stats.TotalDocuments)
	util.Logger.Info().Msgf("Chunks:        %d", stats.TotalChunks)
	util.Logger.Info().Msgf("Total chars:   %d", stats.TotalChars)
	util.Logger.Info().Msg("")

	util.Logger.Info().Msgf("Collections (%d):", stats.CollectionsCount)
	for name := range proj.Config.Collections {
		docs := stats.DocsPerCollection[name]
		chunks := stats.ChunksPerCollection[name]
		util.Logger.Info().Msgf("  %s: %d documents, %d chunks", name, docs, chunks)
	}
	for name, docs := range stats.DocsPerCollection {
		if _, ok := proj.Config.Collections[name]; !ok {
			chunks := stats.ChunksPerCollection[name]
			util.Logger.Warn().Msgf("  %s: %d documents, %d chunks (in database but not in config.yaml)", name, docs, chunks)
		}
	}
	return nil
}

var statusCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show indexing status and statistics",
	Long:  `Displays indexed statistics (collections count, documents per collection, total chunks, total chars).`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(projectPath); err != nil {
			os.Exit(1)
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
