package infrastructure

import (
	"context"

	d "InkLore/internal/agent/domain"
)

// AppendUsageReceipt is an accounting-only CAS, including on terminal Runs:
// cancellation cannot discard a charge that has already settled upstream.
func (r *RunRepositoryPostgres) AppendUsageReceipt(ctx context.Context, tenantID, runID string, expectedRev int64, receiptID string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	q := db.Exec(`UPDATE agent_runs SET usage_receipt_ids=array_append(usage_receipt_ids,?::uuid),
		revision=revision+1,updated_at=clock_timestamp()
		WHERE tenant_id=? AND id=? AND revision=? AND deleted_at IS NULL
		AND NOT (?::uuid=ANY(usage_receipt_ids))`, receiptID, tenantID, runID, expectedRev, receiptID)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return d.ErrVersionConflict(map[string]any{"expected_revision": expectedRev})
	}
	return nil
}
