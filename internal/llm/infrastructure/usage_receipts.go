package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"

	d "InkLore/internal/llm/domain"
)

func (s *Store) SumReceiptCostMicros(ctx context.Context, tenantID string, receiptIDs []string) (int64, error) {
	if len(receiptIDs) == 0 {
		return 0, nil
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return 0, err
	}
	var row struct {
		ReceiptCount int
		Cost         string
	}
	err = db.Raw(`SELECT count(*) AS receipt_count,
		coalesce(sum(u.cost_micros::numeric+coalesce((SELECT sum(a.delta_micros::numeric)
		FROM llm_usage_adjustments a WHERE a.tenant_id=u.tenant_id AND a.usage_id=u.id),0)),0)::text AS cost
		FROM llm_usage_records u WHERE u.tenant_id=? AND u.id IN ?`, tenantID, receiptIDs).Scan(&row).Error
	if err != nil {
		return 0, err
	}
	if row.ReceiptCount != len(receiptIDs) {
		return 0, d.ErrNotFound
	}
	cost, err := strconv.ParseInt(row.Cost, 10, 64)
	if err != nil || cost < 0 {
		return 0, d.ErrBudgetState
	}
	return cost, nil
}

func (s *Store) FindOperationUsage(ctx context.Context, tenantID, operationKey string) (*d.Usage, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var row struct {
		Status   *string
		RawUsage *string
	}
	q := db.Raw(`SELECT r.status,row_to_json(u)::text AS raw_usage
		FROM llm_generation_invocations i
		LEFT JOIN llm_reservations r ON r.tenant_id=i.tenant_id AND r.invocation_id=i.id AND r.attempt=1
		LEFT JOIN llm_usage_records u ON u.tenant_id=i.tenant_id AND u.invocation_id=i.id AND u.attempt=1
		WHERE i.tenant_id=? AND i.operation_key=?`, tenantID, operationKey).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if row.RawUsage != nil {
		var receipt d.Usage
		if err := json.Unmarshal([]byte(*row.RawUsage), &receipt); err != nil {
			return nil, err
		}
		return &receipt, nil
	}
	if row.Status != nil && (*row.Status == "dispatched" || *row.Status == "settled") {
		return nil, d.ErrOutcomeUnknown
	}
	// No dispatch means no upstream charge: a crash before reserve/admission is
	// distinguishable from a dispatched operation whose outcome is still unknown.
	return nil, nil
}

var _ d.ReceiptCostReader = (*Store)(nil)
var _ d.OperationUsageReader = (*Store)(nil)
