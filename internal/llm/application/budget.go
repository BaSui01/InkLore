package application

import (
	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/timeutil"
	"InkLore/internal/platform/uow"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BudgetOptions struct {
	Repository d.BudgetRepository
	UoW        UnitOfWork
	Identity   Authorizer
	OnEvent    func(context.Context, d.BudgetEvent) error
	Now        func() time.Time
}
type Budget struct{ opts BudgetOptions }

// RecoverReservations is a trusted worker entry, never an HTTP operation. Every
// row is rechecked under locks; a renewed live lease is not reclaimed. Crash
// receipts report unknown tokens as zero, estimated=crash, and no secret version.
func (s *Budget) RecoverReservations(ctx context.Context, limit int) (int, error) {
	rows, err := s.opts.Repository.Expired(ctx, s.opts.Now(), limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, v := range rows {
		if v.Status == "reserved" {
			if err = s.release(ctx, v.TenantID, v.ID, true); err != nil {
				return count, err
			}
		} else {
			code := "llm.upstream_outcome_unknown"
			_, err = s.SettleWithMetadata(ctx, d.SettlementInput{Settlement: d.Settlement{TenantID: v.TenantID, ReservationID: v.ID, TraceID: strings.ReplaceAll(uuid.NewString(), "-", ""), Status: "unknown", EstimateReason: "crash", ErrorCode: &code}, OnlyExpired: true})
			if err != nil {
				return count, err
			}
		}
		count++
	}
	return count, nil
}

func NewBudget(o BudgetOptions) (*Budget, error) {
	if o.Repository == nil || o.UoW == nil || o.Identity == nil || o.OnEvent == nil {
		return nil, errors.New("llm: budget dependencies required")
	}
	if o.Now == nil {
		o.Now = timeutil.Now
	}
	return &Budget{opts: o}, nil
}
func (s *Budget) Authorize(ctx context.Context, p identity.Principal, write bool) (identity.Principal, error) {
	roles := []string{"operator", "auditor"}
	if write {
		roles = []string{"operator"}
	}
	return s.opts.Identity.Authorize(ctx, p, "admin", roles...)
}
func (s *Budget) Get(ctx context.Context, p identity.Principal) (d.Budget, error) {
	p, err := s.Authorize(ctx, p, false)
	if err != nil {
		return d.Budget{}, err
	}
	v, err := s.opts.Repository.Get(ctx, p.TenantID, s.opts.Now())
	return v, public(err)
}
func (s *Budget) Configure(ctx context.Context, p identity.Principal, in d.BudgetUpdate) (d.Budget, error) {
	var out d.Budget
	if in.ExpectedRevision < 1 || in.ExpectedRevision >= d.MaxRevision || in.DailyLimitMicros < 0 || in.DailyLimitMicros > d.MaxMicros {
		return out, public(d.ErrInvalid)
	}
	err := s.opts.UoW.Within(ctx, func(tx context.Context) error {
		var err error
		p, err = s.Authorize(tx, p, true)
		if err != nil {
			return err
		}
		out, err = s.opts.Repository.Configure(tx, p.TenantID, in, s.opts.Now())
		if err != nil {
			return err
		}
		return s.opts.OnEvent(tx, d.BudgetEvent{TenantID: p.TenantID, ResourceID: out.ID, Action: "llm.budget_configured", Revision: out.Revision})
	})
	return out, public(err)
}
func (s *Budget) Provision(ctx context.Context, tenant string) error {
	if !uow.InTransaction(ctx) {
		return uow.ErrTransactionRequired
	}
	if !scope.ValidID(tenant) {
		return d.ErrInvalid
	}
	return s.opts.UoW.Within(ctx, func(tx context.Context) error { return s.opts.Repository.Provision(tx, tenant, s.opts.Now()) })
}
func (s *Budget) Reserve(ctx context.Context, in d.ReserveInput) (d.Reservation, error) {
	var out d.Reservation
	if !scope.ValidID(in.TenantID) || !scope.ValidID(in.InvocationID) || !scope.ValidID(in.PolicyVersionID) || !scope.ValidID(in.ModelID) || in.Attempt < 1 || in.Attempt > 4 || in.InputTokens < 0 || in.InputTokens > 60000 || in.MaxOutputTokens < 0 || in.MaxOutputTokens > 16384 || (in.MaxCostMicros != nil && (*in.MaxCostMicros < 0 || *in.MaxCostMicros > d.MaxMicros)) {
		return out, d.ErrInvalid
	}
	err := s.opts.UoW.Within(ctx, func(tx context.Context) error {
		v, changed, err := s.opts.Repository.Reserve(tx, in, s.opts.Now())
		out = v
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return s.opts.OnEvent(tx, d.BudgetEvent{TenantID: in.TenantID, ResourceID: v.ID, Action: "llm.budget_reserved", Revision: 1})
	})
	return out, err
}
func (s *Budget) MarkDispatch(ctx context.Context, in d.DispatchInput) (d.Reservation, error) {
	var out d.Reservation
	if !scope.ValidID(in.TenantID) || !scope.ValidID(in.ReservationID) || !scope.ValidID(in.ChannelID) || !scope.ValidID(in.KeyID) {
		return out, d.ErrInvalid
	}
	err := s.opts.UoW.Within(ctx, func(tx context.Context) error {
		v, changed, err := s.opts.Repository.MarkDispatch(tx, in, s.opts.Now())
		out = v
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return s.opts.OnEvent(tx, d.BudgetEvent{TenantID: in.TenantID, ResourceID: v.ID, Action: "llm.budget_dispatched"})
	})
	return out, err
}
func (s *Budget) Renew(ctx context.Context, tenant, id string, deadline time.Time) error {
	if !scope.ValidID(tenant) || !scope.ValidID(id) {
		return d.ErrInvalid
	}
	return s.opts.UoW.Within(ctx, func(tx context.Context) error { return s.opts.Repository.Renew(tx, tenant, id, deadline, s.opts.Now()) })
}
func (s *Budget) Release(ctx context.Context, tenant, id string) error {
	return s.release(ctx, tenant, id, false)
}
func (s *Budget) release(ctx context.Context, tenant, id string, expired bool) error {
	if !scope.ValidID(tenant) || !scope.ValidID(id) {
		return d.ErrInvalid
	}
	return s.opts.UoW.Within(ctx, func(tx context.Context) error {
		changed, err := s.opts.Repository.Release(tx, tenant, id, s.opts.Now(), expired)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return s.opts.OnEvent(tx, d.BudgetEvent{TenantID: tenant, ResourceID: id, Action: "llm.budget_released"})
	})
}
func (s *Budget) Settle(ctx context.Context, in d.Settlement) (*d.Usage, error) {
	return s.SettleWithMetadata(ctx, d.SettlementInput{Settlement: in})
}
func (s *Budget) SettleWithMetadata(ctx context.Context, in d.SettlementInput) (*d.Usage, error) {
	var out *d.Usage
	if !scope.ValidID(in.TenantID) || !scope.ValidID(in.ReservationID) {
		return nil, d.ErrInvalid
	}
	err := s.opts.UoW.Within(ctx, func(tx context.Context) error {
		v, changed, err := s.opts.Repository.Settle(tx, in, s.opts.Now())
		out = v
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return s.opts.OnEvent(tx, d.BudgetEvent{TenantID: in.TenantID, ResourceID: in.ReservationID, UsageID: v.ID, Action: "llm.budget_settled"})
	})
	return out, err
}
