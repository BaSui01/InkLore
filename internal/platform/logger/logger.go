package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	defaultLogger *slog.Logger
	logFile       *os.File
	once          sync.Once
)

type Config struct {
	Level     string // "debug", "info", "warn", "error"
	Format    string // "json", "text"
	AddSource bool
	FilePath  string // optional file path to write logs
}

func init() {
	once.Do(func() {
		Init(Config{
			Level:     os.Getenv("LOG_LEVEL"),
			Format:    os.Getenv("LOG_FORMAT"),
			FilePath:  os.Getenv("LOG_FILE"),
			AddSource: os.Getenv("LOG_SOURCE") == "true",
		})
	})
}

// Init configures the global slog logger with sensitive data masking and structured outputs.
func Init(cfg Config) {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	var writer io.Writer = os.Stdout
	if strings.TrimSpace(cfg.FilePath) != "" {
		filePath := strings.TrimSpace(cfg.FilePath)
		if dir := filepath.Dir(filePath); dir != "." && dir != "/" {
			_ = os.MkdirAll(dir, 0755)
		}
		if file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			logFile = file
			writer = io.MultiWriter(os.Stdout, file)
		}
	}

	var rawHandler slog.Handler
	if strings.ToLower(strings.TrimSpace(cfg.Format)) == "text" {
		rawHandler = slog.NewTextHandler(writer, opts)
	} else {
		rawHandler = slog.NewJSONHandler(writer, opts)
	}

	sanitizedHandler := NewSanitizingHandler(rawHandler)
	defaultLogger = slog.New(sanitizedHandler)
	slog.SetDefault(defaultLogger)
}

func slogWithSink(handler slog.Handler, sink *projectSink) *slog.Logger {
	sanitized, ok := handler.(*SanitizingHandler)
	if !ok {
		return slog.New(handler)
	}
	result := slog.New(NewSanitizingHandler(&projectHandler{inner: sanitized.inner, sink: sink}))
	slog.SetDefault(result)
	return result
}

// Close releases the optional log file opened by Init.
func Close() error {
	closeProjectSink()
	if logFile == nil {
		return nil
	}
	err := logFile.Close()
	logFile = nil
	return err
}

// Get returns the default structured logger.
func Get() *slog.Logger {
	if defaultLogger == nil {
		return slog.Default()
	}
	return defaultLogger
}

// FromContext extracts contextual trace info and returns an enriched logger.
func FromContext(ctx context.Context) *slog.Logger {
	base := Get()
	if ctx == nil {
		return base
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		base = base.With("trace_id", traceID)
	} else if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		base = base.With("trace_id", reqID)
	}

	return base
}

func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).InfoContext(ctx, msg, args...)
}

func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WarnContext(ctx, msg, args...)
}

func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, args...)
}

func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).DebugContext(ctx, msg, args...)
}

func With(args ...any) *slog.Logger {
	return Get().With(args...)
}
