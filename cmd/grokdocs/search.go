package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"github.com/spf13/cobra"
)

var (
	searchCollection      string
	searchMode            string
	searchLimit           int
	hybridAlpha           float64
	searchGroupMultiplier = 5
)

// semanticSearchFn is set by onnx-enabled builds; nil otherwise.
var semanticSearchFn func(proj *project.Project, ftsDB *project.FTSDatabase, query, collection string, limit int) ([]*project.FTSResult, error)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Perform a search",
	Long:  `Search indexed chunks using FTS5, semantic (vector), or hybrid mode.`,
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

		switch searchMode {
		case "fts", "semantic", "hybrid":
		default:
			util.Logger.Error().Str("mode", searchMode).Msg("invalid search mode (use: fts, semantic, hybrid)")
			os.Exit(1)
		}

		db, err := proj.OpenFTS()
		if err != nil {
			util.Logger.Error().Err(err).Msg("opening database")
			os.Exit(1)
		}
		defer proj.Close()

		limit := searchLimit * searchGroupMultiplier
		var results []*project.FTSResult

		switch searchMode {
		case "fts":
			results, err = db.SearchFTS(query, searchCollection, limit)
		case "semantic":
			if semanticSearchFn == nil {
				util.Logger.Warn().Msg("semantic search unavailable (compile with -tags onnx); falling back to fts")
				results, err = db.SearchFTS(query, searchCollection, limit)
			} else {
				results, err = semanticSearchFn(proj, db, query, searchCollection, limit)
			}
		case "hybrid":
			ftsResults, ftsErr := db.SearchFTS(query, searchCollection, limit)
			if ftsErr != nil {
				util.Logger.Error().Err(ftsErr).Msg("FTS search failed")
				os.Exit(1)
			}
			var semanticResults []*project.FTSResult
			if semanticSearchFn != nil {
				var semErr error
				semanticResults, semErr = semanticSearchFn(proj, db, query, searchCollection, limit)
				if semErr != nil {
					util.Logger.Warn().Err(semErr).Msg("semantic search failed, using FTS only")
				}
			} else {
				util.Logger.Warn().Msg("semantic search unavailable (compile with -tags onnx); using FTS only")
			}
			results = mergeHybridResults(ftsResults, semanticResults, limit)
		}

		if err != nil {
			util.Logger.Error().Err(err).Msg("search failed")
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No matches found.")
			return
		}

		displayResults(db, results, searchLimit)
	},
}

func displayResults(db *project.FTSDatabase, results []*project.FTSResult, maxGroups int) {
	type pathResult struct {
		filePath string
		result   *project.FTSResult
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

	groups := make(map[string][]*project.FTSResult)
	order := []string{}
	for _, pr := range enriched {
		if _, ok := groups[pr.filePath]; !ok {
			order = append(order, pr.filePath)
		}
		groups[pr.filePath] = append(groups[pr.filePath], pr.result)
	}

	if len(order) > maxGroups {
		order = order[:maxGroups]
	}

	for fileGroupOrder, fp := range order {
		fmt.Printf("\n[%d] %s - %d chunks\n", fileGroupOrder+1, fp, len(groups[fp]))
		for i, result := range groups[fp] {
			fmt.Printf("  [%d] %s [L%d-L%d] - score: %f\n", i+1, result.Slug, result.LineStart, result.LineEnd, result.Rank)
			fmt.Printf("  %s\n", result.Snippet)
		}
	}
}

func mergeHybridResults(fts, semantic []*project.FTSResult, limit int) []*project.FTSResult {
	if len(fts) == 0 && len(semantic) == 0 {
		return nil
	}

	ftsScores := make(map[int64]float64, len(fts))
	semScores := make(map[int64]float64, len(semantic))

	if len(fts) > 0 {
		maxFTS := fts[0].Rank
		for _, r := range fts {
			if r.Rank > maxFTS {
				maxFTS = r.Rank
			}
		}
		if maxFTS == 0 {
			maxFTS = 1
		}
		for _, r := range fts {
			ftsScores[r.ID] = r.Rank / maxFTS
		}
	}
	for _, r := range semantic {
		semScores[r.ID] = r.Rank
	}

	seen := make(map[int64]bool, len(fts)+len(semantic))
	result := make([]*project.FTSResult, 0, len(fts)+len(semantic))

	for _, r := range fts {
		seen[r.ID] = true
		r.Rank = hybridAlpha*ftsScores[r.ID] + (1-hybridAlpha)*semScores[r.ID]
		result = append(result, r)
	}
	for _, r := range semantic {
		if !seen[r.ID] {
			seen[r.ID] = true
			r.Rank = (1 - hybridAlpha) * r.Rank
			result = append(result, r)
		}
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
	searchCmd.Flags().StringVar(&searchCollection, "collection", "", "Limit search query to the specified collection")
	searchCmd.Flags().StringVar(&searchMode, "mode", "hybrid", "Search mode (fts, semantic, hybrid)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 5, "Maximum number of search results to return")
	searchCmd.Flags().Float64Var(&hybridAlpha, "alpha", 0.5, "Hybrid weight for FTS vs semantic (0=semantic only, 1=FTS only)")
	rootCmd.AddCommand(searchCmd)
}
