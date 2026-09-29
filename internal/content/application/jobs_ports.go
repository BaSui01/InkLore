package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/identity"
	"context"
	"time"
)

type JobRecord struct {
	d.Job
	AvailableAt                time.Time `json:"-"`
	LeaseToken                 *string   `json:"-"`
	IncludePlanning            bool      `json:"-"`
	IncludeWorldview           bool      `json:"-"`
	IncludeDrafts              bool      `json:"-"`
	VolumeIDs                  []string  `json:"-"`
	CommitVolumeID             *string   `json:"-"`
	CommitItemIDs              []string  `json:"-"`
	ExpectedNovelRevision      *int64    `json:"-"`
	PlanningSnapshotFileID     *string   `json:"-"`
	WorldviewSnapshotFileID    *string   `json:"-"`
	OrganizationSnapshotFileID *string   `json:"-"`
}

type SearchChapterRecord struct {
	ID               string
	Revision         int64
	Title            string
	Body             string
	CurrentVersionID string
	ContentHash      string
	UpdatedAt        time.Time
}

type WritingDayAgg struct {
	Day          string // YYYY-MM-DD
	AddedWords   int64
	RemovedWords int64
	SaveCount    int
}

type FileStorePort interface {
	GetFile(ctx context.Context, tenantID, fileID string) (filename, mediaType string, payload []byte, err error)
	PutFile(ctx context.Context, p identity.Principal, novelID *string, filename, mediaType, purpose string, payload []byte) (fileID string, err error)
	DeleteFile(ctx context.Context, tenantID, fileID string) error
}

type ExportReviewPort interface {
	ValidateReviewTicket(ctx context.Context, p identity.Principal, novelID string, reviewTicket *string, snapshots []d.JobSnapshot) error
}
