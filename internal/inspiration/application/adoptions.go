package application

import (
	d "InkLore/internal/inspiration/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) RecordAdoption(ctx context.Context, p scope.Principal, cardID string, in d.AdoptionCreate) (d.Adoption, error) {
	var out d.Adoption
	if !scope.ValidID(cardID) {
		return out, validationFailed("card_id")
	}
	if in.TargetDomain != d.AdoptionDomainContent && in.TargetDomain != d.AdoptionDomainOutline {
		return out, validationFailed("target_domain")
	}
	if !scope.ValidID(in.TargetID) {
		return out, validationFailed("target_id")
	}
	if in.TargetRevision < 1 {
		return out, validationFailed("target_revision")
	}
	if !d.ValidHexHash(in.TargetHash) {
		return out, validationFailed("target_hash")
	}

	// Validate card existence and ownership
	card, err := s.o.Repository.GetCard(ctx, p.TenantID, cardID, false)
	if err != nil || card.OwnerUserID != p.ActorID {
		return out, missing()
	}
	if card.State == d.CardStateDeleted {
		return out, stateError()
	}
	if err := cas(card.Revision, in.ExpectedRevision); err != nil {
		return out, err
	}

	// Verify target revision against platform scope projection
	if s.o.Scope != nil {
		_, err := s.o.Scope.AssertResourceRevision(ctx, p, scope.Resource{Kind: in.TargetDomain, ID: in.TargetID}, in.TargetRevision, in.TargetHash)
		if err != nil {
			return out, targetRevisionMismatch()
		}
	}

	now := s.now()
	adoptionID := uuid.NewString()
	adoption := d.Adoption{
		ID:             adoptionID,
		TenantID:       p.TenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
		CardID:         cardID,
		ActorUserID:    p.ActorID,
		TargetDomain:   in.TargetDomain,
		TargetID:       in.TargetID,
		TargetRevision: in.TargetRevision,
		TargetHash:     in.TargetHash,
		State:          d.AdoptionStateActive,
	}

	err = s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.o.Repository.CreateAdoption(tx, adoption, in.ExpectedRevision); err != nil {
			return err
		}
		out = adoption
		return s.event(tx, p, "inspiration.adoption_recorded", "inspiration_adoption", adoptionID, 1)
	})
	return out, err
}

func (s *Service) RevokeAdoption(ctx context.Context, p scope.Principal, adoptionID string, in d.ReasonCommand) (d.Adoption, error) {
	var out d.Adoption
	if !scope.ValidID(adoptionID) {
		return out, validationFailed("adoption_id")
	}
	reasonRunes := utf8.RuneCountInString(in.Reason)
	if reasonRunes < 1 || reasonRunes > 500 {
		return out, validationFailed("reason")
	}

	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetAdoption(tx, p.TenantID, adoptionID)
		if err != nil || curr.ActorUserID != p.ActorID {
			return missing()
		}
		if curr.State != d.AdoptionStateActive {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		now := s.now()
		updated, err := s.o.Repository.RevokeAdoption(tx, p.TenantID, adoptionID, in.ExpectedRevision, in.Reason, now)
		if err != nil {
			return err
		}
		out = updated
		return s.event(tx, p, "inspiration.adoption_revoked", "inspiration_adoption", adoptionID, updated.Revision)
	})
	return out, err
}

func (s *Service) ListAdoptions(ctx context.Context, p scope.Principal, cardID string, limit int, cursorStr string) (d.PageResult[d.Adoption], error) {
	if !scope.ValidID(cardID) {
		return d.PageResult[d.Adoption]{}, validationFailed("card_id")
	}
	card, err := s.o.Repository.GetCard(ctx, p.TenantID, cardID, true)
	if err != nil || card.OwnerUserID != p.ActorID {
		return d.PageResult[d.Adoption]{}, missing()
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" && s.o.Cursor != nil {
		var tc timeCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_adoptions", Filters: json.RawMessage(`{"card_id":"` + cardID + `"}`)}
		if err := s.o.Cursor.Decode(cursorStr, binding, &tc); err != nil {
			return d.PageResult[d.Adoption]{}, err
		}
		cursorTime = &tc.At
		cursorID = tc.ID
	}

	items, hasMore, err := s.o.Repository.ListAdoptions(ctx, p.TenantID, cardID, limit, cursorTime, cursorID)
	if err != nil {
		return d.PageResult[d.Adoption]{}, err
	}

	var nextCursor *string
	if hasMore && len(items) > 0 && s.o.Cursor != nil {
		last := items[len(items)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_adoptions", Filters: json.RawMessage(`{"card_id":"` + cardID + `"}`)}
		token, err := s.o.Cursor.Encode(binding, timeCursor{At: last.CreatedAt, ID: last.ID})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.Adoption]{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}
