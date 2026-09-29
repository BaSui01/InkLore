package audit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/timeutil"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

type Store struct{ manager *uow.Manager }

func New(manager *uow.Manager) *Store { return &Store{manager: manager} }

// eventRow is private: jsonb is never directly serialized to an API response.
type eventRow struct {
	ID          string
	TenantID    string
	ActorID     *string
	Audience    string
	Action      string
	ResourceID  *string
	Outcome     string
	RequestID   string
	TraceID     string
	SafeChanges []byte
	CreatedAt   time.Time
}

const eventColumns = "id, tenant_id, actor_id, audience, action, resource_id, outcome, request_id, trace_id, safe_changes, created_at"

// Append requires the caller-owned transaction. Return any error from its UOW
// callback so resource/session changes cannot commit without their audit event.
// No independently committed transaction, idempotency cache, or logging occurs.
func (s *Store) Append(ctx context.Context, record Record) (Audit, error) {
	if s == nil || s.manager == nil {
		return Audit{}, errors.New("audit: UOW manager required")
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return Audit{}, err
	}
	if !validateRecord(record) {
		return Audit{}, invalid()
	}
	changes := record.SafeChanges
	if changes == nil {
		changes = []SafeChange{}
	}
	payload, err := json.Marshal(changes)
	if err != nil {
		return Audit{}, invalid()
	}
	var row eventRow
	result := db.Raw(`INSERT INTO platform_audit_events
 (id, tenant_id, actor_id, audience, action, resource_id, outcome, request_id, trace_id, safe_changes, created_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, clock_timestamp())
 RETURNING `+eventColumns, uuid.NewString(), record.TenantID, record.ActorID, record.Audience, record.Action,
		record.ResourceID, record.Outcome, record.RequestID, record.TraceID, string(payload)).Scan(&row)
	if result.Error != nil {
		return Audit{}, result.Error
	}
	if result.RowsAffected != 1 {
		return Audit{}, errors.New("audit: insert did not return one event")
	}
	return row.safeAudit()
}

// List consumes a live authorization result and its existing UOW. It never
// derives permissions from JWT roles or accepts a separate target tenant.
// from/to are inclusive instants; pagination is strict (created_at,id) descending.
func (s *Store) List(ctx context.Context, reader VerifiedReader, query ListQuery) (Page, error) {
	out := Page{Items: []Audit{}}
	if s == nil || s.manager == nil {
		return out, errors.New("audit: UOW manager required")
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return out, err
	}
	if err = authorizeReader(reader); err != nil {
		return out, err
	}
	if err = validateQuery(query); err != nil {
		return out, err
	}
	limit := query.Limit
	if limit == 0 {
		limit = DefaultLimit
	}
	from, to := retentionWindow(timeutil.Now())
	if query.From != nil && query.From.After(from) {
		from = query.From.UTC()
	}
	if query.To != nil && query.To.Before(to) {
		to = query.To.UTC()
	}
	if from.After(to) {
		return out, nil
	}
	selection := db.Table("platform_audit_events").Select(eventColumns).
		Where("tenant_id = ? AND created_at >= ? AND created_at <= ?", reader.TenantID, from, to)
	if query.Action != "" {
		selection = selection.Where("action = ?", query.Action)
	}
	if query.BeforeCreatedAt != nil {
		selection = selection.Where("(created_at, id) < (?, ?::uuid)", query.BeforeCreatedAt.UTC(), query.BeforeID)
	}
	var rows []eventRow
	if err = selection.Order("created_at DESC, id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return out, err
	}
	out.HasMore = len(rows) > limit
	if out.HasMore {
		rows = rows[:limit]
	}
	for _, row := range rows {
		item, mapErr := row.safeAudit()
		if mapErr != nil {
			return Page{}, mapErr
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// Get is internal only; there is no frozen public audit detail endpoint.
func (s *Store) Get(ctx context.Context, reader VerifiedReader, id string) (Audit, error) {
	if s == nil || s.manager == nil {
		return Audit{}, errors.New("audit: UOW manager required")
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return Audit{}, err
	}
	if err = authorizeReader(reader); err != nil {
		return Audit{}, err
	}
	if !validID(id) {
		return Audit{}, invalid()
	}
	from, to := retentionWindow(timeutil.Now())
	var row eventRow
	result := db.Table("platform_audit_events").Select(eventColumns).
		Where("tenant_id = ? AND id = ? AND created_at >= ? AND created_at <= ?", reader.TenantID, id, from, to).Scan(&row)
	if result.Error != nil {
		return Audit{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Audit{}, apperrors.Contract(404, "platform.resource_not_found", nil)
	}
	return row.safeAudit()
}

func (row eventRow) safeAudit() (Audit, error) {
	changes, err := decodeChanges(row.SafeChanges)
	if err != nil {
		return Audit{}, err
	}
	if !validID(row.ID) || row.CreatedAt.IsZero() || !validateRecord(Record{
		TenantID: row.TenantID, ActorID: row.ActorID, Audience: row.Audience, Action: row.Action,
		ResourceID: row.ResourceID, Outcome: row.Outcome, RequestID: row.RequestID, TraceID: row.TraceID, SafeChanges: changes,
	}) {
		return Audit{}, invalidStored()
	}
	return Audit{ID: row.ID, ActorID: row.ActorID, Audience: row.Audience, Action: row.Action, ResourceID: row.ResourceID,
		Outcome: row.Outcome, RequestID: row.RequestID, TraceID: row.TraceID, SafeChanges: changes, CreatedAt: row.CreatedAt.UTC()}, nil
}
