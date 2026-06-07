package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, "info", FormatText)

	Logger.Info("hello test message", "key", "val")

	output := buf.String()
	if !strings.Contains(output, "level=INFO") {
		t.Errorf("expected output to contain level=INFO, got %q", output)
	}
	if !strings.Contains(output, "msg=\"hello test message\"") {
		t.Errorf("expected output to contain msg, got %q", output)
	}
	if !strings.Contains(output, "key=val") {
		t.Errorf("expected output to contain key=val, got %q", output)
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, "debug", FormatJSON)

	Logger.Debug("debug json message", "number", 42)

	output := buf.String()
	if !strings.Contains(output, `"level":"DEBUG"`) {
		t.Errorf("expected JSON level DEBUG, got %q", output)
	}
	if !strings.Contains(output, `"msg":"debug json message"`) {
		t.Errorf("expected JSON msg, got %q", output)
	}
	if !strings.Contains(output, `"number":42`) {
		t.Errorf("expected JSON attribute number:42, got %q", output)
	}
}

func TestLoggerLevelsFiltering(t *testing.T) {
	var buf bytes.Buffer
	InitLogger(&buf, "warn", FormatText)

	Logger.Debug("this should not show up")
	Logger.Info("neither should this")
	Logger.Warn("this warn message should show up")

	output := buf.String()
	if strings.Contains(output, "this should not show up") {
		t.Error("expected debug message to be filtered out")
	}
	if strings.Contains(output, "neither should this") {
		t.Error("expected info message to be filtered out")
	}
	if !strings.Contains(output, "this warn message should show up") {
		t.Error("expected warn message to be recorded")
	}
}
