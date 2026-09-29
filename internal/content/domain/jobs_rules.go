package domain

import (
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/timeutil"
	"strings"
	"time"
	"unicode/utf8"
)

func CapacityExceeded() error    { return &RuleError{Code: "content.job_capacity_exceeded"} }
func ReviewRequired() error      { return &RuleError{Code: "content.review_required"} }
func ExportTooLarge() error      { return &RuleError{Code: "content.export_too_large"} }
func UnsupportedEncoding() error { return &RuleError{Code: "content.unsupported_encoding"} }
func UnsupportedDocument() error { return &RuleError{Code: "content.unsupported_document"} }

func ValidateCreateImport(r CreateImport) error {
	if r.ExpectedRevision < 1 || r.ExpectedRevision > MaxRevision {
		return Invalid()
	}
	if !scope.ValidID(r.FileID) {
		return Invalid()
	}
	if r.Format != JobFormatTXT && r.Format != JobFormatDOCX {
		return Invalid()
	}
	return nil
}

func ValidateCommitImport(r CommitImport) error {
	if r.ExpectedRevision < 1 || r.ExpectedRevision > MaxRevision || r.ExpectedNovelRevision < 1 || r.ExpectedNovelRevision > MaxRevision {
		return Invalid()
	}
	if !scope.ValidID(r.VolumeID) {
		return Invalid()
	}
	if len(r.SelectedItemIDs) < 1 || len(r.SelectedItemIDs) > 2000 {
		return Invalid()
	}
	seen := make(map[string]bool, len(r.SelectedItemIDs))
	for _, id := range r.SelectedItemIDs {
		if !scope.ValidID(id) || seen[id] {
			return Invalid()
		}
		seen[id] = true
	}
	return nil
}

func ValidateCreateExport(r CreateExport) error {
	if r.ExpectedRevision < 1 || r.ExpectedRevision > MaxRevision {
		return Invalid()
	}
	switch r.Format {
	case JobFormatTXT, JobFormatMarkdown, JobFormatDOCX, JobFormatEPUB:
	default:
		return Invalid()
	}
	if len(r.VolumeIDs) > 100 {
		return Invalid()
	}
	seen := make(map[string]bool, len(r.VolumeIDs))
	for _, id := range r.VolumeIDs {
		if !scope.ValidID(id) || seen[id] {
			return Invalid()
		}
		seen[id] = true
	}
	if r.ReviewTicket != nil && !scope.ValidID(*r.ReviewTicket) {
		return Invalid()
	}
	return nil
}

func ValidateCopyNovel(r CopyNovel) error {
	if r.ExpectedRevision < 1 || r.ExpectedRevision > MaxRevision {
		return Invalid()
	}
	if !Text(r.Title, 1, 120) {
		return Invalid()
	}
	return nil
}

func ValidateSearchQuery(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(trimmed)
	if n < 2 || n > 100 {
		return "", Invalid()
	}
	return trimmed, nil
}

func ValidateDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	loc := timeutil.BeijingLocation()
	fromDate, err := time.ParseInLocation("2006-01-02", fromStr, loc)
	if err != nil {
		return time.Time{}, time.Time{}, Invalid()
	}
	toDate, err := time.ParseInLocation("2006-01-02", toStr, loc)
	if err != nil {
		return time.Time{}, time.Time{}, Invalid()
	}
	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, Invalid()
	}
	days := int(toDate.Sub(fromDate).Hours()/24) + 1
	if days < 1 || days > 366 {
		return time.Time{}, time.Time{}, Invalid()
	}
	// toInclusiveEnd is next day at 00:00:00 CST
	toInclusiveEnd := toDate.AddDate(0, 0, 1)
	return fromDate, toInclusiveEnd, nil
}
