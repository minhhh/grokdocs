VERSION ?= dev

.PHONY: build-fts build-full build-all clean

build-fts:
	go build -tags fts5 -ldflags "-X main.version=$(VERSION)" -o grokdocs-fts ./cmd/grokdocs

build-full:
	go build -tags "fts5 onnx" -ldflags "-X main.version=$(VERSION)" -o grokdocs-full ./cmd/grokdocs

build-all: build-fts build-full

clean:
	rm -f grokdocs-fts grokdocs-full
