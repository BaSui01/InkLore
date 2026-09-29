package config

import (
	"fmt"
	"strings"
	"time"
)

type AuthCenterAuthorizationConfig struct {
	Mode              string   `yaml:"mode"`
	ServiceToken      string   `yaml:"service_token"`
	CheckPath         string   `yaml:"check_path"`
	ScopePath         string   `yaml:"scope_path"`
	VersionsPath      string   `yaml:"versions_path"`
	Timeout           string   `yaml:"timeout"`
	CacheTTL          string   `yaml:"cache_ttl"`
	ShadowPermissions []string `yaml:"shadow_permissions"`
	// AllowLegacyBodyContext is an explicit short-lived compatibility escape
	// hatch. The secure default uses Auth Center v2 Actor JWT binding.
	AllowLegacyBodyContext bool `yaml:"allow_legacy_body_context"`
	// InvalidationWebhookEnabled exposes the internal invalidation webhook.
	// Default false: production remains legacy decision only.
	InvalidationWebhookEnabled bool `yaml:"invalidation_webhook_enabled"`
}

func (c *Config) applyAuthCenterUserRolesDefaults() {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.AuthCenter.UserRoles.PathTemplate) == "" {
		// Prefer Auth Center M2M internal API; admin /admin/users/{id}/roles requires cookie session.
		c.AuthCenter.UserRoles.PathTemplate = "/api/internal/users/{id}/roles"
	}
	if strings.TrimSpace(c.AuthCenter.UserRoles.Timeout) == "" {
		c.AuthCenter.UserRoles.Timeout = "3s"
	}
	// Reuse authorization service token when dedicated user-roles token is empty.
	if strings.TrimSpace(c.AuthCenter.UserRoles.ServiceToken) == "" {
		c.AuthCenter.UserRoles.ServiceToken = strings.TrimSpace(c.AuthCenter.Authorization.ServiceToken)
	}
}

func (c *Config) validateAuthCenterAuthorization() error {
	cfg := c.AuthCenter.Authorization
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode != "off" && mode != "shadow" && mode != "enforce" {
		return fmt.Errorf("Auth Center authorization mode 无效: %s", cfg.Mode)
	}
	if cfg.InvalidationWebhookEnabled && strings.TrimSpace(cfg.ServiceToken) == "" {
		return fmt.Errorf("Auth Center authorization 失效回调启用时必须配置 service_token")
	}
	if mode == "off" {
		return nil
	}
	if strings.TrimSpace(c.AuthCenter.BaseURL) == "" || strings.TrimSpace(cfg.ServiceToken) == "" {
		return fmt.Errorf("Auth Center authorization %s 模式必须配置 base_url 和 service_token", mode)
	}
	for name, path := range map[string]string{"check_path": cfg.CheckPath, "scope_path": cfg.ScopePath, "versions_path": cfg.VersionsPath} {
		if !strings.HasPrefix(strings.TrimSpace(path), "/") {
			return fmt.Errorf("Auth Center authorization %s 必须是绝对路径", name)
		}
	}
	if !cfg.AllowLegacyBodyContext {
		expectedPaths := map[string]string{
			"check_path":    "/api/v2/authorization/check",
			"scope_path":    "/api/v2/authorization/scope",
			"versions_path": "/api/v2/authorization/versions",
		}
		for name, expected := range expectedPaths {
			actual := strings.TrimSpace(map[string]string{
				"check_path":    cfg.CheckPath,
				"scope_path":    cfg.ScopePath,
				"versions_path": cfg.VersionsPath,
			}[name])
			if actual != expected {
				return fmt.Errorf("Auth Center authorization strict actor 模式要求 %s=%s", name, expected)
			}
		}
	} else {
		expectedPaths := map[string]string{
			"check_path":    "/api/authorization/check",
			"scope_path":    "/api/authorization/scope",
			"versions_path": "/api/authorization/versions",
		}
		for name, expected := range expectedPaths {
			actual := strings.TrimSpace(map[string]string{
				"check_path":    cfg.CheckPath,
				"scope_path":    cfg.ScopePath,
				"versions_path": cfg.VersionsPath,
			}[name])
			if actual != expected {
				return fmt.Errorf("Auth Center authorization legacy Body 模式要求 %s=%s", name, expected)
			}
		}
	}
	if mode == "enforce" && cfg.AllowLegacyBodyContext {
		return fmt.Errorf("Auth Center authorization enforce 模式禁止 allow_legacy_body_context")
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(cfg.Timeout))
	if err != nil || timeout <= 0 {
		return fmt.Errorf("Auth Center authorization timeout 无效: %s", cfg.Timeout)
	}
	cacheTTL, err := time.ParseDuration(strings.TrimSpace(cfg.CacheTTL))
	if err != nil || cacheTTL <= 0 || cacheTTL > 30*time.Second {
		return fmt.Errorf("Auth Center authorization cache_ttl 必须大于 0 且不超过 30s")
	}
	return nil
}
