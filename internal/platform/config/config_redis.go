package config

import (
	"fmt"
	"time"
)

// RedisConfig Redis缓存配置
type RedisConfig struct {
	Host         string        `yaml:"host"`
	Password     string        `yaml:"password"`
	Port         int           `yaml:"port"`
	DB           int           `yaml:"db"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	MaxRetries   int           `yaml:"max_retries"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	Enabled      bool          `yaml:"enabled"`
}

// GetRedisAddr 获取Redis地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
