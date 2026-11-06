package application

import (
	"context"
	"strings"

	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/scope"
)

// SumReceiptCostMicros is the read-only boundary for authorized consumers that
// own a persisted receipt list. Adjustments are included once per receipt.
func (s *Catalog) SumReceiptCostMicros(ctx context.Context, tenantID string, receiptIDs []string) (int64, error) {
	if !scope.ValidID(tenantID) {
		return 0, public(d.ErrInvalid)
	}
	if len(receiptIDs) == 0 {
		return 0, nil
	}
	if s == nil {
		return 0, public(d.ErrBudgetState)
	}
	reader, ok := s.opts.Repository.(d.ReceiptCostReader)
	if !ok {
		return 0, public(d.ErrBudgetState)
	}
	seen := make(map[string]bool, len(receiptIDs))
	ids := make([]string, 0, len(receiptIDs))
	for _, id := range receiptIDs {
		if !scope.ValidID(id) {
			return 0, public(d.ErrInvalid)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	cost, err := reader.SumReceiptCostMicros(ctx, tenantID, ids)
	return cost, public(err)
}

// FindOperationUsage recovers only durable accounting. A dispatched operation
// without a settled receipt remains unknown until budget recovery completes.
func (s *Catalog) FindOperationUsage(ctx context.Context, tenantID, operationKey string) (*d.Usage, error) {
	if !scope.ValidID(tenantID) || strings.TrimSpace(operationKey) == "" || len(operationKey) > 256 {
		return nil, public(d.ErrInvalid)
	}
	if s == nil {
		return nil, public(d.ErrBudgetState)
	}
	reader, ok := s.opts.Repository.(d.OperationUsageReader)
	if !ok {
		return nil, public(d.ErrBudgetState)
	}
	receipt, err := reader.FindOperationUsage(ctx, tenantID, operationKey)
	return receipt, public(err)
}

// ListSettledAgentOperations scans durable receipts regardless of Run lifecycle.
// Only trusted worker consumers call this method; no HTTP route exposes it.
func (s *Catalog) ListSettledAgentOperations(ctx context.Context, after string, limit int) ([]d.SettledAgentOperation, error) {
	if s == nil || limit < 1 || limit > 100 || (after != "" && !scope.ValidID(after)) {
		return nil, public(d.ErrInvalid)
	}
	reader, ok := s.opts.Repository.(d.SettledAgentUsageReader)
	if !ok {
		return nil, public(d.ErrBudgetState)
	}
	rows, err := reader.ListSettledAgentOperations(ctx, after, limit)
	return rows, public(err)
}
