package logger

import (
	"context"
	"log/slog"
)

// SanitizingHandler wraps any slog.Handler and automatically scrubs sensitive keys/tokens.
type SanitizingHandler struct {
	inner slog.Handler
}

func NewSanitizingHandler(inner slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{inner: inner}
}

func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	sanitizedRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	// Automatically extract TraceID from Context if record doesn't already have one
	hasTraceID := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "trace_id" || a.Key == "request_id" {
			hasTraceID = true
		}
		return true
	})

	if !hasTraceID && ctx != nil {
		if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
			sanitizedRecord.AddAttrs(slog.String("trace_id", traceID))
		}
	}

	// Sanitize each attribute
	r.Attrs(func(a slog.Attr) bool {
		sanitizedRecord.AddAttrs(h.sanitizeAttr(a))
		return true
	})

	return h.inner.Handle(ctx, sanitizedRecord)
}

func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitizedAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitizedAttrs[i] = h.sanitizeAttr(a)
	}
	return &SanitizingHandler{inner: h.inner.WithAttrs(sanitizedAttrs)}
}

func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{inner: h.inner.WithGroup(name)}
}

func (h *SanitizingHandler) sanitizeAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		sanitized := make([]slog.Attr, len(attrs))
		for i, child := range attrs {
			sanitized[i] = h.sanitizeAttr(child)
		}
		return slog.Attr{
			Key:   a.Key,
			Value: slog.GroupValue(sanitized...),
		}
	}

	sanitizedVal := SanitizeValue(a.Key, a.Value.Any())
	return slog.Any(a.Key, sanitizedVal)
}
