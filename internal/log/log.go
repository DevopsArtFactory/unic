package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxLogSize   = 10 * 1024 * 1024 // 10 MB
	maxOldFiles  = 3
	logFileName  = "unic.log"
	logDirName   = "logs"
	configDirEnv = "XDG_CONFIG_HOME"
	appName      = "unic"
)

var logFile *os.File

// Init sets up structured logging to ~/.config/unic/logs/unic.log.
// If verbose is true, debug-level text output is also written to stderr.
func Init(verbose bool) error {
	logDir, err := logDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, logFileName)
	if err := rotate(logPath); err != nil {
		return fmt.Errorf("log rotation failed: %w", err)
	}

	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: level})

	var handler slog.Handler
	if verbose {
		stderrHandler := newPrettyHandler(os.Stderr, slog.LevelDebug)
		handler = &multiHandler{handlers: []slog.Handler{fileHandler, stderrHandler}}
	} else {
		handler = fileHandler
	}

	slog.SetDefault(slog.New(handler))
	return nil
}

// Close flushes and closes the log file.
func Close() {
	if logFile != nil {
		_ = logFile.Sync()
		_ = logFile.Close()
		logFile = nil
	}
}

// Debug logs a debug-level message with a component tag.
func Debug(component, msg string, attrs ...any) {
	slog.Debug(msg, prepend(component, attrs)...)
}

// Info logs an info-level message with a component tag.
func Info(component, msg string, attrs ...any) {
	slog.Info(msg, prepend(component, attrs)...)
}

// Error logs an error-level message with a component tag.
func Error(component, msg string, attrs ...any) {
	slog.Error(msg, prepend(component, attrs)...)
}

func prepend(component string, attrs []any) []any {
	return append([]any{slog.String("component", component)}, attrs...)
}

func logDir() (string, error) {
	dir := os.Getenv(configDirEnv)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, appName, logDirName), nil
}

// rotate checks the current log file size and rotates if it exceeds maxLogSize.
func rotate(logPath string) error {
	info, err := os.Stat(logPath)
	if err != nil {
		return nil // File doesn't exist yet
	}
	if info.Size() < maxLogSize {
		return nil
	}

	// Shift old files: .3 is deleted, .2→.3, .1→.2, current→.1
	for i := maxOldFiles; i >= 1; i-- {
		src := logPath
		if i > 1 {
			src = fmt.Sprintf("%s.%d", logPath, i-1)
		}
		dst := fmt.Sprintf("%s.%d", logPath, i)

		if i == maxOldFiles {
			_ = os.Remove(dst)
		}
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("failed to rotate %s → %s: %w", src, dst, err)
			}
		}
	}
	return nil
}

// multiHandler fans out log records to multiple slog.Handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(nil, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// ANSI color codes for pretty stderr output.
const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
)

// prettyHandler writes human-friendly, aligned log lines to a writer.
//
//	HH:MM:SS  LEVEL  [component]  message
//	                               ├ key = value
//	                               └ key = value
type prettyHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
}

func newPrettyHandler(w io.Writer, level slog.Level) *prettyHandler {
	return &prettyHandler{w: w, level: level}
}

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// indent is the fixed prefix for attribute lines, matching the width of
// "HH:MM:SS  LEVEL  " (8 + 2 + 5 + 2 = 17 visible chars).
const indent = "                   "

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder

	// Timestamp — short form
	ts := r.Time.Format(time.TimeOnly) // "15:04:05"
	sb.WriteString(colorDim)
	sb.WriteString(ts)
	sb.WriteString(colorReset)
	sb.WriteString("  ")

	// Level — fixed 5-char width, colored
	lvl := r.Level.String()
	switch {
	case r.Level >= slog.LevelError:
		sb.WriteString(colorRed)
	case r.Level >= slog.LevelWarn:
		sb.WriteString(colorYellow)
	case r.Level >= slog.LevelInfo:
		sb.WriteString(colorGreen)
	default:
		sb.WriteString(colorCyan)
	}
	fmt.Fprintf(&sb, "%-5s", lvl)
	sb.WriteString(colorReset)
	sb.WriteString("  ")

	// Extract component from pre-attached attrs and record attrs
	var component string
	type kv struct{ k, v string }
	var pairs []kv

	for _, a := range h.attrs {
		if a.Key == "component" {
			component = a.Value.String()
		} else {
			pairs = append(pairs, kv{a.Key, a.Value.String()})
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "component" {
			component = a.Value.String()
		} else {
			pairs = append(pairs, kv{a.Key, a.Value.String()})
		}
		return true
	})

	// Component tag
	if component != "" {
		sb.WriteString(colorDim)
		sb.WriteString("[")
		sb.WriteString(component)
		sb.WriteString("]")
		sb.WriteString(colorReset)
		sb.WriteString("  ")
	}

	// Message
	sb.WriteString(r.Message)
	sb.WriteString("\r\n")

	// Key-value pairs — each on its own indented line with tree drawing chars
	for i, p := range pairs {
		sb.WriteString(indent)
		sb.WriteString(colorDim)
		if i < len(pairs)-1 {
			sb.WriteString("├ ")
		} else {
			sb.WriteString("└ ")
		}
		sb.WriteString(p.k)
		sb.WriteString(colorReset)
		sb.WriteString(" = ")
		sb.WriteString(p.v)
		sb.WriteString("\r\n")
	}

	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prettyHandler{
		w:     h.w,
		level: h.level,
		attrs: append(h.attrs, attrs...),
		group: h.group,
	}
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	return &prettyHandler{
		w:     h.w,
		level: h.level,
		attrs: h.attrs,
		group: name,
	}
}
