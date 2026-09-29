// Package uow carries PostgreSQL transactions in opaque contexts.
// Application ports use Within only; DB is reserved for infrastructure.
package uow

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"gorm.io/gorm"
)

var (
	ErrTransactionRequired = errors.New("uow: active transaction required")
	ErrForeignTransaction  = errors.New("uow: transaction belongs to another manager")
	ErrClosedTransaction   = errors.New("uow: transaction context already closed")
	ErrRollbackOnly        = errors.New("uow: nested transaction failed")
)

type Manager struct{ db *gorm.DB }
type contextKey struct{}
type transaction struct {
	cause          error
	owner          *Manager
	db             *gorm.DB
	parent         *transaction // set only by a recoverable savepoint boundary
	mu             sync.Mutex
	closed         bool
	readSnapshot   bool // immutable: repeatable-read, database-enforced read-only
	repeatableRead bool
}

// CommitError means commit failed and its outcome must be reconciled by the
// original idempotency key. It never authorizes blindly repeating side effects.
type CommitError struct{ Cause error }

func (e *CommitError) Error() string { return "uow: commit outcome unknown" }
func (e *CommitError) Unwrap() error { return e.Cause }

func New(db *gorm.DB) (*Manager, error) {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil, errors.New("uow: PostgreSQL database required")
	}
	return &Manager{db: db}, nil
}
func state(ctx context.Context) *transaction {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(contextKey{}).(*transaction)
	return t
}
func InTransaction(ctx context.Context) bool {
	t := state(ctx)
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.closed
}
func (m *Manager) DB(ctx context.Context) (*gorm.DB, error) {
	if ctx == nil {
		return nil, errors.New("uow: nil context")
	}
	if t := state(ctx); t != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.owner != m {
			return nil, ErrForeignTransaction
		}
		if t.closed {
			return nil, ErrClosedTransaction
		}
		return t.db.WithContext(ctx), nil
	}
	return m.db.WithContext(ctx), nil
}
func (m *Manager) Require(ctx context.Context) (*gorm.DB, error) {
	if !InTransaction(ctx) {
		return nil, ErrTransactionRequired
	}
	return m.DB(ctx)
}

// Within shares nested transactions without savepoints. A nested failure or
// panic poisons the root even when its caller ignores the returned error.
// Callbacks must be synchronous and must not retain txctx after returning.
func (m *Manager) Within(ctx context.Context, fn func(context.Context) error) error {
	return m.within(ctx, fn, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}

func (m *Manager) within(ctx context.Context, fn func(context.Context) error, options *sql.TxOptions) (err error) {
	if ctx == nil || fn == nil {
		return errors.New("uow: context and callback required")
	}
	if t := state(ctx); t != nil {
		if _, err = m.Require(ctx); err != nil {
			return err
		}
		defer func() {
			if p := recover(); p != nil {
				poisonFatal(t, ErrRollbackOnly)
				panic(p)
			}
			if cancellation := ctx.Err(); cancellation != nil {
				poisonFatal(t, cancellation)
				if err == nil {
					err = cancellation
				}
			}
			if err != nil {
				t.mu.Lock()
				if t.cause == nil {
					t.cause = err
				}
				t.mu.Unlock()
			}
		}()
		if err = ctx.Err(); err != nil {
			return err
		}
		return fn(ctx)
	}
	tx := m.db.WithContext(ctx).Begin(options)
	if tx.Error != nil {
		return tx.Error
	}
	t := &transaction{owner: m, db: tx, readSnapshot: options.ReadOnly && options.Isolation == sql.LevelRepeatableRead, repeatableRead: options.Isolation == sql.LevelRepeatableRead}
	txctx := context.WithValue(ctx, contextKey{}, t)
	defer func() {
		t.mu.Lock()
		t.closed = true
		t.mu.Unlock()
		if p := recover(); p != nil {
			_ = tx.Rollback().Error
			panic(p)
		}
	}()
	err = fn(txctx)
	t.mu.Lock()
	cause := t.cause
	t.mu.Unlock()
	if err == nil && cause != nil {
		err = errors.Join(ErrRollbackOnly, cause)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		if rb := tx.Rollback().Error; rb != nil {
			err = errors.Join(err, rb)
		}
		return err
	}
	if err = tx.Commit().Error; err != nil {
		return &CommitError{Cause: err}
	}
	return nil
}
