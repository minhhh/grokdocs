# grokdocs Architecture

## Technical Stack
- **Language**: Go 1.23+
- **Chunking**: `gomantics/chunkx` (Intelligent, context-aware text chunking)
- **Vector Search**: FAISS (Fast Similarity Search) via Go bindings.
- **Embeddings**: Local ONNX models strictly for offline, on-device embedding generation.
- **Metadata & FTS Storage**: SQLite for both file metadata and Full-Text Search (FTS).
- **CLI Framework**: Cobra

## Design Guidelines
- **Configuration**: Use a single `.yml` file for configuration. Collections should explicitly declare a list of parsers (e.g., `parsers: ["html_bofip", "markdown"]`) rather than relying purely on global file-extension inference.
- **Clean Architecture**: Follow standard Go project layout:
  - `/cmd/grokdocs`: CLI entrypoints.
  - `/internal/ingest`: File traversal and `chunkx` integration.
  - `/internal/search`: FAISS bindings and embedding provider wrappers.
- **Graceful degradation**: Ensure good error handling if the FAISS index is missing or the ONNX model fails to load.

## Chunking Design & Rationale

### Why `gomantics/chunkx`?
We selected `gomantics/chunkx` to handle document parsing and chunking instead of writing raw `tree-sitter` traversal code from scratch.
- **AST-Aware Semantics**: `chunkx` uses AST parsers to divide documents at logical structural boundaries (like Markdown headers or code blocks) rather than arbitrary byte or character counts. This preserves document context for the embedding model.
- **Extensibility**: It supports 30+ programming languages out of the box and features a `Generic` line-based fallback. This enables `grokdocs` to easily expand from Markdown documentation to indexing source code files in the future.
- **Expected Features to Use**:
  - AST-based section/header splitting for `.md` files.
  - Extensible token counters (to match the ONNX model's exact tokenizer).
  - Overlap configuration to preserve boundary context.

### Rationale for Line-Oriented Chunking
`grokdocs` is built as a line-oriented search tool. We map search results back to source files and read the text directly from disk rather than trusting the DB-cached chunks.
- **UX Precision**: Splitting chunks strictly on line boundaries (`\n`) ensures that every search result cleanly maps to integer line ranges (`line_start` to `line_end`). It prevents rendering awkward results that start or end mid-line.
- **Mitigating Semantic Fragmentation via Line Overlap**:
  To prevent sentences or concepts from being cut in half when splitting lines into chunks, we will configure a **line-based overlap** (e.g., carrying over the last 2-3 lines of a chunk to the start of the next chunk). This preserves semantic continuity across boundaries while maintaining clean line-alignment.
