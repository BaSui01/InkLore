// Package audit owns append-only, privacy-filtered platform audit events.
// Callers authorize and hold live identity locks in the same UOW before reads.
package audit

import "time"

// SafeChange mirrors the frozen wire schema. Values are further restricted by
// the field-specific allowlist; arbitrary strings and maps are never accepted.
type SafeChange struct {
	Field  string  `json:"field"`
	Before *string `json:"before"`
	After  *string `json:"after"`
}

type Audit struct {
	ID          string       `json:"id"`
	ActorID     *string      `json:"actor_id"`
	Audience    string       `json:"audience"`
	Action      string       `json:"action"`
	ResourceID  *string      `json:"resource_id"`
	Outcome     string       `json:"outcome"`
	RequestID   string       `json:"request_id"`
	TraceID     string       `json:"trace_id"`
	SafeChanges []SafeChange `json:"safe_changes"`
	CreatedAt   time.Time    `json:"created_at"`
}

// Record contains verified actor/tenant/audience facts, never HTTP-bound input.
// ActorID may be nil for an unrecognized actor; TenantID must identify an existing
// verified tenant. Never invent a tenant or resolve one using a supplied email.
type Record struct {
	TenantID    string
	ActorID     *string
	Audience    string
	Action      string
	ResourceID  *string
	Outcome     string
	RequestID   string
	TraceID     string
	SafeChanges []SafeChange
}

// VerifiedReader is mapped from the result of live, locked Authorize by the
// consuming adapter. It is not a JWT claims object or an HTTP request DTO.
// The caller must keep those locks until List/Get/Export and its UOW finish.
type VerifiedReader struct {
	TenantID string
	ActorID  string
	Audience string
	Roles    []string
}

// GETQuery lists exactly the frozen HTTP query fields. ParseGETQuery validates
// syntax only; the HTTP layer verifies Cursor with the shared HMAC codec.
type GETQuery struct {
	From   string `form:"from"`
	To     string `form:"to"`
	Action string `form:"action"`
	Cursor string `form:"cursor"`
	Limit  string `form:"limit"`
}

// Boundary uses the full PostgreSQL timestamp precision, never second truncation.
// The transport binds it to tenant/actor/filter/sort in the shared HMAC cursor.
type Boundary struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type ListQuery struct {
	From            *time.Time
	To              *time.Time
	Action          string
	Limit           int
	BeforeCreatedAt *time.Time
	BeforeID        string
}

// Page is internal: the transport adds next_cursor using the final item when
// HasMore is true. Do not serialize this as the complete wire page.
type Page struct {
	Items   []Audit
	HasMore bool
}

const (
	ActionLogin               = "auth.login"
	ActionRegister            = "auth.register"
	ActionPasswordReset       = "auth.password_reset"
	ActionRefresh             = "auth.refresh"
	ActionLogout              = "auth.logout"
	ActionWSTicket            = "auth.ws_ticket"
	ActionRefreshReused       = "auth.refresh_reused"
	ActionCreateMember        = "platform.member_created"
	ActionUpdateMember        = "platform.member_updated"
	ActionResetPassword       = "platform.password_reset"
	ActionRevokeSessions      = "platform.sessions_revoked"
	ActionBootstrap           = "system.bootstrap"
	ActionPasswordChanged     = "auth.password_changed"
	ActionTenantSwitched      = "auth.tenant_switched"
	ActionMailSettingsUpdated = "platform.mail_settings_updated"
	ActionMailTestSent        = "platform.mail_test_sent"
)
