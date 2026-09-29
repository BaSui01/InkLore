// Package notifications implements safe R1 WebSocket notifications only.
package notifications

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/outbox"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

type IdentityPort interface {
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}
type ScopePort interface {
	Assert(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
}
type SourcePort interface {
	RecentNotifications(context.Context, string, time.Time, int) ([]outbox.Event, error)
}

// VisibilityPort supports job/run ownership without accessing domain private tables.
type VisibilityPort interface {
	Visible(context.Context, identity.Principal, outbox.Event) (bool, error)
}
type Options struct {
	Identity                   IdentityPort
	Scope                      ScopePort
	Source                     SourcePort
	Visibility                 VisibilityPort
	CreatorOrigin, AdminOrigin string
}
type Service struct {
	manager *uow.Manager
	opts    Options
	origins map[string]string
}
type Notification struct {
	EventID    string    `json:"event_id"`
	Topic      string    `json:"topic"`
	ResourceID string    `json:"resource_id"`
	Revision   int64     `json:"revision"`
	OccurredAt time.Time `json:"occurred_at"`
}

func New(m *uow.Manager, o Options) (*Service, error) {
	if m == nil || o.Identity == nil || o.Scope == nil || o.Source == nil {
		return nil, errors.New("notification dependencies required")
	}
	for _, origin := range []string{o.CreatorOrigin, o.AdminOrigin} {
		u, e := url.Parse(origin)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || origin != u.Scheme+"://"+u.Host {
			return nil, errors.New("exact notification origins required")
		}
	}
	if o.CreatorOrigin == o.AdminOrigin {
		return nil, errors.New("distinct application origins required")
	}
	return &Service{m, o, map[string]string{o.CreatorOrigin: "creator", o.AdminOrigin: "admin"}}, nil
}
func invalidTicket() error { return pe.Contract(401, "auth.ws_ticket_invalid", nil) }

// ConsumeTicket checks the origin-selected audience and current live membership
// before atomically consuming the ticket. Failed/expired sessions never upgrade.
func (s *Service) ConsumeTicket(ctx context.Context, ticket, origin string) (identity.Principal, error) {
	audience, ok := s.origins[origin]
	if !ok {
		return identity.Principal{}, invalidTicket()
	}
	b, e := base64.RawURLEncoding.DecodeString(ticket)
	if e != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != ticket {
		return identity.Principal{}, invalidTicket()
	}
	sum := sha256.Sum256([]byte(ticket))
	hash := hex.EncodeToString(sum[:])
	var out identity.Principal
	e = s.manager.Within(ctx, func(tx context.Context) error {
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var candidate struct{ SessionID, TenantID, UserID, Audience string }
		q := db.Raw("SELECT s.id AS session_id,s.tenant_id,s.user_id,s.audience FROM platform_ws_tickets t JOIN platform_sessions s ON s.id=t.session_id WHERE t.ticket_hash=? AND t.consumed_at IS NULL AND t.expires_at>clock_timestamp()", hash).Scan(&candidate)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 || candidate.Audience != audience {
			return invalidTicket()
		}
		out, e = s.opts.Identity.Authorize(tx, identity.Principal{SessionID: candidate.SessionID, TenantID: candidate.TenantID, UserID: candidate.UserID, Audience: candidate.Audience})
		if e != nil {
			var authError *pe.AppError
			if errors.As(e, &authError) && (authError.HTTPStatus == 401 || authError.HTTPStatus == 403) {
				return invalidTicket()
			}
			return e
		}
		q = db.Exec("UPDATE platform_ws_tickets SET consumed_at=clock_timestamp() WHERE ticket_hash=? AND session_id=? AND consumed_at IS NULL AND expires_at>clock_timestamp()", hash, out.SessionID)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return invalidTicket()
		}
		return nil
	})
	if e != nil {
		return identity.Principal{}, e
	}
	return out, nil
}
func (s *Service) Filter(ctx context.Context, p identity.Principal, event outbox.Event) (Notification, bool, error) {
	current, e := s.opts.Identity.Authorize(ctx, p)
	if e != nil {
		return Notification{}, false, e
	}
	if event.TenantID != current.TenantID || !outbox.ValidTopic(event.Payload.Topic) || !scope.ValidID(event.ID) || !scope.ValidID(event.AggregateID) || event.AggregateRevision < 1 || event.AggregateRevision > 9007199254740991 {
		return Notification{}, false, nil
	}
	allowed := false
	switch event.Payload.Topic {
	case "system.degraded", "budget.warning":
		allowed = current.Audience == "admin"
	default:
		if current.Audience != "creator" || event.Payload.OwnerID != current.UserID {
			return Notification{}, false, nil
		}
		if s.opts.Visibility != nil {
			allowed, e = s.opts.Visibility.Visible(ctx, current, event)
		} else if event.Payload.ResourceKind != "" {
			_, e = s.opts.Scope.Assert(ctx, scope.Principal{TenantID: current.TenantID, ActorID: current.UserID, SessionID: current.SessionID, Audience: current.Audience}, scope.Resource{Kind: event.Payload.ResourceKind, ID: event.AggregateID}, scope.Read)
			allowed = e == nil
			var app *pe.AppError
			if errors.As(e, &app) && (app.HTTPStatus == 404 || app.HTTPStatus == 403) {
				e = nil
			}
		}
	}
	if e != nil {
		return Notification{}, false, e
	}
	if !allowed {
		return Notification{}, false, nil
	}
	return Notification{event.ID, event.Payload.Topic, event.AggregateID, event.AggregateRevision, event.CreatedAt.UTC()}, true, nil
}
func (s *Service) RegisterRoutes(r gin.IRoutes) { r.GET("/v1/platform/events", s.Handler) }
func (s *Service) Handler(c *gin.Context) {
	q, e := url.ParseQuery(c.Request.URL.RawQuery)
	c.Request.URL.RawQuery = ""
	c.Request.RequestURI = c.Request.URL.Path
	if e != nil || len(q) != 1 || len(q["ticket"]) != 1 {
		response.Fail(c, invalidTicket())
		return
	}
	// Remove credentials before any later middleware can log this request URL.
	ticket := q.Get("ticket")
	c.Request.URL.RawQuery = ""
	c.Request.RequestURI = c.Request.URL.Path
	p, e := s.ConsumeTicket(c.Request.Context(), ticket, c.GetHeader("Origin"))
	if e != nil {
		response.Fail(c, e)
		return
	}
	// Origin was matched byte-for-byte above; websocket's wildcard patterns are not used.
	conn, e := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{InsecureSkipVerify: true, CompressionMode: websocket.CompressionDisabled})
	if e != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(1024)
	ctx := conn.CloseRead(c.Request.Context())
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	seen := map[string]time.Time{}
	start := time.Now().UTC().Add(-time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			probe, cancel := context.WithTimeout(ctx, 5*time.Second)
			e = conn.Ping(probe)
			cancel()
			if e != nil {
				return
			}
		case <-tick.C:
			current, e := s.opts.Identity.Authorize(ctx, p)
			if e != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "session unavailable")
				return
			}
			p = current
			since := time.Now().UTC().Add(-time.Minute)
			if start.After(since) {
				since = start
			}
			events, e := s.opts.Source.RecentNotifications(ctx, p.TenantID, since, 200)
			if e != nil {
				_ = conn.Close(websocket.StatusInternalError, "notification unavailable")
				return
			}
			for _, event := range events {
				if _, ok := seen[event.ID]; ok {
					continue
				}
				n, allowed, e := s.Filter(ctx, p, event)
				if e != nil {
					return
				}
				seen[event.ID] = time.Now().UTC()
				if !allowed {
					continue
				}
				raw, e := json.Marshal(n)
				if e != nil {
					return
				}
				send, cancel := context.WithTimeout(ctx, 5*time.Second)
				e = conn.Write(send, websocket.MessageText, raw)
				cancel()
				if e != nil {
					return
				}
			}
			for id, t := range seen {
				if t.Before(since) {
					delete(seen, id)
				}
			}
		}
	}
}
