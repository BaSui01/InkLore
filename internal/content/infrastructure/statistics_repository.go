package infrastructure

import (
	a "InkLore/internal/content/application"
	"context"
	"time"
)

type writingDayRow struct {
	Day          string `gorm:"column:day"`
	AddedWords   int64  `gorm:"column:added_words"`
	RemovedWords int64  `gorm:"column:removed_words"`
	SaveCount    int    `gorm:"column:save_count"`
}

func (r *Repository) WritingStatistics(ctx context.Context, o a.Owner, fromTime, toTime time.Time) ([]a.WritingDayAgg, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	var rows []writingDayRow
	err := db.Raw(`SELECT
		(created_at AT TIME ZONE 'Asia/Shanghai')::date::text as day,
		COALESCE(SUM(added_words), 0) as added_words,
		COALESCE(SUM(removed_words), 0) as removed_words,
		COUNT(*) as save_count
	FROM content_writing_events
	WHERE tenant_id = ? AND novel_id = ? AND created_at >= ? AND created_at < ?
      AND source IN ('confirm', 'delete', 'restore')
	  AND EXISTS (SELECT 1 FROM content_novels n WHERE n.tenant_id = content_writing_events.tenant_id AND n.id = content_writing_events.novel_id AND n.owner_id = ?)
	GROUP BY day
	ORDER BY day ASC`, o.TenantID, o.NovelID, fromTime, toTime, o.ActorID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]a.WritingDayAgg, len(rows))
	for i, r := range rows {
		out[i] = a.WritingDayAgg{
			Day:          r.Day,
			AddedWords:   r.AddedWords,
			RemovedWords: r.RemovedWords,
			SaveCount:    r.SaveCount,
		}
	}
	return out, nil
}
