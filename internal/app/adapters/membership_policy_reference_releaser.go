package adapters

import (
	"context"
	"errors"

	llmapp "InkLore/internal/llm/application"
)

// MembershipPolicyReferenceReleaser forwards guide-session deletions to the
// LLM reference ledger through the composition root, so platform code never
// imports the LLM domain and frozen-version deletion refusal stays accurate
// after member cleanup.
type MembershipPolicyReferenceReleaser struct {
	catalog *llmapp.Catalog
}

func NewMembershipPolicyReferenceReleaser(catalog *llmapp.Catalog) *MembershipPolicyReferenceReleaser {
	return &MembershipPolicyReferenceReleaser{catalog: catalog}
}

func (r *MembershipPolicyReferenceReleaser) ReleaseGuideSessionReferences(ctx context.Context, tenantID string, sessionIDs []string) error {
	if r == nil || r.catalog == nil {
		return errors.New("policy reference releaser is not configured")
	}
	return r.catalog.ReleasePolicyVersionReferences(ctx, tenantID, "guide_session", sessionIDs)
}
