// package domain defines the persistence contract for inspiration aggregates and entities.
package domain

import (
	"context"
	"time"
)

type Repository interface {
	// Asset operations
	InsertAssetWithSegments(ctx context.Context, asset Asset, spans []SegmentSpan) error
	UpdateAsset(ctx context.Context, asset Asset, expectedRevision int64) error
	SoftDeleteAsset(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error)
	RestoreAsset(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error)
	GetAsset(ctx context.Context, tenantID, id string, includeDeleted bool) (Asset, error)
	CheckAssetDuplicate(ctx context.Context, tenantID, ownerUserID, contentHash string) (*string, error)
	ListAssets(ctx context.Context, tenantID, ownerUserID string, limit int, cursorTime *time.Time, cursorID string, includeDeleted bool) ([]Asset, bool, error)
	GetSegments(ctx context.Context, tenantID, assetID string, limit int, cursorSeq int) ([]Segment, bool, error)

	// Analysis operations
	CreateAnalysis(ctx context.Context, a Analysis) error
	GetAnalysis(ctx context.Context, tenantID, id string) (Analysis, error)
	ListAnalyses(ctx context.Context, tenantID, ownerUserID string, novelID *string, state *string, limit int, cursorTime *time.Time, cursorID string) ([]Analysis, bool, error)
	CountInFlightAnalyses(ctx context.Context, tenantID, ownerUserID string) (tenantCount int, authorCount int, err error)
	ClaimAnalysis(ctx context.Context, tenantID, id string, runID, executionKey string, fence int64, expectedRevision int64) (Analysis, error)
	CancelAnalysis(ctx context.Context, tenantID, id string, expectedRevision int64, reason string) error
	FailAnalysis(ctx context.Context, tenantID, id string, fence int64, failureCode string, usedTokens, callsUsed int) error
	CompleteAnalysisWithReport(ctx context.Context, tenantID, id string, fence int64, report Report, usedTokens, callsUsed int) error
	GetReportByAnalysisID(ctx context.Context, tenantID, analysisID string) (Report, error)

	// Style operations
	CreateStyleWithVersion(ctx context.Context, s Style, v StyleVersion) error
	AddStyleVersion(ctx context.Context, styleID string, expectedRevision int64, v StyleVersion, title *string) (Style, StyleVersion, error)
	SoftDeleteStyle(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error)
	RestoreStyle(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error)
	GetStyle(ctx context.Context, tenantID, id string, includeDeleted bool) (Style, error)
	ListStyles(ctx context.Context, tenantID, ownerUserID string, limit int, cursorTime *time.Time, cursorID string, includeDeleted bool) ([]Style, bool, error)
	ListStyleVersions(ctx context.Context, tenantID, styleID string, limit int, cursorTime *time.Time, cursorID string) ([]StyleVersion, bool, error)
	GetStyleVersion(ctx context.Context, tenantID, styleID, versionID string) (StyleVersion, error)

	// Card operations
	CreateCard(ctx context.Context, c Card) error
	UpdateCard(ctx context.Context, c Card, expectedRevision int64) error
	UpdateCardState(ctx context.Context, tenantID, id string, state string, expectedRevision int64) (Card, error)
	SoftDeleteCard(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error)
	RestoreCard(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error)
	GetCard(ctx context.Context, tenantID, id string, includeDeleted bool) (Card, error)
	CheckCardDuplicate(ctx context.Context, tenantID, ownerUserID string, novelID *string, fingerprint string, excludeID string) (*string, error)
	ListCards(ctx context.Context, tenantID, ownerUserID string, novelID *string, category *string, state *string, limit int, cursorTime *time.Time, cursorID string, includeDeleted bool) ([]Card, bool, error)

	// Adoption operations
	CreateAdoption(ctx context.Context, a Adoption, cardExpectedRevision int64) error
	RevokeAdoption(ctx context.Context, tenantID, id string, expectedRevision int64, reason string, now time.Time) (Adoption, error)
	GetAdoption(ctx context.Context, tenantID, id string) (Adoption, error)
	ListAdoptions(ctx context.Context, tenantID, cardID string, limit int, cursorTime *time.Time, cursorID string) ([]Adoption, bool, error)

	// Retention / Redaction
	PurgeSoftDeletedBatch(ctx context.Context, limit int, cutoff time.Time) (int, error)
}
