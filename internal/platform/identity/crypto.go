package identity

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"

	"golang.org/x/crypto/bcrypt"
)

const PasswordCost = 12
const AccessLifetime = 15 * time.Minute
const SessionLifetime = 30 * 24 * time.Hour

func New(manager *uow.Manager, options Options) (*Service, error) {
	if manager == nil {
		return nil, errors.New("identity: transaction manager required")
	}
	if len(options.JWTKey) < 32 || len(options.CSRFKey) < 32 || bytes.Equal(options.JWTKey, options.CSRFKey) {
		return nil, errors.New("identity: distinct JWT and CSRF keys of at least 32 bytes required")
	}
	dummy, err := randomToken()
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(dummy), PasswordCost)
	if err != nil {
		return nil, err
	}
	return &Service{manager: manager, jwtKey: bytes.Clone(options.JWTKey), csrfKey: bytes.Clone(options.CSRFKey), limiter: options.Limiter, memory: newMemoryLimiter(), dummyHash: hash, onEvent: options.OnEvent, resets: newResetStore()}, nil
}
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 6 || len(password) > 72 {
		return invalid()
	}
	return nil
}
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
	return string(hash), err
}
func invalid() error              { return apperrors.Contract(400, "platform.validation_failed", nil) }
func unauthenticated() error      { return apperrors.Contract(401, "auth.unauthenticated", nil) }
func forbidden() error            { return apperrors.Contract(403, "auth.forbidden", nil) }
func credentials() error          { return apperrors.Contract(401, "auth.invalid_credentials", nil) }
func validAudience(a string) bool { return a == "creator" || a == "admin" }
func allowedAudience(p Principal) bool {
	if p.Audience == "admin" && hasGlobalRole(p, "super_admin") {
		return true
	}
	for _, role := range p.Roles {
		if p.Audience == "creator" && role == "creator" || p.Audience == "admin" && (role == "operator" || role == "auditor") {
			return true
		}
	}
	return false
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func validToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == token
}
func (s *Service) csrf(sessionID, audience string, createdAt time.Time) string {
	mac := hmac.New(sha256.New, s.csrfKey)
	mac.Write([]byte(sessionID + "\x00" + audience + "\x00" + createdAt.UTC().Format(time.RFC3339Nano)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type claims struct {
	Subject   string   `json:"sub"`
	TenantID  string   `json:"tid"`
	Audience  string   `json:"aud"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"session_id"`
	Expires   int64    `json:"exp"`
}

func (s *Service) sign(p Principal, now time.Time) (string, error) {
	payload, err := json.Marshal(claims{p.UserID, p.TenantID, p.Audience, p.Roles, p.SessionID, now.Add(AccessLifetime).Unix()})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.jwtKey)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *Service) verify(token string) (claims, error) {
	var c claims
	if len(token) > 8192 {
		return c, unauthenticated()
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, unauthenticated()
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, unauthenticated()
	}
	var h struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	decoder := json.NewDecoder(bytes.NewReader(header))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&h) != nil || h.Alg != "HS256" || h.Typ != "JWT" {
		return c, unauthenticated()
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return c, unauthenticated()
	}
	mac := hmac.New(sha256.New, s.jwtKey)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return c, unauthenticated()
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &c) != nil {
		return c, unauthenticated()
	}
	if !scope.ValidID(c.Subject) || !scope.ValidID(c.TenantID) || !scope.ValidID(c.SessionID) || !validAudience(c.Audience) || c.Expires <= time.Now().Unix() || c.Expires > time.Now().Add(AccessLifetime).Unix() || len(c.Roles) == 0 {
		return c, unauthenticated()
	}
	return c, nil
}
