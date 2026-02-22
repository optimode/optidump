package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger to provide a simplified interface
type Logger struct {
	slogger *slog.Logger
	file    *os.File
}

// ConsoleHandler provides a clean, human-readable console output format
type ConsoleHandler struct {
	opts  slog.HandlerOptions
	out   io.Writer
	attrs []slog.Attr
}

// NewConsoleHandler creates a new console handler with clean formatting
func NewConsoleHandler(out io.Writer, opts *slog.HandlerOptions) *ConsoleHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &ConsoleHandler{
		opts: *opts,
		out:  out,
	}
}

// Enabled reports whether the handler handles records at the given level
func (h *ConsoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle formats and writes a log record to the console
func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	// Format: [HH:MM:SS] LEVEL: message
	timeStr := r.Time.Format("15:04:05")
	level := r.Level.String()

	// Color codes for different log levels (optional)
	var levelStr string
	switch r.Level {
	case slog.LevelDebug:
		levelStr = "DEBUG"
	case slog.LevelInfo:
		levelStr = "INFO "
	case slog.LevelWarn:
		levelStr = "WARN "
	case slog.LevelError:
		levelStr = "ERROR"
	default:
		levelStr = level
	}

	fmt.Fprintf(h.out, "[%s] %s: %s\n", timeStr, levelStr, r.Message)
	return nil
}

// WithAttrs returns a new handler with additional attributes
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ConsoleHandler{
		opts:  h.opts,
		out:   h.out,
		attrs: append(h.attrs, attrs...),
	}
}

// WithGroup returns a new handler with a group
func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	return h
}

// TextFileHandler provides clean text format for log files
type TextFileHandler struct {
	opts  slog.HandlerOptions
	out   io.Writer
	attrs []slog.Attr
}

// NewTextFileHandler creates a new text file handler with clean formatting
func NewTextFileHandler(out io.Writer, opts *slog.HandlerOptions) *TextFileHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &TextFileHandler{
		opts: *opts,
		out:  out,
	}
}

// Enabled reports whether the handler handles records at the given level
func (h *TextFileHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle formats and writes a log record to the file
func (h *TextFileHandler) Handle(_ context.Context, r slog.Record) error {
	// Format: YYYY-MM-DDTHH:MM:SS.mmm+TZ LEVEL message
	timeStr := r.Time.Format("2006-01-02T15:04:05.000-07:00")

	var levelStr string
	switch r.Level {
	case slog.LevelDebug:
		levelStr = "DEBUG"
	case slog.LevelInfo:
		levelStr = "INFO"
	case slog.LevelWarn:
		levelStr = "WARN"
	case slog.LevelError:
		levelStr = "ERROR"
	default:
		levelStr = r.Level.String()
	}

	fmt.Fprintf(h.out, "%s %s %s\n", timeStr, levelStr, r.Message)
	return nil
}

// WithAttrs returns a new handler with additional attributes
func (h *TextFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TextFileHandler{
		opts:  h.opts,
		out:   h.out,
		attrs: append(h.attrs, attrs...),
	}
}

// WithGroup returns a new handler with a group
func (h *TextFileHandler) WithGroup(name string) slog.Handler {
	return h
}

// New creates a new logger instance using slog
func New(filepath, level, format string, useConsole bool) (*Logger, error) {
	// Set default format to "text" if not specified
	if format == "" {
		format = "text"
	}

	var writer io.Writer
	var file *os.File
	var err error

	if useConsole {
		// Log to stdout for interactive terminal use
		writer = os.Stdout
	} else {
		// Open or create log file
		file, err = os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("cannot create log file: %w", err)
		}
		writer = file
	}

	// Parse log level
	logLevel := parseLevel(level)

	// Create handler based on format and output destination
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	if useConsole {
		// Use custom clean console format for interactive terminal
		handler = NewConsoleHandler(writer, opts)
	} else if format == "json" {
		// Use JSON format for log files if specified
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		// Use custom text format for log files (DATETIME LEVEL MSG)
		handler = NewTextFileHandler(writer, opts)
	}

	// Create slog logger
	slogger := slog.New(handler)

	return &Logger{
		slogger: slogger,
		file:    file,
	}, nil
}

// parseLevel converts a string to slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	l.slogger.Debug(message)
}

// Info logs an informational message
func (l *Logger) Info(message string) {
	l.slogger.Info(message)
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.slogger.Error(message)
}

// Close closes the logger's file handle
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
