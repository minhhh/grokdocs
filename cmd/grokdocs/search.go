package main

import (
	"errors"
	"os"
	"path/filepath"
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

		results, err := db.SearchFTS(query, searchCollection, searchLimit)
		if err != nil {
			os.Exit(1)
		}

		if len(results) == 0 {
			util.Logger.Info().Msg("No matches found.")
			return
		}

		util.Logger.Info().Int("count", len(results)).Str("query", query).Msg("search results")
		for idx, result := range results {
			var filePath string
			_ = db.DB().QueryRow("SELECT f.file_path FROM files f JOIN documents d ON f.id = d.file_id WHERE d.id = ?", result.Chunk.DocumentID).Scan(&filePath)

			// Try to read lines from file if exists, fallback to database cached chunk text
			fullPath := filepath.Join(proj.RootPath, filePath)
			fileLines, err := readLinesOfFile(fullPath, result.Chunk.LineStart, result.Chunk.LineEnd)
			if err == nil {
				util.Logger.Info().Int("idx", idx+1).Str("file", filePath).Int("lines_start", result.Chunk.LineStart).Int("lines_end", result.Chunk.LineEnd).Str("section", result.Chunk.SectionTitle).Float64("score", result.Rank).Msg(fileLines)
			} else {
				util.Logger.Info().Int("idx", idx+1).Str("file", filePath).Int("lines_start", result.Chunk.LineStart).Int("lines_end", result.Chunk.LineEnd).Str("section", result.Chunk.SectionTitle).Float64("score", result.Rank).Msg(result.Chunk.TextContent)
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
