package authdomain

import "time"

const (
	OrgSyncRunStatusRunning = "running"
	OrgSyncRunStatusSuccess = "success"
	OrgSyncRunStatusFailed  = "failed"
)

// OrgSyncRun records one server-side pull from Auth Center into local org projection.
type OrgSyncRun struct {
	StartedAt        time.Time  `json:"started_at"`
	CreatedAt        time.Time  `json:"created_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	Mode             string     `json:"mode"`
	Status           string     `json:"status"`
	ErrorReason      string     `json:"error_reason,omitempty"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	ID               int64      `json:"id"`
	DepartmentsCount int        `json:"departments_count"`
	MembershipsCount int        `json:"memberships_count"`
	CreatedBy        int64      `json:"created_by,omitempty"`
}
