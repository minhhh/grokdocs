package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const DefaultStartDir = "."

var projectPath string

var rootCmd = &cobra.Command{
	Use:   "grokdocs",
	Short: "grokdocs is a local-first documentation and code indexer",
	Long:  `grokdocs is a local-first search engine that indexes your Markdown and code files for semantic and full-text search.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&projectPath, "project", "p", "", "Project root path")
}
