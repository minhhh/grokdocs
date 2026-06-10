package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/project"
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
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		configPath := filepath.Join(proj.ConfigDir, project.ConfigFileName)
		_, alreadyInit := os.Stat(configPath)

		if err := proj.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing project: %v\n", err)
			os.Exit(1)
		}

		if alreadyInit == nil {
			fmt.Printf("Project already initialized in %s\n", proj.ConfigDir)
		} else {
			fmt.Printf("Initialized empty grokdocs project in %s\n", proj.ConfigDir)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
