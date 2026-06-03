package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize workspace configuration",
	Long:  `Generates a default grokdocs.yml configuration file in the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("grokdocs init called: initializing workspace configuration (projectPath: '%s')...\n", projectPath)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
