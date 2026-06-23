# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.1] - 2026-06-22
### Added
- Initial release of grokdocs.
- CLI commands: init, sync, embed, search (fts/semantic/hybrid), files, stats, version.
- File ingestion with three-tier filtering (files/include/exclude) and incremental sync.
- Markdown section-aware chunking and AST-based chunking for 30+ languages.
- Full-text search via SQLite FTS5 with BM25 ranking.
- Semantic vector search via FAISS with ONNX-embedded `all-MiniLM-L6-v2` model.
- Hybrid search with Reciprocal Rank Fusion blending.
- Local embedding pipeline with custom SentencePiece Unigram + WordPiece tokenizer.
- YAML-based per-collection configuration.
- Structured logging (text/JSON) and terminal progress bars.
