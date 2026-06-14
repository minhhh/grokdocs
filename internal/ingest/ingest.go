package ingest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/parser"
	"github.com/minhhh/grokdocs/internal/project"
	"github.com/minhhh/grokdocs/internal/util"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultConcurrency = 10
)

func makeSlug(collectionName, relPath string) string {
	s := collectionName + "--" + relPath
	s = strings.ReplaceAll(s, "/", "--")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}

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

// FileState indicates whether a file was unchanged, added as new, or modified.
type FileState int

const (
	FileUnchanged FileState = iota
	FileAdded
	FileModified
)

// SyncProgress is sent on the progress channel during SyncCollection.
type SyncProgress struct {
	FilesProcessed int
	Phase          string
	TotalFiles     int
}

// SyncResult contains the final tallies from a SyncCollection run.
type SyncResult struct {
	Unchanged int
	Added     int
	Modified  int
	Moved     int
	Deleted   int
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
		seen := make(map[string]bool)
		if len(filter.include) > 0 {
			filepath.Walk(collectionRoot, func(absPath string, info os.FileInfo, err error) error {
				if err != nil {
					util.Logger.Warn().Err(err).Str("path", absPath).Msg("walk error")
					select {
					case ch <- WalkResult{Err: err}:
					case <-ctx.Done():
						return ctx.Err()
					}
					return nil
				}

				relPath, _ := filepath.Rel(collectionRoot, absPath)

				if info.IsDir() {
					if len(filter.onlyIncludedFolders) > 0 {
						prefix := relPath
						if prefix != "." {
							prefix += "/"
						}
						keep := false
						for _, folder := range filter.onlyIncludedFolders {
							if relPath == "." || relPath == folder || strings.HasPrefix(relPath, folder+"/") || strings.HasPrefix(folder, prefix) {
								keep = true
								break
							}
						}
						if !keep {
							return filepath.SkipDir
						}
					}

					//util.Logger.Debug().Str("path", absPath).Msg("Directory")
					for _, pattern := range filter.exclude {
						if matchGlob(relPath, pattern) {
							return filepath.SkipDir
						}
					}
					return nil
				}

				if !filter.Match(relPath) {
					return nil
				}

				seen[relPath] = true
				select {
				case ch <- WalkResult{AbsPath: absPath, RelPath: relPath}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			})
		}

		if len(filter.files) > 0 {
			for _, file := range filter.files {
				if seen[file] {
					continue
				}
				absPath := filepath.Join(collectionRoot, filepath.FromSlash(file))
				if fi, err := os.Stat(absPath); err != nil {
					util.Logger.Warn().Err(err).Str("path", absPath).Msg("file not found")
					continue
				} else if fi.IsDir() {
					continue
				}

				util.Logger.Debug().Str("path", absPath).Msg("Send to out")
				select {
				case ch <- WalkResult{AbsPath: absPath, RelPath: file}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

func SyncCollection(proj *project.Project, collectionName string, progress chan<- SyncProgress) (SyncResult, error) {
	cfg, ok := proj.Config.Collections[collectionName]
	if !ok {
		util.Logger.Error().Str("collection", collectionName).Msg("collection not found in config")
		return SyncResult{}, errors.New("collection not found")
	}

	util.Logger.Info().Str("name", collectionName).Str("path", cfg.Path).Msg("Synchronizing collection")

	fileFilter := newFileFilter(cfg.Files, cfg.Include, cfg.Exclude)

	absCollectionPath := filepath.Join(proj.RootPath, cfg.Path)
	info, err := os.Stat(absCollectionPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", absCollectionPath).Msg("collection path error")
		return SyncResult{}, err
	}
	if !info.IsDir() {
		util.Logger.Error().Str("path", absCollectionPath).Msg("collection path is not a directory")
		return SyncResult{}, errors.New("collection path is not a directory")
	}

	db, err := proj.OpenFTS()
	if err != nil {
		util.Logger.Error().Err(err).Msg("failed to open database")
		return SyncResult{}, err
	}

	var (
		seenMu           sync.Mutex
		seenFiles        = make(map[string]bool)
		result           SyncResult
		resultMu         sync.Mutex
		newFileHashes    = make(map[string]FileState) // hash → state of newly ingested files
		movedToState     = make(map[string]FileState) // hash → state of moved-to destination
		processedCount   int32
	)

	g, ctx := errgroup.WithContext(context.Background())
	semaphore := make(chan struct{}, DefaultConcurrency)

	// Count total number of files
	totalFiles := 0
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		count := 0
		for wr := range walkFiles(ctx, absCollectionPath, fileFilter) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if wr.Err != nil {
				continue
			}
			count++
		}
		totalFiles = count
		return nil
	})

	for walkResult := range walkFiles(ctx, absCollectionPath, fileFilter) {
		if walkResult.Err != nil {
			continue
		}

		// Make path project-root-relative for storage and parser resolution
		relPath := filepath.Join(cfg.Path, walkResult.RelPath)

		semaphore <- struct{}{}
		g.Go(func() error {
			defer func() { <-semaphore }()

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			parserName, ok := parser.ResolveParserName(proj.Config, collectionName, relPath)
			if !ok {
				util.Logger.Warn().Str("path", relPath).Msg("no parser matched for file, skipping")
				return nil
			}
			if _, registered := parser.GetParser(parserName); !registered {
				util.Logger.Warn().Str("path", relPath).Str("parser", parserName).Msg("parser not registered, skipping")
				return nil
			}

			seenMu.Lock()
			seenFiles[relPath] = true
			seenMu.Unlock()

			state, hash, err := ingestFile(db, relPath, walkResult.AbsPath, collectionName, parserName, proj.Config)
			if err != nil {
				return err
			}

			resultMu.Lock()
			switch state {
			case FileUnchanged:
				result.Unchanged++
			case FileAdded:
				result.Added++
				newFileHashes[hash] = FileAdded
			case FileModified:
				result.Modified++
				newFileHashes[hash] = FileModified
			}
			resultMu.Unlock()

			if progress != nil {
				currentCount := atomic.AddInt32(&processedCount, 1)
				select {
				case progress <- SyncProgress{FilesProcessed: int(currentCount), Phase: "Processing", TotalFiles: totalFiles}:
				default:
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return SyncResult{}, err
	}

	dbFiles, err := db.ListCollectionFiles(collectionName)
	if err != nil {
		return SyncResult{}, err
	}

	for _, collectionFile := range dbFiles {
		seenMu.Lock()
		_, ok := seenFiles[collectionFile.Path]
		seenMu.Unlock()
		if !ok {
			resultMu.Lock()
			if destState, isMoved := newFileHashes[collectionFile.Hash]; isMoved {
				result.Moved++
				movedToState[collectionFile.Hash] = destState
			} else {
				result.Deleted++
			}
			resultMu.Unlock()

			if err := db.DeleteFile(collectionFile.ID); err != nil {
				util.Logger.Error().Err(err).Str("path", collectionFile.Path).Int64("id", collectionFile.ID).Msg("failed to delete file record")
				return SyncResult{}, err
			}
		}
	}

	// Remove orphaned documents (file_id points to a file that no longer exists).
	// This covers cases where SQLite FK cascade didn't fire (e.g. earlier runs
	// without PRAGMA foreign_keys=ON).
	if err := db.DeleteOrphanedDocuments(collectionName); err != nil {
		util.Logger.Error().Err(err).Str("collection", collectionName).Msg("failed to cleanup orphaned documents")
		return SyncResult{}, err
	}

	// Deduct moved-to files from Added/Modified so each old file maps to
	// exactly one state.
	for _, state := range movedToState {
		switch state {
		case FileAdded:
			result.Added--
		case FileModified:
			result.Modified--
		}
	}

	return result, nil
}

func ingestFile(db *project.FTSDatabase, relPath string, absPath string, collectionName string, parserName string, cfg *config.Config) (FileState, string, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", absPath).Msg("failed to stat file")
		return FileUnchanged, "", err
	}

	size := info.Size()
	mtime := info.ModTime().Unix()

	hash, err := computeSHA256(absPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", absPath).Msg("failed to compute hash")
		return FileUnchanged, "", err
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", absPath).Msg("failed to read file")
		return FileUnchanged, "", err
	}
	content := string(contentBytes)

	fileState := FileModified

	fileRecord, err := db.GetFile(relPath)
	if err != nil {
		if err == sql.ErrNoRows {
			fileState = FileAdded
			fileRecord = &project.FileRecord{
				FilePath:    relPath,
				Filename:    filepath.Base(relPath),
				Size:        size,
				ModifiedAt:  mtime,
				ContentHash: hash,
			}
			if err := db.SaveFile(fileRecord); err != nil {
				util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to save file")
				return FileUnchanged, "", err
			}
		} else {
			util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to query file")
			return FileUnchanged, "", err
		}
	} else {
		if fileRecord.ModifiedAt == mtime {
			util.Logger.Debug().Str("path", relPath).Msg("Skipping file (mtime matches)")
			return FileUnchanged, hash, nil
		}
		if fileRecord.ContentHash == hash {
			util.Logger.Debug().Str("path", relPath).Msg("Skipping file (hash matches)")
			fileRecord.ModifiedAt = mtime
			if err := db.SaveFile(fileRecord); err != nil {
				util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to update modified time")
				return FileUnchanged, "", err
			}
			return FileUnchanged, hash, nil
		}
		fileRecord.Size = size
		fileRecord.ModifiedAt = mtime
		fileRecord.ContentHash = hash
		if err := db.SaveFile(fileRecord); err != nil {
			util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to update file")
			return FileUnchanged, "", err
		}
	}

	docRecord, err := db.GetDocument(fileRecord.ID, collectionName)
	if err != nil {
		if err == sql.ErrNoRows {
			docRecord = &project.DocumentRecord{
				FileID:     fileRecord.ID,
				Collection: collectionName,
				Slug:       makeSlug(collectionName, relPath),
				ChunkCount: 0,
				TotalChars: 0,
			}
		} else {
			util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to get document")
			return FileUnchanged, "", err
		}
	} else {
		if err := db.DeleteChunksForDocument(docRecord.ID); err != nil {
			util.Logger.Error().Err(err).Int64("doc_id", docRecord.ID).Msg("failed to delete old chunks")
			return FileUnchanged, "", err
		}
	}

	docParser, ok := parser.GetParser(parserName)
	if !ok {
		util.Logger.Error().Str("parser", parserName).Msg("parser not found in registry")
		return FileUnchanged, "", errors.New("parser not found")
	}

	util.Logger.Debug().Str("path", relPath).Str("parser", parserName).Msg("Ingesting file")

	parsedDoc, err := docParser.Parse(relPath, content, size)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", relPath).Str("parser", parserName).Msg("failed to parse file")
		return FileUnchanged, "", err
	}

	docRecord.Slug = makeSlug(collectionName, relPath)
	docRecord.ChunkCount = len(parsedDoc.Chunks)
	docRecord.TotalChars = int64(len(content))
	docRecord.Metadata = parsedDoc.Metadata

	if err := db.SaveDocument(docRecord); err != nil {
		util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to save document")
		return FileUnchanged, "", err
	}

	for i, chunk := range parsedDoc.Chunks {
		chunk.DocumentID = docRecord.ID
		chunk.ChunkIndex = i
		chunk.Slug = fmt.Sprintf("%s--%d", docRecord.Slug, i)
	}

	if err := db.SaveChunksBatch(parsedDoc.Chunks); err != nil {
		util.Logger.Error().Err(err).Str("path", relPath).Msg("failed to save chunks")
		return FileUnchanged, "", err
	}

	return fileState, hash, nil
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
	files               []string
	include             []string
	exclude             []string
	onlyIncludedFolders []string // set when we know exactly which folders to scan
}

func newFileFilter(files, include, exclude []string) *fileFilter {
	if len(exclude) == 0 {
		exclude = defaultExcludeList
	}
	if len(include) == 0 && len(files) == 0 {
		include = defaultIncludeList
	}
	f := &fileFilter{files: files, include: include, exclude: exclude}
	f.initOnlyIncludedFolders()
	return f
}

// initOnlyIncludedFolders populates onlyIncludedFolders when we can determine
// a restricted set of directories to walk. Not used when explicit files are
// given — in that case we check file prefixes per-directory instead.
func (f *fileFilter) initOnlyIncludedFolders() {
	if len(f.files) > 0 {
		return
	}

	folders := extractIncludeFolders(f.include)
	if folders == nil {
		return
	}

	set := make(map[string]bool)
	for _, folder := range folders {
		excluded := false
		for _, ex := range f.exclude {
			if matchGlob(folder, ex) {
				excluded = true
				break
			}
		}
		if !excluded {
			set[folder] = true
		}
	}

	f.onlyIncludedFolders = make([]string, 0, len(set))
	for folder := range set {
		f.onlyIncludedFolders = append(f.onlyIncludedFolders, folder)
	}
}

// extractIncludeFolders extracts directory prefixes from include patterns.
// Returns nil if any pattern has no directory prefix (basename-only), meaning
// we must scan everything.
func extractIncludeFolders(patterns []string) []string {
	var folders []string
	set := make(map[string]bool)

	for _, p := range patterns {
		if !strings.Contains(p, "/") {
			return nil
		}

		parts := strings.Split(p, "/")
		var dirParts []string
		for _, part := range parts[:len(parts)-1] {
			if strings.ContainsAny(part, "*?[") {
				break
			}
			dirParts = append(dirParts, part)
		}
		if len(dirParts) == 0 {
			return nil
		}
		dir := strings.Join(dirParts, "/")
		if !set[dir] {
			set[dir] = true
			folders = append(folders, dir)
		}
	}

	if len(folders) == 0 {
		return nil
	}
	return folders
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
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}


