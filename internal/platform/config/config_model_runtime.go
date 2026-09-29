package config

import (
	"os"
	"strconv"
)

const DefaultImageUpstreamGlobalRateLimitRPM = 3600

const StrictGatewayOnlyEnv = "STRICT_GATEWAY_ONLY"

// ModelRuntimeConfig controls distributed provider execution limits.
type ModelRuntimeConfig struct {
	ImageGlobalRateLimitRPM int `yaml:"image_global_rate_limit_rpm"`
	// StrictGatewayOnly 禁止未显式注入网关时回退到直连媒体客户端。
	StrictGatewayOnly bool `yaml:"strict_gateway_only"`
}

// ResolveStrictGatewayOnly 返回模型运行时唯一的 strict gateway policy。
//
// 显式传入 Config 时以已经完成 YAML、默认值和环境覆盖的配置为准；兼容
// 旧无参调用时才读取环境变量。这样配置加载和旧入口共享同一个布尔解析
// 规则，也不会为了跨包传递配置引入全局可变状态。
func ResolveStrictGatewayOnly(cfg *Config) bool {
	if cfg != nil {
		return cfg.ModelRuntime.StrictGatewayOnly
	}
	enabled, _ := strictGatewayOnlyEnv()
	return enabled
}

// StrictGatewayOnlyEnabled 返回当前配置中的 strict gateway policy。
func (c *Config) StrictGatewayOnlyEnabled() bool {
	return ResolveStrictGatewayOnly(c)
}

func strictGatewayOnlyEnv() (bool, bool) {
	raw, ok := os.LookupEnv(StrictGatewayOnlyEnv)
	if !ok {
		return false, false
	}
	return parseBoolEnv(raw), true
}

func (c *Config) setModelRuntimeDefaults() {
	if c.ModelRuntime.ImageGlobalRateLimitRPM <= 0 {
		c.ModelRuntime.ImageGlobalRateLimitRPM = DefaultImageUpstreamGlobalRateLimitRPM
	}
}

func (c *Config) loadModelRuntimeEnv() {
	raw := os.Getenv("IMAGE_UPSTREAM_GLOBAL_RATE_LIMIT_RPM")
	if value, err := strconv.Atoi(raw); err == nil && value > 0 {
		c.ModelRuntime.ImageGlobalRateLimitRPM = value
	}
	if enabled, ok := strictGatewayOnlyEnv(); ok {
		c.ModelRuntime.StrictGatewayOnly = enabled
	}
}
