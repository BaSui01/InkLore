package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/scope"
)

// recordModelUsage makes receipt deduplication and the SSE event one atomic
// Run mutation. A settled charge must survive user cancellation or lease loss.
func (s *Scheduler) recordModelUsage(ctx context.Context, tenantID, runID string, output *d.ModelGenerateOutput) error {
	if output == nil {
		return nil
	}
	if output.UsageReceiptID == nil {
		if output.InputTokens != 0 || output.OutputTokens != 0 || output.CostMicros != 0 {
			return errors.New("model usage has no durable receipt")
		}
		return nil
	}
	if !scope.ValidID(*output.UsageReceiptID) || output.InputTokens < 0 || output.OutputTokens < 0 || output.CostMicros < 0 {
		return errors.New("invalid model usage receipt")
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.manager.Within(cleanup, func(tx context.Context) error {
		run, err := s.repo.LockRun(tx, tenantID, runID)
		if err != nil {
			return err
		}
		for _, receiptID := range run.UsageReceiptIDs {
			if receiptID == *output.UsageReceiptID {
				return nil
			}
		}
		if err := s.repo.AppendUsageReceipt(tx, tenantID, runID, run.Revision, *output.UsageReceiptID); err != nil {
			return err
		}
		payload, err := json.Marshal(d.UsageUpdatedPayload{
			InputTokens: output.InputTokens, OutputTokens: output.OutputTokens,
			CostMicros: output.CostMicros, Currency: "CNY", Estimated: output.Estimated,
		})
		if err != nil {
			return err
		}
		return s.appendWorkerEvent(tx, tenantID, runID, "usage.updated", payload)
	})
}
