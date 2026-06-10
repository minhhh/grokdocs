package ingest

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomantics/chunkx"
	"github.com/gomantics/chunkx/languages"
	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
)

const (
	DefaultChunkMaxSize = 500
)

// defaultIncludeList contains glob patterns used when a collection has no
// explicit include or files field. These match common documentation and
// source file extensions by basename (e.g. *.md matches intro.md in any dir).
var defaultIncludeList = []string{"*.md", "*.markdown", "*.go", "*.py", "*.rs", "*.ts", "*.js", "*.yaml", "*.yml", "*.toml", "*.json", "*.txt"}

// defaultExcludeList contains patterns for files and directories that are
// excluded by default when no user-specified exclude list is configured.
// Patterns are matched against the basename using filepath.Match.
var defaultExcludeList = []string{
	// Directories
	"node_modules",
	"vendor",
	"venv",
	".venv",
	"env",
	".env",
	"__pycache__",
	"dist",
	"build",
	"target",
	".next",
	".nuxt",
	"out",
	"bin",
	"obj",
	"tmp",
	"temp",
	"CVS",

	// Files
	".DS_Store",
	"Thumbs.db",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
}

// SectionHeader represents a parsed Markdown section header.
type SectionHeader struct {
	Title      string
	LineNumber int
}

// ParsedDocument contains document slug, metadata JSON string, and chunks.
type ParsedDocument struct {
	Slug     string
	Metadata string // JSON-encoded string
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

// ChunkxParser wraps the gomantics/chunkx AST-based library.
type ChunkxParser struct {
	DefaultLanguage languages.LanguageName
}

func (cp *ChunkxParser) Parse(relPath string, content string, fileSize int64) (*ParsedDocument, error) {
	var lang languages.LanguageName
	if config, ok := languages.DetectLanguage(relPath); ok {
		lang = config.Name
	} else {
		lang = cp.DefaultLanguage
	}

	chunker := chunkx.NewChunker()
	var cxChunks []chunkx.Chunk
	var err error

	if lang != "" {
		cxChunks, err = chunker.Chunk(
			content,
			chunkx.WithLanguage(lang),
			chunkx.WithMaxSize(DefaultChunkMaxSize),
		)
	} else {
		cxChunks, err = chunker.Chunk(
			content,
			chunkx.WithMaxSize(DefaultChunkMaxSize),
		)
	}
	if err != nil {
		return nil, err
	}

	headers := parseHeaders(content)

	var chunks []*project.ChunkRecord
	for i, cx := range cxChunks {
		sectionTitle := ""
		sectionNum := 0
		for idx, h := range headers {
			if h.LineNumber <= cx.StartLine {
				sectionTitle = h.Title
				sectionNum = idx + 1
			}
		}

		sum := sha256.Sum256([]byte(cx.Content))
		chunkHash := fmt.Sprintf("%x", sum)

		metaMap := map[string]any{
			"filename":      filepath.Base(relPath),
			"section_num":   sectionNum,
			"section_title": sectionTitle,
		}
		metaBytes, _ := json.Marshal(metaMap)

		chunks = append(chunks, &project.ChunkRecord{
			ChunkIndex:   i,
			TextContent:  cx.Content,
			ContentHash:  chunkHash,
			TotalChars:   int64(len(cx.Content)),
			LineStart:    cx.StartLine,
			LineEnd:      cx.EndLine,
			SectionNum:   sectionNum,
			SectionTitle: sectionTitle,
			Metadata:     string(metaBytes),
		})
	}

	docMetadata := map[string]any{
		"path": relPath,
		"size": fileSize,
	}
	docMetaBytes, _ := json.Marshal(docMetadata)

	return &ParsedDocument{
		Slug:     strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath)),
		Metadata: string(docMetaBytes),
		Chunks:   chunks,
	}, nil
}

func init() {
	RegisterParser("markdown", &ChunkxParser{DefaultLanguage: languages.Markdown})
	RegisterParser("chunkx", &ChunkxParser{})
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
	// Exact filename matches (no wildcards, doesn't start with dot)
	if !strings.HasPrefix(pattern, ".") && !strings.ContainsAny(pattern, "*?[]") {
		return PriorityExactFile
	}
	// Complex extension (starts with dot, contains multiple dots like .rfc.md)
	if strings.HasPrefix(pattern, ".") && strings.Count(pattern, ".") > 1 {
		return PriorityComplexExtension
	}
	// Simple extension (starts with dot, one dot like .md)
	if strings.HasPrefix(pattern, ".") && strings.Count(pattern, ".") == 1 {
		return PriorityExtension
	}
	// General wildcard/glob pattern (contains *, ?, etc. like *.md or **/test.md)
	return PriorityWildcard
}

func matchesPattern(path string, pattern string) bool {
	base := filepath.Base(path)
	patternLower := strings.ToLower(pattern)
	pathLower := strings.ToLower(path)
	baseLower := strings.ToLower(base)

	// 1. Extension or complex extension match (starts with a dot)
	if strings.HasPrefix(patternLower, ".") {
		return strings.HasSuffix(pathLower, patternLower)
	}

	// 2. Wildcard/Glob match
	if strings.ContainsAny(patternLower, "*?[]") {
		matched, err := filepath.Match(patternLower, baseLower)
		return err == nil && matched
	}

	// 3. Exact filename match
	return baseLower == patternLower
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

	// Check collection-level parsers
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
		return "", false
	}

	// Find the best candidate
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.priority > best.priority {
			best = c
		} else if c.priority == best.priority {
			// If priority is equal, pick the one with longer pattern length (more specific)
			if len(c.pattern) > len(best.pattern) {
				best = c
			}
		}
	}

	return best.parserName, true
}

func SyncCollection(proj *project.Project, collectionName string) error {
	cfg, ok := proj.Config.Collections[collectionName]
	if !ok {
		return fmt.Errorf("collection %q not found in config", collectionName)
	}

	util.Logger.Info().Str("name", collectionName).Str("path", cfg.Path).Msg("Synchronizing collection")

	// Use user-specified exclude list if provided, otherwise fall back to defaults
	excludeList := cfg.Exclude
	if len(excludeList) == 0 {
		excludeList = defaultExcludeList
	}

	includeList := cfg.Include
	if len(cfg.Files) == 0 && len(includeList) == 0 {
		includeList = defaultIncludeList
	}

	fileFilter := newFileFilter(cfg.Files, includeList, excludeList)

	absCollectionPath := filepath.Join(proj.RootPath, cfg.Path)
	info, err := os.Stat(absCollectionPath)
	if err != nil {
		return fmt.Errorf("collection path %q error: %w", absCollectionPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("collection path %q is not a directory", absCollectionPath)
	}

	db, err := proj.OpenFTS()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	seenFiles := make(map[string]bool)

	err = filepath.WalkDir(absCollectionPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			// Always skip hidden directories like .git and .grokdocs
			if name != "." && name != ".." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			// Skip directories matching the exclude list (glob match on basename)
			for _, pattern := range excludeList {
				matched, mErr := filepath.Match(pattern, name)
				if mErr == nil && matched {
					return filepath.SkipDir
				}
			}
			return nil
		}

		parserName, ok := ResolveParserName(proj.Config, collectionName, path)
		if ok {
			if _, registered := GetParser(parserName); registered {
				relPath, err := filepath.Rel(proj.RootPath, path)
				if err != nil {
					return fmt.Errorf("failed to get relative path for %s: %w", path, err)
				}

				if !fileFilter.Match(relPath) {
					return nil
				}
				seenFiles[relPath] = true

				if err := ingestFile(db, relPath, path, collectionName, parserName, proj.Config); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Cleanup files in database that are no longer on disk for this collection
	rows, err := db.DB().Query(`
		SELECT f.id, f.file_path 
		FROM files f 
		JOIN documents d ON f.id = d.file_id 
		WHERE d.collection = ?`, collectionName)
	if err != nil {
		return fmt.Errorf("failed to query collection files for cleanup: %w", err)
	}
	defer rows.Close()

	type fileInfo struct {
		id   int64
		path string
	}
	var dbFiles []fileInfo
	for rows.Next() {
		var fi fileInfo
		if err := rows.Scan(&fi.id, &fi.path); err != nil {
			return fmt.Errorf("failed to scan file row: %w", err)
		}
		dbFiles = append(dbFiles, fi)
	}

	for _, fi := range dbFiles {
		if !seenFiles[fi.path] {
			if err := db.DeleteFile(fi.id); err != nil {
				return fmt.Errorf("failed to delete file record %s: %w", fi.path, err)
			}
		}
	}

	return nil
}

func ingestFile(db *project.FTSDatabase, relPath string, absPath string, collectionName string, parserName string, cfg *config.Config) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", absPath, err)
	}

	size := info.Size()
	mtime := info.ModTime().Unix()

	hash, err := computeSHA256(absPath)
	if err != nil {
		return fmt.Errorf("failed to compute hash for %s: %w", absPath, err)
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", absPath, err)
	}
	content := string(contentBytes)

	fileRecord, err := db.GetFile(relPath)
	if err != nil {
		if err == sql.ErrNoRows {
			fileRecord = &project.FileRecord{
				FilePath:    relPath,
				Filename:    filepath.Base(relPath),
				Size:        size,
				ModifiedAt:  mtime,
				ContentHash: hash,
			}
			if err := db.SaveFile(fileRecord); err != nil {
				return fmt.Errorf("failed to save file %s: %w", relPath, err)
			}
		} else {
			return fmt.Errorf("failed to query file %s: %w", relPath, err)
		}
	} else {
		if fileRecord.ModifiedAt == mtime {
			util.Logger.Debug().Str("path", relPath).Msg("Skipping file (mtime matches)")
			return nil
		}
		if fileRecord.ContentHash == hash {
			util.Logger.Debug().Str("path", relPath).Msg("Skipping file (hash matches)")
			fileRecord.ModifiedAt = mtime
			if err := db.SaveFile(fileRecord); err != nil {
				return fmt.Errorf("failed to update modified time for %s: %w", relPath, err)
			}
			return nil
		}
		fileRecord.Size = size
		fileRecord.ModifiedAt = mtime
		fileRecord.ContentHash = hash
		if err := db.SaveFile(fileRecord); err != nil {
			return fmt.Errorf("failed to update file %s: %w", relPath, err)
		}
	}

	docRecord, err := db.GetDocument(fileRecord.ID, collectionName)
	if err != nil {
		if err == sql.ErrNoRows {
			docRecord = &project.DocumentRecord{
				FileID:     fileRecord.ID,
				Collection: collectionName,
				Slug:       strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath)),
				ChunkCount: 0,
				TotalChars: 0,
			}
		} else {
			return fmt.Errorf("failed to get document for %s: %w", relPath, err)
		}
	} else {
		if err := db.DeleteChunksForDocument(docRecord.ID); err != nil {
			return fmt.Errorf("failed to delete old chunks for document %d: %w", docRecord.ID, err)
		}
	}

	parser, ok := GetParser(parserName)
	if !ok {
		return fmt.Errorf("parser %q not found in registry", parserName)
	}

	util.Logger.Info().Str("path", relPath).Str("parser", parserName).Msg("Ingesting file")

	parsedDoc, err := parser.Parse(relPath, content, size)
	if err != nil {
		return fmt.Errorf("failed to parse file %s using %s: %w", relPath, parserName, err)
	}

	slug := parsedDoc.Slug
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	}
	docRecord.Slug = slug
	docRecord.ChunkCount = len(parsedDoc.Chunks)
	docRecord.TotalChars = int64(len(content))
	docRecord.Metadata = parsedDoc.Metadata

	if err := db.SaveDocument(docRecord); err != nil {
		return fmt.Errorf("failed to save document for %s: %w", relPath, err)
	}

	for i, chunk := range parsedDoc.Chunks {
		chunk.DocumentID = docRecord.ID
		chunk.ChunkIndex = i
		if err := db.SaveChunk(chunk); err != nil {
			return fmt.Errorf("failed to save chunk %d for %s: %w", i, relPath, err)
		}
	}

	return nil
}

// fileFilter applies files/include/exclude rules from a collection config.
//
// Field precedence:
//   - files:  explicit filenames (basename match). When set, exclude is ignored.
//             Example: ["README.md", "index.md"]
//   - include: glob patterns, matched against basename or full path (supports **).
//             Example: ["*.md", "docs/**/*.go"] — matches any .md (any dir),
//             or any .go under docs/ recursively
//   - exclude: glob patterns matched against basename.
//             Example: ["*_test.go", "*.txt"]
//
// When files is set, a path passes if its basename matches any entry in files
// OR if include matches. When files is not set, include acts as a whitelist
// (if specified) and exclude as a blacklist applied after include.
type fileFilter struct {
	files   []string
	include []string
	exclude []string
}

func newFileFilter(files, include, exclude []string) *fileFilter {
	return &fileFilter{files: files, include: include, exclude: exclude}
}

func (f *fileFilter) Match(path string) bool {
	// If files is specified, only match files listed in files OR matching include globs
	if len(f.files) > 0 {
		base := filepath.Base(path)
		for _, name := range f.files {
			if base == name {
				return true
			}
		}
		for _, pattern := range f.include {
			if matchGlob(path, pattern) {
				return true
			}
		}
		return false
	}

	// No explicit files list: apply include as whitelist
	if len(f.include) > 0 {
		matched := false
		for _, pattern := range f.include {
			if matchGlob(path, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Apply exclude (always matched against basename)
	base := filepath.Base(path)
	for _, pattern := range f.exclude {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return false
		}
	}

	return true
}

// matchGlob reports whether path matches pattern, supporting ** for recursive
// directory matching (tsconfig-style). Without ** it falls back to basename
// matching for backward compatibility.
func matchGlob(path, pattern string) bool {
	if !strings.Contains(pattern, "**") {
		if strings.Contains(pattern, "/") {
			matched, err := filepath.Match(pattern, path)
			return err == nil && matched
		}
		base := filepath.Base(path)
		matched, err := filepath.Match(pattern, base)
		return err == nil && matched
	}

	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	patParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	return matchGlobParts(pathParts, patParts)
}

func matchGlobParts(pathParts, patParts []string) bool {
	if len(patParts) == 0 {
		return len(pathParts) == 0
	}

	if len(pathParts) == 0 {
		return allDoubleStar(patParts)
	}

	p := patParts[0]

	if p == "**" {
		for i := 0; i <= len(pathParts); i++ {
			if matchGlobParts(pathParts[i:], patParts[1:]) {
				return true
			}
		}
		return false
	}

	matched, err := filepath.Match(p, pathParts[0])
	if err != nil || !matched {
		return false
	}
	return matchGlobParts(pathParts[1:], patParts[1:])
}

func allDoubleStar(parts []string) bool {
	return len(parts) == 0 || (parts[0] == "**" && allDoubleStar(parts[1:]))
}

func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
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
