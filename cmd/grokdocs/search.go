package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"math"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var (
	searchCollection      string
	searchMode            string
	searchLimit           int
	rrfK                  float64
	searchGroupMultiplier = 5
)

// semanticSearchFn is set by onnx-enabled builds; nil otherwise.
var semanticSearchFn func(proj *project.Project, ftsDB *project.FTSDatabase, query, collection string, limit int) ([]*project.SearchResult, error)

func runSearch(query, startDir string) error {
	if startDir == "" {
		startDir = DefaultStartDir
	}
	proj, err := project.FindProject(startDir)
	if err != nil {
		return err
	}
	if err := proj.Init(); err != nil {
		return err
	}

	switch searchMode {
	case "fts", "semantic", "hybrid":
	default:
		return fmt.Errorf("invalid search mode %q (use: fts, semantic, hybrid)", searchMode)
	}

	db, err := proj.OpenFTS()
	if err != nil {
		return err
	}
	defer proj.Close()

	limit := searchLimit * searchGroupMultiplier
	var results []*project.SearchResult

	if searchCollection == "" {
		searchCollection = config.DefaultCollectionName
	} else {
		project.AssertCollectionValid(proj, searchCollection)
	}

	if err := initEmbedder(); err != nil {
		util.Logger.Error().Err(err).Msg("failed to initialize embedder")
		return nil
	}
	defer closeEmbedder()

	flatDisplay := false
	switch searchMode {
	case "fts":
		results, err = db.SearchFTS(query, searchCollection, limit)
	case "semantic":
		if semanticSearchFn == nil {
			util.Logger.Warn().Msg("semantic search unavailable (compile with -tags onnx); falling back to fts")
			results, err = db.SearchFTS(query, searchCollection, limit)
		} else {
			results, err = semanticSearchFn(proj, db, query, searchCollection, limit)
			flatDisplay = true
		}
	case "hybrid":
		ftsResults, ftsErr := db.SearchFTS(query, searchCollection, limit)
		if ftsErr != nil {
			err = ftsErr
			break
		}
		var semanticResults []*project.SearchResult
		if semanticSearchFn != nil {
			var semErr error
			semanticResults, semErr = semanticSearchFn(proj, db, query, searchCollection, limit)
			if semErr != nil {
				util.Logger.Warn().Err(semErr).Msg("semantic search failed, using FTS only")
				results = ftsResults
				break
			}
		} else {
			util.Logger.Warn().Msg("semantic search unavailable (compile with -tags onnx); using FTS only")
			results = ftsResults
			break
		}
		results = mergeHybridResults(ftsResults, semanticResults, limit)
		flatDisplay = true
	}

	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No matches found.")
		return nil
	}

	displayResults(db, proj.RootPath, results, searchLimit, flatDisplay)
	return nil
}

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Perform a search",
	Long:  `Search indexed chunks using FTS5, semantic (vector), or hybrid mode.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		if err := runSearch(query, projectPath); err != nil {
			os.Exit(1)
		}
	},
}

func displayResults(db *project.FTSDatabase, rootPath string, results []*project.SearchResult, limit int, flat bool) {
	type pathResult struct {
		filePath string
		result   *project.SearchResult
	}
	enriched := make([]pathResult, 0, len(results))
	for _, result := range results {
		filePath, err := db.GetFilePathByDocumentID(result.DocumentID)
		if err != nil {
			util.Logger.Warn().Err(err).Int64("document_id", result.DocumentID).Msg("skipping result: failed to resolve file path")
			continue
		}
		enriched = append(enriched, pathResult{filePath: filePath, result: result})
	}

	if flat {
		if limit > len(enriched) {
			limit = len(enriched)
		}
		for order, pr := range enriched[:limit] {
			fmt.Printf("\n[%d] %s > %s [L%d-L%d] — score: %.3f (id: %s)\n",
				order + 1, pr.filePath, pr.result.SectionTitle, pr.result.LineStart, pr.result.LineEnd, pr.result.Rank, pr.result.Slug)
			fmt.Printf("  %s\n", pr.result.Snippet)
		}
		return
	}

	groups := make(map[string][]*project.SearchResult)
	fileGroups := []string{}
	for _, pr := range enriched {
		if _, ok := groups[pr.filePath]; !ok {
			fileGroups = append(fileGroups, pr.filePath)
		}
		groups[pr.filePath] = append(groups[pr.filePath], pr.result)
	}

	if len(fileGroups) > limit {
		fileGroups = fileGroups[:limit]
	}

	for fileGroupOrder, finalFilePath := range fileGroups {
		fmt.Printf("\n[%d] %s - %d chunks\n", fileGroupOrder+1, finalFilePath, len(groups[finalFilePath]))
		for i, result := range groups[finalFilePath] {
			fmt.Printf("  [%d] %s [L%d-L%d] - score: %.3f (%s)\n", i+1, result.SectionTitle, result.LineStart, result.LineEnd, result.Rank, result.Slug)
			fmt.Printf("  %s\n", result.Snippet)
		}
	}
}

// Sigmoid Boost go from 0 to 1. It rapidly decreases to 0 for rank < center
// This means we need to choose a good center to reflect the level of
// matching that we want
func sigmoidBoost(rank, center, kLeft, kRight float64) float64 {
	x := rank - center
	if x < 0 {
		return 1.0 / (1.0 + math.Exp(-kLeft * x))
	}
	return 1.0 / (1.0 + math.Exp(-kRight * x))
}

func mergeHybridResults(fts, semantic []*project.SearchResult, limit int) []*project.SearchResult {
	if len(fts) == 0 && len(semantic) == 0 {
		return nil
	}

	scores := make(map[int64]float64, len(fts)+len(semantic))
	seen := make(map[int64]*project.SearchResult, len(fts)+len(semantic))

	for rank, r := range fts {
		scores[r.ID] += 1.0 / (rrfK + float64(rank) + 1)
		boost := 0.1 * sigmoidBoost(r.Rank, 20, 5, 0.5)
		scores[r.ID] += boost / (rrfK + float64(rank) + 1)
		seen[r.ID] = r
	}
	for rank, r := range semantic {
		factor := sigmoidBoost(r.Rank, 0.6, 0.5, 5)
		scores[r.ID] += factor / (rrfK + float64(rank) + 1)
		if _, ok := seen[r.ID]; !ok {
			seen[r.ID] = r
		}
	}

	result := make([]*project.SearchResult, 0, len(seen))
	for id, r := range seen {
		r.Rank = scores[id]
		result = append(result, r)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Rank > result[j].Rank
	})
	if len(result) > limit {
		result = result[:limit]
	}

	return result
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
	searchCmd.Flags().StringVarP(&searchCollection, "collection", "c", "", "Limit search query to the specified collection")
	searchCmd.Flags().StringVarP(&searchMode, "mode", "m", "hybrid", "Search mode (fts, semantic, hybrid)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 5, "Maximum number of search results to return")
	searchCmd.Flags().Float64Var(&rrfK, "rrfk", 60, "RRF constant k for hybrid ranking")
	rootCmd.AddCommand(searchCmd)
}
