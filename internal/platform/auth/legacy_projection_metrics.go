package authdomain

import (
	"sync/atomic"
)

// LegacyProjectionReadReason identifies residual local auth_center_* column reads
// that are not authorization decisions. Counters exist so stop-read progress can be
// observed before an independent column-drop migration.
const (
	LegacyProjectionReadUserRoleFilter   = "user_role_filter"
	LegacyProjectionReadAuditDiagnostics = "audit_diagnostics"
)

var (
	legacyProjectionReadUserRoleFilter   atomic.Int64
	legacyProjectionReadAuditDiagnostics atomic.Int64
)

// ObserveLegacyProjectionRead increments a lightweight deprecation counter.
// Safe to call from hot paths; never changes control flow.
func ObserveLegacyProjectionRead(reason string) {
	switch reason {
	case LegacyProjectionReadUserRoleFilter:
		legacyProjectionReadUserRoleFilter.Add(1)
	case LegacyProjectionReadAuditDiagnostics:
		legacyProjectionReadAuditDiagnostics.Add(1)
	default:
		// Unknown reasons are intentionally ignored to keep the surface minimal.
	}
}

// LegacyProjectionReadSnapshot is a point-in-time copy of deprecation counters.
type LegacyProjectionReadSnapshot struct {
	UserRoleFilter   int64 `json:"user_role_filter"`
	AuditDiagnostics int64 `json:"audit_diagnostics"`
}

// SnapshotLegacyProjectionReads returns current deprecation counters for tests/diagnostics.
func SnapshotLegacyProjectionReads() LegacyProjectionReadSnapshot {
	return LegacyProjectionReadSnapshot{
		UserRoleFilter:   legacyProjectionReadUserRoleFilter.Load(),
		AuditDiagnostics: legacyProjectionReadAuditDiagnostics.Load(),
	}
}

// ResetLegacyProjectionReadsForTest clears counters. Tests only.
func ResetLegacyProjectionReadsForTest() {
	legacyProjectionReadUserRoleFilter.Store(0)
	legacyProjectionReadAuditDiagnostics.Store(0)
}
