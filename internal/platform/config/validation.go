package config

import (
	"errors"
	"fmt"
	"os"

	"InkLore/internal/platform/logger"
)

// Validate 验证配置的有效性
func (c *Config) Validate() error {
	// 验证加密密钥，支持从环境变量读取
	if c.Security.EncryptionKey == "" {
		// 尝试从环境变量获取
		envKey := os.Getenv("ENCRYPTION_KEY")
		if envKey == "" {
			return errors.New("必须设置加密密钥：通过环境变量 ENCRYPTION_KEY 或配置文件 security.encryption_key")
		}
		c.Security.EncryptionKey = envKey
	}

	if len(c.Security.EncryptionKey) != 32 {
		return fmt.Errorf("加密密钥长度必须为 32 字节，当前为 %d 字节", len(c.Security.EncryptionKey))
	}

	// JWT 密钥：优先使用配置，其次回退到 encryption_key
	if c.Auth.JWTSecret == "" {
		c.Auth.JWTSecret = c.Security.EncryptionKey
	}
	if c.Auth.JWTSecret == "" {
		return errors.New("必须设置 JWT 密钥：auth.jwt_secret 或环境变量 JWT_SECRET")
	}
	if len(c.Auth.JWTSecret) < 16 {
		return fmt.Errorf("JWT 密钥过短：至少 16 字节，当前 %d 字节", len(c.Auth.JWTSecret))
	}

	// 验证 Access/Refresh Token TTL
	if c.Auth.AccessTokenTTL <= 0 {
		return fmt.Errorf("access_token_ttl 必须为正整数秒，当前 %d", c.Auth.AccessTokenTTL)
	}
	if c.Auth.RefreshTokenTTL <= 0 {
		return fmt.Errorf("refresh_token_ttl 必须为正整数秒，当前 %d", c.Auth.RefreshTokenTTL)
	}

	if err := c.validateCookiePrefixContract(); err != nil {
		return err
	}

	// 验证生产环境安全配置
	if c.IsProduction() {
		if !c.Auth.CookieSecure {
			return errors.New("生产环境必须启用 CookieSecure")
		}

		// 警告：Redis 密码为空
		if c.Redis.Enabled && c.Redis.Password == "" {
			logger.Warn("production Redis password is empty")
		}
	}

	return nil
}

// ValidateOrPanic 验证配置，失败则 panic
func (c *Config) ValidateOrPanic() {
	if err := c.Validate(); err != nil {
		panic(fmt.Sprintf("配置验证失败: %v", err))
	}
}
