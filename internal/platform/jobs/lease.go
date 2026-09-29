package jobs

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"context"
	"time"
)

// Claim recovers expired leases without counting recovery or approval wake as a
// technical retry. Handlers must reconcile durable receipts before any external call.
func (s *Service) Claim(ctx context.Context, workerID, domain string) (Lease, error) {
	return s.claim(ctx, workerID, domain, nil)
}
func (s *Service) claim(ctx context.Context, workerID, domain string, kinds []string) (Lease, error) {
	var lease Lease
	if !scope.ValidID(workerID) || !validDomain(domain) {
		return lease, invalid()
	}
	if s.draining.Load() {
		return lease, ErrDraining
	}
	e := s.manager.Within(ctx, func(tx context.Context) error {
		if s.draining.Load() {
			return ErrDraining
		}
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var j Job
		filter := ""
		args := []any{domain, MaxRevision}
		if len(kinds) > 0 {
			filter = " AND job_kind IN ?"
			args = append(args, kinds)
		}
		q := db.Raw("SELECT * FROM platform_jobs WHERE domain=? AND revision<?"+filter+" AND ((state IN ('queued','retry_wait') AND available_at<=clock_timestamp()) OR (state='leased' AND lease_until<=clock_timestamp())) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1", args...).Scan(&j)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return ErrNoJob
		}
		recovered := j.State == "leased"
		q = db.Raw("UPDATE platform_jobs SET state='leased',lease_owner=?,lease_epoch=lease_epoch+1,lease_until=clock_timestamp()+interval '30 seconds',attempt=CASE WHEN attempt=0 THEN 1 ELSE attempt END,revision=revision+CASE WHEN state='leased' THEN 0 ELSE 1 END,updated_at=clock_timestamp() WHERE id=? AND tenant_id=? AND lease_epoch<9223372036854775807 RETURNING *", workerID, j.ID, j.TenantID).Scan(&j)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return ErrLeaseLost
		}
		lease = Lease{s, j.TenantID, j.ID, workerID, j.LeaseEpoch, j, recovered}
		return nil
	})
	if e != nil {
		return Lease{}, e
	}
	return lease, nil
}

// AssertLease is the mandatory first lock in a worker checkpoint transaction.
func (s *Service) AssertLease(ctx context.Context, l Lease) (Job, error) {
	var j Job
	if l.service != s || l.id == "" {
		return j, ErrLeaseLost
	}
	db, e := s.manager.Require(ctx)
	if e != nil {
		return j, e
	}
	q := db.Raw("SELECT * FROM platform_jobs WHERE tenant_id=? AND id=? AND state='leased' AND lease_owner=? AND lease_epoch=? AND lease_until>clock_timestamp() FOR UPDATE", l.tenant, l.id, l.owner, l.epoch).Scan(&j)
	if q.Error != nil {
		return j, q.Error
	}
	if q.RowsAffected != 1 {
		return j, ErrLeaseLost
	}
	return j, nil
}
func (s *Service) Checkpoint(ctx context.Context, l Lease, fn func(context.Context, Job) error) error {
	if fn == nil {
		return invalid()
	}
	return s.manager.Within(ctx, func(tx context.Context) error {
		j, e := s.AssertLease(tx, l)
		if e != nil {
			return e
		}
		if e = fn(tx, j); e != nil {
			return e
		}
		// A long checkpoint must not commit after its original lease deadline,
		// even if it already changed the job to a terminal state locally.
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var live bool
		if e = db.Raw("SELECT clock_timestamp() < ?::timestamptz", j.LeaseUntil).Scan(&live).Error; e != nil {
			return e
		}
		if !live {
			return ErrLeaseLost
		}
		return nil
	})
}
func (s *Service) Heartbeat(ctx context.Context, l Lease) error {
	if l.service != s {
		return ErrLeaseLost
	}
	db, e := s.manager.DB(ctx)
	if e != nil {
		return e
	}
	q := db.Exec("UPDATE platform_jobs SET lease_until=clock_timestamp()+interval '30 seconds' WHERE tenant_id=? AND id=? AND state='leased' AND lease_owner=? AND lease_epoch=? AND lease_until>clock_timestamp()", l.tenant, l.id, l.owner, l.epoch)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return ErrLeaseLost
	}
	return nil
}
func (s *Service) Suspend(ctx context.Context, l Lease, wakeAt time.Time) error {
	if wakeAt.IsZero() {
		return invalid()
	}
	return s.Checkpoint(ctx, l, func(tx context.Context, j Job) error {
		if j.Revision >= MaxRevision {
			return invalid()
		}
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		return db.Exec("UPDATE platform_jobs SET state='retry_wait',available_at=?,lease_owner=NULL,lease_until=NULL,revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND id=?", wakeAt.UTC(), j.TenantID, j.ID).Error
	})
}

// Wake requires the authorized, locked command capability. A current worker
// keeps its lease; otherwise the same job is queued without increasing attempt.
func (s *Service) Wake(ctx context.Context, p AuthorizedCommand) error {
	if p.service != s || p.ctx != ctx {
		return ErrLeaseLost
	}
	db, e := s.manager.Require(ctx)
	if e != nil {
		return e
	}
	var j Job
	q := db.Raw("SELECT * FROM platform_jobs WHERE tenant_id=? AND id=? FOR UPDATE", p.job.TenantID, p.job.ID).Scan(&j)
	if q.Error != nil {
		return q.Error
	}
	if terminal(j.State) {
		return ErrTerminal
	}
	if j.Revision >= MaxRevision {
		return invalid()
	}
	return db.Exec("UPDATE platform_jobs SET state='queued',available_at=clock_timestamp(),lease_owner=NULL,lease_until=NULL,revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND id=? AND (state IN ('queued','retry_wait') OR (state='leased' AND lease_until<=clock_timestamp()))", j.TenantID, j.ID).Error
}
func (s *Service) Complete(ctx context.Context, l Lease, state, code string) (Job, error) {
	var out Job
	e := s.Checkpoint(ctx, l, func(tx context.Context, j Job) error { var e error; out, e = s.finish(tx, j, state, code); return e })
	return out, e
}
func (s *Service) CompleteCommand(ctx context.Context, p AuthorizedCommand, state, code string) (Job, error) {
	if p.service != s || p.ctx != ctx {
		return Job{}, ErrLeaseLost
	}
	if _, e := s.manager.Require(ctx); e != nil {
		return Job{}, e
	}
	return s.finish(ctx, p.job, state, code)
}
func (s *Service) finish(ctx context.Context, j Job, state, code string) (Job, error) {
	if !terminal(state) || !safeCode(code) {
		return Job{}, invalid()
	}
	if terminal(j.State) {
		return Job{}, ErrTerminal
	}
	if j.Revision >= MaxRevision {
		return Job{}, invalid()
	}
	db, e := s.manager.Require(ctx)
	if e != nil {
		return Job{}, e
	}
	var out Job
	q := db.Raw("UPDATE platform_jobs SET state=?,last_error_code=NULLIF(?,''),lease_owner=NULL,lease_until=NULL,revision=revision+1,updated_at=clock_timestamp(),cancel_requested_at=CASE WHEN ?='cancelled' THEN COALESCE(cancel_requested_at,clock_timestamp()) ELSE cancel_requested_at END WHERE tenant_id=? AND id=? AND revision=? AND state NOT IN ('succeeded','failed','cancelled') RETURNING *", state, code, state, j.TenantID, j.ID, j.Revision).Scan(&out)
	if q.Error != nil {
		return out, q.Error
	}
	if q.RowsAffected != 1 {
		return Job{}, ErrLeaseLost
	}
	if e = s.event(ctx, "job.terminal", out); e != nil {
		return Job{}, e
	}
	return out, nil
}
func safeCode(code string) bool {
	if code == "" {
		return true
	}
	for _, status := range []int{400, 401, 403, 404, 405, 409, 410, 413, 416, 422, 429, 500, 502, 503, 504} {
		if pe.IsContractStatus(code, status) {
			return true
		}
	}
	return false
}

// Retry is only for a known pre-dispatch failure or an explicitly approved
// read-only, unbilled operation. Unknown external outcomes must use Complete failed.
type RetrySafety uint8

const (
	BeforeExternalDispatch RetrySafety = iota + 1
	ReadOnlyUnbilled
)

func (s *Service) Retry(ctx context.Context, l Lease, code string, safety RetrySafety) error {
	if (safety != BeforeExternalDispatch && safety != ReadOnlyUnbilled) || !safeCode(code) {
		return invalid()
	}
	return s.Checkpoint(ctx, l, func(tx context.Context, j Job) error {
		if j.Attempt >= 4 {
			_, e := s.finish(tx, j, "failed", code)
			return e
		}
		if j.Revision >= MaxRevision {
			return invalid()
		}
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		delay := 1 << uint(j.Attempt-1)
		return db.Exec("UPDATE platform_jobs SET state='retry_wait',attempt=attempt+1,available_at=clock_timestamp()+(? * interval '1 second'),last_error_code=NULLIF(?,''),lease_owner=NULL,lease_until=NULL,revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND id=?", delay, code, j.TenantID, j.ID).Error
	})
}

// Release is reserved for safe pre-dispatch handoff during drain. It invalidates
// the old lease immediately and never records task success.
func (s *Service) Release(ctx context.Context, l Lease) error {
	return s.Suspend(ctx, l, time.Now().UTC())
}
