# grokdocs — Deconstructed Code Reference

---

## 1. High-Level Architecture & Domain Map

```
                  ┌────────────────────────┐
                  │    CLI Entry Points    │ (cmd/grokdocs/)
                  │ init | sync | search  │
                  └───────────┬────────────┘
                              │
                              ▼
                  ┌────────────────────────┐
                  │   Ingestion Pipeline   │ (internal/ingest/)
                  │    SyncCollection()    │
                  └───────────┬────────────┘
                              │
             ┌────────────────┴────────────────┐
             ▼                                 ▼
┌────────────────────────┐         ┌────────────────────────┐
│     Parsing Engine     │         │     Storage Engine     │ (internal/project/)
│   (internal/parser/)   │         │ SQLite (FTS) & FAISS   │
│ Markdown | Chunkx AST  │         │  (chunk_vectors, etc.) │
└────────────┬───────────┘         └───────────▲────────────┘
             │                                 │
             └───────────────┬─────────────────┘
                             │ (if --embed)
                             ▼
                 ┌────────────────────────┐
                 │   Embedding Pipeline   │ (internal/embed/)
                 │ ONNX Runtime / Model   │
                 └────────────────────────┘
```

### Domain Directory Mapping

* **CLI Commands (`cmd/grokdocs/`)**: Entry points bootstrapping the project (`root.go`, `init.go`), triggering synchronization (`sync.go`), running database metrics (`status.go`), or serving queries (`search.go`).

* **Ingestion (`internal/ingest/`)**: The control loop (`ingest.go`) that discovers files using glob rules (`walk.go`, `glob.go`), determines changed files via checksums, and coordinates chunking and storage.

* **Parsing (`internal/parser/`)**: The parser registry (`parser.go`) and implementations (`markdown.go` and `chunkx.go`) that partition files into size-constrained chunks.

* **Storage (`internal/project/`)**: Data lifecycle manager (`project.go`) maintaining the SQLite FTS5 database (`fts.go`) and the FAISS flat indexes (`vector.go`).

* **Embeddings (`internal/embed/`)**: Low-level inference code downloading and executing ONNX models (`embed.go`, `downloader.go`, `tokenizer.go`).

---

## 2. System Parameters & Configuration Precedence

### Core Magic Variables

* `DefaultChunkMaxSizeChar` = `1500` (defined in `internal/parser/parser.go:12`) — hard upper bound on chunk rune length (sized to fit within 512 tokens without truncation).

* `DefaultChunkMinSizeChar` = `200` (defined in `internal/parser/parser.go:13`) — soft lower bound for Markdown heading flushes.

* `DefaultChunkOverlap` = `10` (defined in `internal/parser/parser.go:14`).

### CLI Parameter Precedence

CLI flags (e.g. `--collection`, `--concurrency`, `--prune`) override collection-specific config values defined in `.grokdocs/config.yaml`.

* The configuration defaults to a single collection named `"default"`, targeting the `docs/` folder, including `*.md`, `*.go`, and `*.py`.

* If a collection name is specified via CLI (e.g. `-c default`), the system verifies the collection exists via `project.AssertCollectionValid()`.

---

## 3. Traceability Mapping Matrix

| CLI Command | Package Entry Point | Primary DB Operations | Core Target Struct / Files |
|---|---|---|---|
| `grokdocs init` | `proj.Init` | Writes default `config.yaml` | `cmd/grokdocs/init.go`, `project.go` |
| `grokdocs stats` | `db.GetStats` | Queries counts of files, docs, chunks, collections | `cmd/grokdocs/status.go`, `fts.go` |
| `grokdocs sync` | `ingest.SyncCollection` | `SaveFile`, `SaveDocument`, `SaveChunksBatch` | `cmd/grokdocs/sync.go`, `ingest.go` |
| `grokdocs embed` | `embedCollectionFn` | `GetVectorizedChunkIDs`, `ClearCollectionVectors` | `cmd/grokdocs/embed.go`, `embed.go` |
| `grokdocs search` | `searchCmd` / `searchSemantic` | `SearchFTS`, `GetChunkByID` | `cmd/grokdocs/search.go`, `search_onnx.go` |

---

## 4. Data Models & Interface Specifications

### SQLite/FTS5 Schema (`internal/project/fts.go`)

* Four tables, one FTS5 virtual table, and three triggers:
    * `files`: `id` (PK, INTEGER AUTOINCREMENT), `file_path` (UNIQUE, TEXT), `filename` (TEXT), `size` (INTEGER), `modified_at` (INTEGER), `content_hash` (TEXT).
    * `documents`: `id` (PK, INTEGER AUTOINCREMENT), `file_id` (FK to `files.id` ON DELETE CASCADE), `collection` (TEXT), `slug` (TEXT), `chunk_count` (INTEGER), `total_chars` (INTEGER), `metadata` (TEXT).
    * `chunks`: `id` (PK, INTEGER AUTOINCREMENT), `document_id` (FK to `documents.id` ON DELETE CASCADE), `chunk_index` (INTEGER), `text_content` (TEXT), `total_chars` (INTEGER), `line_start` (INTEGER), `line_end` (INTEGER), `section_num` (INTEGER), `section_title` (TEXT), `slug` (TEXT), `metadata` (TEXT).
    * `chunks_fts`: FTS5 virtual table built on `chunks` (`text_content`), with content sync triggers linking `chunks.id` to `chunks_fts.rowid`.
    * `chunk_vectors`: `chunk_id` (PK, INTEGER), `collection` (TEXT) — tracks which database chunks have corresponding vector entries.

### FAISS Vector Interface (`internal/project/vector.go`)

* `VectorDatabase` wraps the FAISS index file (`grokdocs-{collection}.index`).

* Vectors are stored using 32-bit floats (`[]float32`), with embedding dimension matching the model size.

* **Relational mapping**: The integer IDs added via `AddVectors(ids, vectors)` map **1-to-1** with the `id` field of the `chunks` SQLite table. Consequently, the labels returned by `Search(query, k)` are the SQLite `chunks.id` values.

---

## 5. Entry Points & Bootstrapping

### CLI Bootstrap — `cmd/grokdocs/root.go`

* `rootCmd` (cobra.Command, line 22) — top-level command. `PersistentPreRun` initializes the logger.

* `Execute()` (line 42) — calls `rootCmd.Execute()`, exits on error.

* `init()` (line 49) — registers `--project`, `--verbose`, `--log-format` flags.

### Search Command — `cmd/grokdocs/search.go`

* `searchCmd` (line 28) — `grokdocs search [query]`
    * Resolves `Project` via `project.FindProject(startDir)`.
    * Opens FTS database: `proj.OpenFTS()`.
    * In hybrid mode: runs both FTS and semantic search, then merges results via `mergeHybridResults()` (line 106).

* `readLinesOfFile()` (line 208) — reads source lines from disk for display.

### ONNX Semantic Search — `cmd/grokdocs/search_onnx.go` (build tag: `onnx`)

* `searchSemantic()` (line 18):
    * `embed.Embed(query)` -> vector (line 19).
    * `proj.OpenCollectionVector(collection, embed.Dim())` -> FAISS index (line 24).
    * `vdb.Search(vec, limit)` -> labels (SQLite chunk IDs) + distances (line 29).
    * `ftsDB.GetChunkByID(label)` -> joins chunk details with document/file meta for display (line 40).
    * Rank: `1.0 / (1.0 + distance)` (line 55).

---

## 6. Critical Flow Walkthroughs

### Flow: Ingestion and Storage (`ingest.SyncCollection`)

```
1. Resolve collection configuration, construct file filter, and discover paths via walkFiles().
2. Compare mtime and checksum of files in parallel:
     -> If unchanged (match db.GetFile): skip file.
     -> If changed/new:
          -> Call project.GetDocument() and delete old chunks.
          -> Resolve parser name via parser.ResolveParserName() and call parser.GetParser().
          -> Execute docParser.Parse(relPath, content).
          -> Commit new chunks to SQLite using db.SaveChunksBatch(chunks).
3. If --embed is enabled:
     -> Fetch SQLite chunk IDs and text content of new/modified chunks.
     -> Generate embeddings via ONNX model and add vectors with chunk.id as FAISS label.
     -> Record vectorized status in SQLite `chunk_vectors` table.
4. If --prune is enabled:
     -> Delete orphaned file records, document records, and their FAISS vector entries.
```

### Flow: Concurrency & Progress Throttling (`internal/util/guardedchan.go`)

During execution of `SyncCollection` or `embedCollectionFn`, progress notifications are channeled back to the terminal CLI main thread via `util.GuardedChan`:
- `GuardedChan` wraps standard Go channels with a mutex (`sync.Mutex`) to prevent writing to or closing closed channels.
- **Non-blocking / Dropping policy**: In `Send(v T)` (line 15), if the channel buffer is full, the select block falls back to the `default` case and drops the progress update instead of blocking the file-processing workers.

### Flow: Markdown Parser Chunker (`internal/parser/markdown.go`)

Line-by-line chunker protecting code block fences and splitting on headings:
```
1. Scan content and build inCodeBlock fence map.
2. Iterate lines:
     - Heading line: If accumulated charCount >= parser.DefaultChunkMinSizeChar: flush buffer as completed chunk.
     - Content line: Append line to current buffer.
     - If realCharCount() > parser.DefaultChunkMaxSizeChar:
           -> Invoke findSplit() to find optimal split point.
           -> Flush split portion, leaving remaining tail in buffer.
3. Flush remaining lines.
```

**`findSplit` — 4-tier split strategy** (`markdown.go:184`):
Walks the buffer backward to find a split point where the tail part's character count falls within `[parser.DefaultChunkMinSizeChar, parser.DefaultChunkMaxSizeChar]`:

| Tier | Split Criteria | Precedence |
|------|----------------|------------|
| **1. Blank line** | A blank line (`strings.TrimSpace(line) == ""`) where removing it and everything after brings `totalChars` into `[parser.DefaultChunkMinSizeChar, parser.DefaultChunkMaxSizeChar]` | Highest — cleanest split |
| **2. Sentence separator** | A `.`, `!`, or `?` rune within a line where splitting there brings the tail into `[parser.DefaultChunkMinSizeChar, parser.DefaultChunkMaxSizeChar]` | |
| **3. Word separator** | A space separator (unicode.IsSpace) where splitting there brings the tail into `[parser.DefaultChunkMinSizeChar, parser.DefaultChunkMaxSizeChar]` | |
| **4. Anywhere** | Any rune position (fallback) | Lowest — last resort |

### Flow: AST Code Chunker (`internal/parser/chunkx.go`)

AST-respecting code chunking using `gomantics/chunkx`.
```
1. Call chunkx.NewChunker().Chunk(content) -> returns []chunkx.Chunk.
2. For each chunkx.Chunk:
     - If chunk size <= parser.DefaultChunkMaxSizeChar runes: wrap into single ChunkRecord.
     - If chunk size > parser.DefaultChunkMaxSizeChar runes: split into ceil(numChars/parser.DefaultChunkMaxSizeChar) sub-chunks by rune count.
     - Recalculate line offsets (LineStart/LineEnd) dynamically by counting newlines.
```

### Flow: Hybrid Ranking RRF blending (`cmd/grokdocs/search.go`)

Blends BM25 FTS ranking and FAISS semantic distances using Reciprocal Rank Fusion:
```
1. FTS search -> returns ordered list of ChunkRecords.
2. Semantic search -> returns ordered list of ChunkRecords.
3. Compute score for each chunk:
     score = sum of [ 1 / (rrfK + rank + 1) ] across both lists (rrfK default = 60)
4. Deduplicate results using seen map, sum scores, sort descending, and truncate to limit.
```

---

## 7. Package Boundaries & Interaction Interfaces

The codebase is organized into distinct Go packages designed to decouple concerns. These packages manage cross-directory interactions using formal hand-off structures and interfaces:

### Inter-Package Boundaries & Handoff Specifications

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                cmd/grokdocs                                 │
└──────┬───────────────────────────────┬───────────────────────────────┬──────┘
       │ (Init)                        │ (SyncProgress)                │ (Project, FTS, Vector)
       ▼                               ▼                               ▼
┌──────────────┐                ┌──────────────┐                ┌──────────────┐
│  internal/   │                │  internal/   │                │  internal/   │
│    config    │                │     util     │                │   project    │
└──────┬───────┘                └──────────────┘                └──────┬───────┘
       │ (CollectionConfig)                                            │
       ▼                                                               │
┌──────────────┐ (Parser / ParsedDocument)                             │
│  internal/   │◄──────────────────────────────────────────────────────┤
│    parser    │                                                       │
└──────┬───────┘                                                       │ (FTSDatabase / VectorDatabase)
       │ (Text Content)                                                │
       ▼                                                               │
┌──────────────┐                                                       │
│  internal/   │◄──────────────────────────────────────────────────────┤
│    embed     │                                                       │
└──────┬───────┘                                                       │
       │ (Embeddings Vector)                                           │
       ▼                                                               │
┌──────────────┐                                                       │
│  internal/   │◄──────────────────────────────────────────────────────┘
│    ingest    │ (Saves Chunks & Vectors)
└──────────────┘
```

The system avoids circular package dependencies by mediating all interaction via structural boundaries:
- **Parser Registration Strategy**: The [Parser](file://internal/parser/parser.go#L25) interface and registry mapping are defined in `internal/parser/`. Parsers register themselves automatically (`init()`) from markdown and chunkx files. The ingestion pipeline (`internal/ingest/`) calls [GetParser](file://internal/parser/parser.go#L36) dynamically to process files without hard dependency on concrete parsers.
- **Relational Data Mapping**: Storage structures like [ChunkRecord](file://internal/project/fts.go#L98), [FileRecord](file://internal/project/fts.go#L77), and [DocumentRecord](file://internal/project/fts.go#L87) are defined in `internal/project/`. They cross packages to:
  - `internal/parser/` (returned as a list of chunks wrapped in [ParsedDocument](file://internal/parser/parser.go#L18)).
  - `internal/ingest/` (which passes them directly to `SaveChunksBatch`).
  - `cmd/grokdocs/` (which reads and maps result rows inside `search.go`).
- **Project State Lifecycle**: [Project](file://internal/project/project.go#L21) is instantiated in `cmd/grokdocs` and flows into the ingestion sync and semantic search paths to safely manage database handles (`proj.OpenFTS()`, `proj.OpenCollectionVector()`).
- **Asynchronous Status Updates**: Background workers inside `internal/ingest/` communicate real-time CLI sync metrics using a [GuardedChan](file://internal/util/guardedchan.go#L5) progress channel defined in `internal/util/`. This decouples the worker thread loops from main thread console rendering details.

---

## 8. Development, Testing, and Performance Profiling

### Building the Project

Build the project using standard Go compilation. Specify build tags depending on the required search engine dependencies:

* **FTS5 Search Only** (no ONNX runtime or FAISS dependencies):
  ```bash
  go build -tags fts5 ./cmd/grokdocs
  ```

* **Full-Stack Search** (FTS5 + ONNX Embeddings + FAISS Vectors):
  ```bash
  go build -tags "fts5 onnx" ./cmd/grokdocs
  ```

### Running Tests

Unit and integration tests reside in `*_test.go` files across packages. Run tests with the relevant build tags:

```bash
go test -tags fts5 ./...                      # Runs FTS5-only tests
go test -tags "fts5 onnx" ./...               # Runs FTS5 and ONNX/FAISS tests
```

### Performance Benchmarking

Go benchmarks measure execution throughput and memory allocations. 

* To run all benchmarks:
  ```bash
  go test -tags "fts5 onnx" -bench=. -benchmem ./...
  ```

* To run a specific benchmark function (e.g. within `internal/ingest`):
  ```bash
  go test -tags "fts5 onnx" -bench=BenchmarkName ./internal/ingest/
  ```

### Profiling (CPU & Memory)

Go's `pprof` system gathers runtime metrics for analyzing CPU cycles and heap allocations.

1. **Generate Profile Files** during test or benchmark execution:
   ```bash
   go test -tags "fts5 onnx" -cpuprofile=cpu.pprof -memprofile=mem.pprof ./internal/parser/
   ```

2. **Inspect Profiles** using the interactive pprof visualization tool:
   * CPU Profile:
     ```bash
     go tool pprof cpu.pprof
     ```
   * Memory Profile:
     ```bash
     go tool pprof mem.pprof
     ```

3. **Core pprof Interactive Commands**:
   * `top`: Lists the top resource-consuming functions.
   * `list <function_name>`: Displays source code annotated with execution times or memory usage line-by-line.
   * `web`: Generates an SVG call graph and opens it in a browser.
