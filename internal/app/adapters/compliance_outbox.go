package adapters

import (
	"InkLore/internal/platform/outbox"
	"context"
)

// ComplianceOutboxAdapter adapts platform outbox for compliance domain.
type ComplianceOutboxAdapter struct {
	Outbox *outbox.Service
}

func (a ComplianceOutboxAdapter) Publish(ctx context.Context, tenantID, producer, eventType, aggregateID string, revision int64, payload map[string]any) error {
	_, err := a.Outbox.Publish(ctx, outbox.PublishInput{
		TenantID:          tenantID,
		Producer:          producer,
		EventType:         eventType,
		AggregateID:       aggregateID,
		AggregateRevision: revision,
		Payload: outbox.Payload{
			Topic: "compliance.event",
		},
	})
	return err
}
