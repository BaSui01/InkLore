package app

import (
	"InkLore/internal/platform/audit"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/outbox"
	"context"
)

type replayActorKey struct{}

// ReplayOutbox is an internal operator command, not an additional HTTP route.
// Its capability exists only inside the transaction holding current authorization.
func (a *App) ReplayOutbox(ctx context.Context, p identity.Principal, eventID string) error {
	return a.platform.UoW.Within(ctx, func(tx context.Context) error {
		current, err := a.platform.Identity.Authorize(tx, p, "operator")
		if err != nil {
			return err
		}
		if current.Audience != "admin" {
			return pe.Contract(403, "auth.forbidden", nil)
		}
		return a.domains.Outbox.Replay(context.WithValue(tx, replayActorKey{}, current), eventID)
	})
}

func (a *App) outboxAudit(action string) func(context.Context, outbox.Event) error {
	return func(ctx context.Context, event outbox.Event) error {
		var actor *string
		audience := "system"
		if action == "platform.outbox_replayed" {
			principal, ok := ctx.Value(replayActorKey{}).(identity.Principal)
			if !ok {
				return pe.Contract(403, "auth.forbidden", nil)
			}
			if principal.TenantID != event.TenantID {
				return pe.Contract(404, "platform.resource_not_found", nil)
			}
			actor = &principal.UserID
			audience = principal.Audience
		}
		requestID, traceID := correlation(ctx)
		_, err := a.platform.Audit.Append(ctx, audit.Record{TenantID: event.TenantID, ActorID: actor, Audience: audience, Action: action, ResourceID: &event.ID, Outcome: "succeeded", RequestID: requestID, TraceID: traceID, SafeChanges: []audit.SafeChange{}})
		return err
	}
}
