package infrastructure

import "context"

// FileInUse checks every frozen content_jobs file reference, including retained
// terminal jobs. References must be explicitly released before a file is deleted;
// status or expiration alone does not remove the reference. No row locks are taken.
func (r *Repository) FileInUse(ctx context.Context, tenantID, fileID string) (bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return false, err
	}
	var used bool
	err = db.Raw(`SELECT EXISTS (
 SELECT 1 FROM content_jobs WHERE tenant_id=? AND (
 source_file_id=? OR output_file_id=? OR planning_snapshot_file_id=?
 OR worldview_snapshot_file_id=? OR organization_snapshot_file_id=?
 ))`, tenantID, fileID, fileID, fileID, fileID, fileID).Scan(&used).Error
	return used, err
}
