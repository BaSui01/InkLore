package application

import (
	"context"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
)

func (s *RunService) ListCreatorTaskPolicies(ctx context.Context, p identity.Principal, taskKind, afterID string, limit int) ([]d.CreatorTaskPolicy, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if (taskKind != "" && !d.IsCreatorTaskKind(taskKind)) || limit < 1 || limit > 101 {
		return nil, d.ErrValidationFailed(nil)
	}
	port, ok := s.gateway.(d.CreatorPolicyPort)
	if !ok {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "creator policy catalog is unavailable"})
	}
	return port.ListCreatorTaskPolicies(ctx, p, taskKind, afterID, limit)
}
