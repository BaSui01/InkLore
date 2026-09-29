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
func getMasterKey() []byte {
	pepper := os.Getenv("LLM_SECRET_PEPPER")
	if len(pepper) == 0 {
		pepper = os.Getenv("ENCRYPTION_KEY")
	}
	if len(pepper) == 0 {
		pepper = "inklore_default_secure_vault_master_key_pepper_2026"
	}
	h := sha256.Sum256([]byte("inklore_llm_secret_v1:" + pepper))
	return h[:]
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

	key := getMasterKey()
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

// DecryptSecret 使用 AES-256-GCM 算法解密密文密钥，若非密文则原样返回
func DecryptSecret(encrypted string) (string, error) {
	trimmed := strings.TrimSpace(encrypted)
	if !strings.HasPrefix(trimmed, EncryptedSecretPrefix) {
		return trimmed, nil
	}

	rawB64 := strings.TrimPrefix(trimmed, EncryptedSecretPrefix)
	data, err := base64.RawURLEncoding.DecodeString(rawB64)
	if err != nil {
		return "", err
	}

	key := getMasterKey()
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

// MaskSecret 对密钥进行脱敏处理，用于安全回显给前端
func MaskSecret(secret string) string {
	val := strings.TrimSpace(secret)
	if val == "" {
		return ""
	}
	// 如果是密文，先尝试解密出真实明文再脱敏
	if IsEncryptedSecret(val) {
		decrypted, err := DecryptSecret(val)
		if err == nil && decrypted != "" {
			val = decrypted
		} else {
			return "sk-****"
		}
	}
	// 如果是环境变量引用名，直接返回引用名
	if envNameRegex.MatchString(val) {
		return val
	}
	// 对明文 API Key 进行脱敏
	length := len(val)
	if length <= 6 {
		return "****"
	}
	if strings.HasPrefix(val, "sk-proj-") && length > 16 {
		return "sk-proj-****" + val[length-4:]
	}
	if strings.HasPrefix(val, "sk-") && length > 10 {
		return "sk-****" + val[length-4:]
	}
	if length > 12 {
		return val[:3] + "****" + val[length-4:]
	}
	return val[:2] + "****" + val[length-2:]
}
