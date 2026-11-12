package adapters

import (
	compapp "InkLore/internal/compliance/application"
	contentapp "InkLore/internal/content/application"
	contentdomain "InkLore/internal/content/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/files"
	"InkLore/internal/platform/identity"
	"bytes"
	"context"
	"io"
)

type ContentFiles struct{ Service *files.Service }

var _ contentapp.FileStorePort = ContentFiles{}

func (a ContentFiles) GetFile(ctx context.Context, p identity.Principal, novelID, fileID string) (string, string, []byte, error) {
	// 导入与引用绑定共用事务，防止过期清理或其他作品抢先占用文件。
	if _, err := a.Service.Bind(ctx, p, fileID, "import", &novelID); err != nil {
		return "", "", nil, err
	}
	opened, err := a.Service.Open(ctx, p, fileID)
	if err != nil {
		return "", "", nil, err
	}
	payload, err := io.ReadAll(opened.Reader)
	return opened.Descriptor.Filename, opened.Descriptor.MediaType, payload, err
}

func (a ContentFiles) PutFile(ctx context.Context, p identity.Principal, novelID *string, filename, mediaType, purpose string, payload []byte) (string, error) {
	prepared, err := files.PrepareOutput(bytes.NewReader(payload), filename, mediaType, purpose, novelID)
	if err != nil {
		return "", err
	}
	descriptor, err := a.Service.Put(ctx, p, prepared)
	return descriptor.ID, err
}

type ContentExportReviews struct{ Compliance *compapp.Service }

var _ contentapp.ExportReviewPort = (*ContentExportReviews)(nil)

func (a *ContentExportReviews) ValidateReviewTicket(ctx context.Context, p identity.Principal, novelID string, ticket *string, snapshots []contentdomain.JobSnapshot) error {
	if a.Compliance == nil {
		return pe.Contract(503, "platform.dependency_unavailable", nil)
	}
	if ticket == nil {
		return nil
	}
	inputs := make([]compapp.ExportSnapshot, len(snapshots))
	for i, snapshot := range snapshots {
		inputs[i] = compapp.ExportSnapshot{ChapterID: snapshot.ChapterID, ChapterRevision: snapshot.ChapterRevision, ContentHash: snapshot.ContentHash}
	}
	_, err := a.Compliance.ValidateReviewReceipts(ctx, p.TenantID, p.UserID, inputs, []string{*ticket})
	return err
}
