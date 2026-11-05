package infrastructure

import (
	"context"
	"errors"
	"sync"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/jobs"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
)

// JobsAdapter bridges the platform jobs service to the agent domain JobsPort.
type JobsAdapter struct {
	jobsService *jobs.Service
	manager     *uow.Manager
	mu          sync.RWMutex
	leases      map[string]jobs.Lease // jobID -> platform Lease
}

// NewJobsAdapter constructs a JobsAdapter.
func NewJobsAdapter(jobsService *jobs.Service, manager *uow.Manager) (*JobsAdapter, error) {
	if manager == nil {
		return nil, errors.New("jobs adapter: uow manager required")
	}
	return &JobsAdapter{
		jobsService: jobsService,
		manager:     manager,
		leases:      make(map[string]jobs.Lease),
	}, nil
}

func (a *JobsAdapter) EnqueueRunJob(ctx context.Context, p identity.Principal, runID string) (string, error) {
	if a.jobsService != nil {
		in := jobs.EnqueueInput{
			Principal:    p,
			Domain:       "agent",
			Kind:         "run",
			ResourceID:   runID,
			ExecutionKey: runID,
		}
		job, err := a.jobsService.Enqueue(ctx, in)
		if err != nil {
			return "", err
		}
		return job.ID, nil
	}

	// Direct DB fallback within the current transaction
	db, err := a.manager.Require(ctx)
	if err != nil {
		return "", err
	}
	var jobID string
	err = db.Raw(`
		INSERT INTO platform_jobs (
			tenant_id, owner_id, domain, job_kind, resource_id, execution_key, state, payload
		) VALUES (
			?, ?, 'agent', 'run', ?, ?, 'queued', '{}'::jsonb
		)
		RETURNING id
	`, p.TenantID, p.UserID, runID, runID).Scan(&jobID).Error
	return jobID, err
}

func (a *JobsAdapter) ClaimRunJob(ctx context.Context, workerID string) (*d.RunJobLease, error) {
	if a.jobsService != nil {
		l, err := a.jobsService.Claim(ctx, workerID, "agent")
		if errors.Is(err, jobs.ErrNoJob) {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		a.mu.Lock()
		a.leases[l.Job().ID] = l
		a.mu.Unlock()

		return &d.RunJobLease{
			TenantID:   l.Job().TenantID,
			JobID:      l.Job().ID,
			WorkerID:   workerID,
			LeaseEpoch: l.Epoch(),
			RunID:      l.Job().ResourceID,
			Recovered:  l.Recovered(),
		}, nil
	}

	if !scope.ValidID(workerID) {
		return nil, errors.New("invalid worker id")
	}
	var row platformJobRow
	err := a.manager.Within(ctx, func(tx context.Context) error {
		db, err := a.manager.Require(tx)
		if err != nil {
			return err
		}
		query := db.Raw(`
			SELECT id, tenant_id, resource_id, state, lease_epoch
			FROM platform_jobs
			WHERE domain = 'agent' AND job_kind = 'run' AND revision < ?
			  AND ((state IN ('queued', 'retry_wait') AND available_at <= clock_timestamp())
			    OR (state = 'leased' AND lease_until <= clock_timestamp()))
			ORDER BY available_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`, jobs.MaxRevision).Scan(&row)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected != 1 {
			return jobs.ErrNoJob
		}
		recovered := row.State == "leased"
		updated := db.Raw(`
			UPDATE platform_jobs
			SET state = 'leased', lease_owner = ?, lease_epoch = lease_epoch + 1,
			    lease_until = clock_timestamp() + interval '30 seconds',
			    attempt = CASE WHEN attempt = 0 THEN 1 ELSE attempt END,
			    revision = revision + CASE WHEN state = 'leased' THEN 0 ELSE 1 END,
			    updated_at = clock_timestamp()
			WHERE id = ? AND tenant_id = ? AND lease_epoch < 9223372036854775807
			RETURNING id, tenant_id, resource_id, state, lease_epoch
		`, workerID, row.ID, row.TenantID).Scan(&row)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return jobs.ErrLeaseLost
		}
		row.Recovered = recovered
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &d.RunJobLease{
		TenantID:   row.TenantID,
		JobID:      row.ID,
		WorkerID:   workerID,
		LeaseEpoch: row.LeaseEpoch,
		RunID:      row.ResourceID,
		Recovered:  row.Recovered,
	}, nil
}

type platformJobRow struct {
	ID         string `gorm:"column:id"`
	TenantID   string `gorm:"column:tenant_id"`
	ResourceID string `gorm:"column:resource_id"`
	State      string `gorm:"column:state"`
	LeaseEpoch int64  `gorm:"column:lease_epoch"`
	Recovered  bool
}

func (a *JobsAdapter) HeartbeatRunJob(ctx context.Context, lease *d.RunJobLease) error {
	if lease == nil {
		return nil
	}
	if a.jobsService != nil {
		a.mu.RLock()
		l, ok := a.leases[lease.JobID]
		a.mu.RUnlock()
		if !ok {
			return jobs.ErrLeaseLost
		}
		return a.jobsService.Heartbeat(ctx, l)
	}

	db, err := a.manager.DB(ctx)
	if err != nil {
		return err
	}
	result := db.Exec(`
		UPDATE platform_jobs
		SET lease_until = clock_timestamp() + interval '30 seconds'
		WHERE tenant_id = ? AND id = ? AND state = 'leased'
		  AND lease_owner = ? AND lease_epoch = ? AND lease_until > clock_timestamp()
	`, lease.TenantID, lease.JobID, lease.WorkerID, lease.LeaseEpoch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return jobs.ErrLeaseLost
	}
	return nil
}

func (a *JobsAdapter) SuspendRunJob(ctx context.Context, lease *d.RunJobLease, wakeAt time.Time) error {
	if lease == nil {
		return nil
	}
	if a.jobsService != nil {
		a.mu.RLock()
		l, ok := a.leases[lease.JobID]
		a.mu.RUnlock()
		if ok {
			err := a.jobsService.Suspend(ctx, l, wakeAt)
			if err == nil || errors.Is(err, jobs.ErrLeaseLost) {
				a.mu.Lock()
				delete(a.leases, lease.JobID)
				a.mu.Unlock()
			}
			return err
		}
		return jobs.ErrLeaseLost
	}

	db, err := a.manager.DB(ctx)
	if err != nil {
		return err
	}
	result := db.Exec(`
		UPDATE platform_jobs
		SET state = 'retry_wait', available_at = ?, lease_owner = NULL, lease_until = NULL,
		    revision = revision + 1, updated_at = clock_timestamp()
		WHERE tenant_id = ? AND id = ? AND state = 'leased'
		  AND lease_owner = ? AND lease_epoch = ? AND lease_until > clock_timestamp()
	`, wakeAt.UTC(), lease.TenantID, lease.JobID, lease.WorkerID, lease.LeaseEpoch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return jobs.ErrLeaseLost
	}
	return nil
}

func (a *JobsAdapter) WakeRunJob(ctx context.Context, tenantID, jobID string) error {
	db, err := a.manager.Require(ctx)
	if err != nil {
		return err
	}
	result := db.Exec(`
		UPDATE platform_jobs
		SET state = 'queued', available_at = clock_timestamp(), lease_owner = NULL, lease_until = NULL,
		    revision = revision + 1, updated_at = clock_timestamp()
		WHERE tenant_id = ? AND id = ?
		  AND (state IN ('queued', 'retry_wait') OR (state = 'leased' AND lease_until <= clock_timestamp()))
	`, tenantID, jobID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return jobs.ErrLeaseLost
	}
	return nil
}

func (a *JobsAdapter) CompleteRunJob(ctx context.Context, lease *d.RunJobLease, state, code string) error {
	if lease == nil {
		return nil
	}
	if a.jobsService != nil {
		a.mu.RLock()
		l, ok := a.leases[lease.JobID]
		a.mu.RUnlock()
		if !ok {
			return jobs.ErrLeaseLost
		}
		_, err := a.jobsService.Complete(ctx, l, state, code)
		if err == nil || errors.Is(err, jobs.ErrLeaseLost) {
			a.mu.Lock()
			delete(a.leases, lease.JobID)
			a.mu.Unlock()
		}
		return err
	}

	db, err := a.manager.DB(ctx)
	if err != nil {
		return err
	}
	result := db.Exec(`
		UPDATE platform_jobs
		SET state = ?, last_error_code = NULLIF(?, ''), lease_owner = NULL, lease_until = NULL,
		    revision = revision + 1, updated_at = clock_timestamp()
		WHERE tenant_id = ? AND id = ? AND state = 'leased'
		  AND lease_owner = ? AND lease_epoch = ? AND lease_until > clock_timestamp()
	`, state, code, lease.TenantID, lease.JobID, lease.WorkerID, lease.LeaseEpoch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return jobs.ErrLeaseLost
	}
	return nil
}

func (a *JobsAdapter) CancelRunJobCommand(ctx context.Context, tenantID, jobID string, code string) error {
	db, err := a.manager.Require(ctx)
	if err != nil {
		return err
	}
	return db.Exec(`
		UPDATE platform_jobs
		SET state = 'cancelled', last_error_code = NULLIF(?, ''),
		    lease_owner = NULL, lease_until = NULL, cancel_requested_at = COALESCE(cancel_requested_at, clock_timestamp()),
		    revision = revision + 1, updated_at = clock_timestamp()
		WHERE tenant_id = ? AND id = ? AND state NOT IN ('succeeded', 'failed', 'cancelled')
	`, code, tenantID, jobID).Error
}

var _ d.JobsPort = (*JobsAdapter)(nil)

// WithRunJobLease locks the platform Job before Agent state and validates the
// original lease again before commit, including if the callback suspended it.
func (a *JobsAdapter) WithRunJobLease(ctx context.Context, lease *d.RunJobLease, fn func(context.Context) error) error {
	if lease == nil || fn == nil {
		return jobs.ErrLeaseLost
	}
	if a.jobsService != nil {
		a.mu.RLock()
		l, ok := a.leases[lease.JobID]
		a.mu.RUnlock()
		if !ok {
			return jobs.ErrLeaseLost
		}
		return a.jobsService.Checkpoint(ctx, l, func(tx context.Context, _ jobs.Job) error { return fn(tx) })
	}
	return a.manager.Within(ctx, func(tx context.Context) error {
		db, err := a.manager.Require(tx)
		if err != nil {
			return err
		}
		var until time.Time
		result := db.Raw(`SELECT lease_until FROM platform_jobs WHERE tenant_id=? AND id=? AND domain='agent' AND state='leased' AND lease_owner=? AND lease_epoch=? AND lease_until>clock_timestamp() FOR UPDATE`, lease.TenantID, lease.JobID, lease.WorkerID, lease.LeaseEpoch).Scan(&until)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return jobs.ErrLeaseLost
		}
		if err = fn(tx); err != nil {
			return err
		}
		var live bool
		if err = db.Raw(`SELECT clock_timestamp() < ?::timestamptz`, until).Scan(&live).Error; err != nil {
			return err
		}
		if !live {
			return jobs.ErrLeaseLost
		}
		return nil
	})
}
func (a *JobsAdapter) LockRunJobCommand(ctx context.Context, tenantID, jobID string) error {
	db, err := a.manager.Require(ctx)
	if err != nil {
		return err
	}
	var id string
	result := db.Raw(`SELECT id FROM platform_jobs WHERE tenant_id=? AND id=? AND domain='agent' AND job_kind='run' FOR UPDATE`, tenantID, jobID).Scan(&id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return d.ErrNotFound()
	}
	return nil
}
