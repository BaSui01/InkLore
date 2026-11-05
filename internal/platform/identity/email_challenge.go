package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	stdmail "net/mail"
	"sort"
	"strings"
	"time"

	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type challengeRow struct {
	ID, Purpose, Audience, RecipientHash, CodeHash, Status, RequestHash string
	TenantID, ActorID, RequestedTenantID                                *string
	Attempts                                                            int
	ExpiresAt, CreatedAt                                                time.Time
}

func normalizedEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := stdmail.ParseAddress(value)
	return value, err == nil && address.Address == value && len(value) <= 254 && !strings.ContainsAny(value, "\r\n\x00")
}
func nullableID(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func equalID(value *string, expected string) bool {
	return value == nil && expected == "" || value != nil && *value == expected
}
func (s *Service) emailHash(kind, value string) string {
	mac := hmac.New(sha256.New, s.emailPepper)
	mac.Write([]byte(kind + "\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}
func emailRateLimit() error {
	return pe.Contract(429, "auth.rate_limited", map[string]any{"retry_after_seconds": int64(60)})
}
func emailUnavailable() error { return pe.Contract(503, "platform.dependency_unavailable", nil) }
func challengeResult(row challengeRow) EmailChallenge {
	return EmailChallenge{ChallengeID: row.ID, ExpiresAt: row.ExpiresAt, RetryAfter: 60}
}
func replayChallenge(row challengeRow, hash string) (EmailChallenge, error) {
	if row.RequestHash != hash {
		return EmailChallenge{}, pe.Contract(409, "platform.idempotency_conflict", nil)
	}
	switch row.Status {
	case "sent", "consumed", "expired":
		return challengeResult(row), nil
	case "pending":
		return EmailChallenge{}, pe.Contract(409, "platform.operation_in_progress", map[string]any{"retry_after_seconds": int64(2)})
	default:
		return EmailChallenge{}, emailUnavailable()
	}
}

// RequestEmailChallenge persists a reservation before doing network I/O. An
// ambiguous/failed handoff cannot be resent under the same key. Every well-formed
// mailbox, including an unknown account, uses the identical bounded delivery path.
func (s *Service) RequestEmailChallenge(ctx context.Context, input EmailChallengeInput) (EmailChallenge, error) {
	if uow.InTransaction(ctx) {
		return EmailChallenge{}, errors.New("email challenge requires its own transaction")
	}
	var valid bool
	input.Email, valid = normalizedEmail(input.Email)
	if !valid || !validAudience(input.Audience) || input.Purpose != "login" && input.Purpose != "register" && input.Purpose != "reset" || input.Purpose == "register" && (input.Audience != "creator" || input.TenantID != "") || input.TenantID != "" && !scope.ValidID(input.TenantID) || !scope.ValidID(input.RequestKey) {
		return EmailChallenge{}, invalid()
	}
	if len(s.emailPepper) < 32 {
		return EmailChallenge{}, emailUnavailable()
	}
	recipient := s.emailHash("recipient", input.Email)
	ip := s.emailHash("ip", input.ClientIP)
	requestHash := s.emailHash("request", recipient+"\x00"+input.Purpose+"\x00"+input.Audience+"\x00"+input.TenantID)
	codeNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return EmailChallenge{}, err
	}
	code := fmt.Sprintf("%06d", codeNumber.Int64())
	row := challengeRow{ID: uuid.NewString(), Purpose: input.Purpose, Audience: input.Audience, RecipientHash: recipient, Status: "pending", RequestHash: requestHash, RequestedTenantID: nullableID(input.TenantID)}
	row.CodeHash = s.emailHash("code", row.ID+"\x00"+code)
	replay := false
	var publicErr error
	err = s.manager.Within(ctx, func(txctx context.Context) error {
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		locks := []string{"email:" + recipient, "ip:" + ip, "request:" + input.RequestKey}
		sort.Strings(locks)
		for _, key := range locks {
			if err = db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,98))", key).Error; err != nil {
				return err
			}
		}
		var previous challengeRow
		q := db.Raw("SELECT * FROM platform_email_challenges WHERE request_key=?", input.RequestKey).Scan(&previous)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 1 {
			row = previous
			replay = true
			_, publicErr = replayChallenge(row, requestHash)
			return nil
		}
		capability, err := s.EmailCapabilities(txctx)
		if err != nil {
			return err
		}
		if !capability.Enabled {
			return emailUnavailable()
		}
		var limits struct {
			Cooldown                          bool
			EmailCount, PurposeCount, IPCount int64
		}
		if err = db.Raw(`SELECT EXISTS(SELECT 1 FROM platform_email_challenges WHERE recipient_hash=? AND created_at>clock_timestamp()-interval '60 seconds') AS cooldown,
   (SELECT count(*) FROM platform_email_challenges WHERE recipient_hash=? AND created_at>clock_timestamp()-interval '1 hour') AS email_count,
   (SELECT count(*) FROM platform_email_challenges WHERE recipient_hash=? AND purpose=? AND created_at>clock_timestamp()-interval '1 hour') AS purpose_count,
   (SELECT count(*) FROM platform_email_challenges WHERE ip_hash=? AND created_at>clock_timestamp()-interval '1 hour') AS ip_count`, recipient, recipient, recipient, input.Purpose, ip).Scan(&limits).Error; err != nil {
			return err
		}
		if limits.Cooldown || limits.EmailCount >= 5 || limits.PurposeCount >= 3 || limits.IPCount >= 30 {
			return emailRateLimit()
		}
		if input.Purpose != "register" {
			var binding struct{ ActorID, TenantID string }
			q = db.Raw(`SELECT u.id AS actor_id,m.tenant_id FROM platform_users u JOIN platform_memberships m ON m.user_id=u.id JOIN platform_tenants t ON t.id=m.tenant_id
    WHERE lower(u.email)=? AND u.status='active' AND m.status='active' AND t.status='active'
    AND (?::uuid IS NULL OR m.tenant_id=?::uuid)
    AND CASE WHEN ?='creator' THEN 'creator'=ANY(m.roles) ELSE m.roles && ARRAY['operator','auditor']::text[] OR 'super_admin'=ANY(u.global_roles) END
    ORDER BY m.created_at,m.id LIMIT 1`, input.Email, nullableID(input.TenantID), nullableID(input.TenantID), input.Audience).Scan(&binding)
			if q.Error != nil {
				return q.Error
			}
			if q.RowsAffected == 1 {
				row.ActorID = &binding.ActorID
				row.TenantID = &binding.TenantID
			}
		}
		var timing struct{ CreatedAt, ExpiresAt time.Time }
		if err = db.Raw(`INSERT INTO platform_email_challenges(id,purpose,audience,tenant_id,actor_id,recipient_hash,code_hash,expires_at,requested_tenant_id,request_key,request_hash,ip_hash)
   VALUES(?,?,?,?,?,?,?,clock_timestamp()+interval '5 minutes',?,?,?,?) RETURNING created_at,expires_at`, row.ID, row.Purpose, row.Audience, row.TenantID, row.ActorID, row.RecipientHash, row.CodeHash, row.RequestedTenantID, input.RequestKey, requestHash, ip).Scan(&timing).Error; err != nil {
			return err
		}
		row.CreatedAt, row.ExpiresAt = timing.CreatedAt, timing.ExpiresAt
		return nil
	})
	if err != nil {
		return EmailChallenge{}, err
	}
	if publicErr != nil {
		return EmailChallenge{}, publicErr
	}
	if replay {
		return challengeResult(row), nil
	}
	deliveryErr := s.emailDelivery.SendVerification(ctx, row.ID, input.Email, input.Purpose, code)
	// A disconnected client must not discard the durable provider outcome. This
	// short independent commit contains no retry of the external side effect.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.manager.Within(finishCtx, func(txctx context.Context) error {
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		status := "sent"
		if deliveryErr != nil {
			status = "failed"
		}
		q := db.Exec("UPDATE platform_email_challenges SET status=?,sent_at=CASE WHEN ?='sent' THEN clock_timestamp() ELSE NULL END WHERE id=? AND status='pending'", status, status, row.ID)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return errors.New("email challenge handoff lost its reservation")
		}
		return nil
	})
	if err != nil {
		return EmailChallenge{}, err
	}
	if deliveryErr != nil {
		return EmailChallenge{}, emailUnavailable()
	}
	return challengeResult(row), nil
}

// checkEmailChallenge consumes one attempt under a row lock. Validation failures
// are returned as false so callers commit the attempt before returning a uniform
// credential error. Never call it through a transaction that rolls back on 401.
func (s *Service) checkEmailChallenge(ctx context.Context, input EmailLoginInput, purpose string) (challengeRow, bool, error) {
	db, err := s.manager.Require(ctx)
	if err != nil {
		return challengeRow{}, false, err
	}
	db = db.Session(&gorm.Session{Logger: logger.Discard})
	var row challengeRow
	q := db.Raw("SELECT * FROM platform_email_challenges WHERE id=? FOR UPDATE", input.ChallengeID).Scan(&row)
	if q.Error != nil {
		return row, false, q.Error
	}
	if q.RowsAffected != 1 {
		return row, false, nil
	}
	// A different audience or mailbox must never spend another principal's attempts.
	if row.Purpose != purpose || row.Audience != input.Audience || row.RecipientHash != s.emailHash("recipient", input.Email) || !equalID(row.RequestedTenantID, input.TenantID) {
		return row, false, nil
	}
	var now time.Time
	if err = db.Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
		return row, false, err
	}
	if row.Status != "sent" || row.Attempts >= 5 {
		return row, false, nil
	}
	if !row.ExpiresAt.After(now) {
		return row, false, db.Exec("UPDATE platform_email_challenges SET status='expired' WHERE id=? AND status='sent'", row.ID).Error
	}
	row.Attempts++
	if err = db.Exec("UPDATE platform_email_challenges SET attempts=? WHERE id=? AND status='sent'", row.Attempts, row.ID).Error; err != nil {
		return row, false, err
	}
	if !hmac.Equal([]byte(row.CodeHash), []byte(s.emailHash("code", row.ID+"\x00"+input.Code))) {
		return row, false, nil
	}
	return row, true, nil
}
func (s *Service) consumeEmailChallenge(ctx context.Context, row challengeRow) error {
	db, err := s.manager.Require(ctx)
	if err != nil {
		return err
	}
	q := db.Exec("UPDATE platform_email_challenges SET status='consumed',consumed_at=clock_timestamp() WHERE id=? AND status='sent' AND attempts=? AND expires_at>clock_timestamp()", row.ID, row.Attempts)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return credentials()
	}
	return nil
}
func (s *Service) validateEmailLogin(ctx context.Context, input *EmailLoginInput) error {
	var valid bool
	input.Email, valid = normalizedEmail(input.Email)
	if !valid || !scope.ValidID(input.ChallengeID) || len(input.Code) != 6 || strings.Trim(input.Code, "0123456789") != "" || !validAudience(input.Audience) || input.TenantID != "" && !scope.ValidID(input.TenantID) {
		return invalid()
	}
	if len(s.emailPepper) < 32 {
		return emailUnavailable()
	}
	if !s.allowLogin(ctx, s.emailHash("verify-rate", input.ClientIP+"\x00"+input.Email)) {
		return emailRateLimit()
	}
	return nil
}

// emailMember takes the same tenant -> user -> membership lock order as password
// changes. The exclusive user lock prevents verification/reset lock upgrades.
func (s *Service) emailMember(ctx context.Context, row challengeRow, email string) (Principal, bool, error) {
	if row.ActorID == nil || row.TenantID == nil {
		return Principal{}, false, nil
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return Principal{}, false, err
	}
	db = db.Session(&gorm.Session{Logger: logger.Discard})
	var id string
	q := db.Raw("SELECT id FROM platform_tenants WHERE id=? AND status='active' FOR SHARE", *row.TenantID).Scan(&id)
	if q.Error != nil {
		return Principal{}, false, q.Error
	}
	if q.RowsAffected != 1 {
		return Principal{}, false, nil
	}
	q = db.Raw("SELECT id FROM platform_users WHERE id=? AND lower(email)=? AND status='active' FOR UPDATE", *row.ActorID, email).Scan(&id)
	if q.Error != nil {
		return Principal{}, false, q.Error
	}
	if q.RowsAffected != 1 {
		return Principal{}, false, nil
	}
	p, _, err := s.member(ctx, Principal{UserID: *row.ActorID, TenantID: *row.TenantID, Audience: row.Audience})
	if err != nil {
		var appErr *pe.AppError
		if errors.As(err, &appErr) && (appErr.HTTPStatus == 401 || appErr.HTTPStatus == 403) {
			return Principal{}, false, nil
		}
		return Principal{}, false, err
	}
	return p, true, nil
}
