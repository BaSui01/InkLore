package mail

import (
	"context"
	"errors"
	stdmail "net/mail"
	"strings"
	"unicode/utf8"

	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	plog "InkLore/internal/platform/logger"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func New(manager *uow.Manager, options Options) (*Service, error) {
	if manager == nil || options.Identity == nil {
		return nil, errors.New("mail requires identity and PostgreSQL")
	}
	for _, key := range options.Keys {
		if len(key) != 32 {
			return nil, errors.New("mail encryption key must contain 32 bytes")
		}
	}
	return &Service{manager: manager, options: options, transport: newTransport(options)}, nil
}
func unavailable() error { return pe.Contract(503, "platform.dependency_unavailable", nil) }
func invalid() error     { return pe.Contract(400, "platform.validation_failed", nil) }
func validAddress(value string) bool {
	parsed, err := stdmail.ParseAddress(value)
	return err == nil && parsed.Address == value && len(value) <= 254 && !strings.ContainsAny(value, "\r\n\x00")
}
func (s *Service) Authorize(ctx context.Context, p identity.Principal) error {
	current, err := s.options.Identity.Authorize(ctx, p)
	if err != nil {
		return err
	}
	if current.Audience != "admin" {
		return pe.Contract(403, "auth.forbidden", nil)
	}
	for _, role := range current.GlobalRoles {
		if role == "super_admin" {
			return nil
		}
	}
	return pe.Contract(403, "auth.forbidden", nil)
}
func (s *Service) load(ctx context.Context, locked bool) (settingRow, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return settingRow{}, err
	}
	db = db.Session(&gorm.Session{Logger: logger.Discard})
	suffix := ""
	if locked {
		if _, err = s.manager.Require(ctx); err != nil {
			return settingRow{}, err
		}
		suffix = " FOR UPDATE"
	}
	var row settingRow
	result := db.Raw(`SELECT * FROM platform_mail_settings WHERE id=1` + suffix).Scan(&row)
	if result.Error != nil {
		return row, result.Error
	}
	if result.RowsAffected != 1 {
		return row, unavailable()
	}
	return row, nil
}
func (s *Service) Read(ctx context.Context, p identity.Principal) (Settings, error) {
	if err := s.Authorize(ctx, p); err != nil {
		return Settings{}, err
	}
	row, err := s.load(ctx, false)
	return row.summary(), err
}
func (s *Service) Enabled(ctx context.Context) (bool, error) {
	row, err := s.load(ctx, false)
	if err != nil {
		return false, err
	}
	if !row.Enabled {
		return false, nil
	}
	if _, err = s.credential(row); err != nil {
		return false, nil
	}
	host := row.summary().SMTPHost
	if row.Provider == "resend" {
		host = "api.resend.com"
	}
	return s.transport.allowed[host], nil
}
func (s *Service) Update(ctx context.Context, p identity.Principal, in UpdateInput) (Settings, error) {
	if in.ExpectedRevision < 1 || in.ExpectedRevision > scope.MaxRevision || in.Provider != "disabled" && in.Provider != "smtp" && in.Provider != "resend" {
		return Settings{}, invalid()
	}
	in.SMTPHost = strings.ToLower(strings.TrimSpace(in.SMTPHost))
	in.SenderEmail = strings.ToLower(strings.TrimSpace(in.SenderEmail))
	in.SMTPUsername = strings.TrimSpace(in.SMTPUsername)
	if utf8.RuneCountInString(in.SenderName) > 120 || len(in.SMTPUsername) > 254 || len(in.SMTPHost) > 253 || strings.ContainsAny(in.SenderName+in.SMTPUsername, "\r\n\x00") {
		return Settings{}, invalid()
	}
	if in.SenderEmail != "" && !validAddress(in.SenderEmail) {
		return Settings{}, invalid()
	}
	if in.SMTPHost != "" && !s.transport.allowed[in.SMTPHost] {
		return Settings{}, invalid()
	}
	if in.SMTPSecurity != "" && in.SMTPSecurity != "implicit_tls" && in.SMTPSecurity != "starttls" {
		return Settings{}, invalid()
	}
	if in.SMTPPort != 0 && (in.SMTPPort != 465 || in.SMTPSecurity != "implicit_tls") && (in.SMTPPort != 587 || in.SMTPSecurity != "starttls") {
		return Settings{}, invalid()
	}
	if in.Enabled && (in.Provider == "disabled" || in.SenderEmail == "" || in.Provider == "smtp" && (in.SMTPHost == "" || in.SMTPPort == 0 || in.SMTPUsername == "") || in.Provider == "resend" && !s.transport.allowed["api.resend.com"]) {
		return Settings{}, invalid()
	}
	var out Settings
	err := s.manager.Within(ctx, func(tx context.Context) error {
		if err := s.Authorize(tx, p); err != nil {
			return err
		}
		current, err := s.load(tx, true)
		if err != nil {
			return err
		}
		if current.Revision != in.ExpectedRevision || current.Revision >= scope.MaxRevision {
			return pe.Contract(409, "platform.version_conflict", map[string]any{"current_revision": current.Revision})
		}
		if current.Provider != in.Provider && current.CredentialKind != nil && in.Credential == nil {
			return invalid()
		}
		row := current
		row.Provider = in.Provider
		row.Enabled = in.Enabled
		if in.Credential != nil {
			change := in.Credential
			switch change.Kind {
			case "env":
				if !locatorPattern.MatchString(change.Locator) || change.Value != "" {
					return invalid()
				}
				row.CredentialKind = &change.Kind
				row.CredentialLocator = &change.Locator
				row.CredentialCiphertext = nil
				row.CredentialKeyVersion = nil
			case "encrypted":
				if change.Locator != "" || len(change.Value) < 1 || len(change.Value) > 8192 {
					return invalid()
				}
				ciphertext, version, err := s.encrypt(in.Provider, change.Value)
				if err != nil {
					return unavailable()
				}
				row.CredentialKind = &change.Kind
				row.CredentialLocator = nil
				row.CredentialCiphertext = &ciphertext
				row.CredentialKeyVersion = &version
			default:
				return invalid()
			}
		}
		if in.Enabled {
			if _, err := s.credential(row); err != nil {
				return unavailable()
			}
		}
		db, err := s.manager.Require(tx)
		if err != nil {
			return err
		}
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		result := db.Exec(`UPDATE platform_mail_settings SET provider=?,enabled=?,smtp_host=NULLIF(?,''),smtp_port=NULLIF(?,0),smtp_security=NULLIF(?,''),smtp_username=NULLIF(?,''),sender_email=NULLIF(?,''),sender_name=NULLIF(?,''),credential_kind=?,credential_locator=?,credential_ciphertext=?,credential_key_version=?,revision=revision+1,updated_by=?,updated_at=clock_timestamp() WHERE id=1 AND revision=?`, in.Provider, in.Enabled, in.SMTPHost, in.SMTPPort, in.SMTPSecurity, in.SMTPUsername, in.SenderEmail, in.SenderName, row.CredentialKind, row.CredentialLocator, row.CredentialCiphertext, row.CredentialKeyVersion, p.UserID, in.ExpectedRevision)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return pe.Contract(409, "platform.version_conflict", nil)
		}
		if s.options.OnEvent != nil {
			if err = s.options.OnEvent(tx, Event{Principal: p, Action: "platform.mail_settings_updated", Revision: in.ExpectedRevision + 1}); err != nil {
				return err
			}
		}
		updated, err := s.load(tx, false)
		out = updated.summary()
		return err
	})
	return out, err
}

// SendVerification never logs or returns the recipient, OTP or upstream payload.
// It must execute outside a business transaction; the caller persists delivery state.
func (s *Service) SendVerification(ctx context.Context, id, recipient, purpose, code string) error {
	if uow.InTransaction(ctx) {
		return errors.New("email delivery must not hold a database transaction")
	}
	if !scope.ValidID(id) || !validAddress(recipient) || len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return invalid()
	}
	// Only a fixed label may enter this trusted authentication message. Never
	// interpolate the HTTP purpose value into delivered content, even after the
	// identity layer has validated its allowed values.
	var operation string
	switch purpose {
	case "login":
		operation = "sign-in"
	case "register":
		operation = "registration"
	case "reset":
		operation = "password-reset"
	default:
		return invalid()
	}
	row, err := s.load(ctx, false)
	if err != nil {
		return err
	}
	if !row.Enabled {
		return unavailable()
	}
	secret, err := s.credential(row)
	if err != nil {
		return unavailable()
	}
	body := "InkLore verification code: " + code + "\n\nThis code expires in 5 minutes and can be used once for the requested " + operation + " operation. It does not confirm that an account exists. If you did not request it, ignore this email."
	if err = s.transport.send(ctx, row.summary(), secret, Message{ID: id, To: recipient, Subject: "InkLore verification code", Text: body}); err != nil {
		plog.WarnContext(ctx, "mail.delivery_failed", "domain", "platform", "operation", "mail.send_verification", "error_code", "platform.dependency_unavailable", "provider", row.Provider, "stage", err.Error())
		return unavailable()
	}
	return nil
}
