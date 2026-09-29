package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/identity"
	"context"
)

func (s *Service) Statistics(ctx context.Context, p identity.Principal, novel, fromStr, toStr string) (out d.Statistics, err error) {
	fromDate, toInclusiveEnd, err := d.ValidateDateRange(fromStr, toStr)
	if err != nil {
		return out, err
	}
	fromUTC := fromDate.UTC()
	toUTC := toInclusiveEnd.UTC()

	err = s.read(ctx, p, "novel", novel, false, func(tx context.Context, o Owner) error {
		n, e := s.o.Repo.Novel(tx, o, novel, false)
		if e != nil {
			return e
		}
		aggs, e := s.o.Repo.WritingStatistics(tx, o, fromUTC, toUTC)
		if e != nil {
			return e
		}
		aggMap := make(map[string]WritingDayAgg, len(aggs))
		for _, a := range aggs {
			aggMap[a.Day] = a
		}

		var days []d.DailyStat
		for cur := fromDate; cur.Before(toInclusiveEnd); cur = cur.AddDate(0, 0, 1) {
			dayStr := cur.Format("2006-01-02")
			var added, removed int64
			var saveCount int
			if a, ok := aggMap[dayStr]; ok {
				added = a.AddedWords
				removed = a.RemovedWords
				saveCount = a.SaveCount
			}
			days = append(days, d.DailyStat{
				Date:         dayStr,
				AddedWords:   added,
				RemovedWords: removed,
				NetWords:     added - removed,
				SaveCount:    saveCount,
				Timezone:     "Asia/Shanghai",
			})
		}

		out = d.Statistics{
			NovelID:          novel,
			From:             fromStr,
			To:               toStr,
			Days:             days,
			CurrentWordCount: n.WordCount,
		}
		return nil
	})
	return
}
