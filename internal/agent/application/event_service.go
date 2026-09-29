package application

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
)

const (
	HeartbeatInterval = 15 * time.Second
	EventPollInterval = 500 * time.Millisecond
)

// EventService coordinates persistent SSE event streaming and reconnection replay.
type EventService struct {
	repo       d.RunRepository
	authorizer RunAuthorizer
}

// NewEventService constructs an EventService.
func NewEventService(repo d.RunRepository, authorizer RunAuthorizer) (*EventService, error) {
	if repo == nil || authorizer == nil {
		return nil, errors.New("agent event service: repository required")
	}
	return &EventService{repo: repo, authorizer: authorizer}, nil
}

// AuthorizeCreator verifies that the principal has access to subscribe to run events.
func (s *EventService) AuthorizeCreator(p identity.Principal) error {
	if p.Audience != "creator" && p.Audience != "admin" {
		return d.ErrForbidden()
	}
	return nil
}

// SubscribeEvents streams persisted run events to the client with reconnection support.
// Parameters:
//   - p: authenticated principal
//   - runID: agent run UUID
//   - lastEventID: string from Last-Event-ID header (or query param)
//   - onEvent: callback invoked for each formatted SSE event envelope
//   - onHeartbeat: callback invoked every 15s to send SSE comment heartbeat
func (s *EventService) SubscribeEvents(
	ctx context.Context,
	p identity.Principal,
	runID string,
	lastEventID string,
	onEvent func(envelope d.SSEEnvelope, eventName string) error,
	onHeartbeat func() error,
) error {
	if err := s.AuthorizeCreator(p); err != nil {
		return err
	}
	if !scope.ValidID(runID) {
		return d.ErrValidationFailed(map[string]any{"reason": "invalid run_id"})
	}

	if err := s.authorizer.AuthorizeRun(ctx, p, runID); err != nil {
		return err
	}

	// Verify run existence and ownership
	run, err := s.repo.GetRunOnly(ctx, p.TenantID, runID)
	if err != nil {
		return err
	}
	if run.OwnerUserID != p.UserID && p.Audience != "admin" {
		return d.ErrNotFound()
	}

	// Parse Last-Event-ID
	var fromSeq int64 = 0
	lastEventID = strings.TrimSpace(lastEventID)
	if lastEventID != "" {
		parsed, err := strconv.ParseInt(lastEventID, 10, 64)
		if err != nil || parsed < 0 {
			return d.ErrEventCursorInvalid(map[string]any{
				"last_event_id": lastEventID,
				"reason":        "must be non-negative integer",
			})
		}
		fromSeq = parsed

		// Preflight TTL check: if requested cursor is older than retained_from_seq or expired
		if fromSeq > 0 {
			expired, err := s.repo.CheckEventCursorExpired(ctx, p.TenantID, runID, fromSeq)
			if err != nil {
				return err
			}
			if expired {
				return d.ErrEventCursorExpired(map[string]any{
					"run_id":        runID,
					"last_event_id": fromSeq,
					"message":       "event cursor is older than the 7-day retention window; check GET /v1/agents/runs/{run_id}",
				})
			}
		}
	}

	// 1. Initial replay of existing events
	lastSeq := fromSeq
	terminalReached := false

	events, err := s.repo.GetEventsAfter(ctx, p.TenantID, runID, lastSeq, 1000)
	if err != nil {
		return err
	}

	for _, ev := range events {
		sanitizedPayload, ok := s.sanitizePayload(ev.EventName, ev.Payload)
		if !ok {
			continue // skip any non-public or hidden reasoning
		}

		envelope := d.SSEEnvelope{
			RunID:      ev.RunID,
			Seq:        ev.Seq,
			OccurredAt: ev.CreatedAt,
			Payload:    sanitizedPayload,
		}

		if err := onEvent(envelope, ev.EventName); err != nil {
			return err
		}
		lastSeq = ev.Seq

		if isTerminalEvent(ev.EventName) {
			terminalReached = true
			break
		}
	}

	// If terminal event already sent, finish stream cleanly without [DONE]
	if terminalReached {
		return nil
	}

	// Also check if run is already terminal in DB and terminal event was already before lastSeq
	currentRun, _ := s.repo.GetRunOnly(ctx, p.TenantID, runID)
	if currentRun != nil && currentRun.Status.IsTerminal() && len(events) == 0 {
		return nil
	}

	// 2. Live streaming loop
	heartbeatTicker := time.NewTicker(HeartbeatInterval)
	defer heartbeatTicker.Stop()

	pollTicker := time.NewTicker(EventPollInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-heartbeatTicker.C:
			if err := s.authorizer.AuthorizeRun(ctx, p, runID); err != nil {
				return err
			}
			if err := onHeartbeat(); err != nil {
				return err
			}

		case <-pollTicker.C:
			if err := s.authorizer.AuthorizeRun(ctx, p, runID); err != nil {
				return err
			}
			newEvents, err := s.repo.GetEventsAfter(ctx, p.TenantID, runID, lastSeq, 100)
			if err != nil {
				return err
			}

			for _, ev := range newEvents {
				sanitizedPayload, ok := s.sanitizePayload(ev.EventName, ev.Payload)
				if !ok {
					continue
				}

				envelope := d.SSEEnvelope{
					RunID:      ev.RunID,
					Seq:        ev.Seq,
					OccurredAt: ev.CreatedAt,
					Payload:    sanitizedPayload,
				}

				if err := onEvent(envelope, ev.EventName); err != nil {
					return err
				}
				lastSeq = ev.Seq

				if isTerminalEvent(ev.EventName) {
					return nil // Stream ends cleanly on terminal event
				}
			}

			// Check if run transitioned to terminal without new events
			if len(newEvents) == 0 {
				r, err := s.repo.GetRunOnly(ctx, p.TenantID, runID)
				if err == nil && r != nil && r.Status.IsTerminal() {
					// Wait for terminal event to appear in next poll
				}
			}
		}
	}
}

// sanitizePayload strips any hidden internal reasoning to strictly enforce "绝不泄露隐藏推理".
func (s *EventService) sanitizePayload(eventName string, raw json.RawMessage) (json.RawMessage, bool) {
	switch eventName {
	case "reasoning.summary":
		// Only allow {"summary": "..."}
		var summary d.ReasoningSummaryPayload
		if err := json.Unmarshal(raw, &summary); err != nil {
			return nil, false
		}
		// Bound to 2000 chars as per spec
		if len(summary.Summary) > 2000 {
			summary.Summary = summary.Summary[:2000]
		}
		cleanBytes, _ := json.Marshal(summary)
		return cleanBytes, true

	case "run.started", "text.delta", "tool.started", "tool.completed",
		"approval.required", "node.updated", "usage.updated",
		"run.succeeded", "run.failed", "run.cancelled":
		return raw, true

	default:
		// Unknown event name -> never expose to client
		return nil, false
	}
}

func isTerminalEvent(eventName string) bool {
	return eventName == "run.succeeded" || eventName == "run.failed" || eventName == "run.cancelled"
}
