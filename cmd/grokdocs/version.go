package main

import (
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of grokdocs",
	Run: func(cmd *cobra.Command, args []string) {
		util.Logger.Info().Msg(rootCmd.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
