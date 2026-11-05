package app_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"InkLore/internal/app"
	"InkLore/internal/platform/identity"
	"github.com/google/uuid"
)

// HTTP coverage uses the actual app composition, handlers, live authorization,
// CAS/idempotency/audit stores and provider failure path. Successful TLS provider
// handoff plus email login/reset is covered by mail's socket integration suite.
func TestMailAuthHTTPPostgres(t *testing.T) {
	if os.Getenv("MAIL_AUTH_PG_TEST") != "1" {
		t.Skip("requires isolated PostgreSQL17 + Redis7")
	}
	if !strings.Contains(os.Getenv("DATABASE_DSN"), "127.0.0.1") {
		t.Fatal("requires isolated PostgreSQL")
	}
	t.Setenv("MAIL_OTP_HASH_KEY", strings.Repeat("otp-key-", 8))
	t.Setenv("MAIL_ALLOWED_HOSTS", "smtp.inklore.test")
	keyJSON, _ := json.Marshal(map[string]string{"1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("m"), 32))})
	t.Setenv("MAIL_ENCRYPTION_KEYS", string(keyJSON))
	t.Setenv("MAIL_ENCRYPTION_KEY_VERSION", "1")
	application, err := app.New()
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	if err = application.DB().Exec("UPDATE platform_mail_settings SET provider='disabled',enabled=false,credential_kind=NULL,credential_locator=NULL,credential_ciphertext=NULL,credential_key_version=NULL WHERE id=1").Error; err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any, session *identity.Session, csrf, key, origin string) (int, map[string]any, *httptest.ResponseRecorder) {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", uuid.NewString())
		if session != nil {
			req.Header.Set("Authorization", "Bearer "+session.AccessToken)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		recorder := httptest.NewRecorder()
		application.Handler().ServeHTTP(recorder, req)
		var envelope map[string]any
		if err = json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal("invalid JSON response")
		}
		return recorder.Code, envelope, recorder
	}
	decodeSession := func(envelope map[string]any) identity.Session {
		t.Helper()
		raw, _ := json.Marshal(envelope["data"])
		var session identity.Session
		if err = json.Unmarshal(raw, &session); err != nil {
			t.Fatal(err)
		}
		return session
	}
	status, body, _ := request("GET", "/v1/auth/email-capabilities", nil, nil, "", "", "")
	if status != 200 || body["data"].(map[string]any)["enabled"] != false {
		t.Fatal("mail capabilities should be disabled")
	}
	email := "http-mail-" + uuid.NewString() + "@inklore.test"
	status, body, response := request("POST", "/v1/auth/register", map[string]any{"email": email, "password": "http-password-123", "pen_name": "Mail HTTP"}, nil, "", "", os.Getenv("CREATOR_ORIGIN"))
	if status != 200 {
		t.Fatalf("password registration: %d %v", status, body["code"])
	}
	creator := decodeSession(body)
	if len(response.Result().Cookies()) != 1 || !response.Result().Cookies()[0].HttpOnly {
		t.Fatal("real refresh cookie missing")
	}
	if err = application.DB().Exec("UPDATE platform_memberships SET roles=ARRAY['creator','operator']::text[] WHERE user_id=?", creator.Principal.UserID).Error; err != nil {
		t.Fatal(err)
	}
	status, body, _ = request("POST", "/v1/auth/login", map[string]any{"email": email, "password": "http-password-123", "audience": "admin"}, nil, "", "", os.Getenv("ADMIN_ORIGIN"))
	if status != 200 {
		t.Fatalf("admin login: %d", status)
	}
	admin := decodeSession(body)
	status, _, _ = request("GET", "/v1/platform/mail-settings", nil, &admin, "", "", "")
	if status != 403 {
		t.Fatal("tenant operator gained global mail settings")
	}
	if err = application.DB().Exec("UPDATE platform_users SET global_roles=ARRAY['super_admin']::text[] WHERE id=?", creator.Principal.UserID).Error; err != nil {
		t.Fatal(err)
	}

	status, _, _ = request("GET", "/v1/platform/mail-settings", nil, &creator, "", "", "")
	if status != 403 {
		t.Fatal("Creator gained global mail access")
	}
	status, body, _ = request("GET", "/v1/platform/mail-settings", nil, &admin, "", "", "")
	if status != 200 {
		t.Fatal("super admin settings read failed")
	}
	revision := body["data"].(map[string]any)["revision"]
	input := map[string]any{"expected_revision": revision, "provider": "smtp", "enabled": true, "smtp_host": "smtp.inklore.test", "smtp_port": 465, "smtp_security": "implicit_tls", "smtp_username": "sender", "sender_email": "sender@inklore.test", "sender_name": "InkLore", "credential": map[string]any{"kind": "encrypted", "value": "sensitive-http-mail-credential"}}
	status, _, _ = request("PUT", "/v1/platform/mail-settings", input, &admin, "", uuid.NewString(), os.Getenv("ADMIN_ORIGIN"))
	if status != 403 {
		t.Fatal("missing CSRF accepted")
	}
	status, _, _ = request("PUT", "/v1/platform/mail-settings", input, &admin, admin.CSRFToken, uuid.NewString(), "https://foreign.invalid")
	if status != 403 {
		t.Fatal("foreign origin accepted")
	}
	mutationKey := uuid.NewString()
	status, body, response = request("PUT", "/v1/platform/mail-settings", input, &admin, admin.CSRFToken, mutationKey, os.Getenv("ADMIN_ORIGIN"))
	if status != 200 {
		t.Fatalf("save settings: %d %v", status, body["code"])
	}
	if strings.Contains(response.Body.String(), "sensitive-http-mail-credential") || strings.Contains(response.Body.String(), "ciphertext") || strings.Contains(response.Body.String(), "locator") {
		t.Fatal("mail response disclosed credential")
	}
	updatedRevision := body["data"].(map[string]any)["revision"]
	status, body, _ = request("PUT", "/v1/platform/mail-settings", input, &admin, admin.CSRFToken, mutationKey, os.Getenv("ADMIN_ORIGIN"))
	if status != 200 || body["data"].(map[string]any)["revision"] != updatedRevision {
		t.Fatal("settings idempotency did not replay")
	}
	status, body, _ = request("PUT", "/v1/platform/mail-settings", input, &admin, admin.CSRFToken, uuid.NewString(), os.Getenv("ADMIN_ORIGIN"))
	if status != 409 || body["code"] != "platform.version_conflict" {
		t.Fatal("stale CAS was accepted")
	}
	input["expected_revision"] = updatedRevision
	input["credential"] = map[string]any{"kind": "env", "locator": "JWT_SIGNING_KEY"}
	status, body, _ = request("PUT", "/v1/platform/mail-settings", input, &admin, admin.CSRFToken, uuid.NewString(), os.Getenv("ADMIN_ORIGIN"))
	if status != 400 {
		t.Fatal("unrelated secret locator accepted")
	}
	status, _, _ = request("POST", "/v1/platform/mail-settings/test", map[string]any{"expected_revision": updatedRevision}, &admin, admin.CSRFToken, uuid.NewString(), os.Getenv("ADMIN_ORIGIN"))
	if status != 403 {
		t.Fatal("test delivered to unverified mailbox")
	}
	status, body, _ = request("GET", "/v1/auth/email-capabilities", nil, nil, "", "", "")
	if status != 200 || body["data"].(map[string]any)["enabled"] != true {
		t.Fatal("configured capability unavailable")
	}
	for _, recipient := range []string{email, "unknown-" + uuid.NewString() + "@inklore.test"} {
		id := uuid.NewString()
		status, body, _ = request("POST", "/v1/auth/email-challenges", map[string]any{"email": recipient, "purpose": "login", "audience": "creator"}, nil, "", id, os.Getenv("CREATOR_ORIGIN"))
		if status != 503 || body["code"] != "platform.dependency_unavailable" {
			t.Fatalf("undelivered challenge was not rejected uniformly: %d %v", status, body["code"])
		}
		var challengeStatus string
		if err = application.DB().Raw("SELECT status FROM platform_email_challenges WHERE request_key=?", id).Scan(&challengeStatus).Error; err != nil || challengeStatus != "failed" {
			t.Fatal("delivery failure missing durable status")
		}
	}
	var auditCount int64
	if err = application.DB().Raw("SELECT count(*) FROM platform_audit_events WHERE actor_id=? AND action='platform.mail_settings_updated'", admin.Principal.UserID).Scan(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatal("settings audit/replay mismatch", err)
	}
	status, body, _ = request(http.MethodPost, "/v1/auth/reset-password", map[string]any{"token": "old-development-token", "new_password": "new-password-123"}, nil, "", "", os.Getenv("CREATOR_ORIGIN"))
	if status == 200 {
		t.Fatal("legacy development token accepted")
	}
}
