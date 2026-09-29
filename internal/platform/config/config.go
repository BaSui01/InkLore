package config

import (
	"InkLore/internal/platform/logger"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	strTrue     = "true"
	strPostgres = "postgres"
	strSqlite   = "sqlite"
)

// Load 从文件加载配置，并依次应用默认值、环境变量覆盖和运行时校验。
func Load(path string) (*Config, error) {
	if err := loadEnvFile(".env"); err != nil {
		if os.IsNotExist(err) {
			if errFallback := loadEnvFile("../.env"); errFallback != nil {
				if !os.IsNotExist(errFallback) {
					logger.Warn("failed to load fallback env file", "path", "../.env", "error", errFallback)
				}
			}
		} else {
			logger.Warn("failed to load env file", "path", ".env", "error", err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	if err := cfg.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("加载环境变量失败: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}
	return cfg, nil
}

// LoadFrom 从指定路径加载配置，保留兼容入口。
func LoadFrom(path string) (*Config, error) {
	return Load(path)
}

// IsDevelopment 报告当前是否为开发环境。
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction 报告当前是否为生产环境。
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// GetSessionTTL 返回会话 TTL。
func (c *Config) GetSessionTTL() time.Duration {
	return time.Duration(c.Auth.SessionTTL) * time.Second
}

// GetAccessTokenTTL 返回 Access Token TTL。
func (c *Config) GetAccessTokenTTL() time.Duration {
	if c.Auth.AccessTokenTTL > 0 {
		return time.Duration(c.Auth.AccessTokenTTL) * time.Second
	}
	return c.GetSessionTTL()
}

// GetRefreshTokenTTL 返回 Refresh Token TTL。
func (c *Config) GetRefreshTokenTTL() time.Duration {
	if c.Auth.RefreshTokenTTL > 0 {
		return time.Duration(c.Auth.RefreshTokenTTL) * time.Second
	}
	return c.GetSessionTTL()
}

// GetTaskTimeout 返回队列任务超时时间。
func (c *Config) GetTaskTimeout() time.Duration {
	return time.Duration(c.Queue.TaskTimeout) * time.Second
}

// GetAPITimeout 返回上游 API 默认超时时间。
func (c *Config) GetAPITimeout() time.Duration {
	return time.Duration(c.API.DefaultTimeout) * time.Second
}

// GetConnMaxLifetime 返回数据库连接最大生命周期。
func (c *Config) GetConnMaxLifetime() time.Duration {
	return time.Duration(c.Database.ConnMaxLifetime) * time.Second
}

// GetConnMaxIdleTime 返回数据库连接最大空闲生命周期。
func (c *Config) GetConnMaxIdleTime() time.Duration {
	return time.Duration(c.Database.ConnMaxIdleTime) * time.Second
}

// GetSlowThreshold 返回数据库慢查询日志阈值。
func (c *Config) GetSlowThreshold() time.Duration {
	return time.Duration(c.Database.SlowThresholdMs) * time.Millisecond
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
