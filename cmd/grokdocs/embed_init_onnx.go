//go:build onnx

package main

import "github.com/minhhh/grokdocs/internal/embed"

func initEmbedder() error {
	_, err := embed.GetGlobalEmbedder()
	return err
}

func closeEmbedder() {
	embed.Close()
}
