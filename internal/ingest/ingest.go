package ingest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gomantics/chunkx"
	"github.com/gomantics/chunkx/languages"
	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultChunkMaxSize  = 500
	DefaultConcurrency   = 4
)

// defaultIncludeList contains glob patterns used when a collection has no
// explicit include or files field. These match common documentation and
// source file extensions by basename (e.g. *.md matches intro.md in any dir).
var defaultIncludeList = []string{
	"*.md", "*.markdown",
	"*.bash", "*.c", "*.cc", "*.cjs", "*.cpp", "*.cs", "*.css", "*.cue", "*.cxx",
	"Dockerfile", "*.dockerfile",
	"*.elm", "*.ex", "*.exs",
	"*.go", "*.gradle", "*.groovy",
	"*.h", "*.hcl", "*.hh", "*.hpp", "*.htm", "*.html", "*.hxx",
	"*.java", "*.js", "*.jsx",
	"*.kt", "*.kts",
	"*.lua",
	"*.mjs", "*.ml", "*.mli",
	"*.php", "*.phtml", "*.proto",
	"*.py", "*.pyi", "*.pyw",
	"*.rb", "*.rake", "*.gemspec", "*.rs",
	"*.sc", "*.scala", "*.sh", "*.sql", "*.svelte", "*.swift",
	"*.tf", "*.toml", "*.ts", "*.tsx",
	"*.yaml", "*.yml",
}

// defaultExcludeList contains patterns for files and directories that are
// excluded by default when no user-specified exclude list is configured.
// Patterns are matched against the basename using filepath.Match.
var defaultExcludeList = []string{
	// Directories
	".git",
	".grokdocs",
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

// WalkResult is a single item from walkFiles: either a file path or a walk error.
type WalkResult struct {
	AbsPath string // absolute path on disk (for reading the file)
	RelPath string // project-root-relative path (for matching/DB)
	Err     error
}

// walkFiles walks collectionRoot and streams discovered files through an
// unbuffered channel. Files are emitted only if they pass filter.Match
// (using projectRoot-relative paths). Directory skipping uses filter.exclude
// (with ** support); a directory is descended into regardless if any entry
// in filter.files is rooted inside it. Walk callback errors (e.g. permission
// denied) are sent as WalkResult.Err. The channel is closed when the walk
// completes or the context is cancelled.
func walkFiles(ctx context.Context, collectionRoot string, filter *fileFilter) <-chan WalkResult {
	ch := make(chan WalkResult)
	go func() {
		defer close(ch)
		filepath.Walk(collectionRoot, func(absPath string, info os.FileInfo, err error) error {
			if err != nil {
				select {
				case ch <- WalkResult{Err: fmt.Errorf("walk error at %s: %w", absPath, err)}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			}


			rel, _ := filepath.Rel(collectionRoot, absPath)

			if info.IsDir() {
				prefix := rel
				if prefix != "." {
					prefix += "/"
				}
				for _, f := range filter.files {
					if strings.HasPrefix(f, prefix) {
						return nil
					}
				}

				for _, pattern := range filter.exclude {
					if matchGlob(rel, pattern) {
						return filepath.SkipDir
					}
				}
				return nil
			}

			if !filter.Match(rel) {
				return nil
			}

			select {
			case ch <- WalkResult{AbsPath: absPath, RelPath: rel}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}()
	return ch
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

	fileFilter := newFileFilter(cfg.Files, cfg.Include, cfg.Exclude)

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

	var seenMu sync.Mutex
	seenFiles := make(map[string]bool)

	g, ctx := errgroup.WithContext(context.Background())
	sem := make(chan struct{}, DefaultConcurrency)

	for r := range walkFiles(ctx, absCollectionPath, fileFilter) {
		if r.Err != nil {
			util.Logger.Warn().Err(r.Err).Msg("walk error")
			continue
		}

		// Make path project-root-relative for storage and parser resolution
		relPath := filepath.Join(cfg.Path, r.RelPath)

		sem <- struct{}{}
		g.Go(func() error {
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			parserName, ok := ResolveParserName(proj.Config, collectionName, relPath)
			if !ok {
				return nil
			}
			if _, registered := GetParser(parserName); !registered {
				return nil
			}

			seenMu.Lock()
			seenFiles[relPath] = true
			seenMu.Unlock()

			return ingestFile(db, relPath, r.AbsPath, collectionName, parserName, proj.Config)
		})
	}

	if err := g.Wait(); err != nil {
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
		seenMu.Lock()
		_, ok := seenFiles[fi.path]
		seenMu.Unlock()
		if !ok {
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
//   - files:  explicit filenames or relative paths (basename or full-path match).
//             When set, exclude is ignored.
//             Example: ["README.md", "subdir/doc.md"]
//   - include: glob patterns, matched against basename or full path (supports **).
//             Example: ["*.md", "docs/**/*.go"] — matches any .md (any dir),
//             or any .go under docs/ recursively
//   - exclude: glob patterns matched against path or basename (supports **).
//             Example: ["*_test.go", "**/node_modules/*"]
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
	if len(exclude) == 0 {
		exclude = defaultExcludeList
	}
	if len(include) == 0 {
		include = defaultIncludeList
	}
	return &fileFilter{files: files, include: include, exclude: exclude}
}

func (f *fileFilter) Match(path string) bool {
	// If files is specified, only match files listed in files OR matching include globs
	if len(f.files) > 0 {
		base := filepath.Base(path)
		for _, name := range f.files {
			if path == name || base == name {
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

	// Apply exclude (supports ** via matchGlob)
	for _, pattern := range f.exclude {
		if matchGlob(path, pattern) {
			return false
		}
	}

	return true
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
