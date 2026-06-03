package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var searchCollection string

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Perform a hybrid semantic search",
	Long:  `Queries the FAISS index and SQLite to find the most relevant chunks in your documentation.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		if searchCollection != "" {
			fmt.Printf("grokdocs search called for collection '%s' (projectPath: '%s') with query: '%s'\n", searchCollection, projectPath, query)
		} else {
			fmt.Printf("grokdocs search called (projectPath: '%s') with query: '%s'\n", projectPath, query)
		}
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchCollection, "collection", "", "Limit search query to the specified collection")
	rootCmd.AddCommand(searchCmd)
}
