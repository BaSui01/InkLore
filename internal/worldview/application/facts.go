package application

import (
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	d "InkLore/internal/worldview/domain"
	"context"
	"regexp"

	"github.com/google/uuid"
)

func (s *Service) CheckFacts(ctx context.Context, p scope.Principal, n string, in d.CheckFacts) (d.CheckResult, error) {
	out := d.CheckResult{Conflicts: []d.Conflict{}}
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.o.UoW.WithinRepeatableRead(ctx, func(tx context.Context) error {
		if _, err := s.Authorize(tx, p, "novel", n, false); err != nil {
			return err
		}
		state, err := s.o.Store.Load(tx, p.TenantID, n, false)
		if err != nil {
			return err
		}
		out.WorldRevision = state.World.Revision
		if in.ExpectedWorldRevision != state.World.Revision {
			return d.FactConflict(state.World.Revision, nil)
		}
		out.Conflicts = d.Inspect(state, in.FactProposals)
		out.Valid = len(out.Conflicts) == 0
		return nil
	})
	return out, err
}
func (s *Service) ValidateFacts(ctx context.Context, p scope.Principal, n string, in d.CheckFacts) (d.CheckResult, error) {
	return s.CheckFacts(ctx, p, n, in)
}

// ApplyFacts/CommitFacts must be composed with the caller's chapter confirmation
// transaction. The manager's nested boundary poisons the outer UoW on failure.
// The trusted content adapter must establish that VersionID belongs to ChapterID
// from its locked content-owned version. Platform novel/hash proofs alone cannot
// establish that relationship when two chapters contain the same text.
// This port has no HTTP route and never starts an independent transaction.
func (s *Service) ApplyFacts(ctx context.Context, p scope.Principal, n string, source d.Source, in d.CheckFacts) (d.ApplyResult, error) {
	return s.CommitFacts(ctx, p, n, source, in)
}
func (s *Service) CommitFacts(ctx context.Context, p scope.Principal, n string, source d.Source, in d.CheckFacts) (d.ApplyResult, error) {
	out := d.ApplyResult{EventIDs: []string{}}
	if !uow.InTransaction(ctx) {
		return out, uow.ErrTransactionRequired
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := in.Validate(); err != nil {
			return err
		}
		if !scope.ValidID(source.ChapterID) || !scope.ValidID(source.VersionID) || source.Revision < 1 || source.Revision > d.MaxRevision || !regexp.MustCompile("^[0-9a-f]{64}$").MatchString(source.ContentHash) {
			return d.Malformed()
		}
		if _, err := s.Authorize(tx, p, "novel", n, true); err != nil {
			return err
		}
		chapter, err := s.o.Scope.Assert(tx, p, scope.Resource{Kind: "chapter", ID: source.ChapterID}, scope.Write)
		if err != nil {
			return err
		}
		if chapter.NovelID == nil || *chapter.NovelID != n {
			return d.Missing()
		}
		if _, err = s.o.Scope.AssertResourceRevision(tx, p, scope.Resource{Kind: "chapter", ID: source.ChapterID}, source.Revision, source.ContentHash); err != nil {
			return err
		}
		version, err := s.o.Scope.Assert(tx, p, scope.Resource{Kind: "chapter_version", ID: source.VersionID}, scope.Read)
		if err != nil {
			return err
		}
		if version.CurrentRevision != 1 || version.NovelID == nil || *version.NovelID != n || version.ContentHash == nil || *version.ContentHash != source.ContentHash {
			return d.Missing()
		}
		return s.change(tx, p, "novel", n, "worldview.facts_applied", func(state *d.State, c *d.Changes) error {
			if state.World.Revision != in.ExpectedWorldRevision {
				return d.FactConflict(state.World.Revision, nil)
			}
			if err := apply(state, c, p, in.FactProposals, &source); err != nil {
				return err
			}
			for _, e := range c.Events {
				out.EventIDs = append(out.EventIDs, e.ID)
			}
			out.WorldRevision = c.World.Revision
			return nil
		})
	})
	return out, err
}
func apply(state *d.State, c *d.Changes, p scope.Principal, proposals []d.FactProposal, source *d.Source) error {
	conflicts := d.Inspect(*state, proposals)
	if len(conflicts) > 0 {
		return d.FactConflict(state.World.Revision, conflicts)
	}
	if int64(len(proposals)) > d.MaxRevision-c.LastSeq {
		return d.Version(state.World.Revision)
	}
	affected := map[string]bool{}
	for _, v := range proposals {
		h := state.Head(v.SubjectID, v.Predicate, v.FactKey)
		if h != nil && h.Revision >= d.MaxRevision {
			return d.Version(h.Revision)
		}
		c.LastSeq++
		actor := p.ActorID
		e := d.FactEvent{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: state.World.NovelID, CreatedAt: c.World.UpdatedAt, UpdatedAt: c.World.UpdatedAt, Seq: c.LastSeq, WorldRevision: c.World.Revision, SubjectID: v.SubjectID, Predicate: v.Predicate, FactKey: v.FactKey, Value: v.Value, Action: v.Action, TargetEventID: v.TargetEventID, Resolution: v.Resolution, Reason: v.Reason, ActorID: &actor}
		if source != nil {
			e.SourceChapterID = &source.ChapterID
			e.SourceVersionID = &source.VersionID
			e.SourceContentHash = &source.ContentHash
		}
		c.Events = append(c.Events, e)
		if h == nil {
			state.Heads = append(state.Heads, d.FactHead{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: state.World.NovelID, CreatedAt: c.World.UpdatedAt, SubjectID: v.SubjectID, Predicate: v.Predicate, FactKey: v.FactKey})
			h = &state.Heads[len(state.Heads)-1]
		}
		h.Revision++
		h.UpdatedAt = c.World.UpdatedAt
		h.EventID = e.ID
		h.Value = v.Value
		h.Status = "active"
		if v.Action == "retract" {
			h.Status = "retracted"
			h.Value = nil
		}
		c.Heads = append(c.Heads, *h)
		if v.Predicate == "life_status" || v.Predicate == "realm_rank" {
			affected[v.SubjectID] = true
		}
	}
	for i := range state.Entities {
		e := &state.Entities[i]
		if !affected[e.ID] || e.Kind != "character" {
			continue
		}
		if e.Revision >= d.MaxRevision {
			return d.Version(e.Revision)
		}
		e.LifeStatus = "alive"
		e.RealmRank = 0
		for _, h := range state.Heads {
			if h.SubjectID != e.ID || h.Status != "active" || h.Value == nil {
				continue
			}
			if h.Predicate == "life_status" && h.Value.TextValue != nil {
				e.LifeStatus = *h.Value.TextValue
			}
			if h.Predicate == "realm_rank" && h.Value.NumberValue != nil {
				e.RealmRank = *h.Value.NumberValue
			}
		}
		e.Revision++
		e.UpdatedAt = c.World.UpdatedAt
		c.Entities = append(c.Entities, *e)
	}
	return nil
}
func (s *Service) RetractFact(ctx context.Context, p scope.Principal, id string, in d.RetractFact) (d.FactMutation, error) {
	if err := in.Validate(); err != nil {
		return d.FactMutation{}, err
	}
	return s.mutateFact(ctx, p, id, in.ExpectedRevision, in.ExpectedWorldRevision, nil, "correction", in.Reason)
}
func (s *Service) SupersedeFact(ctx context.Context, p scope.Principal, id string, in d.SupersedeFact) (d.FactMutation, error) {
	if err := in.Validate(); err != nil {
		return d.FactMutation{}, err
	}
	return s.mutateFact(ctx, p, id, in.ExpectedRevision, in.ExpectedWorldRevision, &in.Value, in.Resolution, in.Reason)
}
func (s *Service) mutateFact(ctx context.Context, p scope.Principal, id string, revision, world int64, value *d.FactValue, resolution, reason string) (d.FactMutation, error) {
	var out d.FactMutation
	err := s.change(ctx, p, "fact", id, "worldview.fact_corrected", func(state *d.State, c *d.Changes) error {
		if world != state.World.Revision {
			return d.FactConflict(state.World.Revision, nil)
		}
		var head *d.FactHead
		for i := range state.Heads {
			if state.Heads[i].ID == id {
				head = &state.Heads[i]
				break
			}
		}
		if head == nil {
			return d.Missing()
		}
		if err := expected(revision, head.Revision); err != nil {
			return err
		}
		action := "supersede"
		if value == nil {
			action = "retract"
		}
		target := head.EventID
		proposals := []d.FactProposal{{SubjectID: head.SubjectID, Predicate: head.Predicate, FactKey: head.FactKey, Action: action, Value: value, TargetEventID: &target, Resolution: resolution, Reason: &reason}}
		if err := apply(state, c, p, proposals, nil); err != nil {
			return err
		}
		out = d.FactMutation{Event: c.Events[0], Head: c.Heads[0], WorldRevision: c.World.Revision}
		return nil
	})
	return out, err
}

// ClearFacts is an explicit batch of retract proposals, never a ledger deletion.
func (s *Service) ClearFacts(ctx context.Context, p scope.Principal, n string, in d.CheckFacts) (d.ApplyResult, error) {
	out := d.ApplyResult{EventIDs: []string{}}
	if !uow.InTransaction(ctx) {
		return out, uow.ErrTransactionRequired
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := in.Validate(); err != nil {
			return err
		}
		for _, v := range in.FactProposals {
			if v.Action != "retract" {
				return d.Invalid()
			}
		}
		return s.change(tx, p, "novel", n, "worldview.facts_cleared", func(state *d.State, c *d.Changes) error {
			if in.ExpectedWorldRevision != state.World.Revision {
				return d.FactConflict(state.World.Revision, nil)
			}
			if err := apply(state, c, p, in.FactProposals, nil); err != nil {
				return err
			}
			for _, e := range c.Events {
				out.EventIDs = append(out.EventIDs, e.ID)
			}
			out.WorldRevision = c.World.Revision
			return nil
		})
	})
	return out, err
}
