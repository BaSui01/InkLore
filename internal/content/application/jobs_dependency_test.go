package application

import (
	d "InkLore/internal/content/domain"
	pe "InkLore/internal/platform/errors"
	"context"
	"errors"
	"testing"
)

func TestFileJobsRejectMissingStorageBeforeReportingSuccess(t *testing.T) {
	service := &Service{}
	_, exportErr := service.CreateExport(context.Background(), Command{}, "", d.CreateExport{ExpectedRevision: 1, Format: d.JobFormatTXT})
	_, importErr := service.CreateImport(context.Background(), Command{}, "", d.CreateImport{ExpectedRevision: 1, Format: d.JobFormatTXT, FileID: "e4a53d31-62a4-4a53-a748-0f4792250001"})
	for name, err := range map[string]error{"export": exportErr, "import": importErr} {
		var appError *pe.AppError
		if !errors.As(err, &appError) || appError.Code != "platform.dependency_unavailable" {
			t.Fatalf("%s must fail when storage is missing, got %v", name, err)
		}
	}
}
