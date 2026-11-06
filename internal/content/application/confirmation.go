package application

import (
	d "InkLore/internal/content/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/scope"
	"context"
)

func confirmValid(r Confirm) error {
	if r.ExpectedRevision < 1 || r.ExpectedRevision > d.MaxRevision || r.ExpectedWorldRevision < 1 || r.ExpectedWorldRevision > d.MaxRevision || len(r.FactProposals) > 100 {
		return invalid()
	}
	for _, p := range r.FactProposals {
		if !scope.ValidID(p.SubjectID) || p.Predicate == "" || p.FactKey == "" || p.Action == "" || p.Resolution == "" {
			return invalid()
		}
		if p.Action == "retract" {
			if p.Value != nil || p.TargetEventID == nil || p.Reason == nil {
				return invalid()
			}
		} else if p.Value == nil {
			return invalid()
		}
	}
	return nil
}
func (s *Service) ConfirmChapter(ctx context.Context, c Command, id string, r Confirm) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/chapters/"+id+"/confirm", r)
	if s.o.Facts == nil {
		return idempotency.Result{}, pe.Contract(500, "platform.internal_error", nil)
	}
	if e := confirmValid(r); e != nil {
		return idempotency.Result{}, e
	}
	return s.execute(ctx, c, "chapter", id, false, false, 200, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		ch, e := s.o.Repo.Chapter(tx, o, id, true)
		if e != nil {
			return nil, e
		}
		if ch.Status != "draft" || ch.CurrentVersionID == "" {
			return nil, d.State()
		}
		if e = d.CAS(ch.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		ver, e := s.o.Repo.Version(tx, o, ch.ID, ch.CurrentVersionID)
		if e != nil {
			return nil, e
		}
		if ver.ChapterID != ch.ID || ver.ContentHash != ch.ContentHash {
			return nil, d.Missing()
		}
		before, e := s.confirmedBody(tx, o, ch)
		if e != nil {
			return nil, e
		}
		result, e := s.o.Facts.CommitFacts(tx, Principal(c.Principal), o.NovelID, ConfirmationSource{ChapterID: ch.ID, VersionID: ver.ID, ContentHash: ver.ContentHash, Revision: ch.Revision}, r)
		if e != nil {
			return nil, e
		}
		ch.Status = "confirmed"
		ch.ConfirmedVersionID = &ver.ID
		if e = s.saveChapter(tx, o, &ch); e != nil {
			return nil, e
		}
		if e = s.recordConfirmedWriting(tx, o, &n, ch, ver.ID, "confirm", before, ver.Body); e != nil {
			return nil, e
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		if e = s.event(tx, c.Principal, "chapter.confirmed", "chapter", ch.ID, o.NovelID, ch.Revision); e != nil {
			return nil, e
		}
		return Confirmation{Chapter: ch.Chapter, WorldRevision: result.WorldRevision, EventIDs: result.EventIDs}, nil
	})
}
