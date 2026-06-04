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
	"github.com/minhhh/grokdocs/internal/project"
)

const (
	DefaultChunkMaxSize = 500
)

var parserExtensions = map[string][]string{
	"markdown": {".md", ".markdown"},
}

// SectionHeader represents a parsed Markdown section header.
type SectionHeader struct {
	Title      string
	LineNumber int
}

// SyncCollection scans files and syncs them to the SQLite FTS database.
func SyncCollection(proj *project.Project, collectionName string) error {
	cfg, ok := proj.Config.Collections[collectionName]
	if !ok {
		return fmt.Errorf("collection %q not found in config", collectionName)
	}

	absCollectionPath := filepath.Join(proj.RootPath, cfg.Path)
	info, err := os.Stat(absCollectionPath)
	if err != nil {
		return fmt.Errorf("collection path %q error: %w", absCollectionPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("collection path %q is not a directory", absCollectionPath)
	}

	// Identify allowed extensions
	var extensions []string
	for _, p := range cfg.Parsers {
		if extList, ok := parserExtensions[p]; ok {
			extensions = append(extensions, extList...)
		}
	}
	if len(extensions) == 0 {
		extensions = []string{".md", ".markdown"}
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
			// Ignore hidden directories like .git and .grokdocs
			if name != "." && name != ".." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		allowed := false
		for _, e := range extensions {
			if ext == e {
				allowed = true
				break
			}
		}

		if allowed {
			relPath, err := filepath.Rel(proj.RootPath, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %s: %w", path, err)
			}
			seenFiles[relPath] = true

			if err := ingestFile(db, relPath, path, collectionName); err != nil {
				return err
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

func ingestFile(db *project.FTSDatabase, relPath string, absPath string, collectionName string) error {
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
			return nil
		}
		if fileRecord.ContentHash == hash {
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

	chunks, err := chunkContent(content, relPath, size)
	if err != nil {
		return fmt.Errorf("failed to chunk content of %s: %w", relPath, err)
	}

	docRecord.ChunkCount = len(chunks)
	docRecord.TotalChars = int64(len(content))
	docMetadata := map[string]any{
		"path": relPath,
		"size": size,
	}
	docMetaBytes, _ := json.Marshal(docMetadata)
	docRecord.Metadata = string(docMetaBytes)

	if err := db.SaveDocument(docRecord); err != nil {
		return fmt.Errorf("failed to save document for %s: %w", relPath, err)
	}

	for i, chunk := range chunks {
		chunk.DocumentID = docRecord.ID
		chunk.ChunkIndex = i
		if err := db.SaveChunk(chunk); err != nil {
			return fmt.Errorf("failed to save chunk %d for %s: %w", i, relPath, err)
		}
	}

	return nil
}

func chunkContent(content string, filename string, fileSize int64) ([]*project.ChunkRecord, error) {
	chunker := chunkx.NewChunker()
	cxChunks, err := chunker.Chunk(
		content,
		chunkx.WithLanguage(languages.Markdown),
		chunkx.WithMaxSize(DefaultChunkMaxSize),
	)
	if err != nil {
		return nil, err
	}

	headers := parseHeaders(content)

	var chunks []*project.ChunkRecord
	for _, cx := range cxChunks {
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
			"filename":      filename,
			"section_num":   sectionNum,
			"section_title": sectionTitle,
		}
		metaBytes, _ := json.Marshal(metaMap)

		chunks = append(chunks, &project.ChunkRecord{
			ChunkIndex:   0,
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

	return chunks, nil
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
