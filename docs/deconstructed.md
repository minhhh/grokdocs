# grokdocs — Deconstructed Code Reference

## Entry Points

### CLI Bootstrap — `cmd/grokdocs/root.go`
- `rootCmd` (cobra.Command, line 22) — top-level command, `PersistentPreRun` initializes the logger (line 27-35)
- `Execute()` (line 42) — calls `rootCmd.Execute()`, exits on error
- `init()` (line 49) — registers `--project`, `--verbose`, `--log-format` flags

### Search Command — `cmd/grokdocs/search.go`

- `searchCmd` (line 28) — `grokdocs search [query]`
  - Resolves `Project` via `project.FindProject(startDir)` -> `proj.Init()` (line 39-45)
  - Opens FTS database: `proj.OpenFTS()` (line 54)
  - Three modes (line 73-100):
    - `"fts"`: `db.SearchFTS(query, collection, limit)` (line 75)
    - `"semantic"`: `semanticSearchFn(proj, db, query, collection, limit)` (line 81) — injected by ONNX build
    - `"hybrid"`: runs both FTS and semantic, then `mergeHybridResults()` (line 99)
  - `displayResults()` (line 116) — groups results by file path, prints line ranges + snippets
  - `readLinesOfFile()` (line 204) — reads source lines from disk for display
   - `mergeHybridResults()` (line 166) — RRF (Reciprocal Rank Fusion): sums `1 / (rrfK + rank)` across FTS and semantic lists, sorts by combined score

### ONNX Semantic Search — `cmd/grokdocs/search_onnx.go` (build tag: `onnx`)

- `init()` (line 14) — sets `semanticSearchFn = searchSemantic`
- `searchSemantic()` (line 18):
  - `embed.Embed(query)` -> vector (line 19)
  - `proj.OpenCollectionVector(collection, embed.Dim())` -> FAISS index (line 24)
  - `vdb.Search(vec, limit)` -> labels + distances (line 29)
  - `ftsDB.GetChunkByID(label)` -> enrich results with line/section metadata (line 40)
  - Rank: `1.0 / (1.0 + distance)` (line 55)
- `makeSnippet()` (line 62) — truncates text to `maxLen` runes

### Server Command — `cmd/grokdocs/server.go`
- *(Not yet read — HTTP server for remote search)*

---

## Core Structs & Interfaces

### `internal/project/project.go` — Project Workspace
- `Project` struct (line 21):
  - `RootPath`, `ConfigDir`, `Config`, `ftsDB`, `vectorDB`, `collVectorDBs`
- `NewProject(rootPath)` (line 31) — creates instance with absolute root path
- `FindProject(startDir)` (line 45) — walks up directories looking for `.grokdocs/` folder; falls back to `startDir`
- `Project.Init()` (line 70) — creates `.grokdocs/` dir, writes default `config.yaml` if missing, calls `p.load()`
- `Project.load()` (line 101) — parses `config.yaml` via `config.LoadFromFile()`
- `Project.OpenFTS()` (line 113) — opens `grokdocs.db`, runs `InitSchema()` (creates tables, FTS5, triggers)
- `Project.OpenVector(dim)` (line 131) — opens `grokdocs.index` FAISS file
- `Project.OpenCollectionVector(collection, dim)` (line 150) — per-collection FAISS index (`grokdocs-{collection}.index`)
- `Project.Close()` (line 170) — closes FTS + all vector DBs

### `internal/project/fts.go` — SQLite/FTS5 Storage
- `FTSDatabase` struct (line 13): `Path string`, `db *sql.DB`, `mu sync.Mutex`
- **Schema** (lines 22-69): 3 tables + 1 FTS5 virtual table + 3 triggers:
  - `files`: id, file_path, filename, size, modified_at, content_hash
  - `documents`: id, file_id, collection, slug, chunk_count, total_chars, metadata
  - `chunks`: id, document_id, chunk_index, text_content, total_chars, line_start, line_end, section_num, section_title, slug, metadata
  - `chunks_fts`: FTS5 virtual table with content sync from chunks table
- `FileRecord` (line 73), `DocumentRecord` (line 83), `ChunkRecord` (line 94), `SearchResult` (line 109)
- `OpenFTSDatabase(dbPath)` (line 122) — opens SQLite, pings
- `InitSchema()` (line 158) — enables foreign keys, executes DDL
- **CRUD**: `GetFile` (line 173), `SaveFile` (line 187), `GetDocument` (line 308), `SaveDocument` (line 337), `GetChunkByID` (line 507), `SaveChunksBatch` (line 465), `DeleteChunksForDocument` (line 499), `DeleteFile` (line 262), `DeleteFilesBatch` (line 273)
- **Collection**: `ListCollectionFiles` (line 224), `DeleteOrphanedDocuments` (line 251)
- **Search**: `SearchFTS(queryText, collection, limit)` (line 547) — joins chunks + documents + chunks_fts, uses BM25 rank, `snippet()` for context
- **Stats**: `GetStats()` (line 605) — counts files, documents, chunks, chars, docs/chunks per collection

### `internal/project/vector.go` — FAISS Vector Database
- `VectorDatabase` struct: wraps FAISS index with file path, mutex
- `OpenVectorDatabase(path, dim)` -> creates/loads index
- `AddVectors(ids, vectors)` -> `vdb.AddVectorsWithIDs()`
- `Search(query, k)` -> labels, distances
- `RemoveIDs(ids)`, `Save()`, `Close()`, `Dim()`, `Len()`

### `internal/parser/parser.go` — Parser Registry & Dispatch
- `DefaultChunkMaxSizeChar = 1300` (line 11), `DefaultChunkOverlap = 10` (line 12)
- `ParsedDocument` struct (line 15): `Slug`, `Metadata` (JSON), `Chunks []*ChunkRecord`
- `Parser` interface (line 22): `Parse(relPath string, content string) (*ParsedDocument, error)`
- `parserRegistry map[string]Parser` (line 27)
- `RegisterParser(name, p)` (line 29), `GetParser(name)` (line 33)
- `MatchPriority` (line 41-49): `PriorityNone < PriorityWildcard < PriorityExtension < PriorityComplexExtension < PriorityExactFile`
- `defaultParserMapping` (line 85-142): maps ~50 extensions to `"chunkx"` or `"markdown"`
- `ResolveParserName(cfg, collectionName, path)` (line 144):
  1. Check collection-level `Parsers` map (highest priority by pattern type)
  2. Fall back to `defaultParserMapping[filepath.Ext(path)]`
  3. Fall back to `defaultParserMapping[filepath.Base(path)]`
  4. Returns `("", false)` if no match

### `internal/parser/chunkx.go` — ChunkxParser
- `CharTokenizer` (line 15): `CountTokens(text)` -> `utf8.RuneCountInString(text)`
- `var charTok = &CharTokenizer{}` (line 21) — package-level singleton
- `sectionHeader` (line 23): `Title`, `LineNumber`
- `parseMarkdownHeaders(content)` (line 28) — scans lines starting with `#` followed by space
- `ChunkxParser` struct (line 60): `DefaultLanguage languages.LanguageName`
- `ChunkxParser.Parse(relPath, content)` (line 64):
  1. Detect language from file extension (chunkx auto-detection)
  2. `chunkx.NewChunker().Chunk(content, ...)` with `CharTokenizer`, max 1300 chars, overlap 10
  3. Extract markdown headers (`.md`/`.markdown` only)
  4. For each chunkx chunk: determine section from headers, call `splitChunk()`
  5. Assign sequential `ChunkIndex`, append all sub-chunks
- `splitChunk(cx, sectionTitle, sectionNum, meta)` (line 138):
  - If `numChars <= 1300`: return single record with `cx.StartLine`/`cx.EndLine`
  - Else: split into `ceil(numChars / 1300)` parts by rune count
  - Compute `LineStart`/`LineEnd` for each sub-chunk by counting newlines, capped at `cx.EndLine`
  - Known chunkx off-by-one: when chunkx merges overlapping portions, line numbers can be +1 off
- `init()` (line 186): registers `"chunkx"` parser, `"markdown"` parser with `DefaultLanguage: languages.Markdown`

### `internal/config/config.go` — Configuration
- `Config` struct: `Collections map[string]CollectionConfig`
- `CollectionConfig`: `Path` (string), `Files`, `Include`, `Exclude` (string slices), `Parsers` (map[string]string)
- `LoadConfig(path)` / `LoadFromFile(path)` — YAML parsing
- `DefaultConfig()` — returns config with a single `"default"` collection pointing at `docs/` with `*.md`, `*.go`, `*.py`
- `SaveToFile(path)` / `SaveConfig(cfg, path)`

### `internal/embed/embed.go` (build tag: `onnx`) — Embedding
- `Embedder` struct (line 13): `session`, `tokenizer`, `dim`
- `GetGlobalEmbedder()` (line 24): singleton — downloads model, loads ONNX session, creates tokenizer
- `Embed(text)` (line 83): tokenizes -> ONNX inference -> mean pooling -> L2 normalize -> returns `[]float32`
- `Dim()` (line 134): returns embedding dimension
- `Close()` (line 144): destroys session + ONNX runtime
- `meanPool()` (line 155): attention-masked mean pooling over sequence dimension
- `l2Normalize()` (line 183): in-place L2 normalization

### `internal/embed/tokenizer.go` (build tag: `onnx`)
- `Tokenizer` interface, `UnigramTokenizer` — SentencePiece Unigram model
- `Encode(text, maxSeqLen)` -> `inputIDs`, `attentionMask`, `tokenTypeIDs`
- NFKC normalization, SentencePiece pre-tokenization, Viterbi decoding

### `internal/embed/downloader.go` (build tag: `onnx`)
- `GetOrDownloadModels(cacheDir)` -> `ModelFiles{ModelPath, VocabPath}`
- Downloads model + vocab from HuggingFace, caches in `~/.cache/grokdocs/`

### `internal/ingest/ingest.go` — Ingestion Pipeline
- `vectorIngestFn` (line 26): injected by ONNX build; embeds chunks and pushes to FAISS
- **Globals**: `defaultIncludeList` (line 40), `defaultExcludeList` (line 62)
- `makeSlug(collectionName, relPath)` (line 28): `"default--docs--intro-md"`
- `SyncCollection(proj, collectionName, progress, prune, concurrency)` (line 218):
  1. Look up collection config, build `fileFilter`
  2. Walk files (two-pass: count total, then process with concurrency semaphore)
  3. For each file: resolve parser -> `ingestFile()` -> optionally `vectorIngestFn()`
  4. If `prune`: diff seen files vs DB, handle deletions/moves, remove orphaned documents
  5. Deduct moved-to files from Added/Modified counts
  6. Return `SyncResult{Unchanged, Added, Modified, Moved, Deleted}`
- `ingestFile(db, relPath, absPath, collectionName, parserName, cfg)` (line 428):
  1. Stat file, compute SHA256
  2. Check DB for existing file record (mtime/hash-based change detection)
  3. Create/update `FileRecord` + `DocumentRecord`
  4. Get parser -> `docParser.Parse(relPath, content)`
  5. Save chunks via `SaveChunksBatch()`

### `internal/ingest/walk.go` — File Walker
- `walkFiles(ctx, collectionRoot, filter)` (line 133):
  - If `include` set: recursive `filepath.Walk()`, skip excluded dirs, filter via `filter.Match()`
  - If `files` set: stat each explicitly listed path
  - Uses `onlyIncludedFolders` for directory-pruning optimization

### `internal/ingest/glob.go` — Glob Matching
- `matchGlob(path, pattern)` — supports `**` (double-star) via recursive matching
- `matchGlobParts(parts, patternParts)` — recursive double-star expansion

### `internal/config/config.go`
- `CollectionConfig`: `Path`, `Files`, `Include`, `Exclude`, `Parsers`
- Parser resolution uses priority-based matching (exact file > extension > wildcard)

### `internal/util/logger.go`
- `InitLogger(writer, verbose, format)` — sets up zerolog
- `verboseWriter` / `minimalWriter` — controls log level filtering

---

## Critical Flows

### Flow: File Ingestion -> Chunking -> Storage
```
ingest.SyncCollection()
    -> walkFiles(ctx, absPath, fileFilter)
        -> for each file:
            -> parser.ResolveParserName(cfg, collection, relPath)
                -> collection-level Parsers map (priority: exact > extension > wildcard)
                -> defaultParserMapping[ext] or defaultParserMapping[basename]
            -> parser.GetParser(parserName)
            -> ingestFile(db, relPath, absPath, collection, parserName, cfg)
                -> os.Stat(), computeSHA256()
                -> db.GetFile(relPath) -> check mtime/hash -> skip if unchanged
                -> db.GetDocument(fileID, collection) -> delete old chunks
                -> docParser.Parse(relPath, content)
                    -> ChunkxParser.Parse()
                        -> chunkx.NewChunker().Chunk() -> []chunkx.Chunk
                        -> parseMarkdownHeaders() -> []sectionHeader
                        -> splitChunk(cx, section, meta) -> []ChunkRecord
                -> db.SaveChunksBatch(chunks) -> single transaction
    -> if prune: diff DB files vs seen, delete orphans
```

### Flow: Chunking algorithm

The `Parser` interface (`internal/parser/parser.go:22`) is the extension point for chunking. It consumes raw file content and produces a `ParsedDocument` containing a flat slice of `ChunkRecord` values:

```
Parser.Parse(relPath, content string) -> *ParsedDocument{Chunks []*ChunkRecord}
```

There is exactly one implementation (`ChunkxParser`), registered under two names (`"chunkx"` and `"markdown"`). It delegates to the third-party library `gomantics/chunkx` for AST-aware code chunking.

**The adapter problem:** chunkx respects AST node boundaries, so a single node (e.g., a 3000-char docstring) can exceed `DefaultChunkMaxSizeChar` (1300). The library does not guarantee chunk size — it guarantees structural integrity. This is where the reconciliation/adapter function `splitChunk` (`internal/parser/chunkx.go:132`) comes in:

```
chunkx.NewChunker().Chunk(content) -> []chunkx.Chunk  (AST-respecting, possibly oversized)
    -> for each chunkx.Chunk:
        -> splitChunk(cx, sectionTitle, sectionNum, meta) -> []*ChunkRecord
            -> if len(text) ≤ 1300 -> 1 record (pass-through)
            -> if len(text) > 1300 -> ceil(len/1300) records (rune-split with line tracking)
```

`splitChunk` is the sole post-processing adapter. It re-splits oversized chunks by rune count, recomputing `LineStart`/`LineEnd` for each sub-chunk by counting newlines. This is necessary because chunkx's AST-driven output does not align with the target chunk size without an additional reconciliation pass.

The design pattern: **third-party library produces semantically meaningful chunks -> adapter normalises them to the desired size constraint**, keeping the library swappable — any future chunking library would only need a similar adapter at the same point in the pipeline.

### Flow: Search Query
```
searchCmd.Run()
    -> project.FindProject(startDir) -> proj.Init()
    -> proj.OpenFTS() -> db
    -> mode == "fts":
        -> db.SearchFTS(query, collection, limit)
            -> FTS5 MATCH query on chunks_fts JOIN chunks JOIN documents
    -> mode == "semantic":
        -> searchSemantic(proj, db, query, collection, limit)
            -> embed.Embed(query) -> ONNX inference -> vector
            -> proj.OpenCollectionVector() -> FAISS search -> labels
            -> db.GetChunkByID(label) -> enrich with metadata
    -> mode == "hybrid":
        -> mergeHybridResults(ftsResults, semanticResults, limit)
            -> RRF: sum 1/(k + rank) across both lists, sort by score
    -> displayResults() -> group by file, print with line ranges
```

### Hybrid Ranking — `mergeHybridResults` (`cmd/grokdocs/search.go:166`)

The hybrid mode combines FTS and semantic results using Reciprocal Rank Fusion (RRF). Instead of blending raw scores (which are incomparable — BM25 rank vs cosine similarity), RRF converts each result list position into a rank-based score:

```
score = sum of [ 1 / (k + rank + 1) ] for every list the item appears in
```

Where `rank` is the 0-indexed position in the result list, and `k` (default 60) is a smoothing constant. A result appearing in both lists gets a score contribution from each, naturally boosting it above results found by only one method.

**Code** (`cmd/grokdocs/search.go:174-182`):
```go
for rank, r := range fts {
    scores[r.ID] += 1.0 / (rrfK + float64(rank) + 1)
    seen[r.ID] = r
}
for rank, r := range semantic {
    scores[r.ID] += 1.0 / (rrfK + float64(rank) + 1)
    if _, ok := seen[r.ID]; !ok {
        seen[r.ID] = r
    }
}
```

The `seen` map deduplicates by chunk ID — if the same chunk appears in both lists, the FTS `SearchResult` struct is kept (arbitrary, since both have identical metadata) and the RRF score is the sum of both contributions. Results are then sorted descending by this combined score (`search.go:191-193`) and truncated to `limit`.

The `k` constant is configurable via `--rrfk` (default 60, registered at `search.go:225`). Higher `k` flattens the rank advantage (all scores closer together); lower `k` gives more weight to top-ranked results.

---

## Side Effects / External Boundaries

### Disk Reads
- `os.ReadFile(absPath)` in `ingestFile()` (line 444) — reads source file content
- `os.Stat()` in `walkFiles()` + `ingestFile()` — file metadata
- `computeSHA256()` (line 698) — hashes file content for change detection
- `readLinesOfFile()` (line 204, search.go) — reads source file for snippet display
- `config.LoadFromFile(path)` — reads `config.yaml`

### Disk Writes
- SQLite database at `.grokdocs/grokdocs.db` — all file/doc/chunk storage + FTS5 index
- FAISS index at `.grokdocs/grokdocs.index` (or per-collection `grokdocs-{name}.index`)
- Model cache at `~/.cache/grokdocs/` — downloaded ONNX model + vocab
- Default config written at project init: `.grokdocs/config.yaml`

### External Network
- Model download via HTTP (HuggingFace) in `GetOrDownloadModels()` — only when `-tags onnx` is used

---

## Testing

### `internal/ingest/ingest_test.go`
- `TestFileWalking` — full integration: walks temp dir with `.md`/`.markdown` files, syncs collection, verifies file records in DB
- `TestFileWalkingDefaultIncludeList` / `TestFileWalkingDefaultExcludeList` — default filter behavior
- `TestMarkdownChunking` — parse markdown, verify LineStart/LineEnd/SectionNum/SectionTitle via table-driven test
- `TestSyncAndSearchFTS` — full pipeline: sync -> FTS search -> verify results
- `TestSyncCollectionWithFileFiltering` / `TestSyncSkipsUnchangedFile` / `TestSyncCollectionResult`
- `TestSyncCollectionCollectionNotFound` / `TestSyncCollectionPathIsFile` / `TestSyncCollectionPathDoesNotExist`
- `TestSyncWithVectors` — end-to-end with vector ingestion

### `internal/ingest/glob_test.go` + `internal/ingest/walk_test.go`
- Extensive glob matching and file walking tests (20+ test functions)

### `internal/parser/chunkx_test.go`
- `TestChunkxParserPython` — basic python file, generic invariants
- `TestChunkxParserLongPython` — multiple chunks generated
- `TestRawChunkxParserSingleLongNode` — chunkx's raw splitting behavior
- `TestChunkxParserSingleLongNode` — table-driven test verifying ChunkIndex, TotalChars, LineStart, LineEnd, SectionNum after splitChunk

### `internal/parser/parser_test.go`
- `TestParseMarkdownHeaders` — table-driven header parsing
- `TestParseHeaders` — extension-based dispatch

### `internal/project/project_test.go`
- `TestRootDiscoveryFallsBackToStartDir` / `TestRootDiscoveryFindsMarkerInAncestor`
- `TestProjectInitializationAndLoading` / `TestReInitPreservesConfigContent` / `TestInitIsIdempotent`
- `TestInitFailsOnInvalidRootPath`
- `TestDatabasesLifecycle` / `TestSQLiteDatabase` / `TestGetStats`
- `TestFAISSIndex` / `TestFAISSIndexEmpty` / `TestFAISSRemoveIDs` / `TestFAISSIndexWithDim`

### `internal/config/config_test.go`
- *(Not yet read — config loading tests)*

### `internal/embed/embed_test.go`
- `TestGetOrDownloadModels` — downloads model, verifies files exist
- `TestONNXEmbeddings` — runs ONNX inference, validates output dimension

### `internal/util/util_test.go`
- `TestLoggerTextDefault` / `TestLoggerTextVerbose` / `TestLoggerJSONFormat`
- `TestLoggerNonVerboseFiltersInfo` / `TestLoggerVerboseShowsTrace`

---

## Configurations

### Build Tags
- `onnx` — enables ONNX runtime embedding pipeline (embed package + semantic search)
- `fts5` — enables SQLite FTS5 extension (required for full-text search)

### Default Config (`config.DefaultConfig()`)
```yaml
collections:
  default:
    path: docs
    include: ["*.md", "*.go", "*.py"]
```

### CLI Flags
- `--project, -p` — project root path
- `--verbose, -v` — verbose logging
- `--log-format` — text or json
- `--collection, -c` — limit search to collection
- `--mode` — fts, semantic, hybrid
- `--limit` — max result groups
- `--alpha` — hybrid blend weight
