package application

import (
	"context"
	"time"
)

func (s *Service) PurgeSoftDeleted(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	cutoff := s.now().Add(-30 * 24 * time.Hour)
	var total int
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		n, err := s.o.Repository.PurgeSoftDeletedBatch(tx, limit, cutoff)
		if err != nil {
			return err
		}
		total = n
		return nil
	})
	return total, err
}
