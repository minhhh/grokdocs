package util

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// LogFormat defines the format of the logger output (JSON or Text).
// It is an opaque struct to prevent callers from passing arbitrary values.
type LogFormat struct {
	id int
}

var (
	FormatText = LogFormat{id: 0}
	FormatJSON = LogFormat{id: 1}
)

// Logger is the global structured logger. It is initialized with a default fallback to os.Stderr.
var Logger *slog.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// InitLogger initializes the global structured logger with the specified writer, level, and format.
func InitLogger(w io.Writer, level string, format LogFormat) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	var handler slog.Handler
	if format == FormatJSON {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}
