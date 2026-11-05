package mail

import (
	"context"
	"errors"
	"time"

	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	plog "InkLore/internal/platform/logger"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"github.com/google/uuid"
)

type TestInput struct {
	ExpectedRevision int64 `json:"expected_revision"`
}
type TestResult struct {
	DeliveryID string `json:"delivery_id"`
	Accepted   bool   `json:"accepted"`
}

// Test reserves a single handoff outside the global idempotency transaction: SMTP
// acceptance cannot be rolled back. Retries inspect the durable outcome instead.
func (s *Service) Test(ctx context.Context, p identity.Principal, key string, input TestInput) (TestResult, error) {
	if uow.InTransaction(ctx) {
		return TestResult{}, errors.New("mail test requires its own transaction")
	}
	if !scope.ValidID(key) || input.ExpectedRevision < 1 || input.ExpectedRevision > scope.MaxRevision {
		return TestResult{}, invalid()
	}
	var row settingRow
	var recipient, id string
	replay := false
	var publicErr error
	err := s.manager.Within(ctx, func(tx context.Context) error {
		if err := s.Authorize(tx, p); err != nil {
			return err
		}
		var err error
		recipient, err = s.options.Identity.VerifiedEmail(tx, p)
		if err != nil {
			return err
		}
		db, err := s.manager.Require(tx)
		if err != nil {
			return err
		}
		if err = db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,98))", "mail-test:"+p.UserID).Error; err != nil {
			return err
		}
		var previous struct {
			ID, TenantID, Status string
			SettingsRevision     int64
		}
		q := db.Raw("SELECT id,tenant_id,status,settings_revision FROM platform_mail_test_deliveries WHERE actor_id=? AND request_key=?", p.UserID, key).Scan(&previous)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 1 {
			replay = true
			id = previous.ID
			if previous.TenantID != p.TenantID || previous.SettingsRevision != input.ExpectedRevision {
				publicErr = pe.Contract(409, "platform.idempotency_conflict", nil)
				return nil
			}
			if previous.Status == "pending" {
				publicErr = pe.Contract(409, "platform.operation_in_progress", map[string]any{"retry_after_seconds": int64(2)})
			}
			if previous.Status == "failed" {
				publicErr = unavailable()
			}
			return nil
		}
		var cooldown bool
		if err = db.Raw("SELECT EXISTS(SELECT 1 FROM platform_mail_test_deliveries WHERE actor_id=? AND created_at>clock_timestamp()-interval '60 seconds')", p.UserID).Scan(&cooldown).Error; err != nil {
			return err
		}
		if cooldown {
			return pe.Contract(429, "auth.rate_limited", map[string]any{"retry_after_seconds": int64(60)})
		}
		row, err = s.load(tx, false)
		if err != nil {
			return err
		}
		if row.Revision != input.ExpectedRevision {
			return pe.Contract(409, "platform.version_conflict", map[string]any{"current_revision": row.Revision})
		}
		if !row.Enabled {
			return unavailable()
		}
		id = uuid.NewString()
		return db.Exec("INSERT INTO platform_mail_test_deliveries(id,actor_id,tenant_id,request_key,settings_revision,status) VALUES(?,?,?,?,?,'pending')", id, p.UserID, p.TenantID, key, row.Revision).Error
	})
	if err != nil {
		return TestResult{}, err
	}
	if publicErr != nil {
		return TestResult{}, publicErr
	}
	if replay {
		return TestResult{DeliveryID: id, Accepted: true}, nil
	}
	secret, deliveryErr := s.credential(row)
	if deliveryErr == nil {
		deliveryErr = s.transport.send(ctx, row.summary(), secret, Message{ID: id, To: recipient, Subject: "InkLore email delivery test", Text: "Your InkLore mail configuration delivered this test message."})
	}
	if deliveryErr != nil {
		plog.WarnContext(ctx, "mail.delivery_failed", "domain", "platform", "operation", "mail.send_test", "error_code", "platform.dependency_unavailable", "provider", row.Provider, "stage", deliveryErr.Error())
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.manager.Within(finish, func(tx context.Context) error {
		db, err := s.manager.Require(tx)
		if err != nil {
			return err
		}
		status := "sent"
		if deliveryErr != nil {
			status = "failed"
		}
		q := db.Exec("UPDATE platform_mail_test_deliveries SET status=?,completed_at=clock_timestamp() WHERE id=? AND status='pending'", status, id)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return errors.New("mail test lost its reservation")
		}
		if status == "sent" && s.options.OnEvent != nil {
			return s.options.OnEvent(tx, Event{Principal: p, Action: "platform.mail_test_sent", Revision: row.Revision})
		}
		return nil
	})
	if err != nil {
		return TestResult{}, err
	}
	if deliveryErr != nil {
		return TestResult{}, unavailable()
	}
	return TestResult{DeliveryID: id, Accepted: true}, nil
}
