package uow

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
)

var savepointSequence atomic.Uint64

// poisonFatal crosses recoverable boundaries. Ordinary callback failures stay
// local; connection/savepoint failures, cancellation and panic do not.
func poisonFatal(t *transaction, cause error) {
	for current := t; current != nil; current = current.parent {
		current.mu.Lock()
		current.cause = errors.Join(current.cause, cause)
		current.mu.Unlock()
	}
}

// Recoverable runs a synchronous command inside an existing transaction on the
// same connection. It is intended for deterministic command rejections whose
// result will be committed by the outer idempotency transaction. It is not a
// replacement for Within: only this explicit boundary isolates rollback-only.
//
// Ordinary errors, including nested Within failures, are rolled back to the
// savepoint and returned with their error chain intact. Infrastructure failures,
// context cancellation and panic poison every enclosing transaction. Callers
// must never retain the supplied context or use it concurrently.
func (m *Manager) Recoverable(ctx context.Context, fn func(context.Context) error) (err error) {
	db, err := m.Require(ctx)
	if err != nil {
		return err
	}
	if fn == nil {
		return errors.New("uow: callback required")
	}
	outer := state(ctx)
	if err = ctx.Err(); err != nil {
		poisonFatal(outer, err)
		return err
	}
	outer.mu.Lock()
	existingCause := outer.cause
	outer.mu.Unlock()
	if existingCause != nil {
		return errors.Join(ErrRollbackOnly, existingCause)
	}
	// Names contain only a fixed prefix and an internally generated integer.
	// No user input can become a SQL identifier.
	name := "uow_recoverable_" + strconv.FormatUint(savepointSequence.Add(1), 10)
	if err = db.Exec("SAVEPOINT " + name).Error; err != nil {
		poisonFatal(outer, err)
		return err
	}
	local := &transaction{owner: m, db: outer.db, parent: outer, readSnapshot: outer.readSnapshot, repeatableRead: outer.repeatableRead}
	localctx := context.WithValue(ctx, contextKey{}, local)
	defer func() {
		local.mu.Lock()
		local.closed = true
		local.mu.Unlock()
		if p := recover(); p != nil {
			// The root must roll back even if an application recovers this panic.
			poisonFatal(outer, ErrRollbackOnly)
			panic(p)
		}
	}()
	err = fn(localctx)
	local.mu.Lock()
	cause := local.cause
	local.mu.Unlock()
	if cause != nil {
		err = errors.Join(err, ErrRollbackOnly, cause)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		poisonFatal(outer, err)
	}
	if cancellation := ctx.Err(); cancellation != nil {
		err = errors.Join(err, cancellation)
		poisonFatal(outer, cancellation)
		// Cancellation can make SQL cleanup impossible. Root rollback owns cleanup.
		return err
	}
	if err != nil {
		if rollbackError := db.Exec("ROLLBACK TO SAVEPOINT " + name).Error; rollbackError != nil {
			poisonFatal(outer, rollbackError)
			return errors.Join(err, rollbackError)
		}
	}
	if releaseError := db.Exec("RELEASE SAVEPOINT " + name).Error; releaseError != nil {
		poisonFatal(outer, releaseError)
		return errors.Join(err, releaseError)
	}
	// A cancellation racing the final release still prohibits root commit.
	if cancellation := ctx.Err(); cancellation != nil {
		poisonFatal(outer, cancellation)
		return errors.Join(err, cancellation)
	}
	return err
}
