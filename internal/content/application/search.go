package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/identity"
	"context"
)

func (s *Service) Search(ctx context.Context, p identity.Principal, novel string, rawQ string, q ListQuery) (out d.SearchPage, err error) {
	query, err := d.ValidateSearchQuery(rawQ)
	if err != nil {
		return out, err
	}
	params, b, e := s.pagination(p, "content_search", novel, q, "updated_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "novel", novel, false, func(tx context.Context, o Owner) error {
		records, e := s.o.Repo.SearchChapters(tx, o, query, params)
		if e != nil {
			return e
		}
		pageRecords, nextCursor, hasMore, e := finishPage(s, records, params, b, func(r SearchChapterRecord) Key {
			return Key{Time: r.UpdatedAt, ID: r.ID}
		})
		if e != nil {
			return e
		}
		hits := make([]d.SearchHit, len(pageRecords))
		for i, rec := range pageRecords {
			snippet, start, end := d.MatchSnippet(rec.Title, rec.Body, query)
			hits[i] = d.SearchHit{
				ChapterID:       rec.ID,
				VersionID:       rec.CurrentVersionID,
				Title:           rec.Title,
				ContentHash:     rec.ContentHash,
				ChapterRevision: rec.Revision,
				Snippet:         snippet,
				StartOffset:     start,
				EndOffset:       end,
			}
		}
		out.Items = hits
		out.NextCursor = nextCursor
		out.HasMore = hasMore
		return nil
	})
	return
}
