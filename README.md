# grokdocs

## Overview

**grokdocs** is a local-first search engine that indexes your documentation,
source code, and plain-text files for full-text and semantic search. It
splits files into semantically meaningful chunks, generates local
embeddings, and indexes them using SQLite (FTS5) and FAISS for fast
keyword and similarity search.

## Core Features

### 1. Document Ingestion & Chunking

- **Multi-format Support**: Parse Markdown, source code, config, and
  plain-text files — anything your project uses, with 50+ extensions
  auto-included and 20+ patterns auto-excluded by default.
- **Intelligent Chunking**: Section-aware markdown chunker (4-tier split
  strategy) and AST-based chunking via `gomantics/chunkx` for 30+ languages.
- **Incremental Sync**: Tracks mtime + SHA-256 content hash per file.
  Only re-chunks and re-embeds files that have actually changed.
- **Three-Tier File Filtering**: `files` (explicit, exclude-proof),
  `include` (glob whitelist), `exclude` (glob blacklist) — with `**`
  recursive matching and basename vs full-path semantics.

### 2. Three Search Modes

- **Full-Text Search**: BM25 ranking via SQLite FTS5. Always available.
- **Semantic Search**: 384-dim vector embeddings via local ONNX model,
  searched with FAISS `IDMap,Flat` inner product index. Requires `-tags onnx`.
- **Hybrid Search**: Blends BM25 and vector scores via Reciprocal Rank
  Fusion (configurable `--rrfk`, default 60). Falls back gracefully to
  FTS-only when semantic is unavailable.

### 3. Local Embedding Pipeline (requires `-tags onnx`)

- **ONNX Inference**: `all-MiniLM-L6-v2` model auto-downloaded from
  HuggingFace on first use. Mean pooling + L2 normalization.
- **Custom Tokenizer**: SentencePiece Unigram (Viterbi) and WordPiece
  tokenizers, auto-detected from `tokenizer.json`.
- **Concurrent Batch Processing**: Worker pool with configurable concurrency,
  batch flushing of 1000 vectors to FAISS + SQLite. Rebuild mode and orphan
  pruning included.

### 4. Storage & Indexing

- **SQLite/FTS5**: 4 tables + FTS5 virtual table with auto-sync triggers.
  Tracks files, documents, chunks, and vector status.
- **FAISS Vector Index**: Per-collection `IDMap,Flat` indices persisted to
  `grokdocs-{collection}.index` files.

### 5. CLI Commands

| Command | Purpose |
|---|---|
| `init` | Create `.grokdocs/` workspace with default `config.yaml` |
| `sync` | Scan files, diff with DB, parse, chunk, store, optionally embed (`--embed`), prune |
| `embed` | Batch compute vector embeddings with rebuild (`--rebuild`) and orphan pruning |
| `search` | Search via FTS, semantic, or hybrid mode |
| `files` | List indexed files with pagination |
| `stats` | Show indexing statistics per collection |
| `stats root` | Print the active `.grokdocs/` directory path |
| `version` | Print version number |

## Build & Installation

### Prerequisites

- Go 1.25 or later.
- C/C++ compiler (e.g., `gcc` or `clang`) if building with CGO dependencies (such as FAISS via `go-faiss`).

### Build Tags

One build tag controls feature inclusion:

| Tag    | Required? | Enables                                                              |
|--------|-----------|----------------------------------------------------------------------|
| `fts5` | Always    | SQLite FTS5 full-text search (BM25 ranking)                          |
| `onnx` | Optional  | Local ONNX embeddings + FAISS vector search (semantic/hybrid search) |

Without `onnx`, the binary supports FTS search only — semantic and hybrid modes are disabled.
Without `fts5`, the binary cannot search at all.

```bash
go build -tags fts5 ./cmd/grokdocs              # FTS search only
go build -tags "fts5 onnx" ./cmd/grokdocs        # FTS + semantic search
```

This produces a `grokdocs` executable in the current directory.

You can inject a version string at build time via `ldflags`. The version is
displayed by `grokdocs --version` (or `grokdocs version`). When not set, it
defaults to `dev`:

```bash
# Build with a specific version
go build -tags fts5 -ldflags "-X main.version=0.0.1" -o grokdocs ./cmd/grokdocs
./grokdocs --version   # prints "grokdocs version 0.0.1"

# Build without ldflags — defaults to "dev"
go build -tags fts5 -o grokdocs ./cmd/grokdocs
./grokdocs --version   # prints "grokdocs version dev"
```

### Installing from Source

Install the binary to `$GOPATH/bin` so it's available from anywhere:

```bash
go install -tags "fts5 onnx" -ldflags "-X main.version=0.0.1" ./cmd/grokdocs
```

Ensure `$(go env GOPATH)/bin` is in your `PATH`.

### Running Tests

Run all tests that don't require build tags (some tests may be skipped):

```bash
go test ./...
```

Run tests:

```bash
go test -tags fts5 ./...                           # FTS tests only
go test -tags "fts5 onnx" ./...                     # FTS + semantic tests
```

Tests gated behind `//go:build onnx` (embedding, vector ingestion, semantic
search) are only compiled and executed when the `onnx` tag is supplied.

### Quick Start

To view all available commands and flags, run:

```bash
./grokdocs --help
```

## Project Structure & Workspace Configuration

`grokdocs` organizes index data and configuration inside a `.grokdocs` folder located at the root of your project:

```text
my-project/
├── .grokdocs/
│   ├── config.yaml              # Project configuration
│   ├── grokdocs.db              # FTS database
│   └── grokdocs-default.index   # Vector database (FAISS index, per collection)
├── docs/
│   └── readme.md
└── src/
    └── main.go
```

### Configuration File (`config.yaml`)

A default `config.yaml` is generated inside `.grokdocs/` upon calling `grokdocs init`:

```yaml
collections:
  default:
    path: "."
    parsers:
      .md: markdown
```

### File Filtering

Control which files get ingested with `files`, `include`, and `exclude`:

```yaml
collections:
  default:
    path: "."
    include:
      - "*.md"
      - "src/**/*.go"
    exclude:
      - "vendor/**"
      - "*_test.go"
```

- `files`: explicit filenames or paths (basename or full-path match). Files matched here are **always** indexed — `exclude` cannot remove them. When `files` is set, the final set is the union of `files` and `include` matches, but `exclude` is not applied.
- `include`: glob whitelist (supports `**`). Matches are added to the index unless filtered out by `exclude`.
- `exclude`: glob blacklist (supports `**`). Removes matching files from the `include` set. Has no effect when `files` is set.

When both `files` and `include` are omitted, a built-in default allowlist covers common source and doc extensions (`.md`, `.go`, `.py`, `.rs`, `.toml`, etc.). A default exclude list skips `.git`, `node_modules`, `vendor`, `__pycache__`, and similar.

#### Pattern Matching: Basename vs Full Path

The way a pattern is matched depends on whether it contains a `/`:

| Pattern | Contains `/`? | Match behavior | Example matches |
|---|---|---|---|
| `node_modules` | No | Matches **basename** at **any depth** via `filepath.Match(pattern, filepath.Base(path))` | `node_modules/pkg/foo.js`, `a/b/node_modules/foo.js` |
| `node_modules/**` | Yes | Matches **full relative path** (supports `**` glob) | `node_modules/pkg/foo.js` only (root-level), NOT `a/b/node_modules/foo.js` |
| `*.md` | No | Matches **basename** at any depth | `readme.md`, `docs/intro.md`, `a/b/c/file.md` |
| `docs/*.md` | Yes | Matches **full path** | `docs/intro.md` only, NOT `a/docs/foo.md` |

In short: patterns without `/` match the filename anywhere in the tree; patterns with `/` anchor to the full relative path. This applies to both `include` and `exclude` lists, as well as the built-in defaults.

The default exclude list uses bare names (`node_modules`, `.git`, `vendor`, etc.) to match those directories at **any depth**. To restrict an exclude to the **root level only**, use a path-prefixed pattern like `node_modules/**`.

### Root Directory Lookup Algorithm

All CLI commands support a global `-p, --project <path>` flag to specify the
workspace root. If the flag is omitted:

1. `grokdocs` starts searching at the current working directory (`.`).
2. It walks up the parent directories recursively looking for a `.grokdocs` folder.
3. If no `.grokdocs` directory is found up to the filesystem root, it falls
   back to the current directory (`.`) as the project root.

## CLI Interface

### Global Flags

All commands inherit these persistent flags:

| Flag | Type | Default | Description |
|---|---|---|---|
| `-p, --project <path>` | `string` | `""` (auto-detect) | Project root path |
| `-v, --verbose` | `bool` | `false` | Verbose output with timestamps and TRACE log level |
| `--log-format <format>` | `string` | `"text"` | Log format (`text` or `json`) |

### Command Tree

```
grokdocs
├── init              Initialize workspace configuration
├── sync              Synchronize files with the database
├── search [query]    Search indexed chunks
├── embed             Compute and store vector embeddings
├── files             List files in a collection
├── stats             Show indexing statistics
│   └── root          Show path to the active .grokdocs directory
└── version           Print the version number
```

### `grokdocs init`

Generates a default `config.yaml` inside the `.grokdocs` folder at the project root.

No command-specific flags.

### `grokdocs sync`

Scan folders and synchronize files into SQLite and the FAISS index.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` | `bool` | `false` | Synchronize all configured collections |

  (mutually exclusive with `--collection`)

| `-c, --collection <name>` | `string` | `""` | Synchronize only the specified collection |
| `--prune` | `bool` | `true` | Remove orphaned file records (files deleted since last sync) |
| `--concurrency <n>` | `int` | `1` | Number of files to process concurrently |
| `--embed` | `bool` | `false` | Compute and store vector embeddings during sync |

### `grokdocs embed`

Compute and store vector embeddings for chunks missing them, and optionally prune orphaned vectors.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` | `bool` | `false` | Embed all configured collections |

  (mutually exclusive with `--collection`)

| `-c, --collection <name>` | `string` | `""` | Embed only the specified collection |
| `--concurrency <n>` | `int` | `1` | Number of chunks to embed concurrently |
| `--prune` | `bool` | `true` | Remove orphaned vectors after embedding |
| `--rebuild` | `bool` | `false` | Clear and re-embed all chunks |

Requires compilation with `-tags onnx`.

### `grokdocs search <query>`

Search indexed chunks using FTS5, semantic (vector), or hybrid mode.

```text
[0] docs/architecture.md - 1 chunks
  [0] Architecture [L1-L35] — score: 0.123 (architecture-docs-architecture-md--0)

[1] internal/parser/chunkx.go - 2 chunks
  [0] [L1-L107] — score: 0.456 (default-internal-parser-chunkx-go--0)
  [1] [L108-L200] — score: 0.321 (default-internal-parser-chunkx-go--1)
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `-c, --collection <name>` | `string` | `""` | Limit search to the specified collection |
| `-m, --mode <mode>` | `string` | `"hybrid"` | Search mode: `fts`, `semantic`, or `hybrid` |
| `--limit <n>` | `int` | `5` | Maximum number of results to return |
| `--rrfk <k>` | `float64` | `60` | RRF constant `k` for hybrid ranking |

Semantic and hybrid modes require compilation with `-tags onnx`.

### `grokdocs files`

List indexed files in one or more collections with pagination.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` | `bool` | `false` | List files in all configured collections |

  (mutually exclusive with `--collection`)

| `-c, --collection <name>` | `string` | `""` | List files in the specified collection |
| `--limit <n>` | `int` | `0` | Maximum files to list (0 = unlimited) |
| `--offset <n>` | `int` | `0` | Number of files to skip |

### `grokdocs stats`

Display indexed statistics: collection count, documents per collection, total chunks, and total characters.

No command-specific flags.

#### `grokdocs stats root`

Print the absolute path of the discovered `.grokdocs` directory.

### `grokdocs version`

Print the version number (set via `ldflags` at build time; defaults to `"dev"`).

### Profiling

Profile CPU or memory usage by setting environment variables before running
any command:

```bash
PPROF_CPU=cpu.pprof PPROF_MEM=mem.pprof grokdocs sync
```

After the command finishes, the profiles are written to the specified files
for analysis with `go tool pprof`:

```bash
go tool pprof -http :8080 cpu.pprof
go tool pprof -http :8081 mem.pprof
```
