# grokdocs Benchmarks

This directory contains scripts and datasets used to performance-test the
ingestion and vector-search capabilities of `grokdocs`.

## Running Benchmarks

Benchmarks must be run with the appropriate build tags to match the feature
set you want to measure.

### FTS5-only benchmarks

```bash
go test -tags fts5 -bench=. ./...
```

### Full-stack benchmarks (FTS5 + ONNX + FAISS)

```bash
go test -tags fts5,onnx -bench=. ./...
```

### Run a specific benchmark

```bash
go test -tags fts5,onnx -bench=BenchmarkName ./internal/ingest/
```

Use `-benchmem` to include memory allocation statistics:

```bash
go test -tags fts5,onnx -bench=. -benchmem ./...
```

> **Note:** Benchmarks that depend on ONNX models or FAISS will only compile
> and run when the `onnx` build tag is supplied (gated by `//go:build onnx`).
> The `fts5` tag is always required for FTS5-related functionality.
