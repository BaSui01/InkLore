package adapters

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
)

// AuthorizeResource checks live ownership without making CAS a replay precondition.
func (a AgentTaskResources) AuthorizeResource(ctx context.Context, p identity.Principal, ref *d.ResourceRef) error {
	if ref == nil || a.Scope == nil || a.Identity == nil {
		return d.ErrNotFound()
	}
	if _, err := a.Identity.Authorize(ctx, p, "creator"); err != nil {
		return err
	}
	principal := scope.Principal{TenantID: p.TenantID, ActorID: p.UserID, SessionID: p.SessionID, Audience: p.Audience}
	checkNovel := func(id *string) error {
		if id == nil {
			return nil
		}
		_, err := a.Scope.Assert(ctx, principal, scope.Resource{Kind: "novel", ID: *id}, scope.Read)
		return err
	}
	if err := checkNovel(ref.NovelID); err != nil {
		return err
	}
	switch ref.ResourceType {
	case "chapter", "novel":
		actual, err := a.Scope.Assert(ctx, principal, scope.Resource{Kind: ref.ResourceType, ID: ref.ResourceID}, scope.Read)
		if err != nil {
			return err
		}
		if ref.NovelID != nil {
			if ref.ResourceType == "novel" && actual.ResourceID != *ref.NovelID {
				return d.ErrNotFound()
			}
			if ref.ResourceType == "chapter" && (actual.NovelID == nil || *actual.NovelID != *ref.NovelID) {
				return d.ErrNotFound()
			}
		}
		return nil
	case "analysis":
		if a.Inspiration == nil {
			return d.ErrNotFound()
		}
		value, err := a.Inspiration.GetAnalysis(ctx, principal, ref.ResourceID)
		if err != nil {
			return err
		}
		if (ref.NovelID == nil) != (value.NovelID == nil) || ref.NovelID != nil && *ref.NovelID != *value.NovelID {
			return d.ErrNotFound()
		}
		return checkNovel(value.NovelID)
	case "report", "rewrite":
		if a.Compliance == nil {
			return d.ErrNotFound()
		}
		reportID := ref.ResourceID
		if ref.ResourceType == "rewrite" {
			value, err := a.Compliance.GetRewrite(ctx, principal, ref.ResourceID)
			if err != nil {
				return err
			}
			reportID = value.ReportID
		}
		value, err := a.Compliance.GetReport(ctx, principal, reportID)
		if err != nil {
			return err
		}
		if ref.NovelID == nil || *ref.NovelID != value.NovelID {
			return d.ErrNotFound()
		}
		return checkNovel(&value.NovelID)
	default:
		return d.ErrNotFound()
	}
}
