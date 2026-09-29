package adapters

import (
	content "InkLore/internal/content/application"
	"InkLore/internal/platform/scope"
	"context"
)

// ChapterContextAdapter adapts the content-owned metadata reader for outline.
type ChapterContextAdapter struct{ Content *content.Service }

func (a ChapterContextAdapter) ChapterVolume(ctx context.Context, p scope.Principal, novel, chapter string) (string, error) {
	return a.Content.ReadChapterVolume(ctx, p, novel, chapter)
}
