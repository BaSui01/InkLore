package infrastructure

import (
	a "InkLore/internal/content/application"
	"context"
	"strings"
	"time"
)

type searchChapterModel struct {
	ID               string    `gorm:"column:id"`
	Revision         int64     `gorm:"column:revision"`
	Title            string    `gorm:"column:title"`
	Body             string    `gorm:"column:body"`
	CurrentVersionID string    `gorm:"column:current_version_id"`
	ContentHash      string    `gorm:"column:content_hash"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func escapeLikePattern(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '%':
			b.WriteString("\\%")
		case '_':
			b.WriteString("\\_")
		default:
			b.WriteRune(r)
		}
	}
	return "%" + b.String() + "%"
}

func (r *Repository) SearchChapters(ctx context.Context, o a.Owner, query string, p a.Query) ([]a.SearchChapterRecord, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	pattern := escapeLikePattern(query)
	q := db.Table("content_chapters").
		Select("id, revision, title, body, current_version_id, content_hash, updated_at").
		Where("content_chapters.tenant_id=? AND content_chapters.novel_id=? AND content_chapters.deleted_at IS NULL", o.TenantID, o.NovelID).
		Where("EXISTS (SELECT 1 FROM content_novels n WHERE n.tenant_id=content_chapters.tenant_id AND n.id=content_chapters.novel_id AND n.owner_id=?)", o.ActorID).
		Where("(title ILIKE ? ESCAPE '\\' OR body ILIKE ? ESCAPE '\\')", pattern, pattern)

	if p.After != nil {
		q = q.Where("(updated_at,id) < (?,?::uuid)", p.After.Time, p.After.ID)
	}

	var models []searchChapterModel
	err := q.Order("updated_at DESC,id DESC").Limit(p.Limit + 1).Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]a.SearchChapterRecord, len(models))
	for i, m := range models {
		out[i] = a.SearchChapterRecord{
			ID:               m.ID,
			Revision:         m.Revision,
			Title:            m.Title,
			Body:             m.Body,
			CurrentVersionID: m.CurrentVersionID,
			ContentHash:      m.ContentHash,
			UpdatedAt:        m.UpdatedAt,
		}
	}
	return out, nil
}
