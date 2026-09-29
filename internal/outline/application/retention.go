package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
	"time"
)

// PurgeNovel is a short retention step, never a public route. It requires an
// expired novel tombstone and current owner authorization on every invocation.
func (s *Service) PurgeNovel(ctx context.Context, p scope.Principal, novel string) (bool, error) {
	done := false
	err := s.UoW.Within(ctx, func(tx context.Context) error {
		v, err := s.Scope.AssertTrashOwner(tx, p, scope.Resource{Kind: "novel", ID: novel}, scope.Write)
		if err != nil {
			return mapScope(err)
		}
		if v.DeletedAt == nil || v.DeletedAt.After(time.Now().UTC().Add(-30*24*time.Hour)) {
			return stateError()
		}
		done, err = s.Repository.PurgeBatch(tx, d.Scope{TenantID: p.TenantID, NovelID: novel})
		if err != nil {
			return err
		}
		return s.OnMutation(tx, Mutation{Principal: p, NovelID: novel, ResourceKind: "novel", ResourceID: novel, Action: "purgeNovel", Revision: v.CurrentRevision})
	})
	return done, err
}
