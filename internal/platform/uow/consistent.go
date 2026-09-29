package uow

import (
	"context"
	"database/sql"
	"errors"
)

// WithinRepeatableRead supplies a consistent snapshot while permitting technical
// idempotency metadata. Domain preview callbacks must not mutate domain state.
func (m *Manager) WithinRepeatableRead(ctx context.Context, fn func(context.Context) error) error {
	if t := state(ctx); t != nil {
		if _, err := m.Require(ctx); err != nil {
			return err
		}
		if !t.repeatableRead {
			err := errors.New("uow: repeatable read requires its own outer boundary")
			poisonFatal(t, err)
			return err
		}
	}
	return m.within(ctx, fn, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
}
