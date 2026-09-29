package logger

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantContext attaches a tenant selected by a live, verified identity. Never
// call this with an unverified request parameter or an untrusted worker payload.
type verifiedTenantKey struct{}

func WithVerifiedTenant(ctx context.Context, tenantID string) context.Context {
	if ctx == nil || uuid.Validate(tenantID) != nil {
		return ctx
	}
	return context.WithValue(ctx, verifiedTenantKey{}, tenantID)
}

var errorCodeToken = regexp.MustCompile(`^(?:none|[a-z][a-z0-9_]{0,39}\.[a-z][a-z0-9_]{0,39})$`)
var routeToken = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS) /(v1/[a-z0-9-]+(?:/(?:[a-z0-9-]+|:[a-z_]+))*)$`)
var traceToken = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

type ProjectLog struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Level      string    `json:"level"`
	Event      string    `json:"event"`
	Domain     string    `json:"domain"`
	Operation  string    `json:"operation"`
	ErrorCode  string    `json:"error_code"`
	RequestID  string    `json:"request_id"`
	TraceID    string    `json:"trace_id"`
	Status     *int      `json:"status"`
	DurationMS *int64    `json:"duration_ms"`
}

type projectLogRow struct {
	ProjectLog
	TenantID *string `json:"-"`
}

func projectRecord(ctx context.Context, r slog.Record, bound []slog.Attr) projectLogRow {
	// Persist only the fixed HTTP completion event. Free-form slog messages and
	// their attributes may contain user content even when their shape looks like
	// a machine token; stdout/file remain the diagnostic channel for those.
	if r.Message != "http_request" {
		return projectLogRow{}
	}
	row := projectLogRow{ProjectLog: ProjectLog{ID: uuid.NewString(), CreatedAt: r.Time.UTC(), Event: "http_request"}}
	if r.Time.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	switch {
	case r.Level >= slog.LevelError:
		row.Level = "ERROR"
	case r.Level >= slog.LevelWarn:
		row.Level = "WARN"
	case r.Level >= slog.LevelInfo:
		row.Level = "INFO"
	default:
		row.Level = "DEBUG"
	}
	// The event name is fixed above; never copy a free-form log message.
	if ctx != nil {
		if tenant, ok := ctx.Value(verifiedTenantKey{}).(string); ok && uuid.Validate(tenant) == nil {
			row.TenantID = &tenant
		}
	}
	// Unscoped startup, login and background logs stay on stdout/file only.
	// Never materialize an unverified or missing tenant in the shared database.
	if row.TenantID == nil {
		return projectLogRow{}
	}
	set := func(a slog.Attr) {
		if a.Value.Kind() != slog.KindString && a.Value.Kind() != slog.KindInt64 && a.Value.Kind() != slog.KindUint64 {
			return
		}
		value := a.Value.Resolve()
		switch a.Key {
		case "domain":
			if value.Kind() == slog.KindString {
				switch value.String() {
				case "platform", "auth", "content", "outline", "worldview", "inspiration", "compliance", "agents", "llm":
					row.Domain = value.String()
				}
			}
		case "operation":
			if value.Kind() == slog.KindString && routeToken.MatchString(value.String()) {
				row.Operation = value.String()
			}
		case "error_code":
			if value.Kind() == slog.KindString && errorCodeToken.MatchString(value.String()) {
				row.ErrorCode = value.String()
			}
		case "request_id":
			if value.Kind() == slog.KindString && uuid.Validate(value.String()) == nil {
				row.RequestID = value.String()
			}
		case "trace_id":
			if value.Kind() == slog.KindString && traceToken.MatchString(value.String()) {
				row.TraceID = strings.ToLower(value.String())
			}
		case "status":
			if value.Kind() == slog.KindInt64 {
				n := value.Int64()
				if n >= 100 && n <= 599 {
					x := int(n)
					row.Status = &x
				}
			}
		case "duration_ms":
			if value.Kind() == slog.KindInt64 {
				n := value.Int64()
				if n >= 0 && n <= int64((24*time.Hour)/time.Millisecond) {
					row.DurationMS = &n
				}
			}
		}
	}
	for _, a := range bound {
		set(a)
	}
	r.Attrs(func(a slog.Attr) bool { set(a); return true })
	if row.Status != nil {
		switch {
		case *row.Status >= 500:
			row.Level = "ERROR"
		case *row.Status >= 400:
			row.Level = "WARN"
		}
	}
	return row
}

// projectHandler receives only the output of SanitizingHandler, never raw attrs.
// Group members are not projected: flattening nested keys would make innocuous
// names inside credentials or payload groups appear safe.
type projectHandler struct {
	inner   slog.Handler
	sink    *projectSink
	attrs   []slog.Attr
	grouped bool
}

func (h *projectHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}
func (h *projectHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.inner.Handle(ctx, r)
	if h.sink != nil && r.Message == "http_request" {
		if h.grouped {
			clean := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
			h.sink.enqueue(projectRecord(ctx, clean, h.attrs))
		} else {
			h.sink.enqueue(projectRecord(ctx, r, h.attrs))
		}
	}
	return err
}
func (h *projectHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &projectHandler{inner: h.inner.WithAttrs(attrs), sink: h.sink, grouped: h.grouped, attrs: append([]slog.Attr(nil), h.attrs...)}
	if !h.grouped {
		next.attrs = append(next.attrs, attrs...)
	}
	return next
}
func (h *projectHandler) WithGroup(name string) slog.Handler {
	return &projectHandler{inner: h.inner.WithGroup(name), sink: h.sink, attrs: append([]slog.Attr(nil), h.attrs...), grouped: true}
}

// The sink uses a dedicated silent GORM session. SQL failures on that
// session must not produce a second projected log event.
type projectSink struct {
	db    *gorm.DB
	queue chan projectLogRow
	stop  chan struct{}
	done  chan struct{}
}
