package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- parseLevel ---

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"DEBUG", slog.LevelInfo}, // case sensitive, defaults to info
	}

	for _, tt := range tests {
		t.Run("level_"+tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// --- ConsoleHandler ---

func TestConsoleHandler_Enabled(t *testing.T) {
	handler := NewConsoleHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be disabled when level is info")
	}
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be enabled when level is info")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("error should be enabled when level is info")
	}
}

func TestConsoleHandler_Enabled_DefaultLevel(t *testing.T) {
	handler := NewConsoleHandler(os.Stdout, nil)

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be disabled with default level")
	}
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be enabled with default level")
	}
}

func TestConsoleHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	handler := NewConsoleHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	now := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "14:30:45") {
		t.Errorf("output missing time, got: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("output missing level, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("output missing message, got: %s", output)
	}
}

func TestConsoleHandler_Handle_AllLevels(t *testing.T) {
	levels := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
	}

	for _, tt := range levels {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewConsoleHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

			now := time.Now()
			record := slog.NewRecord(now, tt.level, "msg", 0)
			handler.Handle(context.Background(), record)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected %q in output, got: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestConsoleHandler_WithAttrs(t *testing.T) {
	handler := NewConsoleHandler(os.Stdout, nil)
	newHandler := handler.WithAttrs([]slog.Attr{slog.String("key", "val")})

	if newHandler == nil {
		t.Fatal("WithAttrs returned nil")
	}
	if _, ok := newHandler.(*ConsoleHandler); !ok {
		t.Error("WithAttrs should return *ConsoleHandler")
	}
}

func TestConsoleHandler_WithGroup(t *testing.T) {
	handler := NewConsoleHandler(os.Stdout, nil)
	newHandler := handler.WithGroup("group")

	if newHandler != handler {
		t.Error("WithGroup should return the same handler")
	}
}

// --- TextFileHandler ---

func TestTextFileHandler_Enabled(t *testing.T) {
	handler := NewTextFileHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	})

	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be disabled when level is error")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("error should be enabled when level is error")
	}
}

func TestTextFileHandler_Enabled_DefaultLevel(t *testing.T) {
	handler := NewTextFileHandler(os.Stdout, nil)

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be disabled with default level")
	}
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be enabled with default level")
	}
}

func TestTextFileHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextFileHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	now := time.Date(2025, 6, 15, 14, 30, 45, 123000000, time.UTC)
	record := slog.NewRecord(now, slog.LevelInfo, "file log message", 0)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "2025-06-15T14:30:45.123") {
		t.Errorf("output missing datetime, got: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("output missing level, got: %s", output)
	}
	if !strings.Contains(output, "file log message") {
		t.Errorf("output missing message, got: %s", output)
	}
}

func TestTextFileHandler_Handle_AllLevels(t *testing.T) {
	levels := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
	}

	for _, tt := range levels {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewTextFileHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

			record := slog.NewRecord(time.Now(), tt.level, "msg", 0)
			handler.Handle(context.Background(), record)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected %q in output, got: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestTextFileHandler_WithAttrs(t *testing.T) {
	handler := NewTextFileHandler(os.Stdout, nil)
	newHandler := handler.WithAttrs([]slog.Attr{slog.String("key", "val")})

	if newHandler == nil {
		t.Fatal("WithAttrs returned nil")
	}
	if _, ok := newHandler.(*TextFileHandler); !ok {
		t.Error("WithAttrs should return *TextFileHandler")
	}
}

func TestTextFileHandler_WithGroup(t *testing.T) {
	handler := NewTextFileHandler(os.Stdout, nil)
	newHandler := handler.WithGroup("group")

	if newHandler != handler {
		t.Error("WithGroup should return the same handler")
	}
}

// --- New ---

func TestNew_Console(t *testing.T) {
	log, err := New("", "info", "text", true)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer log.Close()

	if log.slogger == nil {
		t.Error("slogger should not be nil")
	}
	if log.file != nil {
		t.Error("file should be nil for console logger")
	}
}

func TestNew_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")

	log, err := New(path, "debug", "text", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer log.Close()

	if log.file == nil {
		t.Error("file should not be nil for file logger")
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestNew_FileJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.json.log")

	log, err := New(path, "info", "json", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer log.Close()

	if log.file == nil {
		t.Error("file should not be nil")
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/nonexistent/dir/test.log", "info", "text", false)
	if err == nil {
		t.Fatal("New() expected error for invalid file path")
	}
}

func TestNew_EmptyFormat(t *testing.T) {
	log, err := New("", "", "", true)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer log.Close()
	// Should default to text format without error
}

// --- Logger methods ---

func TestLogger_InfoWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")

	log, err := New(path, "info", "text", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	log.Info("hello world")
	log.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("log file should contain message, got: %s", string(data))
	}
	if !strings.Contains(string(data), "INFO") {
		t.Errorf("log file should contain level, got: %s", string(data))
	}
}

func TestLogger_DebugNotWrittenAtInfoLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")

	log, err := New(path, "info", "text", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	log.Debug("should not appear")
	log.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if strings.Contains(string(data), "should not appear") {
		t.Error("debug message should not appear at info level")
	}
}

func TestLogger_ErrorWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")

	log, err := New(path, "info", "text", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	log.Error("something failed")
	log.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if !strings.Contains(string(data), "something failed") {
		t.Errorf("log file should contain error message, got: %s", string(data))
	}
	if !strings.Contains(string(data), "ERROR") {
		t.Errorf("log file should contain ERROR level, got: %s", string(data))
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")

	log, err := New(path, "info", "json", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	log.Info("json test")
	log.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"msg"`) && !strings.Contains(content, `"message"`) {
		t.Errorf("JSON log should contain msg field, got: %s", content)
	}
	if !strings.Contains(content, "json test") {
		t.Errorf("JSON log should contain message, got: %s", content)
	}
}

// --- Close ---

func TestLogger_Close_NilFile(t *testing.T) {
	log, err := New("", "info", "text", true)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := log.Close(); err != nil {
		t.Errorf("Close() on console logger should not error, got: %v", err)
	}
}

func TestLogger_Close_WithFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")

	log, err := New(path, "info", "text", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := log.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
