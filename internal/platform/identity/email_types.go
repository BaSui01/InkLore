package identity

import (
	"bytes"
	"context"
	"errors"
	"time"
)

// EmailDelivery is the narrow transport port; identity owns codes and consumption.
// SendVerification returns nil only after a real provider accepts the message.
type EmailDelivery interface {
	Enabled(context.Context) (bool, error)
	SendVerification(context.Context, string, string, string, string) error
}
type EmailCapabilities struct {
	Enabled    bool `json:"enabled"`
	CodeLength int  `json:"code_length"`
	ExpiresIn  int  `json:"expires_in"`
	RetryAfter int  `json:"retry_after_seconds"`
}
type EmailChallengeInput struct {
	Email      string `json:"email"`
	Purpose    string `json:"purpose"`
	Audience   string `json:"audience"`
	TenantID   string `json:"tenant_id,omitempty"`
	RequestKey string `json:"-"`
	ClientIP   string `json:"-"`
}
type EmailChallenge struct {
	ChallengeID string    `json:"challenge_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	RetryAfter  int       `json:"retry_after_seconds"`
}
type EmailLoginInput struct {
	CaptchaID   string `json:"captcha_id,omitempty"`
	CaptchaCode string `json:"captcha_code,omitempty"`
	Email       string `json:"email"`
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
	Audience    string `json:"audience"`
	TenantID    string `json:"tenant_id,omitempty"`
	ClientIP    string `json:"-"`
}
type ResetPasswordInput struct {
	EmailLoginInput
	NewPassword string `json:"new_password"`
}

// ConfigureEmail is startup-only. The OTP pepper is independent of token keys;
// losing it invalidates outstanding challenges, and it must be shared by replicas.
func (s *Service) ConfigureEmail(delivery EmailDelivery, pepper []byte) error {
	if len(pepper) != 0 && (len(pepper) < 32 || bytes.Equal(pepper, s.jwtKey) || bytes.Equal(pepper, s.csrfKey)) {
		return errors.New("email OTP requires an independent key of at least 32 bytes")
	}
	s.emailDelivery = delivery
	s.emailPepper = bytes.Clone(pepper)
	return nil
}
func (s *Service) EmailCapabilities(ctx context.Context) (EmailCapabilities, error) {
	out := EmailCapabilities{CodeLength: 6, ExpiresIn: 300, RetryAfter: 60}
	if s.emailDelivery == nil || len(s.emailPepper) < 32 {
		return out, nil
	}
	var err error
	out.Enabled, err = s.emailDelivery.Enabled(ctx)
	return out, err
}
