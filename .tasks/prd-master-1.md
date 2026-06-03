# grokdocs - Phase 1 Implementation Tasks

---

## 1. Product Specification

*Core vision, requirements, and user needs.*

### Feature Overview

grokdocs is a CLI tool for document ingestion, processing, and vector-based semantic search. Phase 1 focuses on setting up the codebase structure, benchmarking harness, Cobra CLI commands, configuration management, SQLite metadata/chunk database, file ingestion/chunking, local ONNX embeddings, and FAISS indexing.

### User Stories / Requirements

- **PRD-001**: Scaffold the Go project structure (`cmd`, `internal`, `pkg`).
- **PRD-002**: Add benchmarks folder and setup initial benchmark harness.
- **PRD-003**: Implement CLI commands structure (`init`, `sync`, `search`) using Cobra with project path flag (`-p`) support.
- **PRD-004**: Initialize and read the configuration folder (containing `config.yaml`, the SQLite DB, and the FAISS file).
- **PRD-005**: Create SQLite model/database to store document chunks and metadata.
- **PRD-006**: Integrate file walking and `gomantics/chunkx` for processing and chunking supported files.
- **PRD-007**: Integrate local ONNX models to convert chunks to vectors on-device.
- **PRD-008**: Integrate FAISS via `go-faiss` to store vectors and perform similarity searches.
- **PRD-009**: Integrate and implement the end-to-end `sync` command pipeline.
- **PRD-010**: Integrate and implement the end-to-end `search` command pipeline.

---

## 2. Active Dashboard

*Current tracker.*

- [x] **PRD-003**: Implement CLI commands structure (`init`, `sync`, `search`) using Cobra with project path flag (`-p`) support
- [ ] **PRD-004**: Initialize and read the configuration folder (containing `config.yaml`, the SQLite DB, and the FAISS file)
- [ ] **PRD-005**: Create SQLite model/database to store document chunks and metadata
- [ ] **PRD-006**: Integrate file walking and `gomantics/chunkx` for processing and chunking supported files
- [ ] **PRD-007**: Integrate local ONNX models to convert chunks to vectors on-device
- [ ] **PRD-008**: Integrate FAISS via `go-faiss` to store vectors and perform similarity searches
- [ ] **PRD-009**: Integrate and implement the end-to-end `sync` command pipeline
- [ ] **PRD-010**: Integrate and implement the end-to-end `search` command pipeline

---

## 3. Active Task Details

*Only contains details/checklists for active tasks in Section 2.*

### PRD-003: Implement CLI commands structure (init, sync, search)

- **Objective**: Provide the command line interface structure for `grokdocs` using Cobra with `-p` flag support, enabling command parsing and basic flag validation.

- **Checklist**:
  - [x] Create CLI command handlers for `init`, `sync`, and `search` using Cobra in `cmd/grokdocs/main.go` or subpackages
  - [x] Support global `-p` flag to specify the project root path for all commands
  - [x] Stub command handlers to print placeholder/debug statements confirming basic functionality and `-p` flag value
  - [x] Ensure command runs correctly and shows Cobra help/errors for invalid options

- **Validation**:
  - Verify `go run ./cmd/grokdocs --help` prints the usage instructions containing all three commands (`init`, `sync`, `search`) and the `-p` flag.
  - Verify invoking `go run ./cmd/grokdocs init -p /tmp/test-project` prints the stub text confirming the parsed path `/tmp/test-project`.

### PRD-004: Initialize and read the configuration folder

- **Objective**: Initialize and read the `.grokdocs` configuration folder (hosting `config.yaml`, the SQLite database, and the FAISS index file) using the root path lookup algorithm.

- **Checklist**:
  - [ ] Implement configuration directory lookup algorithm:
    - If `-p` is specified: Use that path directly as the project root (look for `.grokdocs` inside it).
    - If `-p` is not specified: Start at current working directory (`.`) and walk up parent directories searching for `.grokdocs`.
    - If no `.grokdocs` folder is found up to filesystem root: Use the current directory (`.`) as the fallback root.
  - [ ] Create/generate default `config.yaml` inside `.grokdocs` folder if it does not exist (on `init` command)
  - [ ] Implement loading and parsing of `config.yaml` from the discovered `.grokdocs` directory
  - [ ] Establish paths to the SQLite database (`grokdocs.db`) and FAISS index (`grokdocs.index`) within the discovered `.grokdocs` directory
  - [ ] Document configuration folder lookup mechanism and hierarchical search rationale in `README.md`

- **Validation**:
  - Add a unit test `TestRootDiscovery` verifying the lookup algorithm walks up the directory tree to find `.grokdocs` and returns the expected project root.
  - Verify that running `grokdocs init` creates the `.grokdocs` folder and generates a valid default `config.yaml`.

### PRD-005: Create SQLite model/database to store document chunks and metadata

- **Objective**: Design and implement the SQLite database schema and data access layers to store file chunks and metadata, including FTS5 support.

- **Checklist**:
  - [ ] Add SQLite Go driver package
  - [ ] Design the SQLite schema:
    - `documents` table: `file_path`, `content_hash`, `modified_at` (modified timestamp)
    - `chunks` table: `chunk_id` (numeric or string), `document_path` (foreign key), `chunk_index`, `text_content`, `content_hash`
  - [ ] Enable and configure SQLite FTS5 virtual table indexing on chunk text content
  - [ ] Implement database initialization and schema migration logic (including FTS5 table setup)
  - [ ] Implement database CRUD operations for storing, fetching, and updating chunks while maintaining the FTS5 index

- **Validation**:
  - Run a unit test `TestSQLiteDatabase` that initializes an in-memory database, creates the schema/FTS5 tables, inserts dummy documents/chunks, performs matches on the FTS5 table, and deletes files to ensure cascading updates work.

### PRD-006: Integrate file walking and gomantics/chunkx for processing and chunking supported files

- **Objective**: Implement file system walking to find supported files (Markdown, etc.) and use `chunkx` to process them.

- **Checklist**:
  - [ ] Implement file walker to scan target directories for supported files
  - [ ] Integrate `gomantics/chunkx` library
  - [ ] Process and chunk files into text chunks

- **Validation**:
  - Create a unit test `TestFileWalkingAndChunking` with a temp directory containing dummy Markdown files, verify the file walker discovers them, and verify `chunkx` splits them into text chunks with valid contents and indices.

### PRD-007: Integrate local ONNX models to convert chunks to vectors on-device

- **Objective**: Use a local ONNX model to generate vector embeddings for text chunks.

- **Checklist**:
  - [ ] Load ONNX model locally
  - [ ] Feed text chunks into the model to generate embedding vectors

- **Validation**:
  - Add a unit test `TestONNXEmbeddings` verifying that loading the model and embedding a test text sentence returns a valid floating-point slice of expected dimension (e.g. 384 or 768 float values).

### PRD-008: Integrate FAISS via go-faiss to store vectors and perform similarity searches

- **Objective**: Embed FAISS index storage and similarity search capability using the `go-faiss` bindings.

- **Checklist**:
  - [ ] Integrate `go-faiss` dependency
  - [ ] Save embedding vectors to the FAISS index
  - [ ] Query FAISS index for nearest neighbors

- **Validation**:
  - Implement a unit test `TestFAISSIndex` which creates a FAISS index, writes dummy embedding vectors, queries the index for similarity, and asserts that the returned nearest IDs and distance values are correct.

### PRD-009: Integrate and implement the end-to-end sync command pipeline

- **Objective**: Connect file chunking, SQLite storage, ONNX embedding generation, and FAISS indexing into a cohesive document synchronization pipeline.

- **Checklist**:
  - [ ] Support and parse `--all` (boolean flag) and `--collection <name>` (string flag) for `sync` command
  - [ ] Validate mutual exclusion of `--all` and `--collection`
  - [ ] Integrate SQLite DB and FAISS index setup with config path lookup
  - [ ] Run file walker to discover supported files
  - [ ] Implement change-detection algorithm for discovered files:
    - Compare file modified timestamp (`mtime`) with stored `modified_at` in SQLite. If equal, skip the file.
    - If `mtime` differs, compute the file's current content hash and compare with the stored `content_hash` in SQLite.
    - If content hash matches, update only the stored `modified_at` timestamp in SQLite, and skip processing.
    - If content hash differs, proceed to re-chunk the file.
  - [ ] Compute ONNX embeddings for newly chunked/modified content
  - [ ] Save chunk metadata and content hash in SQLite database, and vectors in FAISS index (removing old records and vectors for modified files)

- **Validation**:
  - Sync a directory of test files: verify SQLite has chunks and FAISS has embeddings.
  - Run sync again: verify from logs/performance that the scan completes instantly by skipping files based on matching modified time.
  - Touch a file (change modification timestamp but not content), run sync: verify that it skips embedding computation and only updates SQLite timestamp.
  - Modify file content, run sync: verify only that file's chunks are recomputed and stored.

### PRD-010: Integrate and implement the end-to-end search command pipeline

- **Objective**: Connect query input, local ONNX query embedding, similarity search, chunk retrieval, and terminal formatting into a functional CLI search interface with hybrid FTS and vector search capabilities.

- **Checklist**:
  - [ ] Support and parse CLI flags: `--collection <name>`, `--mode <hybrid|vector|fts>` (default: `hybrid`), `--alpha <float>` (default: `0.5`, range: `0.0` to `1.0`), and `--limit <int>` (default: `5`)
  - [ ] Validate `--alpha` value bounds (must be between 0.0 and 1.0)
  - [ ] Accept query string and load FAISS index + SQLite DB
  - [ ] In `fts` mode: Perform full-text keyword search in SQLite FTS5 index
  - [ ] In `vector` mode: Generate query embedding using local ONNX model and retrieve nearest vectors from FAISS index
  - [ ] In `hybrid` mode: Run both FTS and vector searches, normalize scores, apply linear interpolation weight ($score = \alpha \cdot vector\_score + (1 - \alpha) \cdot fts\_score$), and rerank
  - [ ] Map matching result IDs to SQLite chunk records to retrieve text content and metadata
  - [ ] Print matching search results with file paths and snippet contents to the console

- **Validation**:
  - Verify running `grokdocs search --mode fts "test"` returns only keyword search matches.
  - Verify running `grokdocs search --mode vector "test"` returns semantic search matches.
  - Verify running `grokdocs search --mode hybrid --alpha 0.5 "test"` retrieves both and correctly reranks results.
  - Verify invalid `--alpha 1.5` or `-0.1` throws an argument error.

---

## 4. Future Roadmap & Backlog

*Placeholders for future tasks.*

*(No backlog items for now, all items are part of Phase 1)*

---

## 5. History / Archive

*When a task is completed, cut-and-paste both its Dashboard status and its Detailed Requirements/Checklist here.*

- [x] **PRD-001**: Scaffold the Go project structure (`cmd`, `internal`, `pkg`)
- [x] **PRD-002**: Add benchmarks folder and setup initial benchmark harness

### PRD-001: Scaffold Go Project Structure

- **Objective**: Establish the foundational directory and package structure for `grokdocs` following standard Go layout conventions.

- **Checklist**:
  - [x] Create `cmd/grokdocs/` directory to hold the main entrypoint (`main.go`)
  - [x] Create `internal/` directory to encapsulate private application logic
  - [x] Create `internal/ingest/` for parsing and chunking logic
  - [x] Create `internal/search/` for vector search and embedding
  - [x] Create `internal/config/` for configuration settings
  - [x] Create `pkg/` directory for shared types or utilities
  - [x] Initialize dummy files to ensure module builds successfully
  - [x] Setup module path as `github.com/minhhh/grokdocs` in `go.mod`
  - [x] Add `LICENSE` (MIT) file
  - [x] Add `CHANGELOG.md` file

### PRD-002: Add Benchmarks Folder

- **Objective**: Establish a dedicated structure for performance testing the vector search and file chunking logic.

- **Checklist**:
  - [x] Create a `benchmarks/` folder at the root of the project
  - [x] Include a `README.md` within it to explain how to run the Go benchmarks
