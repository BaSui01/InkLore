package adapters

import (
	content "InkLore/internal/content/application"
	"InkLore/internal/platform/scope"
	worldapp "InkLore/internal/worldview/application"
	world "InkLore/internal/worldview/domain"
	"context"
)

// WorldFactAdapter adapts content-owned confirmation values to worldview's typed ledger port.
type WorldFactAdapter struct{ World *worldapp.Service }

func (a WorldFactAdapter) CommitFacts(ctx context.Context, p scope.Principal, novel string, source content.ConfirmationSource, in content.Confirm) (content.FactCommitResult, error) {
	proposals := make([]world.FactProposal, 0, len(in.FactProposals))
	for _, v := range in.FactProposals {
		w := world.FactProposal{SubjectID: v.SubjectID, Predicate: v.Predicate, FactKey: v.FactKey, Action: v.Action, TargetEventID: v.TargetEventID, Resolution: v.Resolution, Reason: v.Reason}
		if v.Value != nil {
			w.Value = &world.FactValue{Kind: v.Value.Kind, TextValue: v.Value.TextValue, NumberValue: v.Value.NumberValue, BooleanValue: v.Value.BooleanValue, EntityID: v.Value.EntityID}
		}
		proposals = append(proposals, w)
	}
	out, err := a.World.CommitFacts(ctx, p, novel, world.Source{ChapterID: source.ChapterID, VersionID: source.VersionID, ContentHash: source.ContentHash, Revision: source.Revision}, world.CheckFacts{ExpectedWorldRevision: in.ExpectedWorldRevision, FactProposals: proposals})
	if err != nil {
		return content.FactCommitResult{}, err
	}
	return content.FactCommitResult{WorldRevision: out.WorldRevision, EventIDs: out.EventIDs}, err
}
