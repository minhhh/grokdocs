package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minhhh/grokdocs/internal/project"
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
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := proj.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing project: %v\n", err)
			os.Exit(1)
		}

		if searchMode != "fts" && searchMode != "hybrid" && searchMode != "semantics" {
			fmt.Fprintf(os.Stderr, "Error: invalid search mode %q. Allowed modes are: fts, semantics, hybrid\n", searchMode)
			os.Exit(1)
		}

		if searchMode != "fts" {
			fmt.Printf("Warning: mode %q is not fully implemented in this phase. Falling back to 'fts' mode.\n", searchMode)
		}

		db, err := proj.OpenFTS()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer proj.Close()

		results, err := db.SearchFTS(query, searchCollection, searchLimit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No matches found.")
			return
		}

		fmt.Printf("Found %d matches for %q:\n\n", len(results), query)
		for idx, res := range results {
			var filePath string
			_ = db.DB().QueryRow("SELECT f.file_path FROM files f JOIN documents d ON f.id = d.file_id WHERE d.id = ?", res.Chunk.DocumentID).Scan(&filePath)

			fmt.Printf("[%d] File: %s (Lines: %d-%d) | Section: %s (Score: %.4f)\n",
				idx+1, filePath, res.Chunk.LineStart, res.Chunk.LineEnd, res.Chunk.SectionTitle, res.Rank)

			// Try to read lines from file if exists, fallback to database cached chunk text
			fullPath := filepath.Join(proj.RootPath, filePath)
			fileLines, err := readLinesOfFile(fullPath, res.Chunk.LineStart, res.Chunk.LineEnd)
			if err == nil {
				fmt.Println(fileLines)
			} else {
				fmt.Println(res.Chunk.TextContent)
			}
			fmt.Println(strings.Repeat("-", 60))
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
		return "", fmt.Errorf("start line out of bounds")
	}
	if end > len(lines) {
		end = len(lines)
	}
	if end < start {
		return "", fmt.Errorf("invalid line range")
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}

func init() {
	searchCmd.Flags().StringVar(&searchCollection, "collection", "", "Limit search query to the specified collection")
	searchCmd.Flags().StringVar(&searchMode, "mode", "hybrid", "Search mode (fts, semantics, hybrid)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 5, "Maximum number of search results to return")
	rootCmd.AddCommand(searchCmd)
}
