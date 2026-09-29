package application

import (
	d "InkLore/internal/agent/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"context"
	"errors"
)

// RunAuthorizer rechecks source access without comparing historical revisions.
type RunAuthorizer interface {
	AuthorizeRun(context.Context, identity.Principal, string) error
}

func (s *RunService) AuthorizeRun(ctx context.Context, p identity.Principal, id string) error {
	if err := s.AuthorizeCreator(p); err != nil {
		return err
	}
	run, err := s.repo.GetRunOnly(ctx, p.TenantID, id)
	if err != nil {
		return err
	}
	if run.OwnerUserID != p.UserID || s.resources == nil {
		return d.ErrNotFound()
	}
	return s.resources.AuthorizeResource(ctx, p, s.buildRunView(run, nil).Resource)
}

func hiddenResource(err error) bool {
	var public *pe.AppError
	return errors.As(err, &public) && public.HTTPStatus == 404
}
