// Package cursor signs scoped keyset cursors for the current list contract.
package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/idempotency"
)

type Binding struct {
	TenantID           string          `json:"tenant"`
	ActorID            string          `json:"actor"`
	Audience           string          `json:"audience"`
	Operation          string          `json:"operation"`
	PermissionRevision int64           `json:"permission_revision"`
	Filters            json.RawMessage `json:"filters"`
	Sort               string          `json:"sort"`
}

type Codec struct {
	secret []byte
	now    func() time.Time
}
type payload struct {
	Version int             `json:"v"`
	Binding string          `json:"binding"`
	Issued  int64           `json:"issued"`
	Expires int64           `json:"expires"`
	Key     json.RawMessage `json:"key"`
}

func New(secret []byte) (*Codec, error) {
	if len(secret) < 32 {
		return nil, errors.New("cursor: independent key must contain at least 32 bytes")
	}
	return &Codec{secret: append([]byte(nil), secret...), now: time.Now}, nil
}
func invalid() error { return platformerr.Contract(400, "platform.cursor_invalid", nil) }
func bindingHash(binding Binding) (string, error) {
	if len(binding.Filters) == 0 {
		binding.Filters = json.RawMessage("{}")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return "", invalid()
	}
	raw, err = idempotency.CanonicalJSON(raw)
	if err != nil {
		return "", invalid()
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func (c *Codec) sign(raw []byte) []byte {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write(raw)
	return mac.Sum(nil)
}
func (c *Codec) Encode(binding Binding, key any) (string, error) {
	hash, err := bindingHash(binding)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(key)
	if err != nil {
		return "", invalid()
	}
	now := c.now().UTC().Unix()
	body, err := json.Marshal(payload{Version: 1, Binding: hash, Issued: now, Expires: now + 86400, Key: raw})
	if err != nil {
		return "", invalid()
	}
	token := base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(c.sign(body))
	if len(token) > 2048 {
		return "", invalid()
	}
	return token, nil
}
func (c *Codec) Decode(token string, binding Binding, key any) error {
	if token == "" || len(token) > 2048 {
		return invalid()
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return invalid()
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return invalid()
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, c.sign(raw)) {
		return invalid()
	}
	var body payload
	if json.Unmarshal(raw, &body) != nil {
		return invalid()
	}
	hash, err := bindingHash(binding)
	if err != nil {
		return err
	}
	now := c.now().UTC().Unix()
	if body.Version != 1 || body.Binding != hash || body.Expires <= now || body.Issued > now || body.Expires-body.Issued != 86400 || len(body.Key) == 0 {
		return invalid()
	}
	if json.Unmarshal(body.Key, key) != nil {
		return invalid()
	}
	return nil
}
