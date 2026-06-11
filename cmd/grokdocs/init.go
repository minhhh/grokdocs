package main

import (
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize workspace configuration",
	Long:  `Generates a default config.yaml configuration file inside the .grokdocs folder at the project root.`,
	Run: func(cmd *cobra.Command, args []string) {
		startDir := projectPath
		if startDir == "" {
			startDir = DefaultStartDir
		}
		proj, err := project.NewProject(startDir)
		if err != nil {
			util.Logger.Error().Err(err).Msg("creating project")
			os.Exit(1)
		}
		configPath := filepath.Join(proj.ConfigDir, project.ConfigFileName)
		_, alreadyInit := os.Stat(configPath)

		if err := proj.Init(); err != nil {
			util.Logger.Error().Err(err).Msg("initializing project")
			os.Exit(1)
		}

		if alreadyInit == nil {
			util.Logger.Info().Str("dir", proj.ConfigDir).Msg("Project already initialized")
		} else {
			util.Logger.Info().Str("dir", proj.ConfigDir).Msg("Initialized empty grokdocs project")
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
