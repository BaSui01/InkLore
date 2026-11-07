package mail

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var locatorPattern = regexp.MustCompile(`^MAIL_CREDENTIAL_[A-Z0-9_]{1,80}$`)

// EnvironmentOptions deliberately has no fallback encryption key or host wildcard.
// Historical key versions must remain available until all ciphertext is rotated.
func EnvironmentOptions() (Options, error) {
	o := Options{Keys: map[int][]byte{}}
	if raw := os.Getenv("MAIL_ENCRYPTION_KEYS"); raw != "" {
		var values map[string]string
		if json.Unmarshal([]byte(raw), &values) != nil {
			return o, errors.New("invalid MAIL_ENCRYPTION_KEYS")
		}
		for version, encoded := range values {
			v, err := strconv.Atoi(version)
			if err != nil || v < 1 {
				return o, errors.New("invalid mail key version")
			}
			key, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil || len(key) != 32 {
				return o, errors.New("mail encryption keys must be 32-byte base64 values")
			}
			o.Keys[v] = key
		}
		v, err := strconv.Atoi(os.Getenv("MAIL_ENCRYPTION_KEY_VERSION"))
		if err != nil || len(o.Keys[v]) != 32 {
			return o, errors.New("MAIL_ENCRYPTION_KEY_VERSION must identify an available key")
		}
		o.ActiveKeyVersion = v
	}
	for _, host := range strings.Split(os.Getenv("MAIL_ALLOWED_HOSTS"), ",") {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			o.AllowedHosts = append(o.AllowedHosts, host)
		}
	}
	return o, nil
}
func (s *Service) encrypt(provider, value string) (string, int, error) {
	key := s.options.Keys[s.options.ActiveKeyVersion]
	if len(key) != 32 {
		return "", 0, errors.New("mail encryption key is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", 0, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", 0, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", 0, err
	}
	aad := []byte(fmt.Sprintf("inklore/mail/%s/%d", provider, s.options.ActiveKeyVersion))
	sealed := aead.Seal(nonce, nonce, []byte(value), aad)
	return "enc:v1:" + base64.StdEncoding.EncodeToString(sealed), s.options.ActiveKeyVersion, nil
}
func (s *Service) credential(r settingRow) (string, error) {
	if r.CredentialKind == nil {
		return "", errors.New("mail credential is unavailable")
	}
	if *r.CredentialKind == "env" && r.CredentialLocator != nil && locatorPattern.MatchString(*r.CredentialLocator) {
		secret := os.Getenv(*r.CredentialLocator)
		if len(secret) == 0 || len(secret) > 8192 {
			return "", errors.New("mail credential is unavailable")
		}
		return secret, nil
	}
	if *r.CredentialKind != "encrypted" || r.CredentialCiphertext == nil || r.CredentialKeyVersion == nil {
		return "", errors.New("invalid mail credential")
	}
	key := s.options.Keys[*r.CredentialKeyVersion]
	if len(key) != 32 {
		return "", errors.New("mail decryption key is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	encoded := strings.TrimPrefix(*r.CredentialCiphertext, "enc:v1:")
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(sealed) < aead.NonceSize()+aead.Overhead() {
		return "", errors.New("invalid encrypted mail credential")
	}
	aad := []byte(fmt.Sprintf("inklore/mail/%s/%d", r.Provider, *r.CredentialKeyVersion))
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], aad)
	if err != nil {
		return "", errors.New("mail credential authentication failed")
	}
	return string(plain), nil
}
