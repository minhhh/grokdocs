package main

import (
	"os"
	"strings"

	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

const DefaultStartDir = "."

// version is set at build time via ldflags.
var version = "0.0.1"

var (
	projectPath string
	verbose     bool
	logFormat   string
)

var rootCmd = &cobra.Command{
	Use:     "grokdocs",
	Short:   "grokdocs is a local-first documentation indexer",
	Long:    `grokdocs is a local-first search engine that indexes Markdown and source files for semantic and full-text search.`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var logFormatVal util.LogFormat
		if strings.ToLower(logFormat) == "json" {
			logFormatVal = util.FormatJSON
		} else {
			logFormatVal = util.FormatText
		}

		util.InitLogger(os.Stderr, verbose, logFormatVal)
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		util.Logger.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&projectPath, "project", "p", "", "Project root path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output with timestamps and TRACE log level")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format (text, json)")
}
