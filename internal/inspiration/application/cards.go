package application

import (
	d "InkLore/internal/inspiration/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/scope"
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) CreateCard(ctx context.Context, p scope.Principal, in d.CardCreate) (d.Card, error) {
	var out d.Card
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 {
		return out, validationFailed("title")
	}
	if !d.ValidateCardCategory(in.Category) {
		return out, validationFailed("category")
	}
	bodyRunes := utf8.RuneCountInString(in.Body)
	if bodyRunes < 1 || bodyRunes > d.MaxCardBodyRunes {
		return out, validationFailed("body")
	}
	if utf8.RuneCountInString(in.Rationale) > d.MaxCardRationaleRunes {
		return out, validationFailed("rationale")
	}
	if in.SourceKind != d.CardSourceKindOriginal && in.SourceKind != d.CardSourceKindReference {
		return out, validationFailed("source_kind")
	}
	if in.SourceReportID != nil && !scope.ValidID(*in.SourceReportID) {
		return out, validationFailed("source_report_id")
	}
	if in.NovelID != nil && !scope.ValidID(*in.NovelID) {
		return out, validationFailed("novel_id")
	}
	if len(in.Evidence) > 20 {
		return out, validationFailed("evidence")
	}

	fp := d.CardFingerprint(in.Category, in.Body)
	existingID, err := s.o.Repository.CheckCardDuplicate(ctx, p.TenantID, p.ActorID, in.NovelID, fp, "")
	if err != nil {
		return out, err
	}
	if existingID != nil {
		return out, cardDuplicate()
	}

	now := s.now()
	card := d.Card{
		ID:             uuid.NewString(),
		TenantID:       p.TenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
		OwnerUserID:    p.ActorID,
		NovelID:        in.NovelID,
		Title:          in.Title,
		Category:       in.Category,
		Body:           in.Body,
		Rationale:      in.Rationale,
		SourceKind:     in.SourceKind,
		SourceReportID: in.SourceReportID,
		Evidence:       in.Evidence,
		Fingerprint:    fp,
		State:          d.CardStateSuggested,
	}

	err = s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.o.Repository.CreateCard(tx, card); err != nil {
			return err
		}
		out = card
		return s.event(tx, p, "inspiration.card_created", "inspiration_card", card.ID, 1)
	})
	return out, err
}

func (s *Service) UpdateCard(ctx context.Context, p scope.Principal, id string, in d.CardUpdate) (d.Card, error) {
	var out d.Card
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 {
		return out, validationFailed("title")
	}
	if !d.ValidateCardCategory(in.Category) {
		return out, validationFailed("category")
	}
	bodyRunes := utf8.RuneCountInString(in.Body)
	if bodyRunes < 1 || bodyRunes > d.MaxCardBodyRunes {
		return out, validationFailed("body")
	}
	if utf8.RuneCountInString(in.Rationale) > d.MaxCardRationaleRunes {
		return out, validationFailed("rationale")
	}
	if in.SourceKind != d.CardSourceKindOriginal && in.SourceKind != d.CardSourceKindReference {
		return out, validationFailed("source_kind")
	}
	if in.SourceReportID != nil && !scope.ValidID(*in.SourceReportID) {
		return out, validationFailed("source_report_id")
	}
	if in.NovelID != nil && !scope.ValidID(*in.NovelID) {
		return out, validationFailed("novel_id")
	}
	if len(in.Evidence) > 20 {
		return out, validationFailed("evidence")
	}

	fp := d.CardFingerprint(in.Category, in.Body)
	existingID, err := s.o.Repository.CheckCardDuplicate(ctx, p.TenantID, p.ActorID, in.NovelID, fp, id)
	if err != nil {
		return out, err
	}
	if existingID != nil {
		return out, cardDuplicate()
	}

	err = s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetCard(tx, p.TenantID, id, false)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.CardStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		curr.NovelID = in.NovelID
		curr.Title = in.Title
		curr.Category = in.Category
		curr.Body = in.Body
		curr.Rationale = in.Rationale
		curr.SourceKind = in.SourceKind
		curr.SourceReportID = in.SourceReportID
		curr.Evidence = in.Evidence
		curr.Fingerprint = fp
		curr.Revision = curr.Revision + 1
		curr.UpdatedAt = s.now()

		if err := s.o.Repository.UpdateCard(tx, curr, in.ExpectedRevision); err != nil {
			return err
		}
		out = curr
		return s.event(tx, p, "inspiration.card_updated", "inspiration_card", id, curr.Revision)
	})
	return out, err
}

func (s *Service) SetCardState(ctx context.Context, p scope.Principal, id string, in d.CardStateCommand) (d.Card, error) {
	var out d.Card
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	if in.State != d.CardStateSuggested && in.State != d.CardStateArchived {
		return out, validationFailed("state")
	}

	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetCard(tx, p.TenantID, id, false)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.CardStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		updated, err := s.o.Repository.UpdateCardState(tx, p.TenantID, id, in.State, in.ExpectedRevision)
		if err != nil {
			return err
		}
		out = updated
		return s.event(tx, p, "inspiration.card_state_updated", "inspiration_card", id, updated.Revision)
	})
	return out, err
}

func (s *Service) DeleteCard(ctx context.Context, p scope.Principal, id string, in d.Expected) (d.Deleted, error) {
	var out d.Deleted
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetCard(tx, p.TenantID, id, false)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.CardStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		now := s.now()
		newRev, err := s.o.Repository.SoftDeleteCard(tx, p.TenantID, id, in.ExpectedRevision, now)
		if err != nil {
			return err
		}
		out = d.Deleted{ID: id, Revision: newRev, Deleted: true}
		return s.event(tx, p, "inspiration.card_deleted", "inspiration_card", id, newRev)
	})
	return out, err
}

func (s *Service) RestoreCard(ctx context.Context, p scope.Principal, id string, in d.Expected) (d.Card, error) {
	var out d.Card
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetCard(tx, p.TenantID, id, true)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State != d.CardStateDeleted || !d.Restorable(curr.DeletedAt, s.now()) {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		existingID, err := s.o.Repository.CheckCardDuplicate(tx, p.TenantID, p.ActorID, curr.NovelID, curr.Fingerprint, id)
		if err != nil {
			return err
		}
		if existingID != nil {
			return restoreConflict()
		}
		now := s.now()
		newRev, err := s.o.Repository.RestoreCard(tx, p.TenantID, id, in.ExpectedRevision, now)
		if err != nil {
			return err
		}
		curr.State = d.CardStateSuggested
		curr.DeletedAt = nil
		curr.Revision = newRev
		curr.UpdatedAt = now
		out = curr
		return s.event(tx, p, "inspiration.card_restored", "inspiration_card", id, newRev)
	})
	return out, err
}

func (s *Service) GetCard(ctx context.Context, p scope.Principal, id string, includeDeleted bool) (d.Card, error) {
	if !scope.ValidID(id) {
		return d.Card{}, validationFailed("id")
	}
	card, err := s.o.Repository.GetCard(ctx, p.TenantID, id, includeDeleted)
	if err != nil || card.OwnerUserID != p.ActorID {
		return d.Card{}, missing()
	}
	if card.State == d.CardStateDeleted && !includeDeleted {
		return d.Card{}, missing()
	}
	return card, nil
}

func (s *Service) ListCards(ctx context.Context, p scope.Principal, novelID *string, category *string, state *string, limit int, cursorStr string, includeDeleted bool) (d.PageResult[d.Card], error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" && s.o.Cursor != nil {
		var tc timeCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_cards"}
		if err := s.o.Cursor.Decode(cursorStr, binding, &tc); err != nil {
			return d.PageResult[d.Card]{}, err
		}
		cursorTime = &tc.At
		cursorID = tc.ID
	}

	items, hasMore, err := s.o.Repository.ListCards(ctx, p.TenantID, p.ActorID, novelID, category, state, limit, cursorTime, cursorID, includeDeleted)
	if err != nil {
		return d.PageResult[d.Card]{}, err
	}

	var nextCursor *string
	if hasMore && len(items) > 0 && s.o.Cursor != nil {
		last := items[len(items)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_cards"}
		token, err := s.o.Cursor.Encode(binding, timeCursor{At: last.UpdatedAt, ID: last.ID})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.Card]{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}
