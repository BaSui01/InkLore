package application

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"
	"errors"
)

// GovernanceService coordinates administrative governance and creator catalog queries.
type GovernanceService struct {
	repo         d.GovernanceRepository
	allowlist    d.MCPDeploymentAllowlistPort
	manager      *uow.Manager
	builtinTools d.BuiltinToolCatalogPort
}

// NewGovernanceService constructs the service.
func NewGovernanceService(repo d.GovernanceRepository, allowlist d.MCPDeploymentAllowlistPort, manager *uow.Manager) (*GovernanceService, error) {
	if repo == nil {
		return nil, errors.New("agent governance: repository required")
	}
	if allowlist == nil {
		return nil, errors.New("agent governance: allowlist required")
	}
	if manager == nil {
		return nil, errors.New("agent governance: uow manager required")
	}
	return &GovernanceService{
		repo:      repo,
		allowlist: allowlist,
		manager:   manager,
	}, nil
}

func (s *GovernanceService) SetBuiltinToolCatalog(catalog d.BuiltinToolCatalogPort) {
	s.builtinTools = catalog
}

// AuthorizeAdmin verifies that the principal is an authenticated administrator
// with the necessary roles (operator for write; operator or auditor for read).
func (s *GovernanceService) AuthorizeAdmin(p identity.Principal, write bool) error {
	if p.Audience != "admin" {
		return d.ErrForbidden()
	}
	hasRole := false
	for _, r := range p.Roles {
		if r == "operator" {
			hasRole = true
			break
		}
		if !write && r == "auditor" {
			hasRole = true
			break
		}
	}
	if !hasRole {
		return d.ErrForbidden()
	}
	return nil
}

// AuthorizeCreator verifies that the principal has access to catalog endpoints.
func (s *GovernanceService) AuthorizeCreator(p identity.Principal) error {
	if p.Audience == "creator" {
		for _, r := range p.Roles {
			if r == "creator" {
				return nil
			}
		}
	} else if p.Audience == "admin" {
		for _, r := range p.Roles {
			if r == "operator" || r == "auditor" {
				return nil
			}
		}
	}
	return d.ErrForbidden()
}
