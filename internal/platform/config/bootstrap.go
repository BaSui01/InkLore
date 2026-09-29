package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"InkLore/internal/platform/logger"

	"gopkg.in/yaml.v3"
)

// DefaultPath 是运行配置文件的默认位置。
const DefaultPath = "configs/config.yaml"

// Bootstrap loads configuration consumed by the runtime. Retired
// encryption, auth-center and object-store settings cannot gate startup.
// Secrets are resolved from the process environment by their owning adapters.
func Bootstrap(path string) (*Config, error) {
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
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}
	cfg := &Config{}
	raw, err := os.ReadFile(path)
	if err == nil {
		if yaml.Unmarshal(raw, cfg) != nil {
			return nil, fmt.Errorf("invalid configuration YAML")
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot read configuration")
	}
	if value := os.Getenv("APP_ENV"); value != "" {
		cfg.App.Env = value
	}
	if cfg.App.Env == "" {
		cfg.App.Env = "development"
	}
	switch cfg.App.Env {
	case "development", "production", "test":
	default:
		return nil, fmt.Errorf("APP_ENV must be development, production or test")
	}
	if os.IsNotExist(err) && cfg.IsProduction() {
		return nil, fmt.Errorf("production requires a mounted configuration file")
	}
	if value := os.Getenv("APP_PORT"); value != "" {
		if p, err := strconv.Atoi(value); err == nil && p >= 1 && p <= 65535 {
			cfg.App.Port = p
		}
	}
	if value := os.Getenv("APP_HOST"); value != "" {
		cfg.App.Host = value
	}
	if cfg.App.Port == 0 {
		cfg.App.Port = 8080
	}
	if cfg.App.Host == "" {
		cfg.App.Host = "0.0.0.0"
	}
	if cfg.App.Port < 1 || cfg.App.Port > 65535 {
		return nil, fmt.Errorf("invalid HTTP port")
	}
	if value := os.Getenv("DATABASE_DSN"); value != "" {
		cfg.Database.DSN = value
	}
	if cfg.Database.Driver != "" && cfg.Database.Driver != "postgres" {
		return nil, fmt.Errorf("runtime requires PostgreSQL")
	}
	if value, present := os.LookupEnv("API_TRUSTED_PROXY_CIDRS"); present {
		cfg.RateLimit.TrustedProxyCIDRs = splitCSV(value)
	}
	cfg.Database.Driver = "postgres"
	return cfg, nil
}
