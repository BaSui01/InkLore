package outbox

import (
	"InkLore/internal/platform/scope"
	"context"
	"errors"
	"time"
)

// RecentNotifications supports process-independent low frequency WS polling.
// Delivery is advisory; bounded windows may miss bursts and never promise replay.
func (s *Service) RecentNotifications(ctx context.Context, tenantID string, since time.Time, limit int) ([]Event, error) {
	if !scope.ValidID(tenantID) || limit < 1 || limit > 200 {
		return nil, errors.New("invalid notification query")
	}
	db, e := s.manager.DB(ctx)
	if e != nil {
		return nil, e
	}
	var rows []row
	e = db.Raw("SELECT * FROM platform_outbox WHERE tenant_id=? AND created_at>=? AND payload->>'topic' IN ('run.terminal','job.terminal','resource.changed','budget.warning','system.degraded') ORDER BY created_at DESC,id DESC LIMIT ?", tenantID, since.UTC(), limit).Scan(&rows).Error
	if e != nil {
		return nil, e
	}
	events := make([]Event, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		event, e := decode(rows[i])
		if e != nil {
			return nil, e
		}
		events = append(events, event)
	}
	return events, nil
}
