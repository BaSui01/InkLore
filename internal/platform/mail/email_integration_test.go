package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var public *pe.AppError
	if !errors.As(err, &public) || public.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}
func receivedCode(t *testing.T, sink *smtpSink) string {
	t.Helper()
	parts := regexp.MustCompile(`verification code: ([0-9]{6})`).FindStringSubmatch(sink.message(t))
	if len(parts) != 2 {
		t.Fatal("SMTP message does not contain one six-digit OTP")
	}
	return parts[1]
}

// This test requires the isolated PostgreSQL/Redis runner. Mail travels through
// a real authenticated TLS SMTP socket with its test CA trusted explicitly; the
// private dial seam maps the already-validated public destination to the sink.
func TestEmailPostgresSMTPAuthentication(t *testing.T) {
	if os.Getenv("MAIL_AUTH_PG_TEST") != "1" {
		t.Skip("requires isolated PostgreSQL17 + Redis7")
	}
	dsn := os.Getenv("DATABASE_DSN")
	if !strings.Contains(dsn, "127.0.0.1") {
		t.Fatal("only isolated loopback PostgreSQL is accepted")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	manager, err := uow.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var limiter identity.LoginLimiter
	if address := os.Getenv("REDIS_ADDR"); address != "" {
		client := redis.NewClient(&redis.Options{Addr: address})
		defer client.Close()
		if err = client.Ping(ctx).Err(); err != nil {
			t.Fatal(err)
		}
		limiter = identity.NewRedisLoginLimiter(client)
	}
	keys := identity.Options{JWTKey: bytes.Repeat([]byte("j"), 32), CSRFKey: bytes.Repeat([]byte("c"), 32), Limiter: limiter}
	auth, err := identity.New(manager, keys)
	if err != nil {
		t.Fatal(err)
	}
	sink, roots := newSMTPSink(t, false)
	service, err := New(manager, Options{Identity: auth, Keys: map[int][]byte{1: bytes.Repeat([]byte("m"), 32)}, ActiveKeyVersion: 1, AllowedHosts: []string{"smtp.inklore.test", "api.resend.com"}, RootCAs: roots})
	if err != nil {
		t.Fatal(err)
	}
	service.transport = pinnedTransport(t, service.options, sink.listener)
	pepper := bytes.Repeat([]byte("p"), 32)
	if err = auth.ConfigureEmail(service, pepper); err != nil {
		t.Fatal(err)
	}
	newUser := func(t *testing.T, label string) identity.Session {
		t.Helper()
		session, err := auth.RegisterCreator(ctx, identity.RegisterInput{Email: label + "-" + uuid.NewString() + "@inklore.test", Password: "old-password-123", PenName: "Mail acceptance", ClientIP: uuid.NewString()})
		if err != nil {
			t.Fatal(err)
		}
		return session
	}
	emailFor := func(t *testing.T, p identity.Principal) string {
		t.Helper()
		var email string
		if err = db.Raw("SELECT email FROM platform_users WHERE id=?", p.UserID).Scan(&email).Error; err != nil {
			t.Fatal(err)
		}
		return email
	}
	adminCreator := newUser(t, "admin")
	email := emailFor(t, adminCreator.Principal)
	if err = db.Exec("UPDATE platform_users SET global_roles=ARRAY['super_admin']::text[] WHERE id=?", adminCreator.Principal.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("UPDATE platform_memberships SET roles=ARRAY['creator','operator']::text[] WHERE user_id=?", adminCreator.Principal.UserID).Error; err != nil {
		t.Fatal(err)
	}
	admin, err := auth.Login(ctx, identity.LoginInput{Email: email, Password: "old-password-123", Audience: "admin", ClientIP: "admin-password"})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := auth.EmailCapabilities(ctx)
	if err != nil || capability.Enabled {
		t.Fatal("disabled mail was advertised")
	}
	ordinary := newUser(t, "ordinary")
	_, err = service.Read(ctx, ordinary.Principal)
	requireCode(t, err, "auth.forbidden")
	current, err := service.Read(ctx, admin.Principal)
	if err != nil {
		t.Fatal(err)
	}
	input := UpdateInput{ExpectedRevision: current.Revision, Provider: "smtp", Enabled: true, SMTPHost: "smtp.inklore.test", SMTPPort: 465, SMTPSecurity: "implicit_tls", SMTPUsername: "sender", SenderEmail: "sender@inklore.test", SenderName: "InkLore", Credential: &CredentialChange{Kind: "encrypted", Value: "test-mail-secret"}}
	current, err = service.Update(ctx, admin.Principal, input)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(current)
	if strings.Contains(string(raw), "test-mail-secret") || strings.Contains(string(raw), "ciphertext") || strings.Contains(string(raw), "locator") || !current.HasCredential {
		t.Fatal("credential summary is unsafe")
	}
	var stored string
	if err = db.Raw("SELECT credential_ciphertext FROM platform_mail_settings WHERE id=1").Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "enc:v1:") || strings.Contains(stored, "test-mail-secret") {
		t.Fatal("credential is not encrypted")
	}
	_, err = service.Update(ctx, admin.Principal, input)
	requireCode(t, err, "platform.version_conflict")
	_, err = service.Test(ctx, admin.Principal, uuid.NewString(), TestInput{ExpectedRevision: current.Revision})
	requireCode(t, err, "auth.forbidden")
	request := func(t *testing.T, email, purpose, audience, tenant string) (identity.EmailChallenge, string, identity.EmailChallengeInput) {
		t.Helper()
		in := identity.EmailChallengeInput{Email: email, Purpose: purpose, Audience: audience, TenantID: tenant, RequestKey: uuid.NewString(), ClientIP: uuid.NewString()}
		out, err := auth.RequestEmailChallenge(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		return out, receivedCode(t, sink), in
	}
	t.Run("durable binding and authenticated admin test", func(t *testing.T) {
		challenge, code, in := request(t, email, "login", "admin", admin.Principal.TenantID)
		before := sink.received()
		replay, err := auth.RequestEmailChallenge(ctx, in)
		if err != nil || replay.ChallengeID != challenge.ChallengeID || sink.received() != before {
			t.Fatal("send replay did not reuse original handoff")
		}
		changed := in
		changed.Email = "different-" + uuid.NewString() + "@inklore.test"
		_, err = auth.RequestEmailChallenge(ctx, changed)
		requireCode(t, err, "platform.idempotency_conflict")
		in.RequestKey = uuid.NewString()
		_, err = auth.RequestEmailChallenge(ctx, in)
		requireCode(t, err, "auth.rate_limited")
		proof := identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Audience: "creator", TenantID: admin.Principal.TenantID, ClientIP: "binding-test"}
		_, err = auth.LoginEmail(ctx, proof)
		requireCode(t, err, "auth.invalid_credentials")
		proof.Audience = "admin"
		proof.TenantID = uuid.NewString()
		_, err = auth.LoginEmail(ctx, proof)
		requireCode(t, err, "auth.invalid_credentials")
		proof.TenantID = admin.Principal.TenantID
		restarted, err := identity.New(manager, keys)
		if err != nil {
			t.Fatal(err)
		}
		if err = restarted.ConfigureEmail(service, pepper); err != nil {
			t.Fatal(err)
		}
		logged, err := restarted.LoginEmail(ctx, proof)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.Authenticate(ctx, logged.AccessToken, "admin"); err != nil {
			t.Fatal(err)
		}
		_, err = restarted.LoginEmail(ctx, proof)
		requireCode(t, err, "auth.invalid_credentials")
		var state struct {
			Status   string
			Attempts int
			CodeHash string
		}
		if err = db.Raw("SELECT status,attempts,code_hash FROM platform_email_challenges WHERE id=?", challenge.ChallengeID).Scan(&state).Error; err != nil {
			t.Fatal(err)
		}
		if state.Status != "consumed" || state.Attempts != 1 || len(state.CodeHash) != 64 || state.CodeHash == code {
			t.Fatal("challenge was not consumed exactly once")
		}
		testKey := uuid.NewString()
		out, err := service.Test(ctx, logged.Principal, testKey, TestInput{ExpectedRevision: current.Revision})
		if err != nil || !out.Accepted {
			t.Fatal("verified administrator test failed", err)
		}
		if !strings.Contains(sink.message(t), "delivery test") {
			t.Fatal("test message missing")
		}
		before = sink.received()
		again, err := service.Test(ctx, logged.Principal, testKey, TestInput{ExpectedRevision: current.Revision})
		if err != nil || again.DeliveryID != out.DeliveryID || sink.received() != before {
			t.Fatal("test replay sent twice")
		}
	})
	t.Run("five failures survive restart", func(t *testing.T) {
		user := newUser(t, "attempts")
		email := emailFor(t, user.Principal)
		challenge, code, _ := request(t, email, "login", "creator", "")
		wrong := "000000"
		if code == wrong {
			wrong = "000001"
		}
		proof := identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: wrong, Audience: "creator"}
		for i := 0; i < 5; i++ {
			proof.ClientIP = uuid.NewString()
			_, err := auth.LoginEmail(ctx, proof)
			requireCode(t, err, "auth.invalid_credentials")
		}
		restarted, err := identity.New(manager, keys)
		if err != nil {
			t.Fatal(err)
		}
		if err = restarted.ConfigureEmail(service, pepper); err != nil {
			t.Fatal(err)
		}
		proof.Code = code
		proof.ClientIP = uuid.NewString()
		_, err = restarted.LoginEmail(ctx, proof)
		requireCode(t, err, "auth.invalid_credentials")
		var attempts int
		if err = db.Raw("SELECT attempts FROM platform_email_challenges WHERE id=?", challenge.ChallengeID).Scan(&attempts).Error; err != nil || attempts != 5 {
			t.Fatal("attempt limit not durable")
		}
	})
	t.Run("concurrent single consumption", func(t *testing.T) {
		user := newUser(t, "race")
		email := emailFor(t, user.Principal)
		challenge, code, _ := request(t, email, "login", "creator", "")
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := auth.LoginEmail(ctx, identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Audience: "creator", ClientIP: uuid.NewString()})
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		success := 0
		for err := range results {
			if err == nil {
				success++
			} else {
				requireCode(t, err, "auth.invalid_credentials")
			}
		}
		if success != 1 {
			t.Fatalf("concurrent consumers: %d successes", success)
		}
		var count int
		if err = db.Raw("SELECT count(*) FROM platform_sessions WHERE user_id=?", user.Principal.UserID).Scan(&count).Error; err != nil || count != 2 {
			t.Fatal("duplicate session created")
		}
	})
	t.Run("unknown account same real send response", func(t *testing.T) {
		email := "unknown-" + uuid.NewString() + "@inklore.test"
		challenge, code, binding := request(t, email, "login", "creator", uuid.NewString())
		_, err := auth.LoginEmail(ctx, identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Audience: "creator", TenantID: binding.TenantID, ClientIP: uuid.NewString()})
		requireCode(t, err, "auth.invalid_credentials")
		var actor *string
		if err = db.Raw("SELECT actor_id FROM platform_email_challenges WHERE id=?", challenge.ChallengeID).Scan(&actor).Error; err != nil {
			t.Fatal(err)
		}
		if actor != nil {
			t.Fatal("unknown challenge bound to an account")
		}
	})
	t.Run("verified registration consumes atomically", func(t *testing.T) {
		email := "verified-register-" + uuid.NewString() + "@inklore.test"
		challenge, code, _ := request(t, email, "register", "creator", "")
		input := identity.RegisterInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Password: "registered-password", PenName: "Verified author", ClientIP: uuid.NewString()}
		out, err := auth.RegisterCreator(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		var verified bool
		if err = db.Raw("SELECT email_verified_at IS NOT NULL FROM platform_users WHERE id=?", out.Principal.UserID).Scan(&verified).Error; err != nil || !verified {
			t.Fatal("verified registration missing email proof")
		}
		_, err = auth.RegisterCreator(ctx, input)
		requireCode(t, err, "auth.invalid_credentials")
	})
	t.Run("reset revokes all sessions and survives process restart", func(t *testing.T) {
		user := newUser(t, "reset")
		email := emailFor(t, user.Principal)
		second, err := auth.Login(ctx, identity.LoginInput{Email: email, Password: "old-password-123", Audience: "creator", ClientIP: uuid.NewString()})
		if err != nil {
			t.Fatal(err)
		}
		challenge, code, _ := request(t, email, "reset", "creator", user.Principal.TenantID)
		restarted, err := identity.New(manager, keys)
		if err != nil {
			t.Fatal(err)
		}
		if err = restarted.ConfigureEmail(service, pepper); err != nil {
			t.Fatal(err)
		}
		proof := identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Audience: "creator", TenantID: user.Principal.TenantID, ClientIP: uuid.NewString()}
		err = restarted.ResetPassword(ctx, identity.ResetPasswordInput{EmailLoginInput: proof, NewPassword: "new-password-123"})
		if err != nil {
			t.Fatal(err)
		}
		for _, session := range []identity.Session{user, second} {
			_, err = auth.Authenticate(ctx, session.AccessToken, "creator")
			requireCode(t, err, "auth.unauthenticated")
			_, err = auth.Refresh(ctx, session.RefreshToken, "creator", session.CSRFToken)
			requireCode(t, err, "auth.unauthenticated")
		}
		_, err = auth.Login(ctx, identity.LoginInput{Email: email, Password: "old-password-123", Audience: "creator", ClientIP: uuid.NewString()})
		requireCode(t, err, "auth.invalid_credentials")
		if _, err = auth.Login(ctx, identity.LoginInput{Email: email, Password: "new-password-123", Audience: "creator", ClientIP: uuid.NewString()}); err != nil {
			t.Fatal(err)
		}
		err = restarted.ResetPassword(ctx, identity.ResetPasswordInput{EmailLoginInput: proof, NewPassword: "another-password"})
		requireCode(t, err, "auth.invalid_credentials")
	})

	t.Run("failed session transaction does not consume proof", func(t *testing.T) {
		user := newUser(t, "rollback")
		email := emailFor(t, user.Principal)
		challenge, code, _ := request(t, email, "login", "creator", "")
		brokenOptions := keys
		brokenOptions.OnEvent = func(context.Context, identity.Event) error { return errors.New("injected audit write failure") }
		broken, err := identity.New(manager, brokenOptions)
		if err != nil {
			t.Fatal(err)
		}
		if err = broken.ConfigureEmail(service, pepper); err != nil {
			t.Fatal(err)
		}
		proof := identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Audience: "creator", ClientIP: uuid.NewString()}
		if _, err = broken.LoginEmail(ctx, proof); err == nil {
			t.Fatal("injected transaction failure succeeded")
		}
		var row struct {
			Status   string
			Attempts int
		}
		if err = db.Raw("SELECT status,attempts FROM platform_email_challenges WHERE id=?", challenge.ChallengeID).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.Status != "sent" || row.Attempts != 0 {
			t.Fatal("rolled back session consumed OTP")
		}
		var count int
		if err = db.Raw("SELECT count(*) FROM platform_sessions WHERE user_id=?", user.Principal.UserID).Scan(&count).Error; err != nil || count != 1 {
			t.Fatal("failed transaction retained session")
		}
		if _, err = auth.LoginEmail(ctx, proof); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("anonymous IP budget has no account bypass", func(t *testing.T) {
		for i := 0; i < 31; i++ {
			in := identity.EmailChallengeInput{Email: "ip-rate-" + uuid.NewString() + "@inklore.test", Purpose: "reset", Audience: "admin", RequestKey: uuid.NewString(), ClientIP: "same-unrecognized-client"}
			_, err := auth.RequestEmailChallenge(ctx, in)
			if i == 30 {
				requireCode(t, err, "auth.rate_limited")
			} else {
				if err != nil {
					t.Fatal(err)
				}
				receivedCode(t, sink)
			}
		}
	})
	t.Run("configuration CAS serializes competing admins", func(t *testing.T) {
		in := input
		in.ExpectedRevision = current.Revision
		in.Credential = nil
		type result struct {
			settings Settings
			err      error
		}
		results := make(chan result, 2)
		var group sync.WaitGroup
		for i := 0; i < 2; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				value, err := service.Update(ctx, admin.Principal, in)
				results <- result{value, err}
			}()
		}
		group.Wait()
		close(results)
		accepted := 0
		for value := range results {
			if value.err == nil {
				accepted++
				current = value.settings
			} else {
				requireCode(t, value.err, "platform.version_conflict")
			}
		}
		if accepted != 1 {
			t.Fatal("configuration CAS permitted multiple winners")
		}
	})
	t.Run("expired and failed delivery remain truthful", func(t *testing.T) {
		user := newUser(t, "expiry")
		email := emailFor(t, user.Principal)
		challenge, code, _ := request(t, email, "login", "creator", "")
		if err = db.Exec("UPDATE platform_email_challenges SET created_at=clock_timestamp()-interval '6 minutes',expires_at=clock_timestamp()-interval '1 second' WHERE id=?", challenge.ChallengeID).Error; err != nil {
			t.Fatal(err)
		}
		_, err = auth.LoginEmail(ctx, identity.EmailLoginInput{Email: email, ChallengeID: challenge.ChallengeID, Code: code, Audience: "creator", ClientIP: uuid.NewString()})
		requireCode(t, err, "auth.invalid_credentials")
		original := service.transport.lookup
		service.transport.lookup = func(context.Context, string) ([]net.IPAddr, error) {
			return nil, errors.New("isolated resolver unavailable")
		}
		defer func() { service.transport.lookup = original }()
		in := identity.EmailChallengeInput{Email: "failed-" + uuid.NewString() + "@inklore.test", Purpose: "login", Audience: "creator", ClientIP: uuid.NewString(), RequestKey: uuid.NewString()}
		_, err = auth.RequestEmailChallenge(ctx, in)
		requireCode(t, err, "platform.dependency_unavailable")
		var status string
		if err = db.Raw("SELECT status FROM platform_email_challenges WHERE request_key=?", in.RequestKey).Scan(&status).Error; err != nil || status != "failed" {
			t.Fatal("failed delivery reported sent")
		}
		_, err = auth.RequestEmailChallenge(ctx, in)
		requireCode(t, err, "platform.dependency_unavailable")
	})
}
