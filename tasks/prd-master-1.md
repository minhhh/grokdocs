# grokdocs - Phase 1 Implementation Tasks

- [x] **PRD-001**: Scaffold the Go project structure (`cmd`, `internal`, `pkg`).
- [ ] **PRD-002**: Implement CLI commands structure (`init`, `sync`, `search`, `mcp`) using Cobra.
- [ ] **PRD-003**: Create default configuration generation for `grokdocs.yml`.
- [ ] **PRD-004**: Integrate file walking and `gomantics/chunkx` for processing and chunking `.md` files.
- [ ] **PRD-005**: Integrate local ONNX models to convert chunks to vectors on-device.
- [ ] **PRD-006**: Integrate FAISS via `go-faiss` to store vectors and perform similarity searches.
- [ ] **PRD-007**: Build terminal output formatting for search results (displaying file paths and matched chunk text).
- [x] **PRD-008**: Add benchmarks folder and setup initial benchmark harness.

---

## PRD-001: Scaffold Go Project Structure

### Objective
Establish the foundational directory and package structure for `grokdocs` following standard Go layout conventions.

### Requirements
- Create `cmd/grokdocs/` directory to hold the main entrypoint (`main.go`).
- Create `internal/` directory to encapsulate private application logic:
  - `internal/ingest/`: For file parsing and chunking logic.
  - `internal/search/`: For vector search and embedding logic.
  - `internal/config/`: For parsing and managing application settings.
- Create `pkg/` directory for any shared types or utilities that could theoretically be consumed externally.
- Initialize necessary dummy files to ensure the module builds successfully.
- Setup module path as `github.com/minhhh/grokdocs`.
- Add `LICENSE` (MIT) file.
- Add `CHANGELOG.md` file.

### Acceptance Criteria
- `go build ./cmd/grokdocs` successfully compiles an executable.
- The directory tree cleanly separates CLI bootstrapping from core logic.

---

## PRD-002: Implement CLI commands structure (init, sync, search, mcp)

### Objective
Provide the command line interface structure for `grokdocs` using Cobra, enabling command parsing and basic flag validation.

### Requirements
- Implement the following commands:
  - `init`: Setup/initialize a new workspace.
  - `sync`: Synchronize files. Must support:
    - `--all` (boolean flag)
    - `--collection <name>` (string flag)
    - Mutually exclusive: `--all` and `--collection` cannot be used together.
    - Default behavior: Running without flags synchronizes the default collection.
  - `search`: Query indexed documents. Must support:
    - `--collection <name>` (string flag)
  - `mcp`: Start the Model Context Protocol (MCP) server.
- Stub command handlers that print placeholder/debug statements indicating they were called with the parsed flags.
- Commands must be integrated under `cmd/grokdocs/main.go`.

### Acceptance Criteria
- Running `grokdocs sync` with no flags prints a message confirming sync for the "default" collection.
- Running `grokdocs sync --all` prints a message confirming the sync-all execution.
- Running `grokdocs sync --collection my_docs` prints a message confirming sync for "my_docs".
- Running `grokdocs sync --all --collection my_docs` fails with a mutual exclusion error.
- Running `grokdocs search "test query" --collection my_docs` prints the search query and collection name.
- Running `grokdocs mcp` prints a message confirming that the MCP server is starting.
- Invalid options or missing required arguments should trigger standard Cobra usage/error output.

---

## PRD-008: Add Benchmarks Folder

### Objective
Establish a dedicated structure for performance testing the vector search and file chunking logic.

### Requirements
- Create a `benchmarks/` folder at the root of the project.
- Include a `README.md` within it to explain how to run the Go benchmarks.

### Acceptance Criteria
- The `benchmarks/` directory exists and is tracked by Git.
