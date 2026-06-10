package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// LogFormat defines the format of the logger output (text or JSON).
type LogFormat int

const (
	FormatText LogFormat = iota
	FormatJSON
)

func init() {
	zerolog.TimeFieldFormat = time.StampMilli
}

// Logger is the global structured logger.
var Logger zerolog.Logger = zerolog.New(&minimalWriter{w: os.Stderr}).Level(zerolog.InfoLevel)

// verboseWriter formats log events with timestamp and level label.
type verboseWriter struct {
	w io.Writer
}

func (vw *verboseWriter) Write(p []byte) (int, error) {
	var evt map[string]any
	if err := json.Unmarshal(p, &evt); err != nil {
		return len(p), nil
	}

	ts, _ := evt["time"].(string)
	lvl, _ := evt["level"].(string)
	msg, _ := evt["message"].(string)

	parts := []string{}
	if ts != "" {
		parts = append(parts, ts)
	}
	if lvl != "" {
		switch lvl {
		case "trace":
			lvl = "TRC"
		case "debug":
			lvl = "DBG"
		case "info":
			lvl = "INF"
		case "warn":
			lvl = "WRN"
		case "error":
			lvl = "ERR"
		case "fatal":
			lvl = "FTL"
		case "panic":
			lvl = "PAN"
		}
		parts = append(parts, lvl)
	}
	parts = append(parts, msg)

	// Append key=val fields
	for k, v := range evt {
		if k == "time" || k == "level" || k == "message" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	fmt.Fprintln(vw.w, strings.Join(parts, " "))
	return len(p), nil
}

// minimalWriter formats log events without timestamp or level label (except
// WARN/ERROR which get a level prefix).
type minimalWriter struct {
	w io.Writer
}

func (mw *minimalWriter) Write(p []byte) (int, error) {
	var evt map[string]any
	if err := json.Unmarshal(p, &evt); err != nil {
		return len(p), nil
	}

	lvl, _ := evt["level"].(string)
	msg, _ := evt["message"].(string)

	prefix := ""
	if lvl == "warn" {
		prefix = "WARN: "
	} else if lvl == "error" || lvl == "fatal" || lvl == "panic" {
		prefix = "ERROR: "
	}

	parts := []string{prefix + msg}

	// Append key=val fields
	for k, v := range evt {
		if k == "time" || k == "level" || k == "message" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	fmt.Fprintln(mw.w, strings.Join(parts, " "))
	return len(p), nil
}

// InitLogger initializes the global logger.
func InitLogger(w io.Writer, verbose bool, format LogFormat) {
	var out io.Writer
	withTimestamp := false
	if format == FormatJSON {
		out = w
		withTimestamp = true
	} else if verbose {
		out = &verboseWriter{w: w}
		withTimestamp = true
	} else {
		out = &minimalWriter{w: w}
	}

	var lvl zerolog.Level
	if verbose {
		lvl = zerolog.TraceLevel
	} else {
		lvl = zerolog.InfoLevel
	}

	l := zerolog.New(out)
	if withTimestamp {
		l = l.With().Timestamp().Logger()
	}
	Logger = l.Level(lvl)
}
