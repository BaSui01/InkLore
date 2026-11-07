// Package mail owns global delivery configuration and SMTP/Resend transports.
package mail

import (
	"context"
	"crypto/x509"
	"time"

	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"
)

type Settings struct {
	Provider       string    `json:"provider"`
	Enabled        bool      `json:"enabled"`
	SMTPHost       string    `json:"smtp_host"`
	SMTPPort       int       `json:"smtp_port"`
	SMTPSecurity   string    `json:"smtp_security"`
	SMTPUsername   string    `json:"smtp_username"`
	SenderEmail    string    `json:"sender_email"`
	SenderName     string    `json:"sender_name"`
	HasCredential  bool      `json:"has_credential"`
	CredentialKind string    `json:"credential_kind"`
	Revision       int64     `json:"revision"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type CredentialChange struct {
	Kind    string `json:"kind"`
	Locator string `json:"locator,omitempty"`
	Value   string `json:"value,omitempty"`
}
type UpdateInput struct {
	ExpectedRevision int64             `json:"expected_revision"`
	Provider         string            `json:"provider"`
	Enabled          bool              `json:"enabled"`
	SMTPHost         string            `json:"smtp_host"`
	SMTPPort         int               `json:"smtp_port"`
	SMTPSecurity     string            `json:"smtp_security"`
	SMTPUsername     string            `json:"smtp_username"`
	SenderEmail      string            `json:"sender_email"`
	SenderName       string            `json:"sender_name"`
	Credential       *CredentialChange `json:"credential,omitempty"`
}
type Event struct {
	Principal identity.Principal
	Action    string
	Revision  int64
}
type Options struct {
	Identity         *identity.Service
	Keys             map[int][]byte
	ActiveKeyVersion int
	AllowedHosts     []string
	Timeout          time.Duration
	RootCAs          *x509.CertPool
	OnEvent          func(context.Context, Event) error
}
type Service struct {
	manager   *uow.Manager
	options   Options
	transport *transport
}
type Message struct{ ID, To, Subject, Text string }
type settingRow struct {
	ID                   int16
	Provider             string
	Enabled              bool
	SMTPHost             *string
	SMTPPort             *int
	SMTPSecurity         *string
	SMTPUsername         *string
	SenderEmail          *string
	SenderName           *string
	CredentialKind       *string
	CredentialLocator    *string
	CredentialCiphertext *string
	CredentialKeyVersion *int
	Revision             int64
	UpdatedAt            time.Time
}

func (r settingRow) summary() Settings {
	value := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	port := 0
	if r.SMTPPort != nil {
		port = *r.SMTPPort
	}
	return Settings{Provider: r.Provider, Enabled: r.Enabled, SMTPHost: value(r.SMTPHost), SMTPPort: port, SMTPSecurity: value(r.SMTPSecurity), SMTPUsername: value(r.SMTPUsername), SenderEmail: value(r.SenderEmail), SenderName: value(r.SenderName), HasCredential: r.CredentialKind != nil, CredentialKind: value(r.CredentialKind), Revision: r.Revision, UpdatedAt: r.UpdatedAt}
}
