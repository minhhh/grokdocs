# grokdocs Architecture

## Technical Stack
- **Language**: Go 1.25+
- **Chunking**: `gomantics/chunkx` (AST-based semantic chunking, 30+ languages) + custom markdown line-by-line chunker
- **Vector Search**: FAISS via Go bindings (local, offline)
- **Embeddings**: Local ONNX models (SentenceTransformer, offline inference)
- **Metadata & FTS Storage**: SQLite + FTS5 for file tracking, document metadata, and full-text search
- **Tokenizer**: Custom SentencePiece Unigram (Viterbi) + WordPiece tokenizers for ONNX model compatibility
- **CLI Framework**: Cobra

## Project Layout
```
cmd/grokdocs/          CLI entry points (Cobra)
internal/
  config/              YAML config loading/saving
  embed/               ONNX embedding pipeline (build tag: onnx)
  ingest/              File traversal, filtering, ingestion orchestration
  parser/              Parser interface, chunkx wrapper, splitChunk logic
  project/             Project lifecycle, SQLite/FTS5, FAISS vector DB
  util/                Logging utilities
docs/                  Documentation (architecture + deconstructed code ref)
```

See [deconstructed.md](./deconstructed.md) for the complete file-by-file, function-by-function code reference with exact line numbers.

## Design Principles
- **Local-first**: All processing runs offline — no external API calls for chunking or embedding. Model download is the only network dependency (once, at setup).
- **Graceful degradation**: Missing FAISS index or ONNX model degrades to FTS-only search. No single component failure blocks the pipeline.
- **Line-oriented UX**: Search results map to integer line ranges and display snippets from the FTS5 index, with file paths for direct disk access when needed.

## Data Flow (High Level)

```
Source files on disk
    -> walk + filter
        -> ingest.SyncCollection()
            -> per file: resolve parser -> ingestFile()
                -> Parser.Parse(relPath, content)
                    -> markdown: line-by-line section-aware chunker (4-tier split)
                       chunkx: AST-based semantic chunking (30+ languages)
                    -> splitChunk() post-processing for oversized chunks
                    -> markdown header -> section mapping
                -> ChunkRecord[] -> SQLite (chunks table + FTS5 index)
                    -> optional: ONNX embed -> FAISS vector index
    -> Search: FTS5 BM25 + FAISS vector (hybrid merge)
```

## Component Responsibilities

### `internal/parser/` — Chunking & Section Detection
- **Parser interface + registry**: Extensible via `RegisterParser()` / `GetParser()`. Collection config can override the default extension-to-parser mapping using priority-based matching (exact file > extension > wildcard).
- **`MarkdownParser`**: Line-by-line, section-aware chunker. Respects code fence boundaries. Splits on headings when >= 200 chars accumulated. Four-tier split for oversized chunks: blank line > sentence separator > word separator > anywhere.
- **`ChunkxParser`**: Wraps `gomantics/chunkx` for AST-aware semantic chunking. Uses `CharTokenizer` (Unicode rune counting) for size enforcement. Auto-detects language from file path. Registered as `"chunkx"`.
- **`splitChunk()`**: chunkx's AST boundaries may not respect the 1500-char max size (e.g., a 3000-char docstring). `splitChunk` divides oversized chunks into roughly equal parts by rune count, computing sub-chunk line ranges from newline offsets (capped at the original chunk's `EndLine`).
- **Section tracking**: Markdown headers are parsed and each chunk is tagged with the most recent header's `SectionNum` and `SectionTitle`.

### `internal/ingest/` — File Traversal & Ingestion
- **`SyncCollection()`**: Orchestrates ingestion — walks files, diffs against the DB (mtime/hash), processes new/changed files concurrently, optionally prunes deleted files and their chunks.
- **`ingestFile()`**: Per-file pipeline — stat -> hash -> read -> parse -> save chunks. Uses content hashing for change detection to avoid re-indexing unchanged files.
- **File filtering**: Configurable include/exclude lists with `**` (double-star) glob support. Directory-walking optimization via `onlyIncludedFolders`. Sensible defaults for common documentation and source code extensions.

### `internal/embed/` (build tag: `onnx`) — Embedding
- Singleton `Embedder` with lazy initialization. ONNX inference pipeline: tokenize (custom SentencePiece Unigram) -> model forward pass -> mean pooling -> L2 normalization -> output vector.

### `internal/project/` — Storage & Project Lifecycle
- **SQLite/FTS5** (`fts.go`): Schema with 4 tables (`files`, `documents`, `chunks`, `chunk_vectors`) + FTS5 virtual table with auto-sync triggers. CRUD operations for all entities, batch chunk saving, FTS5 BM25 search with snippet generation.
- **FAISS** (`vector.go`): In-memory index persisted to disk. Per-collection index files. Supports add, search, remove, and save operations.
- **Project discovery**: Walks up parent directories to find `.grokdocs/` marker, falls back to start directory.

### `internal/config/` — Configuration

- YAML-based config with per-collection settings (path, files/include/exclude filters, parser overrides).
- **Built-in defaults includes and excludes**: 50+ extensions auto-included, 20+ patterns auto-excluded
- **Base path**: Default config path points at `.`
    * For the default collection, there's no option to provide a different path. If you want a different root you should put the config file there. Typescript team also removed `basePath` option in TS 7 for this reason.
    * The path of other collections other than the default collection can be customized.

## Key Design Decisions

### Why AST-based chunking via chunkx?
Chunking at AST boundaries (function declarations, class definitions, markdown sections) preserves semantic coherence better than byte/character-count splitting. The `splitChunk` fallback handles edge cases where a single AST node exceeds the target chunk size.

### Why both FTS5 and vector search?
FTS5 provides fast, reliable keyword matching with BM25 ranking. Vector search enables semantic (meaning-based) retrieval. Hybrid mode blends both scores using Reciprocal Rank Fusion (`--rrfk`, default 60).

### Why line-oriented everything?
Search results map to integer line ranges, allowing the CLI to read source lines directly from disk for display. This avoids duplicating content in the database and keeps the snippet generation simple and accurate.

## Configurations & Build Tags
- **Build tag `onnx`**: Enables ONNX runtime for embedding generation and semantic search.
- **Build tag `fts5`**: Enables SQLite FTS5 extension (required for full-text search).
- **Default config**: Single `"default"` collection at `"."` with no explicit overrides (built-in 50+ extension defaults apply).
