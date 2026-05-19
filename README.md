# grokdocs

## Overview
**grokdocs** is a local-first indexer that helps you search code and documentation both semantically and via full-text. It features a built-in MCP (Model Context Protocol) server, allowing LLMs to search your knowledge base locally, saving you massive amounts of context tokens.

## Core Features
### 1. Document Ingestion & Chunking
- **Multi-format Support**: Parse Markdown, plain text, and Go source files.
- **Intelligent Chunking**: Use `gomantics/chunkx` to break down large documents into semantically meaningful chunks rather than arbitrary character counts.
- **Incremental Indexing**: Track file modification times (mtime) and sizes. Only re-chunk and re-embed files that have changed to save time and compute.

### 2. Hybrid Search Engine
- Generate embeddings for each chunk strictly using a local ONNX model.
- Store embeddings in a local FAISS index for semantic search, and insert chunk text into SQLite for Full-Text Search.
- When querying, combine the vector similarity and exact text match to retrieve the `Top K` most relevant chunks.

## CLI Interface
- `grokdocs init`: Generate a default configuration file (`grokdocs.yml`).
- `grokdocs index [path]`: Scan the directory, chunk documents, generate embeddings, and update the FAISS index.
- `grokdocs search "how does the auth system work?"`: Perform a semantic search and return beautifully formatted results in the terminal, including the file path, line numbers, and a preview of the chunk.

### 3. LLM Integration
- **Model Context Protocol (MCP)**: Run an MCP server so your LLMs (like Claude Desktop) can autonomously search your documentation.
- **Token Efficiency**: Save massive amounts of context window tokens by only injecting the exact relevant document chunks the LLM needs to answer your question, rather than pasting entire files.
