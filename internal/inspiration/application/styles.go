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

func (s *Service) CreateStyle(ctx context.Context, p scope.Principal, in d.StyleCreate) (d.Style, error) {
	var out d.Style
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 {
		return out, validationFailed("title")
	}
	instRunes := utf8.RuneCountInString(in.Instructions)
	if instRunes < 1 || instRunes > 4000 {
		return out, validationFailed("instructions")
	}
	if in.Metrics.MeanSentenceLength < 0 || in.Metrics.DialogueRatio < 0 || in.Metrics.DialogueRatio > 1 || !d.ValidateSensoryBias(in.Metrics.SensoryBias) {
		return out, validationFailed("metrics")
	}
	if in.SourceReportID != nil && !scope.ValidID(*in.SourceReportID) {
		return out, validationFailed("source_report_id")
	}

	now := s.now()
	styleID := uuid.NewString()
	versionID := uuid.NewString()

	style := d.Style{
		ID:               styleID,
		TenantID:         p.TenantID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
		OwnerUserID:      p.ActorID,
		Title:            in.Title,
		CurrentVersionID: versionID,
		State:            d.StyleStateActive,
	}

	version := d.StyleVersion{
		ID:             versionID,
		TenantID:       p.TenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
		StyleID:        styleID,
		VersionNo:      1,
		Instructions:   in.Instructions,
		Metrics:        in.Metrics,
		SourceReportID: in.SourceReportID,
	}

	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.o.Repository.CreateStyleWithVersion(tx, style, version); err != nil {
			return err
		}
		out = style
		return s.event(tx, p, "inspiration.style_created", "inspiration_style", styleID, 1)
	})
	return out, err
}

func (s *Service) UpdateStyle(ctx context.Context, p scope.Principal, id string, in d.StyleUpdate) (d.Style, error) {
	var out d.Style
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 {
		return out, validationFailed("title")
	}
	instRunes := utf8.RuneCountInString(in.Instructions)
	if instRunes < 1 || instRunes > 4000 {
		return out, validationFailed("instructions")
	}
	if in.Metrics.MeanSentenceLength < 0 || in.Metrics.DialogueRatio < 0 || in.Metrics.DialogueRatio > 1 || !d.ValidateSensoryBias(in.Metrics.SensoryBias) {
		return out, validationFailed("metrics")
	}
	if in.SourceReportID != nil && !scope.ValidID(*in.SourceReportID) {
		return out, validationFailed("source_report_id")
	}

	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetStyle(tx, p.TenantID, id, false)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.StyleStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}

		newVersionID := uuid.NewString()
		now := s.now()
		version := d.StyleVersion{
			ID:             newVersionID,
			TenantID:       p.TenantID,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
			StyleID:        id,
			Instructions:   in.Instructions,
			Metrics:        in.Metrics,
			SourceReportID: in.SourceReportID,
		}

		updatedStyle, _, err := s.o.Repository.AddStyleVersion(tx, id, in.ExpectedRevision, version, &in.Title)
		if err != nil {
			return err
		}
		out = updatedStyle
		return s.event(tx, p, "inspiration.style_updated", "inspiration_style", id, updatedStyle.Revision)
	})
	return out, err
}

func (s *Service) DeleteStyle(ctx context.Context, p scope.Principal, id string, in d.Expected) (d.Deleted, error) {
	var out d.Deleted
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetStyle(tx, p.TenantID, id, false)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.StyleStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		now := s.now()
		newRev, err := s.o.Repository.SoftDeleteStyle(tx, p.TenantID, id, in.ExpectedRevision, now)
		if err != nil {
			return err
		}
		out = d.Deleted{ID: id, Revision: newRev, Deleted: true}
		return s.event(tx, p, "inspiration.style_deleted", "inspiration_style", id, newRev)
	})
	return out, err
}

func (s *Service) RestoreStyle(ctx context.Context, p scope.Principal, id string, in d.Expected) (d.Style, error) {
	var out d.Style
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetStyle(tx, p.TenantID, id, true)
		if err != nil || curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State != d.StyleStateDeleted || !d.Restorable(curr.DeletedAt, s.now()) {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		now := s.now()
		newRev, err := s.o.Repository.RestoreStyle(tx, p.TenantID, id, in.ExpectedRevision, now)
		if err != nil {
			return err
		}
		curr.State = d.StyleStateActive
		curr.DeletedAt = nil
		curr.Revision = newRev
		curr.UpdatedAt = now
		out = curr
		return s.event(tx, p, "inspiration.style_restored", "inspiration_style", id, newRev)
	})
	return out, err
}

func (s *Service) GetStyle(ctx context.Context, p scope.Principal, id string, includeDeleted bool) (d.Style, error) {
	if !scope.ValidID(id) {
		return d.Style{}, validationFailed("id")
	}
	style, err := s.o.Repository.GetStyle(ctx, p.TenantID, id, includeDeleted)
	if err != nil || style.OwnerUserID != p.ActorID {
		return d.Style{}, missing()
	}
	if style.State == d.StyleStateDeleted && !includeDeleted {
		return d.Style{}, missing()
	}
	return style, nil
}

func (s *Service) ListStyles(ctx context.Context, p scope.Principal, limit int, cursorStr string, includeDeleted bool) (d.PageResult[d.Style], error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" && s.o.Cursor != nil {
		var tc timeCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_styles"}
		if err := s.o.Cursor.Decode(cursorStr, binding, &tc); err != nil {
			return d.PageResult[d.Style]{}, err
		}
		cursorTime = &tc.At
		cursorID = tc.ID
	}

	items, hasMore, err := s.o.Repository.ListStyles(ctx, p.TenantID, p.ActorID, limit, cursorTime, cursorID, includeDeleted)
	if err != nil {
		return d.PageResult[d.Style]{}, err
	}

	var nextCursor *string
	if hasMore && len(items) > 0 && s.o.Cursor != nil {
		last := items[len(items)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_styles"}
		token, err := s.o.Cursor.Encode(binding, timeCursor{At: last.UpdatedAt, ID: last.ID})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.Style]{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *Service) ListStyleVersions(ctx context.Context, p scope.Principal, styleID string, limit int, cursorStr string) (d.PageResult[d.StyleVersion], error) {
	if !scope.ValidID(styleID) {
		return d.PageResult[d.StyleVersion]{}, validationFailed("style_id")
	}
	style, err := s.o.Repository.GetStyle(ctx, p.TenantID, styleID, false)
	if err != nil || style.OwnerUserID != p.ActorID {
		return d.PageResult[d.StyleVersion]{}, missing()
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" && s.o.Cursor != nil {
		var tc timeCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_style_versions", Filters: json.RawMessage(`{"style_id":"` + styleID + `"}`)}
		if err := s.o.Cursor.Decode(cursorStr, binding, &tc); err != nil {
			return d.PageResult[d.StyleVersion]{}, err
		}
		cursorTime = &tc.At
		cursorID = tc.ID
	}

	items, hasMore, err := s.o.Repository.ListStyleVersions(ctx, p.TenantID, styleID, limit, cursorTime, cursorID)
	if err != nil {
		return d.PageResult[d.StyleVersion]{}, err
	}

	var nextCursor *string
	if hasMore && len(items) > 0 && s.o.Cursor != nil {
		last := items[len(items)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_style_versions", Filters: json.RawMessage(`{"style_id":"` + styleID + `"}`)}
		token, err := s.o.Cursor.Encode(binding, timeCursor{At: last.CreatedAt, ID: last.ID})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.StyleVersion]{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *Service) GetStyleVersion(ctx context.Context, p scope.Principal, styleID, versionID string) (d.StyleVersion, error) {
	if !scope.ValidID(styleID) || !scope.ValidID(versionID) {
		return d.StyleVersion{}, validationFailed("id")
	}
	style, err := s.o.Repository.GetStyle(ctx, p.TenantID, styleID, false)
	if err != nil || style.OwnerUserID != p.ActorID {
		return d.StyleVersion{}, missing()
	}
	ver, err := s.o.Repository.GetStyleVersion(ctx, p.TenantID, styleID, versionID)
	if err != nil {
		return d.StyleVersion{}, missing()
	}
	return ver, nil
}
