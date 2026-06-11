package parser

import (
	"path/filepath"
	"strings"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
)

const DefaultChunkMaxSize = 500

// SectionHeader represents a parsed Markdown section header.
type SectionHeader struct {
	Title      string
	LineNumber int
}

// ParsedDocument contains document slug, metadata JSON string, and chunks.
type ParsedDocument struct {
	Slug     string
	Metadata string
	Chunks   []*project.ChunkRecord
}

// Parser defines the behavior for a parser implementation.
type Parser interface {
	Parse(relPath string, content string, fileSize int64) (*ParsedDocument, error)
}

// Global registry of parsers
var parserRegistry = make(map[string]Parser)

func RegisterParser(name string, p Parser) {
	parserRegistry[name] = p
}

func GetParser(name string) (Parser, bool) {
	p, ok := parserRegistry[name]
	return p, ok
}

func parseHeaders(content string) []SectionHeader {
	lines := strings.Split(content, "\n")
	var headers []SectionHeader
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			hCount := 0
			for hCount < len(trimmed) && trimmed[hCount] == '#' {
				hCount++
			}
			if hCount < len(trimmed) && (trimmed[hCount] == ' ' || trimmed[hCount] == '\t') {
				title := strings.TrimSpace(trimmed[hCount:])
				headers = append(headers, SectionHeader{
					Title:      title,
					LineNumber: lineNum,
				})
			}
		}
	}
	return headers
}

// Match priority ranking
type MatchPriority int

const (
	PriorityNone MatchPriority = iota
	PriorityWildcard
	PriorityExtension
	PriorityComplexExtension
	PriorityExactFile
)

func getMatchPriority(pattern string) MatchPriority {
	if !strings.HasPrefix(pattern, ".") && !strings.ContainsAny(pattern, "*?[]") {
		return PriorityExactFile
	}
	if strings.HasPrefix(pattern, ".") && strings.Count(pattern, ".") > 1 {
		return PriorityComplexExtension
	}
	if strings.HasPrefix(pattern, ".") && strings.Count(pattern, ".") == 1 {
		return PriorityExtension
	}
	return PriorityWildcard
}

func matchesPattern(path string, pattern string) bool {
	base := filepath.Base(path)
	patternLower := strings.ToLower(pattern)
	pathLower := strings.ToLower(path)
	baseLower := strings.ToLower(base)

	if strings.HasPrefix(patternLower, ".") {
		return strings.HasSuffix(pathLower, patternLower)
	}

	if strings.ContainsAny(patternLower, "*?[]") {
		matched, err := filepath.Match(patternLower, baseLower)
		return err == nil && matched
	}

	return baseLower == patternLower
}

// defaultParserMapping maps file extensions to parser names when no
// collection-level parsers are configured. Extensions not listed here
// fall back to the chunkx auto-detection parser.
var defaultParserMapping = map[string]string{
	".md":         "markdown",
	".markdown":   "markdown",
	".bash":       "chunkx",
	".c":          "chunkx",
	".cc":         "chunkx",
	".cjs":        "chunkx",
	".cpp":        "chunkx",
	".cs":         "chunkx",
	".css":        "chunkx",
	".cxx":        "chunkx",
	".dockerfile": "chunkx",
	".elm":        "chunkx",
	".ex":         "chunkx",
	".exs":        "chunkx",
	".go":         "chunkx",
	".gradle":     "chunkx",
	".groovy":     "chunkx",
	".h":          "chunkx",
	".hcl":        "chunkx",
	".hh":         "chunkx",
	".hpp":        "chunkx",
	".htm":        "chunkx",
	".html":       "chunkx",
	".hxx":        "chunkx",
	".java":       "chunkx",
	".js":         "chunkx",
	".jsx":        "chunkx",
	".kt":         "chunkx",
	".kts":        "chunkx",
	".lua":        "chunkx",
	".mjs":        "chunkx",
	".ml":         "chunkx",
	".mli":        "chunkx",
	".php":        "chunkx",
	".phtml":      "chunkx",
	".proto":      "chunkx",
	".py":         "chunkx",
	".pyi":        "chunkx",
	".pyw":        "chunkx",
	".rake":       "chunkx",
	".rb":         "chunkx",
	".rs":         "chunkx",
	".sc":         "chunkx",
	".scala":      "chunkx",
	".sh":         "chunkx",
	".sql":        "chunkx",
	".svelte":     "chunkx",
	".swift":      "chunkx",
	".tf":         "chunkx",
	".toml":       "chunkx",
	".ts":         "chunkx",
	".tsx":        "chunkx",
	".yaml":       "chunkx",
	".yml":        "chunkx",
	"Dockerfile":  "chunkx",
	".gemspec":    "chunkx",
}

func ResolveParserName(cfg *config.Config, collectionName string, path string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	collCfg, ok := cfg.Collections[collectionName]
	if !ok {
		return "", false
	}

	type matchCandidate struct {
		pattern    string
		parserName string
		priority   MatchPriority
	}

	var candidates []matchCandidate

	for pattern, parserName := range collCfg.Parsers {
		if matchesPattern(path, pattern) {
			candidates = append(candidates, matchCandidate{
				pattern:    pattern,
				parserName: parserName,
				priority:   getMatchPriority(pattern),
			})
		}
	}

	if len(candidates) == 0 {
		if parserName, ok := defaultParserMapping[filepath.Ext(path)]; ok {
			return parserName, true
		}
		if parserName, ok := defaultParserMapping[filepath.Base(path)]; ok {
			return parserName, true
		}
		return "", false
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.priority > best.priority {
			best = c
		} else if c.priority == best.priority {
			if len(c.pattern) > len(best.pattern) {
				best = c
			}
		}
	}

	return best.parserName, true
}
