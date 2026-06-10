# grokdocs

## Overview
**grokdocs** is a local-first search engine that indexes your Markdown and code files for semantic and full-text search. It splits your documentation and code files into semantically meaningful chunks, generates local embeddings, and indexes them using SQLite (FTS5) and FAISS for fast similarity and keyword search.

## Core Features
### 1. Document Ingestion & Chunking
- **Multi-format Support**: Parse Markdown, plain text, and Go source files.
- **Intelligent Chunking**: Use `gomantics/chunkx` to break down large documents into semantically meaningful chunks rather than arbitrary character counts.
- **Incremental Indexing**: Track file modification times (mtime) and content hashes. Only re-chunk and re-embed files that have actually changed to save compute time.

### 2. Hybrid Search Engine
- Generate embeddings for each chunk using a local ONNX model on-device.
- Store embeddings in a local FAISS index for semantic search, and insert chunk text into SQLite for Full-Text Search.
- Combine vector similarity and lexical search to retrieve the most relevant chunks using an adjustable hybrid weighting scheme (`--alpha`).

## Build & Installation

### Prerequisites
- Go 1.21 or later.
- C/C++ compiler (e.g., `gcc` or `clang`) if building with CGO dependencies (such as FAISS via `go-faiss`).

### Building from Source
Clone the repository and build the binary:
```bash
go build -tags fts5 ./cmd/grokdocs
```
This produces a `grokdocs` executable in the root directory.

### Running Tests
```bash
go test ./...
```
Note: Tests requiring SQLite FTS5 (`go test -tags fts5`) will be skipped if your system SQLite build lacks the FTS5 module. Run them explicitly with:
```bash
go test -tags fts5 ./...
```

### Quick Start
To view all available commands and flags, run:
```bash
./grokdocs --help
```

## Project Structure & Workspace Configuration

`grokdocs` organizes index data and configuration inside a `.grokdocs` folder located at the root of your project:

```
my-project/
├── .grokdocs/
│   ├── config.yaml      # Project configuration
│   ├── grokdocs.db      # FTS database (SQLite schema and FTS5 keyword index)
│   └── grokdocs.index   # Vector database (FAISS index file for semantic search)
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
      - markdown
```

### Root Directory Lookup Algorithm
All CLI commands support a global `-p, --project <path>` flag to specify the workspace root. If the flag is omitted:
1. `grokdocs` starts searching at the current working directory (`.`).
2. It walks up the parent directories recursively looking for a `.grokdocs` folder.
3. If no `.grokdocs` directory is found up to the filesystem root, it falls back to the current directory (`.`) as the project root.

## CLI Interface
All commands support the global `-p, --project <path>` flag. If `-p` is omitted, the CLI uses the lookup algorithm above to discover the project workspace.

- `grokdocs init`: Initialize the workspace and generate a default configuration file (`config.yaml`) inside the `.grokdocs` directory.
- `grokdocs sync`: Scan directories and synchronize files into the SQLite database and FAISS index.
  - `--all`: Sync all configured collections.
  - `--collection <name>`: Sync only the specified collection.
- `grokdocs search "<query>"`: Perform a hybrid search and return matching chunks.
  - `--collection <name>`: Limit search query to the specified collection.
  - `--mode <hybrid|vector|fts>`: Specify the search strategy (default: `hybrid`).
  - `--alpha <float>`: Weight multiplier between FTS and vector results (0.0 = pure FTS, 1.0 = pure Vector, default: 0.5).
  - `--limit <int>`: Number of results to return (default: 5).
