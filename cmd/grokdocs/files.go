package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var (
	filesAll        bool
	filesCollection string
	filesLimit      int
	filesOffset     int
)

func runFiles(startDir string) error {
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

	var targets []string
	if filesAll {
		for name := range proj.Config.Collections {
			targets = append(targets, name)
		}
	} else if filesCollection != "" {
		project.AssertCollectionValid(proj, filesCollection)
		targets = []string{filesCollection}
	} else {
		targets = []string{config.DefaultCollectionName}
	}

	for _, coll := range targets {
		results, total, err := db.ListCollectionFilesPaginated(coll, filesLimit, filesOffset)
		if err != nil {
			util.Logger.Error().Err(err).Str("collection", coll).Msg("failed to list files")
			return err
		}

		absPath := filepath.Join(proj.RootPath, proj.Config.Collections[coll].Path)
		fmt.Printf("Collection %q (%s): %d files\n", coll, absPath, total)

		if filesLimit > 0 && (filesOffset > 0 || filesLimit < total) {
			end := filesOffset + len(results)
			fmt.Printf("  Showing %d-%d of %d\n", filesOffset+1, end, total)
		}

		for _, r := range results {
			fmt.Printf("  %s  (%d chunks, %d chars)\n", r.FilePath, r.ChunkCount, r.TotalChars)
		}
	}
	return nil
}

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "List files in a collection",
	Long:  `Lists all files indexed in one or more collections, with pagination support.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if filesAll && filesCollection != "" {
			util.Logger.Error().Msg("--all and --collection flags are mutually exclusive")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runFiles(projectPath); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	filesCmd.Flags().BoolVar(&filesAll, "all", false, "List files in all configured collections")
	filesCmd.Flags().StringVarP(&filesCollection, "collection", "c", "", "List files in the specified collection")
	filesCmd.Flags().IntVar(&filesLimit, "limit", 0, "Maximum number of files to list (0 = unlimited)")
	filesCmd.Flags().IntVar(&filesOffset, "offset", 0, "Number of files to skip")

	_ = filesCmd.RegisterFlagCompletionFunc("collection", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listCollectionNames(), cobra.ShellCompDirectiveDefault
	})

	rootCmd.AddCommand(filesCmd)
}

func listCollectionNames() []string {
	startDir := projectPath
	if startDir == "" {
		startDir = DefaultStartDir
	}
	proj, err := project.FindProject(startDir)
	if err != nil {
		return nil
	}
	if err := proj.Init(); err != nil {
		return nil
	}
	defer proj.Close()
	names := make([]string, 0, len(proj.Config.Collections))
	for name := range proj.Config.Collections {
		names = append(names, name)
	}
	return names
}
