package audit

import (
	"context"
	"errors"
	"time"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/timeutil"
)

const MaxExportItems = 10000
const MaxExportDays = 31

// ExportGETQuery contains only the frozen export query parameters.
type ExportGETQuery struct {
	From   string `form:"from"`
	To     string `form:"to"`
	Action string `form:"action"`
}

type ExportQuery struct {
	From   *time.Time
	To     *time.Time
	Action string
}

func ParseExportGETQuery(q ExportGETQuery) (ExportQuery, error) {
	parsed, err := ParseGETQuery(GETQuery{From: q.From, To: q.To, Action: q.Action})
	if err != nil {
		return ExportQuery{}, err
	}
	return ExportQuery{From: parsed.From, To: parsed.To, Action: parsed.Action}, nil
}

func exportLimitExceeded() error {
	return apperrors.Contract(422, "platform.audit_export_limit_exceeded", nil)
}

// exportWindow checks the requested window before retention clipping, so a wide
// request cannot bypass the 31-day limit by selecting expired or future dates.
// The optional to defaults to now, and from defaults to 31 Beijing days before to.
// Explicit from with omitted to still uses now, and is rejected if wider than 31 days.
func exportWindow(q ExportQuery, now time.Time) (time.Time, time.Time, error) {
	if err := validateQuery(ListQuery{From: q.From, To: q.To, Action: q.Action}); err != nil {
		return time.Time{}, time.Time{}, err
	}
	to := now.UTC()
	if q.To != nil {
		to = q.To.UTC()
	}
	from := timeutil.InBeijing(to).AddDate(0, 0, -MaxExportDays).UTC()
	if q.From != nil {
		from = q.From.UTC()
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, invalid()
	}
	if from.Before(timeutil.InBeijing(to).AddDate(0, 0, -MaxExportDays).UTC()) {
		return time.Time{}, time.Time{}, exportLimitExceeded()
	}
	retainedFrom, retainedTo := retentionWindow(now)
	if from.Before(retainedFrom) {
		from = retainedFrom
	}
	if to.After(retainedTo) {
		to = retainedTo
	}
	return from, to, nil
}

// Export returns a complete, validated batch or nil plus an error. Callers must
// buffer all NDJSON encoding before sending headers or any bytes. It is a read
// in the existing UOW, with the same live authorization and 90-day privacy window
// as List. It does not create files, accept arbitrary JSON, or write audit rows.
func (s *Store) Export(ctx context.Context, reader VerifiedReader, query ExportQuery) ([]Audit, error) {
	if s == nil || s.manager == nil {
		return nil, errors.New("audit: UOW manager required")
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err = authorizeReader(reader); err != nil {
		return nil, err
	}
	from, to, err := exportWindow(query, timeutil.Now())
	if err != nil {
		return nil, err
	}
	if from.After(to) {
		return []Audit{}, nil
	}
	selection := db.Table("platform_audit_events").Select(eventColumns).
		Where("tenant_id = ? AND created_at >= ? AND created_at <= ?", reader.TenantID, from, to)
	if query.Action != "" {
		selection = selection.Where("action = ?", query.Action)
	}
	var rows []eventRow
	if err = selection.Order("created_at DESC, id DESC").Limit(MaxExportItems + 1).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > MaxExportItems {
		return nil, exportLimitExceeded()
	}
	items := make([]Audit, 0, len(rows))
	for _, row := range rows {
		item, safeErr := row.safeAudit()
		if safeErr != nil {
			return nil, safeErr
		}
		items = append(items, item)
	}
	return items, nil
}
