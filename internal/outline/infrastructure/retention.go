package infrastructure

import (
	d "InkLore/internal/outline/domain"
	"context"
)

// PurgeBatch deletes at most 1000 rows per call, children before parents.
func (r *Repository) PurgeBatch(ctx context.Context, s d.Scope) (bool, error) {
	db, err := r.manager.Require(ctx)
	if err != nil {
		return false, err
	}
	for _, q := range []struct{ table, filter string }{{"outline_clue_events", ""}, {"outline_clues", ""}, {"outline_scenes", ""}, {"outline_beats", ""}, {"outline_nodes", " AND level='chapter'"}, {"outline_nodes", " AND level='volume'"}, {"outline_nodes", " AND level='master'"}} {
		result := db.Exec("DELETE FROM "+q.table+" WHERE tenant_id=? AND novel_id=? AND id IN (SELECT id FROM "+q.table+" WHERE tenant_id=? AND novel_id=?"+q.filter+" ORDER BY id LIMIT 1000)", s.TenantID, s.NovelID, s.TenantID, s.NovelID)
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected > 0 {
			return false, nil
		}
	}
	return true, nil
}
