package application

import (
	d "InkLore/internal/inspiration/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type timeCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

type seqCursor struct {
	Seq int `json:"seq"`
}

func (s *Service) CreateAsset(ctx context.Context, p scope.Principal, in d.AssetCreate) (d.Asset, error) {
	var out d.Asset
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 {
		return out, validationFailed("title")
	}
	if utf8.RuneCountInString(in.Author) > 120 {
		return out, validationFailed("author")
	}
	noteRunes := utf8.RuneCountInString(in.SourceNote)
	if noteRunes < 1 || noteRunes > 2000 {
		return out, validationFailed("source_note")
	}
	if in.SourceURL != nil && len(*in.SourceURL) > 2048 {
		return out, validationFailed("source_url")
	}
	if !d.ValidateRightsStatus(in.RightsStatus) {
		return out, validationFailed("rights_status")
	}

	var rawText string
	var fileID string

	switch in.SourceKind {
	case d.AssetSourceKindFile:
		if in.FileID == nil || !scope.ValidID(*in.FileID) || in.PasteText != nil {
			return out, invalidSource()
		}
		fileID = *in.FileID
		bytes, err := s.o.Files.ReadAuthorized(ctx, identity.Principal{
			TenantID:  p.TenantID,
			UserID:    p.ActorID,
			SessionID: p.SessionID,
			Audience:  p.Audience,
		}, fileID)
		if err != nil {
			return out, invalidSource()
		}
		if int64(len(bytes)) > 20<<20 {
			return out, invalidSource()
		}
		rawText = string(bytes)

	case d.AssetSourceKindPaste:
		if in.PasteText == nil || in.FileID != nil {
			return out, invalidSource()
		}
		if len(*in.PasteText) > 2<<20 {
			return out, invalidSource()
		}
		rawText = *in.PasteText

	default:
		return out, validationFailed("source_kind")
	}

	normalized, err := d.NormalizeText(rawText)
	if err != nil {
		return out, invalidSource()
	}
	count := d.CountCodepoints(normalized)
	if count < 1 {
		return out, invalidSource()
	}
	if count > d.MaxAssetCodepoints {
		return out, analysisTooLarge()
	}
	hash := d.ContentHash(normalized)

	// Deduplication check
	existingID, err := s.o.Repository.CheckAssetDuplicate(ctx, p.TenantID, p.ActorID, hash)
	if err != nil {
		return out, err
	}
	if existingID != nil {
		return out, assetDuplicate(*existingID)
	}

	// Store pasted text to platform_files
	if in.SourceKind == d.AssetSourceKindPaste {
		storedID, err := s.o.Files.StoreText(ctx, identity.Principal{
			TenantID:  p.TenantID,
			UserID:    p.ActorID,
			SessionID: p.SessionID,
			Audience:  p.Audience,
		}, in.Title+".txt", normalized, "inspiration_source")
		if err != nil {
			return out, err
		}
		fileID = storedID
	}

	spans, err := d.ChunkText(normalized)
	if err != nil {
		return out, invalidSource()
	}

	now := s.now()
	asset := d.Asset{
		ID:             uuid.NewString(),
		TenantID:       p.TenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
		OwnerUserID:    p.ActorID,
		FileID:         fileID,
		Title:          in.Title,
		SourceKind:     in.SourceKind,
		Author:         in.Author,
		SourceURL:      in.SourceURL,
		SourceNote:     in.SourceNote,
		RightsStatus:   in.RightsStatus,
		ContentHash:    hash,
		CodepointCount: count,
		State:          d.AssetStateReady,
	}

	err = s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.o.Repository.InsertAssetWithSegments(tx, asset, spans); err != nil {
			return err
		}
		return s.event(tx, p, "inspiration.asset_created", "inspiration_asset", asset.ID, 1)
	})
	if err != nil {
		return out, err
	}
	return asset, nil
}

func (s *Service) UpdateAsset(ctx context.Context, p scope.Principal, id string, in d.AssetUpdate) (d.Asset, error) {
	var out d.Asset
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 {
		return out, validationFailed("title")
	}
	if utf8.RuneCountInString(in.Author) > 120 {
		return out, validationFailed("author")
	}
	noteRunes := utf8.RuneCountInString(in.SourceNote)
	if noteRunes < 1 || noteRunes > 2000 {
		return out, validationFailed("source_note")
	}
	if in.SourceURL != nil && len(*in.SourceURL) > 2048 {
		return out, validationFailed("source_url")
	}
	if !d.ValidateRightsStatus(in.RightsStatus) {
		return out, validationFailed("rights_status")
	}

	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetAsset(tx, p.TenantID, id, false)
		if err != nil {
			return missing()
		}
		if curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.AssetStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		curr.Title = in.Title
		curr.Author = in.Author
		curr.SourceURL = in.SourceURL
		curr.SourceNote = in.SourceNote
		curr.RightsStatus = in.RightsStatus
		curr.Revision = curr.Revision + 1
		curr.UpdatedAt = s.now()

		if err := s.o.Repository.UpdateAsset(tx, curr, in.ExpectedRevision); err != nil {
			return err
		}
		out = curr
		return s.event(tx, p, "inspiration.asset_updated", "inspiration_asset", curr.ID, curr.Revision)
	})
	return out, err
}

func (s *Service) DeleteAsset(ctx context.Context, p scope.Principal, id string, in d.Expected) (d.Deleted, error) {
	var out d.Deleted
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetAsset(tx, p.TenantID, id, false)
		if err != nil {
			return missing()
		}
		if curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State == d.AssetStateDeleted {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		now := s.now()
		newRev, err := s.o.Repository.SoftDeleteAsset(tx, p.TenantID, id, in.ExpectedRevision, now)
		if err != nil {
			return err
		}
		out = d.Deleted{ID: id, Revision: newRev, Deleted: true}
		return s.event(tx, p, "inspiration.asset_deleted", "inspiration_asset", id, newRev)
	})
	return out, err
}

func (s *Service) RestoreAsset(ctx context.Context, p scope.Principal, id string, in d.Expected) (d.Asset, error) {
	var out d.Asset
	if !scope.ValidID(id) {
		return out, validationFailed("id")
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetAsset(tx, p.TenantID, id, true)
		if err != nil {
			return missing()
		}
		if curr.OwnerUserID != p.ActorID {
			return missing()
		}
		if curr.State != d.AssetStateDeleted || !d.Restorable(curr.DeletedAt, s.now()) {
			return stateError()
		}
		if err := cas(curr.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		// Check deduplication uniqueness before restoring
		existingID, err := s.o.Repository.CheckAssetDuplicate(tx, p.TenantID, p.ActorID, curr.ContentHash)
		if err != nil {
			return err
		}
		if existingID != nil && *existingID != id {
			return restoreConflict()
		}
		now := s.now()
		newRev, err := s.o.Repository.RestoreAsset(tx, p.TenantID, id, in.ExpectedRevision, now)
		if err != nil {
			return err
		}
		curr.State = d.AssetStateReady
		curr.DeletedAt = nil
		curr.Revision = newRev
		curr.UpdatedAt = now
		out = curr
		return s.event(tx, p, "inspiration.asset_restored", "inspiration_asset", id, newRev)
	})
	return out, err
}

func (s *Service) GetAsset(ctx context.Context, p scope.Principal, id string, includeDeleted bool) (d.Asset, error) {
	if !scope.ValidID(id) {
		return d.Asset{}, validationFailed("id")
	}
	asset, err := s.o.Repository.GetAsset(ctx, p.TenantID, id, includeDeleted)
	if err != nil {
		return d.Asset{}, missing()
	}
	if asset.OwnerUserID != p.ActorID {
		return d.Asset{}, missing()
	}
	if asset.State == d.AssetStateDeleted && !includeDeleted {
		return d.Asset{}, missing()
	}
	return asset, nil
}

func (s *Service) ListAssets(ctx context.Context, p scope.Principal, limit int, cursorStr string, includeDeleted bool) (d.PageResult[d.Asset], error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" && s.o.Cursor != nil {
		var tc timeCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_assets"}
		if err := s.o.Cursor.Decode(cursorStr, binding, &tc); err != nil {
			return d.PageResult[d.Asset]{}, err
		}
		cursorTime = &tc.At
		cursorID = tc.ID
	}

	items, hasMore, err := s.o.Repository.ListAssets(ctx, p.TenantID, p.ActorID, limit, cursorTime, cursorID, includeDeleted)
	if err != nil {
		return d.PageResult[d.Asset]{}, err
	}

	var nextCursor *string
	if hasMore && len(items) > 0 && s.o.Cursor != nil {
		last := items[len(items)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_assets"}
		token, err := s.o.Cursor.Encode(binding, timeCursor{At: last.UpdatedAt, ID: last.ID})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.Asset]{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *Service) ListSegments(ctx context.Context, p scope.Principal, assetID string, limit int, cursorStr string) (d.PageResult[d.Segment], error) {
	if !scope.ValidID(assetID) {
		return d.PageResult[d.Segment]{}, validationFailed("asset_id")
	}
	asset, err := s.o.Repository.GetAsset(ctx, p.TenantID, assetID, false)
	if err != nil || asset.OwnerUserID != p.ActorID {
		return d.PageResult[d.Segment]{}, missing()
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	cursorSeq := 0
	if cursorStr != "" && s.o.Cursor != nil {
		var sc seqCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_segments", Filters: json.RawMessage(`{"asset_id":"` + assetID + `"}`)}
		if err := s.o.Cursor.Decode(cursorStr, binding, &sc); err != nil {
			return d.PageResult[d.Segment]{}, err
		}
		cursorSeq = sc.Seq
	}

	segments, hasMore, err := s.o.Repository.GetSegments(ctx, p.TenantID, assetID, limit, cursorSeq)
	if err != nil {
		return d.PageResult[d.Segment]{}, err
	}

	// Read normalized text to populate segment slice texts
	var rawRunes []rune
	if asset.State != d.AssetStateDeleted && asset.RedactedAt == nil {
		bytes, err := s.o.Files.ReadAuthorized(ctx, identity.Principal{
			TenantID:  p.TenantID,
			UserID:    p.ActorID,
			SessionID: p.SessionID,
			Audience:  p.Audience,
		}, asset.FileID)
		if err == nil {
			normText, nErr := d.NormalizeText(string(bytes))
			if nErr == nil {
				rawRunes = []rune(normText)
			}
		}
	}

	for i := range segments {
		if len(rawRunes) >= segments[i].EndOffset && segments[i].StartOffset >= 0 {
			segments[i].Text = string(rawRunes[segments[i].StartOffset:segments[i].EndOffset])
		}
	}

	var nextCursor *string
	if hasMore && len(segments) > 0 && s.o.Cursor != nil {
		last := segments[len(segments)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_segments", Filters: json.RawMessage(`{"asset_id":"` + assetID + `"}`)}
		token, err := s.o.Cursor.Encode(binding, seqCursor{Seq: last.Seq})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.Segment]{Items: segments, NextCursor: nextCursor, HasMore: hasMore}, nil
}
