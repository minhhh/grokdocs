//go:build !onnx

package main

func initEmbedder() error { return nil }
func closeEmbedder() {}
