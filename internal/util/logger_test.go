package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerTextDefault(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, false, FormatText)

	Logger.Info().Str("key", "val").Msg("hello test message")

	output := buf.String()
	if !strings.Contains(output, "hello test message") {
		t.Errorf("expected output to contain message, got %q", output)
	}
}

func TestLoggerTextVerbose(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, true, FormatText)

	Logger.Info().Str("key", "val").Msg("verbose message")

	output := buf.String()
	if !strings.Contains(output, "INF") {
		t.Errorf("expected verbose output to contain level, got %q", output)
	}
	if !strings.Contains(output, "verbose message") {
		t.Errorf("expected output to contain message, got %q", output)
	}
	if !strings.Contains(output, "key=val") {
		t.Errorf("expected output to contain key=val, got %q", output)
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, true, FormatJSON)

	Logger.Debug().Int("number", 42).Msg("debug json message")

	output := buf.String()
	if !strings.Contains(output, `"level":"debug"`) {
		t.Errorf("expected JSON level debug, got %q", output)
	}
	if !strings.Contains(output, `"message":"debug json message"`) {
		t.Errorf("expected JSON message, got %q", output)
	}
	if !strings.Contains(output, `"number":42`) {
		t.Errorf("expected JSON attribute number:42, got %q", output)
	}
}

func TestLoggerNonVerboseFiltersInfo(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, false, FormatText)

	Logger.Debug().Msg("this should not show up")
	Logger.Trace().Msg("neither should this")
	Logger.Warn().Msg("this warn should show up")

	output := buf.String()
	if strings.Contains(output, "this should not show up") {
		t.Error("expected debug message to be filtered out in non-verbose mode")
	}
	if strings.Contains(output, "neither should this") {
		t.Error("expected trace message to be filtered out in non-verbose mode")
	}
	if !strings.Contains(output, "this warn should show up") {
		t.Error("expected warn message to be recorded")
	}
}

func TestLoggerVerboseShowsTrace(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, true, FormatText)

	Logger.Trace().Msg("trace message only in verbose")

	output := buf.String()
	if !strings.Contains(output, "trace message only in verbose") {
		t.Errorf("expected trace message to appear in verbose mode, got %q", output)
	}
}

func TestLoggerVerboseLevelLabels(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, true, FormatText)

	Logger.Error().Int("code", 500).Msg("error message")
	Logger.Warn().Msg("warn message")

	output := buf.String()
	if !strings.Contains(output, "ERR") {
		t.Errorf("expected ERR level label, got %q", output)
	}
	if !strings.Contains(output, "WRN") {
		t.Errorf("expected WRN level label, got %q", output)
	}
	if !strings.Contains(output, "code=500") {
		t.Errorf("expected code=500 field, got %q", output)
	}
}

func TestLoggerMinimalErrorPrefix(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, false, FormatText)

	Logger.Error().Msg("this is an error")

	output := buf.String()
	if !strings.Contains(output, "ERROR: this is an error") {
		t.Errorf("expected ERROR prefix, got %q", output)
	}
}

func TestLoggerMinimalWarnPrefix(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, false, FormatText)

	Logger.Warn().Msg("this is a warning")

	output := buf.String()
	if !strings.Contains(output, "WARN: this is a warning") {
		t.Errorf("expected WARN prefix, got %q", output)
	}
}


