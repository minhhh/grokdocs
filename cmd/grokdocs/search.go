package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var (
	searchCollection string
	searchMode       string
	searchLimit      int
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Perform a search",
	Long:  `Queries the SQLite FTS5 database to find matching text chunks.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		startDir := projectPath
		if startDir == "" {
			startDir = DefaultStartDir
		}
		proj, err := project.FindProject(startDir)
		if err != nil {
			os.Exit(1)
		}
		if err := proj.Init(); err != nil {
			os.Exit(1)
		}

		if searchMode != "fts" && searchMode != "hybrid" && searchMode != "semantics" {
			util.Logger.Error().Str("mode", searchMode).Msg("invalid search mode")
			os.Exit(1)
		}

		if searchMode != "fts" {
			util.Logger.Warn().Str("mode", searchMode).Msg("search mode not fully implemented, falling back to fts")
		}

		db, err := proj.OpenFTS()
		if err != nil {
			util.Logger.Error().Err(err).Msg("opening database")
			os.Exit(1)
		}
		defer proj.Close()

		results, err := db.SearchFTS(query, searchCollection, searchLimit * 5)
		if err != nil {
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No matches found.")
			return
		}

		// First pass: collect file_path for each result
		type pathResult struct {
			filePath string
			result   *project.FTSResult
		}
		enriched := make([]pathResult, len(results))
		for i, result := range results {
			filePath, _ := db.GetFilePathByDocumentID(result.DocumentID)
			enriched[i] = pathResult{filePath: filePath, result: result}
		}

		// Group by file_path, preserving rank order within each file
		groups := make(map[string][]*project.FTSResult)
		order := []string{}
		for _, pr := range enriched {
			if _, ok := groups[pr.filePath]; !ok {
				order = append(order, pr.filePath)
			}
			groups[pr.filePath] = append(groups[pr.filePath], pr.result)
		}

		if len(order) > searchLimit {
			order = order[:searchLimit]
		}

		for file_group_order, fp := range order {
			fmt.Printf("\n[%d] %s: - %d chunks\n", file_group_order+1, fp, len(groups[fp]))
			for i, result := range groups[fp] {
				fmt.Printf("  [%d] %s [L%d-L%d] - score: %f\n", i+1, result.Slug, result.LineStart, result.LineEnd, result.Rank)
				fmt.Printf("  %s\n", result.Snippet)
			}
		}
	},
}

func readLinesOfFile(path string, start, end int) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(bytes), "\n")
	if start < 1 || start > len(lines) {
		util.Logger.Error().Int("start", start).Int("total_lines", len(lines)).Msg("start line out of bounds")
		return "", errors.New("start line out of bounds")
	}
	if end > len(lines) {
		end = len(lines)
	}
	if end < start {
		util.Logger.Error().Int("start", start).Int("end", end).Msg("invalid line range")
		return "", errors.New("invalid line range")
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}

func init() {
	searchCmd.Flags().StringVar(&searchCollection, "collection", "", "Limit search query to the specified collection")
	searchCmd.Flags().StringVar(&searchMode, "mode", "hybrid", "Search mode (fts, semantics, hybrid)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 5, "Maximum number of search results to return")
	rootCmd.AddCommand(searchCmd)
}
