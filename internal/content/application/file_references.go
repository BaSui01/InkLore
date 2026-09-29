package application

import (
	"InkLore/internal/platform/scope"
	"context"
)

// FileInUse is called by the composition-root file reference adapter after file
// authorization and locking. It reveals only a boolean, joins the supplied UoW,
// and must not acquire scope or business write locks in reverse order.
func (s *Service) FileInUse(ctx context.Context, tenantID, fileID string) (bool, error) {
	if !scope.ValidID(tenantID) || !scope.ValidID(fileID) {
		return false, invalid()
	}
	return s.o.Repo.FileInUse(ctx, tenantID, fileID)
}
