package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRequestIDInjected(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{
		Format:    "json",
		Level:     slog.LevelDebug,
		Writer:    &buf,
		RequestID: func(context.Context) string { return "abc123" },
	})

	l.Info("hello", "key", "value")
	got := buf.String()
	if !strings.Contains(got, `"rid":"abc123"`) {
		t.Fatalf("request_id not injected: %s", got)
	}
	if !strings.Contains(got, `"key":"value"`) {
		t.Fatalf("original attrs lost: %s", got)
	}
}

func TestRequestIDNotInjectedWithoutContext(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{
		Format:    "json",
		Level:     slog.LevelDebug,
		Writer:    &buf,
		RequestID: func(context.Context) string { return "" },
	})

	l.Info("hello")
	if strings.Contains(buf.String(), "rid") {
		t.Fatalf("unexpected rid: %s", buf.String())
	}
}

func TestRequestIDSurvivesWithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{
		Format:    "json",
		Level:     slog.LevelDebug,
		Writer:    &buf,
		RequestID: func(context.Context) string { return "abc123" },
	})

	l.With("component", "test").WithGroup("nested").Info("hello")
	if !strings.Contains(buf.String(), `"rid":"abc123"`) {
		t.Fatalf("request_id lost after With/WithGroup: %s", buf.String())
	}
}

func TestFormats(t *testing.T) {
	for _, format := range []string{"text", "plain", "json", ""} {
		var buf bytes.Buffer
		l := New(Options{Format: format, Level: slog.LevelInfo, Writer: &buf})
		l.Info("hello")
		if buf.Len() == 0 {
			t.Errorf("format %q produced no output", format)
		}
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"info":  slog.LevelInfo,
		"":      slog.LevelInfo,
		"junk":  slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
