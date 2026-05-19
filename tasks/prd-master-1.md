# grokdocs - Phase 1 Implementation Tasks

- [x] **PRD-001**: Scaffold the Go project structure (`cmd`, `internal`, `pkg`).
- [ ] **PRD-002**: Implement CLI commands structure (`init`, `index`, `search`) using Cobra.
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

## PRD-008: Add Benchmarks Folder

### Objective
Establish a dedicated structure for performance testing the vector search and file chunking logic.

### Requirements
- Create a `benchmarks/` folder at the root of the project.
- Include a `README.md` within it to explain how to run the Go benchmarks.

### Acceptance Criteria
- The `benchmarks/` directory exists and is tracked by Git.
