package uow

import (
	"context"
	"database/sql"
	"errors"
)

// ReadSnapshot identifies a database-enforced read-only repeatable-read snapshot.
func ReadSnapshot(ctx context.Context) bool {
	t := state(ctx)
	return t != nil && t.readSnapshot && InTransaction(ctx)
}

// WithinReadSnapshot starts before authorization/query work. It cannot upgrade an
// existing read-committed transaction; callers must give it the outer boundary.
func (m *Manager) WithinReadSnapshot(ctx context.Context, fn func(context.Context) error) error {
	if t := state(ctx); t != nil {
		if _, err := m.Require(ctx); err != nil {
			return err
		}
		if !t.readSnapshot {
			err := errors.New("uow: read snapshot requires its own outer boundary")
			poisonFatal(t, err)
			return err
		}
	}
	return m.within(ctx, fn, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}
