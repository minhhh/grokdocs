//go:build onnx

package main

import "github.com/minhhh/grokdocs/internal/embed"

func initEmbedder() {
	embed.GetGlobalEmbedder()
}

func closeEmbedder() {
	embed.Close()
}
