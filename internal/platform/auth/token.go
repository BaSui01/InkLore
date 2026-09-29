package authdomain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"InkLore/internal/platform/timeutil"
)

// 平台自签令牌（HS256）。刻意保持最小实现，不引入外部 JWT 依赖：
//
//   - 只支持 HS256，显式拒绝 alg=none 等降级攻击；
//   - 签名比较使用 hmac.Equal（常数时间）；
//   - 密钥只从进程环境读取（ADR-0004），本文件不落盘、不打印密钥。
//
// 注意：这是"平台自签"的过渡形态。接入认证中心（OIDC）后应改为校验证书链，
// 届时本文件退化为兼容层。

const (
	// TokenIssuer 是平台自签令牌的默认签发者。
	TokenIssuer = "inklore"
	// DefaultTokenTTL 是默认有效期。
	DefaultTokenTTL = 24 * time.Hour
	// PrincipalSourcePlatformToken 标识主体来自平台自签令牌。
	PrincipalSourcePlatformToken = "platform_token"

	tokenAlgorithm = "HS256"
)

var (
	ErrTokenSecretMissing = errors.New("令牌签名密钥未配置")
	ErrTokenMalformed     = errors.New("令牌格式非法")
	ErrTokenSignature     = errors.New("令牌签名校验失败")
	ErrTokenExpired       = errors.New("令牌已过期")
	ErrTokenAlgorithm     = errors.New("令牌使用了不支持的签名算法")
)

// Claims 是平台自签令牌的载荷。
type Claims struct {
	Subject   string   `json:"sub"`
	TenantID  string   `json:"tid"`
	Roles     []string `json:"roles,omitempty"`
	Issuer    string   `json:"iss,omitempty"`
	UserID    int64    `json:"uid,omitempty"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}

// SignToken 使用 HS256 签发令牌。
func SignToken(claims Claims, secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", ErrTokenSecretMissing
	}
	claims.Subject = strings.TrimSpace(claims.Subject)
	if claims.Subject == "" {
		return "", fmt.Errorf("令牌缺少 sub（主体标识）")
	}
	if strings.TrimSpace(claims.Issuer) == "" {
		claims.Issuer = TokenIssuer
	}
	if claims.IssuedAt == 0 {
		claims.IssuedAt = timeutil.Now().Unix()
	}
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = claims.IssuedAt + int64(DefaultTokenTTL/time.Second)
	}
	if claims.ExpiresAt <= claims.IssuedAt {
		return "", fmt.Errorf("令牌有效期非法：exp 必须晚于 iat")
	}

	headerBytes, err := json.Marshal(map[string]string{"alg": tokenAlgorithm, "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(payloadBytes)
	return signingInput + "." + sign(signingInput, secret), nil
}

// ParseToken 校验签名、算法与有效期并返回声明。
func ParseToken(token string, secret string) (Claims, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Claims{}, ErrTokenSecretMissing
	}

	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenMalformed
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if header.Alg != tokenAlgorithm {
		return Claims{}, ErrTokenAlgorithm
	}

	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	expected, err := base64.RawURLEncoding.DecodeString(sign(parts[0]+"."+parts[1], secret))
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if !hmac.Equal(expected, provided) {
		return Claims{}, ErrTokenSignature
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if claims.ExpiresAt > 0 && timeutil.Now().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrTokenExpired
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return Claims{}, ErrTokenMalformed
	}
	return claims, nil
}

func sign(signingInput string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// PrincipalFromClaims 把令牌声明转换为平台主体。
func PrincipalFromClaims(claims Claims) Principal {
	return Principal{
		Source:   PrincipalSourcePlatformToken,
		Subject:  strings.TrimSpace(claims.Subject),
		TenantID: strings.TrimSpace(claims.TenantID),
		UserID:   claims.UserID,
		Roles:    append([]string(nil), claims.Roles...),
	}
}
