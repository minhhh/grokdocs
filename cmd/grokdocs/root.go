package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

const DefaultStartDir = "."

var (
	projectPath string
	logLevel    string
	logFormat   string
)

var rootCmd = &cobra.Command{
	Use:   "grokdocs",
	Short: "grokdocs is a local-first documentation and code indexer",
	Long:  `grokdocs is a local-first search engine that indexes your Markdown and code files for semantic and full-text search.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var fmtVal util.LogFormat
		if strings.ToLower(logFormat) == "json" {
			fmtVal = util.FormatJSON
		} else {
			fmtVal = util.FormatText
		}
		util.InitLogger(os.Stderr, logLevel, fmtVal)
	},
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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format (text, json)")
}
