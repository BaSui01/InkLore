// Package identity implements identities independently of legacy auth tokens.
package identity

import (
	"InkLore/internal/platform/uow"
	"context"
	"time"
)

type Principal struct {
	UserID             string   `json:"user_id"`
	TenantID           string   `json:"tenant_id"`
	DisplayName        string   `json:"display_name"`
	Roles              []string `json:"roles"`
	GlobalRoles        []string `json:"global_roles"`
	Audience           string   `json:"audience"`
	SessionID          string   `json:"session_id"`
	UserRevision       int64    `json:"user_revision"`
	MembershipRevision int64    `json:"-"`
}
type LoginInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	TenantID string `json:"tenant_id"`
	Audience string `json:"audience"`
	ClientIP string `json:"-"`
}
type RegisterInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	PenName     string `json:"pen_name"`
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
	ClientIP    string `json:"-"`
}
type Session struct {
	AccessToken      string    `json:"access_token"`
	ExpiresIn        int       `json:"expires_in"`
	CSRFToken        string    `json:"csrf_token"`
	Principal        Principal `json:"principal"`
	RefreshToken     string    `json:"-"`
	RefreshExpiresAt time.Time `json:"-"`
}
type CSRF struct {
	CSRFToken string `json:"csrf_token"`
}
type LogoutResult struct {
	Revoked bool `json:"revoked"`
}
type Ticket struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

type TenantSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LoginLimiter receives a SHA256 digest, never the IP or email plaintext.
type LoginLimiter interface {
	Allow(context.Context, string) (bool, error)
}

// Event contains only safe identifiers; OnEvent executes inside the transaction.
// A hook must never retain ctx or emit credentials. Composition owns audit storage.
type Event struct {
	Principal                               Principal
	ActorID, TenantID, Audience, ResourceID string
	Action, Outcome                         string
}
type Options struct {
	JWTKey  []byte
	CSRFKey []byte
	Limiter LoginLimiter
	OnEvent func(context.Context, Event) error
}
type Service struct {
	manager         *uow.Manager
	jwtKey, csrfKey []byte
	limiter         LoginLimiter
	memory          *memoryLimiter
	dummyHash       []byte
	onEvent         func(context.Context, Event) error
	emailDelivery   EmailDelivery
	emailPepper     []byte
}
