# Features — grokdocs v0.0.1

## CLI Commands

▼ **Global Flags**

* **`-p, --project <path>`**: Override project root (default: walk up to `.grokdocs/`).
* **`-v, --verbose`**: Enable verbose (trace-level) logging.
* **`--log-format <text|json>`**: Structured log output format.

▼ **`grokdocs init`**

* Initializes a `.grokdocs/` workspace directory at the project root.
* Writes a default `config.yaml` with a single `default` collection (path `.`, include `*.md`).

▼ **`grokdocs sync`**

* Scans files matching collection config, diffs against DB, parses new/changed files, stores chunks.
* **Flags**: `--all`, `-c/--collection`, `--prune` (default: true), `--concurrency` (default: 1), `--embed` (default: false).
* Skips unchanged files via mtime + SHA-256 hash comparison.

▼ **`grokdocs embed`**

* Computes vector embeddings for chunks missing them, flushes to FAISS + SQLite.
* **Flags**: `--all`, `-c/--collection`, `--concurrency`, `--prune`, `--rebuild`.
* Requires `-tags onnx` build tag.

▼ **`grokdocs search <query>`**

* Searches indexed content in FTS, semantic, or hybrid mode.
* **Flags**: `-c/--collection`, `-m/--mode` (`fts`, `semantic`, `hybrid`), `--limit` (default: 5), `--rrfk` (default: 60).
* Displays results flat or grouped-by-file with file path, section title, line range, score, and snippet.

▼ **`grokdocs files`**

* Lists indexed files per collection with pagination.
* **Flags**: `--all`, `-c/--collection`, `--limit`, `--offset`.
* Includes shell completion for `--collection`.

▼ **`grokdocs stats`**

* Displays indexing statistics: project root, config dir, DB path, file/document/chunk/chars counts per collection.
* Warns about DB-only collections not in config.

▼ **`grokdocs stats root`**

* Prints the absolute path of the active `.grokdocs/` directory.

▼ **`grokdocs version`**

* Prints the application version number.

## File Ingestion & Filtering

▼ **File Traversal**

* Producer-consumer pattern with `WalkQueue` (condition-variable bounded queue) decoupling file discovery from processing.
* Directory-optimized walking: only descends into folders that may contain included files.

▼ **Three-Tier Filtering**

* `files`: explicit filenames/paths (basename or full-path match). Exclude is not applied when `files` is set.
* `include`: glob whitelist with `**` recursive matching. Unioned with `files`.
* `exclude`: glob blacklist with `**` support. Applied after include.

▼ **Default Includes**

* 50+ extensions auto-indexed: `.md`, `.go`, `.py`, `.rs`, `.js`, `.ts`, `.java`, `.c`, `.cpp`, `.yaml`, `.toml`, `Dockerfile`, and more.

▼ **Default Excludes**

* 20+ patterns: `.git`, `node_modules`, `vendor`, `venv`, `__pycache__`, `dist`, `build`, `.DS_Store`, lock files, and more.

▼ **Incremental Sync**

* Change detection via mtime + SHA-256 content hash. Files matching both are skipped.
* If mtime differs but hash matches, mtime is updated and file is still skipped.

## Document Parsing & Chunking

▼ **Parser Registry**

* Extensible via `RegisterParser()` / `GetParser()`.
* Priority-based resolution: exact file extension > complex extension > simple extension > wildcard.

▼ **Markdown Parser**

* Line-by-line, section-aware chunker respecting code fence boundaries.
* Splits on markdown headings when accumulated chars >= 200.
* Four-tier split strategy for oversized chunks: blank line > sentence separator > word separator > anywhere.
* Tracks `section_num` and `section_title` per chunk.

▼ **AST Chunking (30+ languages)**

* Wraps `gomantics/chunkx` for AST-aware chunking.
* Post-processing divides oversized AST chunks by rune count into roughly equal sub-chunks.
* Markdown header-to-section mapping in code files.

▼ **Chunk Size Configuration**

* `DefaultChunkMaxSizeChar` = 1500 (hard upper bound).
* `DefaultChunkMinSizeChar` = 200 (soft lower bound for heading flushes).
* `DefaultChunkOverlap` = 10.

▼ **Parser Override**

* Per-collection `parsers` map in YAML config overrides default extension-to-parser mapping.

## Search

▼ **Full-Text Search (FTS5)**

* BM25 ranking via SQLite FTS5 virtual table.
* Snippet generation using FTS5 `snippet()` function.
* Per-collection filtering. Always available.

▼ **Semantic (Vector) Search** (requires `-tags onnx`)

* Queries embedded via same ONNX pipeline as indexing.
* FAISS `IDMap,Flat` index with inner product similarity (L2 normalized vectors).
* Maps FAISS labels (SQLite `chunks.id`) back to full chunk records.

▼ **Hybrid Search**

* Blends FTS5 BM25 and semantic scores using Reciprocal Rank Fusion (RRF).
* Configurable `--rrfk` constant (default: 60).
* Deduplicates across result sets. Falls back to FTS-only if semantic search fails.

## Embedding Pipeline (requires `-tags onnx`)

▼ **Model Management**

* Model: `all-MiniLM-L6-v2` (384-dimensional embeddings).
* Auto-downloads `model.onnx` and `tokenizer.json` from HuggingFace on first use.
* Cache: `~/.cache/grokdocs/models/all-MiniLM-L6-v2/`.
* Max sequence length: 512 tokens.

▼ **ONNX Inference**

* Singleton `Embedder` with lazy init via `GetGlobalEmbedder()`.
* Pipeline: tokenize → model forward → mean pooling (attention-mask-aware) → L2 normalization → output vector.
* Concurrent-safe (mutex). Clean shutdown via `Close()` with `sync.Once`.

▼ **Custom Tokenizer**

* Supports SentencePiece Unigram (Viterbi decoding) and WordPiece formats.
* Auto-detection from `tokenizer.json`.
* Unigram: SentencePiece pre-tokenization (whitespace split with `▁` meta char), NFKC normalization.
* WordPiece: BERT-style pre-tokenization, NFKD + diacritic removal normalization.

▼ **Vector Ingestion**

* Batched embedding during sync (`--embed` flag).
* Skips empty/whitespace-only chunks.
* Adds vectors to per-collection FAISS index and tracks in `chunk_vectors` table.

▼ **Batch Processing**

* Concurrent worker pool with `errgroup`.
* Batch flushing at 1000 vectors to FAISS + SQLite.
* Rebuild mode: resets FAISS index, clears vector tracking, re-embeds all.
* Orphan pruning: removes FAISS vectors whose chunks no longer exist.

## Storage

▼ **SQLite/FTS5 Schema**

* `files`: file metadata (path, name, size, mtime, SHA-256 hash).
* `documents`: file-to-collection mapping (one file in multiple collections).
* `chunks`: chunk content with line ranges, section info, slug.
* `chunks_fts`: FTS5 virtual table auto-synced via triggers on `chunks`.
* `chunk_vectors`: tracks vectorized chunks per collection.

▼ **CRUD Operations**

* Transactional batch saves (`SaveChunksBatch`), batched deletes (`DeleteFilesBatch` by 100).
* Pagination via `ListCollectionFilesPaginated()` with limit/offset.
* Orphan detection via `GetOrphanedVectorChunks()`.

▼ **FAISS Vector Index**

* Per-collection files: `grokdocs-{collection}.index`.
* Index type: `IDMap,Flat` with inner product metric.
* Operations: add, search (top-k), remove by ID, reset, save, close.
* FAISS labels are 1:1 with SQLite `chunks.id`.

▼ **Project Lifecycle**

* Project discovery walks up parent directories looking for `.grokdocs/`, falls back to CWD.
* Lazy DB/FAISS initialization, cached in-memory after first open.

## Configuration

▼ **YAML Config (`config.yaml`)**

* Per-collection settings: `path`, `parsers`, `files`, `include`, `exclude`.
* Default config on `init`: single `default` collection at `.` with no overrides.
* Load/Save from file or `io.Reader`/`io.Writer`.

## Developer & Operations

▼ **Profiling**

* `PPROF_CPU` env var: write CPU profile to given path.
* `PPROF_MEM` env var: write heap profile on exit.

▼ **Logging**

* Three modes: `text` (minimal, WARN/ERROR only), `text` with `--verbose` (timestamps + levels + key=value), `json` (structured).
* Based on `rs/zerolog`.

▼ **Progress Tracking**

* Non-blocking `GuardedChan[T]` for real-time progress updates.
* Three phases: `"Processing"`, `"Embedding"`, `"Pruning"`.
* Terminal display via `progressbar/v3`.

▼ **Concurrency Primitives**

* `WalkQueue`: condition-variable bounded queue decoupling producer from consumer.
* `GuardedChan[T]`: thread-safe channel, drops when buffer full.

▼ **Graceful Degradation**

* Missing ONNX/FAISS components (no `onnx` build tag) degrades to FTS-only search.
* No single point of failure in the pipeline.

▼ **Build & Test**

* Build: `go build -tags "fts5 onnx" ./cmd/grokdocs`.
* Install: `go install -tags "fts5 onnx" -ldflags "-X main.version=X.Y.Z" ./cmd/grokdocs`.
* Test: `go test -tags "fts5 onnx" ./...`.
* Benchmarks: `go test -tags "fts5 onnx" -bench=. -benchmem ./...`.

▼ **Dev Tooling**

* lefthook git hooks enforcing Conventional Commits.
* commitlint config for commit message validation.
* markdownlint for markdown style consistency.
