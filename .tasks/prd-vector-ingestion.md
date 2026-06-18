# Vector Embedding Ingestion

---

## 1. Product Specification

*Wire up ONNX embedding + FAISS vector insertion during `sync`, align chunk size with model's 512-token limit, and validate the end-to-end search flow.*

### User Stories / Requirements

- **us-01**: As a user, when I run `grokdocs sync`, text chunks are automatically embedded and stored in a per-collection FAISS index so that semantic/hybrid search finds results.
- **us-02**: As a user, chunks fit within the model's 512-token context window so embeddings aren't silently truncated.
- **us-03**: As a user, when files are deleted or moved during sync, stale vectors are pruned from FAISS to match SQLite.
- **us-04**: As a user, I can build with or without the `onnx` tag — vector ingestion only runs when the tag is present.

---

## 2. Active Dashboard

*All tasks complete — empty.*

---

## 3. Active Task Details

*All tasks archived.*

### us-01: Embed chunks and push vectors to per-collection FAISS indexes during sync

- **Objective**: After `SaveChunksBatch` in `ingestFile`, embed each chunk's text via ONNX and call `vdb.AddWithIDs(chunkIDs, vectors)`. Each collection gets its own FAISS index file (`grokdocs-{collection}.index`), eliminating the need for post-filter collection checks during search.

- **Design**:
  - `Project` gains `OpenCollectionVector(collection string)` which opens `grokdocs-{collection}.index`
  - `VectorIndexName` constant replaced with `func CollectionIndexName(collection string) string`
  - During `SyncCollection`, open only that collection's vector DB
  - During search:
    - `--collection docs --mode semantic`: open `grokdocs-docs.index`, no post-filter
    - `--mode semantic` (no collection): use the **default** collection (same as sync default)
    - No more post-filter collection check in `search_onnx.go`

- **Checklist**:
  - [x] Add `CollectionIndexName(collection string)` helper in `project.go`
  - [x] Add `OpenCollectionVector(collection string)` method on `Project`
  - [x] Keep the old `OpenVector()` for backward compat (still used in tests)
  - [x] Open per-collection vector DB in `SyncCollection` (only when onnx tag present)
  - [x] After `SaveChunksBatch` succeeds, iterate saved chunks, call `embedder.Embed(chunk.TextContent)` for each, collect `(chunkID, vector)` pairs
  - [x] Flush all vectors to FAISS via `vdb.AddWithIDs(ids, flatVectorSlice)`
  - [x] Save FAISS index via `vdb.Save()` after sync completes
  - [x] Update `search_onnx.go`: remove collection post-filter, resolve collection from `--collection` flag or default
  - [x] Handle build-tag: extraction into onnx-guarded files (similar to `search_onnx.go`)
  - [x] Write test: `TestSyncWithVectors` end-to-end, `TestFAISSRemoveIDs` unit test

### us-02: Align chunk size with 512-token model limit

- **Objective**: Bump `maxSeqLen` to 512 and adjust `DefaultChunkMaxSize` so chunks fit without truncation.

- **Details**: chunkx's `SimpleTokenCounter` counts whitespace-separated words. 400 words → ~500 BERT WordPiece tokens → fits within 512 with margin. `DefaultChunkMaxSize`: 500 → 400. `maxSeqLen`: 256 → 512.

- **Checklist**:
  - [x] Change `maxSeqLen` in `internal/embed/downloader.go` from 256 to 512
  - [x] Change `DefaultChunkMaxSize` in `internal/parser/parser.go` from 500 to 400
  - [ ] Verify with a markdown file that chunks stay under 512 BERT tokens

### us-03: Prune stale vectors on delete/move

- **Objective**: When `SyncCollection` deletes files (prune phase), also remove their chunk vectors from FAISS using `RemoveIDs`. No rebuild needed — `IDMap,Flat` supports selective deletion via `faiss.IDSelectorBatch`.

- **Checklist**:
  - [x] Add `RemoveIDs` method to `VectorDatabase` using `faiss.NewIDSelectorBatch`
  - [x] Add `GetChunkIDsByFileID` helper to `FTSDatabase`
  - [x] Wire up pruning in `SyncCollection`: collect chunk IDs before `DeleteFilesBatch`, call `vdb.RemoveIDs` + `vdb.Save` after
  - [x] Verify: builds and tests pass

### us-04: Conditional compilation with onnx build tag

- **Objective**: Vector ingestion is guarded by `//go:build onnx`. Without the tag, sync skips embedding and FAISS entirely (SQLite-only, as today).

- **Checklist**:
  - [x] Extract onnx-dependent ingestion code into `ingest_onnx.go` / `ingest_noonnx.go` pattern (same as `search_onnx.go`)
  - [x] Without onnx tag: `SyncCollection` opens only FTS DB, no vector operations
  - [x] With onnx tag: `SyncCollection` opens both DBs, runs embedding + FAISS write

---

## 4. Future Roadmap & Backlog

- [ ] **us-xx**: Add `--all` flag to search to query across all collections (iterate per-collection FAISS indexes and merge results)
- [ ] **us-xx**: Batch embedding — embed multiple chunks in one ONNX run for throughput
- [ ] **us-xx**: Progress reporting for embedding phase during sync
- [ ] **us-xx**: Hierarchical chunking — max parent size 1000 words, split into overlapping sub-chunks of ~400 words (10% overlap), embed sub-chunks for FAISS, search on sub-chunks, roll up by max score to parent
  - New table `sub_chunks(id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES chunks(id) ON DELETE CASCADE)` — no content, no line numbers, just the link
  - Sub-chunks are embedded into FAISS (sub-chunk ID as label), parent chunks stored in SQLite with content
  - Search returns sub-chunk IDs → lookup `parent_id` in `sub_chunks` → get parent's file path and line range → LLM reads source file directly
  - Pruning: when parent chunks are deleted, `ON DELETE CASCADE` removes `sub_chunks` rows. Before deletion, collect affected sub-chunk IDs via `SELECT id FROM sub_chunks WHERE parent_id IN (...)`, then call `vdb.RemoveIDs(ids)`

---

## 5. Archive

- [x] **us-01**: Embed chunks and push vectors to per-collection FAISS indexes during sync
- [x] **us-02**: Align chunk size with 512-token model limit
- [x] **us-03**: Prune stale vectors on delete/move
- [x] **us-04**: Conditional compilation with onnx build tag

### us-01: Embed chunks and push vectors to per-collection FAISS indexes during sync

- **Objective**: After `SaveChunksBatch` in `ingestFile`, embed each chunk's text via ONNX and call `vdb.AddWithIDs(chunkIDs, vectors)`. Each collection gets its own FAISS index file (`grokdocs-{collection}.index`).

- **Checklist**:
  - [x] Add `CollectionIndexName(collection string)` helper in `project.go`
  - [x] Add `OpenCollectionVector(collection string)` method on `Project`
  - [x] Keep the old `OpenVector()` for backward compat (still used in tests)
  - [x] Open per-collection vector DB in `SyncCollection` (only when onnx tag present)
  - [x] After `SaveChunksBatch` succeeds, iterate saved chunks, call `embedder.Embed(chunk.TextContent)` for each, collect `(chunkID, vector)` pairs
  - [x] Flush all vectors to FAISS via `vdb.AddWithIDs(ids, flatVectorSlice)`
  - [x] Save FAISS index via `vdb.Save()` after sync completes
  - [x] Update `search_onnx.go`: remove collection post-filter, resolve collection from `--collection` flag or default
  - [x] Handle build-tag: extraction into onnx-guarded files (similar to `search_onnx.go`)
  - [x] Write test: `TestSyncWithVectors` end-to-end, `TestFAISSRemoveIDs` unit test

### us-02: Align chunk size with 512-token model limit

- **Objective**: Bump `maxSeqLen` to 512 and adjust `DefaultChunkMaxSize` so chunks fit without truncation.

- **Checklist**:
  - [x] Change `maxSeqLen` in `internal/embed/downloader.go` from 256 to 512
  - [x] Change `DefaultChunkMaxSize` in `internal/parser/parser.go` from 500 to 400
  - [ ] Verify with a markdown file that chunks stay under 512 BERT tokens

### us-03: Prune stale vectors on delete/move

- **Objective**: When `SyncCollection` deletes files (prune phase), also remove their chunk vectors from FAISS using `RemoveIDs`.

- **Checklist**:
  - [x] Add `RemoveIDs` method to `VectorDatabase` using `faiss.NewIDSelectorBatch`
  - [x] Add `GetChunkIDsByFileID` helper to `FTSDatabase`
  - [x] Wire up pruning in `SyncCollection`: collect chunk IDs before `DeleteFilesBatch`, call `vdb.RemoveIDs` + `vdb.Save` after
  - [x] Verify: builds and tests pass

### us-04: Conditional compilation with onnx build tag

- **Objective**: Vector ingestion is guarded by `//go:build onnx`. Without the tag, sync skips embedding and FAISS entirely (SQLite-only, as today).

- **Checklist**:
  - [x] Extract onnx-dependent ingestion code into `ingest_onnx.go` / `ingest_noonnx.go` pattern (same as `search_onnx.go`)
  - [x] Without onnx tag: `SyncCollection` opens only FTS DB, no vector operations
  - [x] With onnx tag: `SyncCollection` opens both DBs, runs embedding + FAISS write
