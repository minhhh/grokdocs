# grokdocs - Phase 1 Implementation Tasks

---

## 1. Product Specification

*Core vision, requirements, and user needs.*

### Feature Overview

grokdocs is a CLI tool for document ingestion, processing, and Full-Text Search. Phase 1 focuses on setting up the codebase structure, benchmarking harness, Cobra CLI commands, configuration management, SQLite metadata/chunk database, file ingestion/chunking, decoupled/custom parsing, structured logging utility, and diagnostic status reporting. All semantic vector search, local ONNX embedding generation, and FAISS indexing features are deferred to the Future Roadmap & Backlog.

### User Stories / Requirements

- **PRD-001**: Scaffold the Go project structure (`cmd`, `internal`, `pkg`).
- **PRD-002**: Add benchmarks folder and setup initial benchmark harness.
- **PRD-003**: Implement CLI commands structure (`init`, `sync`, `search`) using Cobra with project path flag (`-p`) support.
- **PRD-004**: Initialize project workspace and FTS database.
- **PRD-005**: Create SQLite model/database to store document chunks and metadata.
- **PRD-006**: Integrate file walking and `gomantics/chunkx` for processing and chunking supported files.
- **PRD-007**: Integrate and implement the end-to-end `sync` command pipeline (FTS only).
- **PRD-008**: Integrate and implement the end-to-end `search` command pipeline (FTS only).
- **PRD-009**: Decoupled & Flexible File Parsing with Chunkx Support.
- **PRD-010**: Add Util Package and Slog Logger Wrapper.
- **PRD-011**: Implement status and status root CLI commands.
- **PRD-012**: Move Util to Internal and Expose Logger Variable.

---

## 2. Active Dashboard

*Current tracker.*

- [x] **PRD-012**: Move Util to Internal and Expose Logger Variable

---

## 3. Active Task Details

*Only contains details/checklists for active tasks in Section 2.*

### PRD-012: Move Util to Internal and Expose Logger Variable

- **Objective**: Refactor the codebase to move the utility package from `pkg/util` to `internal/util`. Change the structured logging design to use a global package-level `util.Logger` variable. Additionally, refactor `LogFormat` using the opaque struct pattern to prevent callers from passing arbitrary values.

- **Checklist**:
  - [x] Move files in `pkg/util/` to `internal/util/`.
  - [x] Refactor `LogFormat` to use the compile-time type-safe opaque struct pattern (struct with unexported private field).
  - [x] Expose a global package-level logger variable `var Logger *slog.Logger` in package `util`.
  - [x] Initialize `Logger` with a default/fallback logger (e.g. text format, info level to stderr) so that imports don't panic before `InitLogger` is called.
  - [x] Refactor `InitLogger` to configure the global `Logger` variable instead of a hidden package global.
  - [x] Update all import paths referencing `github.com/minhhh/grokdocs/pkg/util` to `github.com/minhhh/grokdocs/internal/util`.
  - [x] Update all logging calls in the codebase (`cmd/grokdocs`, `internal/ingest`, `internal/project`, etc.) from package functions like `util.Info` or `util.Debug` to `util.Logger.Info` and `util.Logger.Debug`.
  - [x] Update all tests referencing `pkg/util` to reference `internal/util`, ensuring they pass successfully.

- **Validation**:
  - [x] Verify that the codebase compiles cleanly.
  - [x] Run `go test -tags fts5 ./...` to ensure all tests pass.
  - [x] Verify that running CLI commands produces structured log outputs as expected.

---

## 4. Future Roadmap & Backlog

*Placeholders for future tasks.*

- [ ] **PRD-013**: Integrate local ONNX models to convert chunks to vectors on-device
- [ ] **PRD-014**: Integrate FAISS via `go-faiss` to store vectors and perform similarity searches

### PRD-013: Integrate local ONNX models to convert chunks to vectors on-device

- **Objective**: Use a local ONNX model (such as `all-MiniLM-L6-v2` or `bge-small-en-v1.5`) to generate vector embeddings for text chunks. The application should dynamically download the model and tokenizer files on first run if they are not cached.

- **Checklist**:
  - [ ] Implement dynamic model downloader:
    - Define default download URLs (e.g. Hugging Face CDN) for the ONNX model and tokenizer/vocab.
    - Check if the model files exist in the `.grokdocs` cache directory on run.
    - If missing, download them dynamically and save them locally.
  - [ ] Load ONNX model locally using ONNX Runtime Go bindings.
  - [ ] Feed text chunks into the model to generate embedding vectors.

- **Validation**:
  - Add a unit test `TestONNXEmbeddings` verifying that:
    - The downloader successfully retrieves the model and tokenizer files.
    - Loading the model and embedding a test text sentence returns a valid floating-point slice of expected dimension (e.g., 384 float values for MiniLM).

### PRD-014: Integrate FAISS via go-faiss to store vectors and perform similarity searches

- **Objective**: Embed FAISS index storage and similarity search capability using the `go-faiss` bindings inside the `VectorDatabase` wrapper.

- **Checklist**:
  - [ ] Integrate `go-faiss` dependency
  - [ ] Implement vector addition and saving/loading in `VectorDatabase`
  - [ ] Implement nearest neighbor queries in `VectorDatabase`

- **Validation**:
  - Implement a unit test `TestFAISSIndex` which initializes a `VectorDatabase` instance, writes dummy embedding vectors, queries the index for similarity, and asserts that the returned nearest IDs and distance values are correct.

---

## 5. History / Archive

*When a task is completed, cut-and-paste both its Dashboard status and its Detailed Requirements/Checklist here.*

- [x] **PRD-001**: Scaffold the Go project structure (`cmd`, `internal`, `pkg`)
- [x] **PRD-002**: Add benchmarks folder and setup initial benchmark harness
- [x] **PRD-003**: Implement CLI commands structure (`init`, `sync`, `search`) using Cobra with project path flag (`-p`) support
- [x] **PRD-004**: Initialize project workspace and FTS database
- [x] **PRD-005**: Create SQLite model/database to store document chunks and metadata
- [x] **PRD-006**: Integrate file walking and `gomantics/chunkx` for processing and chunking supported files, and implement FTS search and sync
- [x] **PRD-007**: Integrate and implement the end-to-end `sync` command pipeline (FTS only)
- [x] **PRD-008**: Integrate and implement the end-to-end `search` command pipeline (FTS only)
- [x] **PRD-009**: Decoupled & Flexible File Parsing with Chunkx Support
- [x] **PRD-010**: Add Util Package and Slog Logger Wrapper
- [x] **PRD-011**: Implement status and status root CLI commands

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

### PRD-004: Initialize project workspace and FTS database

- **Objective**: Establish the concept of a `Project` that manages configuration and the FTS database (SQLite-based metadata/text search storage). It will initialize the workspace directories, generate defaults, and support opening/loading the FTS database.

- **Checklist**:
  - [x] Implement `Project` struct representing the workspace:
    - Holds the root path and directory path for the configuration folder (`.grokdocs`).
    - Holds the configuration state (parsed from `config.yaml`).
    - Manages lifecycle of the FTS database.
  - [x] Implement project directory lookup algorithm:
    - If `-p` is specified: Use that path directly as the project root (look for `.grokdocs` inside it).
    - If `-p` is not specified: Start at current working directory (`.`) and walk up parent directories searching for `.grokdocs`.
    - If no `.grokdocs` folder is found up to filesystem root: Use the current directory (`.`) as the fallback root.
  - [x] Implement configuration file initialization & loading:
    - Create/generate default `config.yaml` inside `.grokdocs` folder if it does not exist (on `init` command).
    - Implement loading and parsing of `config.yaml` from the discovered `.grokdocs` directory.
  - [x] Implement the `FTSDatabase` wrapper/interface:
    - Encapsulates SQLite connection and paths for SQLite metadata storage (e.g., `.grokdocs/grokdocs.db`).
    - Exposes basic initialization / opening methods.
  - [x] Document project structure, database lookup mechanism, and hierarchy search in `README.md`.

- **Validation**:
  - Add unit tests verifying:
    - The lookup algorithm walks up parent directories to find `.grokdocs` and returns the expected root.
    - Initializing a project generates `.grokdocs` and the default `config.yaml`.
    - A Project can open/close the `FTSDatabase`.

### PRD-005: Create SQLite model/database to store document chunks and metadata

- **Objective**: Design and implement the normalized `FTSDatabase` SQLite schema and data access layers to store files, document collection maps, chunks, and keyword Full-Text Search (FTS5) indices.

- **Checklist**:
  - [x] Implement database initialization and schema migration logic, enabling FOREIGN KEY support.
  - [x] Design the SQLite schema inside the `FTSDatabase` structure:
    - `files` table: `id` (INTEGER PRIMARY KEY AUTOINCREMENT), `file_path` (TEXT UNIQUE), `filename` (TEXT), `size` (INTEGER), `modified_at` (INTEGER), `content_hash` (TEXT)
    - `documents` table: `id` (INTEGER PRIMARY KEY AUTOINCREMENT), `file_id` (INTEGER, FOREIGN KEY to `files(id)`), `collection` (TEXT), `slug` (TEXT), `chunk_count` (INTEGER), `total_chars` (INTEGER), and a UNIQUE constraint on `(file_id, collection)`
    - `chunks` table: `id` (INTEGER PRIMARY KEY AUTOINCREMENT, maps 1-to-1 to FTS virtual table IDs), `document_id` (INTEGER, FOREIGN KEY to `documents(id)`), `chunk_index` (INTEGER), `text_content` (TEXT), `content_hash` (TEXT), `total_chars` (INTEGER)
  - [x] Enable and configure SQLite FTS5 virtual table (`chunks_fts`) indexing on chunk text content.
  - [x] Implement CRUD operations for storing, fetching, and updating chunks, documents, and files while keeping the FTS5 index synchronized.
  - [x] Move the database schema query strings in InitSchema to constants.

- **Validation**:
  - Run a unit test `TestSQLiteDatabase` that initializes `FTSDatabase` (e.g., using an in-memory database), creates the tables (including FTS5), inserts dummy files, document maps, and chunks, performs keyword matches on the FTS5 table, and deletes files/documents to verify cascading deletes and synchronization work as expected.

### PRD-006: Integrate file walking and gomantics/chunkx for processing and chunking supported files, and implement FTS search and sync

- **Objective**: Implement recursive directory scanning to discover supported files, integrate the `gomantics/chunkx` library to split documents into semantically coherent chunks, extract structural metadata, and implement end-to-end command-line `sync` and `search` functionality using the SQLite Full-Text Search (FTS) database.

- **Checklist**:
  - [x] Run `go get github.com/gomantics/chunkx` and update the dependency graph.
  - [x] Implement directory traversal package under `internal/ingest` with ignore rules for dotfiles/directories.
  - [x] Implement chunking integration that passes document contents to `chunkx`.
  - [x] Implement metadata extractor to map chunk outputs back to line numbers (`line_start`/`line_end`).
  - [x] Implement Markdown heading parser to extract `section_title` and track `section_num`.
  - [x] Generate JSON metadata for the documents and chunks.
  - [x] Integrate file walking and chunking under the package `internal/ingest`.
  - [x] Implement FTS `sync` logic connecting the walker, chunker, and database with change-detection (timestamp and hash comparison) and cleanup of deleted files.
  - [x] Connect CLI `sync` command to run the FTS sync pipeline for collections configured in `config.yaml`.
  - [x] Implement FTS `search` logic connecting CLI `search` command to SQLite FTS5 keyword queries.

- **Validation**:
  - Create a unit test `TestFileWalking` using `t.TempDir()` to verify that valid files are discovered and excluded directories are ignored.
  - Create a unit test `TestMarkdownChunking` to verify chunking correctness, line range mappings, heading/section details parsing, and JSON serialization.

### PRD-007: Integrate and implement the end-to-end sync command pipeline (FTS only)

- **Objective**: Connect file traversal, SQLite storage, chunking, and change detection (modification time and SHA-256 hash checking) into a cohesive offline FTS document synchronization command.

- **Checklist**:
  - [x] Support and parse `--all` (boolean flag) and `--collection <name>` (string flag) for `sync` command
  - [x] Validate mutual exclusion of `--all` and `--collection`
  - [x] Initialize the `Project` workspace and obtain the `FTSDatabase` instance
  - [x] Run file walker to discover supported files
  - [x] Implement change-detection algorithm for discovered files comparing `modified_at` and `content_hash`
  - [x] Clean up dangling database file records for files that no longer exist on disk (cascade deletes)
  - [x] Connect CLI `sync` command to run the FTS sync pipeline for collections configured in `config.yaml`

- **Validation**:
  - Run sync on a temp directory of Markdown files: verify SQLite is populated.
  - Modify/touch a file, run sync: verify incremental sync skips/updates records appropriately.

### PRD-008: Integrate and implement the end-to-end search command pipeline (FTS only)

- **Objective**: Connect CLI query input, keyword FTS search on SQLite virtual table, result formatting, line mapping, and file text snippet reading directly from disk into a functional command-line search interface.

- **Checklist**:
  - [x] Support and parse CLI flags: `--collection <name>`, `--mode <hybrid|semantics|fts>` (default: `hybrid` with warning fallback to `fts`), and `--limit <int>` (default: `5`)
  - [x] Accept query string and query `SearchFTS`
  - [x] Map matching results back to source file lines and display snippets directly read from disk

- **Validation**:
  - Run search CLI command: verify keyword matches are printed with correct line ranges and snippet outputs.

### PRD-009: Decoupled & Flexible File Parsing with Chunkx Support

- **Objective**: Implement a flexible document-parsing architecture in `grokdocs` to extract structured data from various file types. This system maps file patterns (filenames, complex extensions, and simple extensions) to specific parsers, resolving matches using a built-in priority algorithm where specific filenames have the highest priority.

- **Checklist**:
  - [x] Define the generic `Parser` interface and registry in `internal/ingest` to separate parsing logic from file-pattern mapping.
  - [x] Update configuration models in `internal/config` to support `ParserMap` as a `map[string]string` mapping pattern to parser names.
  - [x] Support global `default_parsers` map at the root of `config.yaml` and collection-level parser map overrides.
  - [x] Implement a general-purpose `ChunkxParser` that wraps `chunkx` AST chunker and maps extension/language types to their corresponding `chunkx.WithLanguage()` options.
  - [x] Implement the priority-based routing algorithm to select the best-matching parser (e.g. Exact File Match > Complex Extension > Simple Extension > Wildcard).
  - [x] Integrate parser routing with `SyncCollection` in `internal/ingest`.
  - [x] Write unit tests to verify matching precedence (e.g., ensuring `hello.md` matches `HelloMarkdown` instead of `.md` / `*.md` parser), chunkx parser routing, and fallback behavior.

- **Validation**:
  - [x] Add unit tests verifying `matchesRule` and priority resolution for exact files, complex extensions, and simple extensions.
  - [x] Add tests verifying parser override resolution and the generic chunkx parser on different files.

### PRD-010: Add Util Package and Slog Logger Wrapper

- **Objective**: Implement a shared utility package under `pkg/util` providing a structured logging module that wraps Go's standard library `log/slog`. It should support multiple levels (Debug, Info, Warn, Error), custom output format selection (JSON vs. readable Text), and be integrated into the CLI commands and internal packages to unify logging output.

- **Checklist**:
  - [x] Create package directory `pkg/util`.
  - [x] Implement `logger.go` wrapping `log/slog` handlers.
  - [x] Support log level configuration (Debug, Info, Warn, Error).
  - [x] Support format configuration (JSON for machines, human-readable Text for CLI).
  - [x] Integrate the logger into CLI command handlers (`cmd/grokdocs`) and internal modules (`internal/ingest` and `internal/project`) to replace raw `fmt.Printf` / `fmt.Fprintf` statements.
  - [x] Write unit tests verifying logger outputs, custom levels, and format rendering.

- **Validation**:
  - [x] Add unit tests validating that the logger correctly writes structured attributes, respects configured log levels, and switches formats (JSON vs Text).
  - [x] Verify that running CLI commands (`sync`, `search`) output logs using the structured format when requested.

### PRD-011: Implement status and status root CLI commands

- **Objective**: Add a diagnostic `status` command and a `status root` subcommand to report the state of the grokdocs workspace, collection metadata, and file indexing stats.

- **Checklist**:
  - [x] Implement `statusCmd` in `cmd/grokdocs` that displays indexed statistics.
  - [x] Implement `statusRootCmd` as a subcommand under `status` that prints the resolved absolute path of the `.grokdocs` directory.
  - [x] Query FTSDatabase to fetch summary statistics:
    - Count of configured collections.
    - Total files indexed.
    - Total documents indexed per collection.
    - Total chunks in database.
    - Total characters indexed.
  - [x] Render the statistics to stdout in a clean, human-readable format.
  - [x] Verify that running with an invalid workspace root reports the fallback paths correctly.

- **Validation**:
  - [x] Verify invoking `./grokdocs status root` prints the correct absolute path of the local `.grokdocs` folder.
  - [x] Verify invoking `./grokdocs status` prints exact counts matching the SQLite FTS database contents (e.g. comparing chunks, document counts).

