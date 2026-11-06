package infrastructure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
)

const EncryptedSecretPrefix = "enc:v1:"

var envNameRegex = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// getMasterKey 返回派生自服务端配置的 32 字节 AES-256 密钥
func getMasterKey() ([]byte, error) {
	pepper := os.Getenv("LLM_SECRET_PEPPER")
	if len(pepper) == 0 {
		pepper = os.Getenv("ENCRYPTION_KEY")
	}
	if len(pepper) < 32 {
		return nil, errors.New("llm secret encryption key is not configured")
	}
	h := sha256.Sum256([]byte("inklore_llm_secret_v1:" + pepper))
	return h[:], nil
}

// EncryptSecret 使用 AES-256-GCM 算法加密明文密钥
func EncryptSecret(plaintext string) (string, error) {
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return "", errors.New("cannot encrypt empty secret")
	}
	if strings.HasPrefix(trimmed, EncryptedSecretPrefix) {
		return trimmed, nil
	}

	key, err := getMasterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(trimmed), nil)
	return EncryptedSecretPrefix + base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts legacy ciphertext using an explicitly configured key.
// Unrecognized input is never treated as credential plaintext.
func DecryptSecret(encrypted string) (string, error) {
	trimmed := strings.TrimSpace(encrypted)
	if !strings.HasPrefix(trimmed, EncryptedSecretPrefix) {
		return "", errors.New("encrypted secret required")
	}

	rawB64 := strings.TrimPrefix(trimmed, EncryptedSecretPrefix)
	data, err := base64.RawURLEncoding.DecodeString(rawB64)
	if err != nil {
		return "", err
	}

	key, err := getMasterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("malformed encrypted secret payload")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("decrypt secret failed: authentication tag mismatch or corrupted")
	}

	return string(plaintext), nil
}

// IsEncryptedSecret 判断字符串是否为系统加密密文
func IsEncryptedSecret(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), EncryptedSecretPrefix)
}

// IsMaskedSecret 判断字符串是否为前端脱敏展示串（包含连续4个*）
func IsMaskedSecret(s string) bool {
	return strings.Contains(s, "****")
}

// MaskSecret never decrypts values or guesses whether uppercase content is a
// reference. Only a typed, validated env reference may be displayed separately.
func MaskSecret(secret string) string {
	val := strings.TrimSpace(secret)
	if val == "" {
		return ""
	}
	return "****"
}
