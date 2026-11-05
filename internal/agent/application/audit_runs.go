package application

import (
	"context"
	"errors"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
)

func authorizeRunAudit(p identity.Principal) error {
	if p.Audience != "admin" || !scope.ValidID(p.TenantID) || !scope.ValidID(p.UserID) {
		return d.ErrForbidden()
	}
	for _, role := range p.Roles {
		if role == "operator" || role == "auditor" {
			return nil
		}
	}
	return d.ErrForbidden()
}

func (s *RunService) auditView(ctx context.Context, tenantID string, record d.AuditRunRecord) (d.AuditRunView, error) {
	view := record.View
	if len(record.ReceiptIDs) == 0 {
		return view, nil
	}
	usage, ok := s.gateway.(d.RunUsagePort)
	if !ok {
		return d.AuditRunView{}, errors.New("agent run audit: usage reader unavailable")
	}
	cost, err := usage.SumReceiptCostMicros(ctx, tenantID, record.ReceiptIDs)
	if err != nil {
		return d.AuditRunView{}, err
	}
	view.CostMicros = cost
	return view, nil
}

func (s *RunService) ListAuditRuns(ctx context.Context, p identity.Principal, query d.ListQuery) (*d.AuditRunPage, error) {
	if err := authorizeRunAudit(p); err != nil {
		return nil, err
	}
	records, more, err := s.repo.ListAuditRuns(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}
	page := &d.AuditRunPage{Items: make([]d.AuditRunView, 0, len(records)), HasMore: more}
	for _, record := range records {
		view, err := s.auditView(ctx, p.TenantID, record)
		if err != nil {
			return nil, err
		}
		page.Items = append(page.Items, view)
	}
	return page, nil
}

func (s *RunService) GetAuditRun(ctx context.Context, p identity.Principal, id string) (*d.AuditRunView, error) {
	if err := authorizeRunAudit(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrNotFound()
	}
	record, err := s.repo.GetAuditRun(ctx, p.TenantID, id)
	if err != nil {
		return nil, err
	}
	view, err := s.auditView(ctx, p.TenantID, *record)
	return &view, err
}
