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
