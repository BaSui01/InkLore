package logger

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"github.com/google/uuid"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const projectLogRetention = 90 * 24 * time.Hour

// EnableProjectLogs starts a bounded, best-effort worker. Call only after R1
// migrations have been verified, before accepting requests. Disabled on Close.
func EnableProjectLogs(db *gorm.DB) error {
	if db == nil {
		return errors.New("project logs: PostgreSQL connection required")
	}
	projectMu.Lock()
	defer projectMu.Unlock()
	if activeProjectSink != nil {
		return errors.New("project logs: already enabled")
	}
	// This session never emits ORM SQL/error logs. The global application
	// connection retains its normal safe logger; the projection cannot recurse.
	sinkDB := db.Session(&gorm.Session{NewDB: true, Logger: gormlogger.Discard})
	sink := &projectSink{db: sinkDB, queue: make(chan projectLogRow, 1024), stop: make(chan struct{}), done: make(chan struct{})}
	activeProjectSink = sink
	// Init is process bootstrap; replacing only the handler keeps stdout/file live.
	if defaultLogger != nil {
		defaultLogger = slogWithSink(defaultLogger.Handler(), sink)
	}
	go sink.run()
	return nil
}

var projectMu sync.Mutex
var activeProjectSink *projectSink

func (s *projectSink) enqueue(row projectLogRow) {
	if row.TenantID == nil {
		return
	}
	select {
	case s.queue <- row:
	default:
	} // overload must never block business work
}
func (s *projectSink) run() {
	defer close(s.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	s.cleanup()
	for {
		select {
		case row := <-s.queue:
			s.write(row)
		case <-ticker.C:
			s.cleanup()
		case <-s.stop:
			for {
				select {
				case row := <-s.queue:
					s.write(row)
				default:
					return
				}
			}
		}
	}
}
func (s *projectSink) write(row projectLogRow) {
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	var requestID, traceID any
	if row.RequestID != "" {
		requestID = row.RequestID
	}
	if row.TraceID != "" {
		traceID = row.TraceID
	}
	// Only the safe projection fields are persisted. Discarding ORM diagnostics
	// prevents an INSERT failure from emitting another log event.
	_ = s.db.WithContext(ctx).Table("platform_project_logs").Create(map[string]any{
		"id": row.ID, "tenant_id": row.TenantID, "created_at": row.CreatedAt,
		"level": row.Level, "event": row.Event, "domain": row.Domain,
		"operation": row.Operation, "error_code": row.ErrorCode,
		"request_id": requestID, "trace_id": traceID,
		"status": row.Status, "duration_ms": row.DurationMS,
	}).Error
}
func (s *projectSink) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Bounded per pass; 1,000 oldest expired rows each minute. Read-time
	// retention applies even if this worker or the database is unavailable.
	expired := s.db.Table("platform_project_logs").Select("id").
		Where("created_at < ?", time.Now().UTC().Add(-projectLogRetention)).
		Order("created_at").Limit(1000)
	_ = s.db.WithContext(ctx).Table("platform_project_logs").
		Where("id IN (?)", expired).Delete(&projectLogRow{}).Error
}
func closeProjectSink() {
	projectMu.Lock()
	sink := activeProjectSink
	activeProjectSink = nil
	if sink != nil && defaultLogger != nil {
		if sanitized, ok := defaultLogger.Handler().(*SanitizingHandler); ok {
			if projecting, ok := sanitized.inner.(*projectHandler); ok && projecting.sink == sink {
				defaultLogger = slog.New(NewSanitizingHandler(projecting.inner))
				slog.SetDefault(defaultLogger)
			}
		}
	}
	projectMu.Unlock()
	if sink != nil {
		close(sink.stop)
		select {
		case <-sink.done:
		case <-time.After(3 * time.Second):
		}
	}
}

// ProjectLogReader is created from live Authorize within the caller's UoW.
type ProjectLogReader struct {
	TenantID, ActorID, Audience string
	Roles                       []string
}
type ProjectLogQuery struct {
	Limit                            int
	Level, Domain, Operation, Search string
	From, To                         *time.Time
	BeforeAt                         *time.Time
	BeforeID                         string
}
type ProjectLogPage struct {
	Items   []ProjectLog
	HasMore bool
}
type ProjectLogStore struct{ manager *uow.Manager }

func NewProjectLogStore(manager *uow.Manager) *ProjectLogStore {
	return &ProjectLogStore{manager: manager}
}

func (s *ProjectLogStore) List(ctx context.Context, reader ProjectLogReader, q ProjectLogQuery) (ProjectLogPage, error) {
	out := ProjectLogPage{Items: []ProjectLog{}}
	if s == nil || s.manager == nil {
		return out, errors.New("project logs: UoW required")
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return out, err
	}
	if uuid.Validate(reader.TenantID) != nil || uuid.Validate(reader.ActorID) != nil {
		return out, platformerr.Contract(401, "auth.unauthenticated", nil)
	}
	allowed := false
	for _, role := range reader.Roles {
		if role == "operator" || role == "auditor" {
			allowed = true
		}
	}
	if reader.Audience != "admin" || !allowed {
		return out, platformerr.Contract(403, "auth.forbidden", nil)
	}
	if q.Limit < 1 || q.Limit > 100 || len(q.Search) > 800 || len([]rune(q.Search)) > 200 || len(q.Level) > 8 || len(q.Domain) > 80 || len(q.Operation) > 160 ||
		(q.Level != "" && q.Level != "DEBUG" && q.Level != "INFO" && q.Level != "WARN" && q.Level != "ERROR") ||
		(q.BeforeAt == nil) != (q.BeforeID == "") || (q.BeforeID != "" && uuid.Validate(q.BeforeID) != nil) ||
		(q.From != nil && q.To != nil && q.From.After(*q.To)) {
		return out, platformerr.Contract(400, "platform.validation_failed", nil)
	}
	from := time.Now().UTC().Add(-projectLogRetention)
	if q.From != nil && q.From.After(from) {
		from = q.From.UTC()
	}
	to := time.Now().UTC()
	if q.To != nil && q.To.Before(to) {
		to = q.To.UTC()
	}
	if from.After(to) {
		return out, nil
	}
	args := []any{reader.TenantID, from, to}
	where := "tenant_id = ? AND created_at >= ? AND created_at <= ?"
	if q.Level != "" {
		where += " AND level = ?"
		args = append(args, q.Level)
	}
	if q.Domain != "" {
		where += " AND domain = ?"
		args = append(args, q.Domain)
	}
	if q.Operation != "" {
		where += " AND operation = ?"
		args = append(args, q.Operation)
	}
	if q.Search != "" {
		// Search only projected, bounded machine identifiers, never log messages.
		where += " AND (strpos(lower(event), lower(?)) > 0 OR strpos(lower(operation), lower(?)) > 0 OR strpos(lower(error_code), lower(?)) > 0)"
		for i := 0; i < 3; i++ {
			args = append(args, q.Search)
		}
	}
	if q.BeforeAt != nil {
		where += " AND (created_at,id) < (?,?::uuid)"
		args = append(args, q.BeforeAt.UTC(), q.BeforeID)
	}
	var rows []struct {
		ID         string
		CreatedAt  time.Time
		Level      string
		Event      string
		Domain     string
		Operation  string
		ErrorCode  string
		RequestID  sql.NullString
		TraceID    sql.NullString
		Status     sql.NullInt64
		DurationMS sql.NullInt64
	}
	err = db.Table("platform_project_logs").Select("id,created_at,level,event,domain,operation,error_code,request_id,trace_id,status,duration_ms").Where(where, args...).Order("created_at DESC,id DESC").Limit(q.Limit + 1).Scan(&rows).Error
	if err != nil {
		return out, err
	}
	out.HasMore = len(rows) > q.Limit
	if out.HasMore {
		rows = rows[:q.Limit]
	}
	for _, row := range rows {
		item := ProjectLog{ID: row.ID, CreatedAt: row.CreatedAt.UTC(), Level: row.Level, Event: row.Event, Domain: row.Domain, Operation: row.Operation, ErrorCode: row.ErrorCode}
		if row.RequestID.Valid {
			item.RequestID = row.RequestID.String
		}
		if row.TraceID.Valid {
			item.TraceID = row.TraceID.String
		}
		if row.Status.Valid {
			n := int(row.Status.Int64)
			item.Status = &n
		}
		if row.DurationMS.Valid {
			n := row.DurationMS.Int64
			item.DurationMS = &n
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}
