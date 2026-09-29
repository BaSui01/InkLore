// Package outbox owns durable safe events and atomic consumer inboxes.
package outbox

import (
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrNoEvent = errors.New("no eligible outbox event")
var keyPattern = regexp.MustCompile("^[a-z][a-z0-9_.]{0,99}$")

type Payload struct {
	OwnerID      string `json:"owner_id,omitempty"`
	NovelID      string `json:"novel_id,omitempty"`
	ResourceKind string `json:"resource_kind,omitempty"`
	Topic        string `json:"topic,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
}
type Event struct {
	ID, TenantID, Producer, EventType, AggregateID string
	AggregateRevision                              int64
	Payload                                        Payload
	CreatedAt                                      time.Time
}
type PublishInput struct {
	TenantID, Producer, EventType, AggregateID string
	AggregateRevision                          int64
	Payload                                    Payload
}
type Consumer struct {
	Name       string
	EventTypes []string
	Apply      func(context.Context, Event) error
}
type row struct {
	ID, TenantID, Producer, EventType, AggregateID, Payload, State string
	AggregateRevision                                              int64
	Attempt                                                        int
	CreatedAt                                                      time.Time
}
type Options struct {
	OnDeadLetter func(context.Context, Event) error
	OnReplay     func(context.Context, Event) error
}
type Service struct {
	manager   *uow.Manager
	mu        sync.RWMutex
	consumers map[string]Consumer
	opts      Options
}

func New(m *uow.Manager, o Options) (*Service, error) {
	if m == nil {
		return nil, errors.New("outbox manager required")
	}
	return &Service{manager: m, consumers: map[string]Consumer{}, opts: o}, nil
}
func safePayload(p Payload) bool {
	for _, id := range []string{p.OwnerID, p.NovelID, p.RequestID} {
		if id != "" && !scope.ValidID(id) {
			return false
		}
	}
	if p.ResourceKind != "" {
		for _, c := range p.ResourceKind {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
				return false
			}
		}
		if len(p.ResourceKind) > 100 {
			return false
		}
	}
	if p.Topic != "" && !ValidTopic(p.Topic) {
		return false
	}
	if p.TraceID != "" {
		if len(p.TraceID) != 32 {
			return false
		}
		for _, c := range p.TraceID {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}
func ValidTopic(s string) bool {
	switch s {
	case "run.terminal", "job.terminal", "resource.changed", "budget.warning", "system.degraded":
		return true
	}
	return false
}
func (s *Service) Publish(ctx context.Context, in PublishInput) (Event, error) {
	var out Event
	db, e := s.manager.Require(ctx)
	if e != nil {
		return out, e
	}
	if !scope.ValidID(in.TenantID) || !scope.ValidID(in.AggregateID) || !keyPattern.MatchString(in.Producer) || !keyPattern.MatchString(in.EventType) || in.AggregateRevision < 1 || in.AggregateRevision > 9007199254740991 || !safePayload(in.Payload) {
		return out, errors.New("invalid safe outbox event")
	}
	b, e := json.Marshal(in.Payload)
	if e != nil {
		return out, e
	}
	var r row
	q := db.Raw("INSERT INTO platform_outbox (id,tenant_id,producer,event_type,aggregate_id,aggregate_revision,payload) VALUES (?,?,?,?,?,?,?::jsonb) ON CONFLICT (tenant_id,producer,aggregate_id,aggregate_revision,event_type) DO NOTHING RETURNING *", uuid.NewString(), in.TenantID, in.Producer, in.EventType, in.AggregateID, in.AggregateRevision, string(b)).Scan(&r)
	if q.Error != nil {
		return out, q.Error
	}
	if q.RowsAffected == 0 {
		q = db.Raw("SELECT * FROM platform_outbox WHERE tenant_id=? AND producer=? AND aggregate_id=? AND aggregate_revision=? AND event_type=?", in.TenantID, in.Producer, in.AggregateID, in.AggregateRevision, in.EventType).Scan(&r)
		if q.Error != nil {
			return out, q.Error
		}
		old, e := decode(r)
		if e != nil {
			return out, e
		}
		if old.Payload != in.Payload {
			return out, errors.New("outbox event key payload conflict")
		}
		return old, nil
	}
	return decode(r)
}
func decode(r row) (Event, error) {
	var p Payload
	d := json.NewDecoder(strings.NewReader(r.Payload))
	d.DisallowUnknownFields()
	if e := d.Decode(&p); e != nil || !safePayload(p) {
		return Event{}, errors.New("invalid stored safe event")
	}
	return Event{r.ID, r.TenantID, r.Producer, r.EventType, r.AggregateID, r.AggregateRevision, p, r.CreatedAt}, nil
}
func (s *Service) Register(c Consumer) error {
	if !keyPattern.MatchString(c.Name) || c.Apply == nil || len(c.EventTypes) == 0 {
		return errors.New("invalid outbox consumer")
	}
	for _, t := range c.EventTypes {
		if !keyPattern.MatchString(t) {
			return errors.New("invalid event type")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.consumers[c.Name]; ok {
		return errors.New("consumer already registered")
	}
	c.EventTypes = append([]string(nil), c.EventTypes...)
	s.consumers[c.Name] = c
	return nil
}
func (s *Service) matching(eventType string) []Consumer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Consumer{}
	for _, c := range s.consumers {
		for _, t := range c.EventTypes {
			if t == eventType {
				out = append(out, c)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Consume runs only the registered projection. The inbox insertion and callback
// use one UoW; a duplicate delivery never invokes Apply again.
func (s *Service) Consume(ctx context.Context, consumer, eventID string) error {
	s.mu.RLock()
	c, ok := s.consumers[consumer]
	s.mu.RUnlock()
	if !ok || !scope.ValidID(eventID) {
		return errors.New("unregistered consumer or event")
	}
	return s.manager.Within(ctx, func(tx context.Context) error {
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var r row
		q := db.Raw("SELECT * FROM platform_outbox WHERE id=? FOR UPDATE", eventID).Scan(&r)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return ErrNoEvent
		}
		event, e := decode(r)
		if e != nil {
			return e
		}
		allowed := false
		for _, t := range c.EventTypes {
			if t == event.EventType {
				allowed = true
			}
		}
		if !allowed {
			return errors.New("consumer event mismatch")
		}
		return s.apply(tx, c, event)
	})
}
func (s *Service) apply(ctx context.Context, c Consumer, event Event) error {
	db, e := s.manager.Require(ctx)
	if e != nil {
		return e
	}
	q := db.Exec("INSERT INTO platform_inbox (consumer,event_id) VALUES (?,?) ON CONFLICT DO NOTHING", c.Name, event.ID)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return nil
	}
	return c.Apply(ctx, event)
}

type DispatchResult struct {
	EventID               string
	Delivered, DeadLetter bool
	Attempt               int
}

// DispatchOne retains the row lock while local projection callbacks execute.
// Callbacks must not perform network I/O or publish to sockets before commit.
func (s *Service) DispatchOne(ctx context.Context) (DispatchResult, error) {
	var out DispatchResult
	e := s.manager.Within(ctx, func(tx context.Context) error {
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var r row
		q := db.Raw("SELECT * FROM platform_outbox WHERE state='pending' AND available_at<=clock_timestamp() ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1").Scan(&r)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return ErrNoEvent
		}
		event, e := decode(r)
		if e != nil {
			return e
		}
		out.EventID = event.ID
		consumers := s.matching(event.EventType)
		failed := len(consumers) == 0
		for _, c := range consumers {
			e = s.manager.Recoverable(tx, func(local context.Context) error { return s.apply(local, c, event) })
			if e != nil {
				failed = true
				break
			}
		}
		if !failed {
			out.Delivered = true
			return db.Exec("UPDATE platform_outbox SET state='delivered',delivered_at=clock_timestamp() WHERE id=?", event.ID).Error
		}
		out.Attempt = r.Attempt + 1
		delays := []int{1, 2, 4, 8, 16, 32, 60, 60}
		if out.Attempt >= 8 {
			out.DeadLetter = true
			if s.opts.OnDeadLetter != nil {
				if e = s.opts.OnDeadLetter(tx, event); e != nil {
					return e
				}
			}
			return db.Exec("UPDATE platform_outbox SET attempt=?,state='dead_letter' WHERE id=?", out.Attempt, event.ID).Error
		}
		return db.Exec("UPDATE platform_outbox SET attempt=?,available_at=clock_timestamp()+(? * interval '1 second') WHERE id=?", out.Attempt, delays[out.Attempt-1], event.ID).Error
	})
	return out, e
}

// Replay is a CLI-only fixed-consumer redelivery hook. Audit is mandatory and
// existing inbox records remain, so successful consumers are never re-applied.
func (s *Service) Replay(ctx context.Context, eventID string) error {
	if !scope.ValidID(eventID) || s.opts.OnReplay == nil {
		return errors.New("audited CLI replay unavailable")
	}
	return s.manager.Within(ctx, func(tx context.Context) error {
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var r row
		q := db.Raw("SELECT * FROM platform_outbox WHERE id=? AND state='dead_letter' FOR UPDATE", eventID).Scan(&r)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return ErrNoEvent
		}
		event, e := decode(r)
		if e != nil {
			return e
		}
		if len(s.matching(event.EventType)) == 0 {
			return errors.New("event has no registered consumer")
		}
		if e = s.opts.OnReplay(tx, event); e != nil {
			return e
		}
		return db.Exec("UPDATE platform_outbox SET state='pending',attempt=0,available_at=clock_timestamp() WHERE id=?", eventID).Error
	})
}
