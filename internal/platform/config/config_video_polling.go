package config

import (
	"os"
	"strconv"
)

func (c *Config) setVideoPollingDefaults() {
	if c.BackgroundTask.VideoPollingEnabled == nil {
		enabled := true
		c.BackgroundTask.VideoPollingEnabled = &enabled
	}
	if c.BackgroundTask.VideoPollingIntervalSeconds <= 0 {
		c.BackgroundTask.VideoPollingIntervalSeconds = 3
	}
	if c.BackgroundTask.VideoPollingBatchSize <= 0 {
		c.BackgroundTask.VideoPollingBatchSize = 50
	}
	if c.BackgroundTask.VideoGenerationTimeoutSeconds <= 0 {
		c.BackgroundTask.VideoGenerationTimeoutSeconds = 1800
	}
	if c.BackgroundTask.VideoRecoveryLookbackHours <= 0 {
		c.BackgroundTask.VideoRecoveryLookbackHours = 24
	}
	if c.BackgroundTask.VideoPollingLeaseSeconds <= 0 {
		c.BackgroundTask.VideoPollingLeaseSeconds = 60
	}
}

func (c *Config) loadVideoPollingEnv() {
	if value := os.Getenv("VIDEO_POLLING_ENABLED"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			c.BackgroundTask.VideoPollingEnabled = &parsed
		}
	}
	if value := os.Getenv("VIDEO_POLLING_INTERVAL_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			c.BackgroundTask.VideoPollingIntervalSeconds = parsed
		}
	}
	if value := os.Getenv("VIDEO_POLLING_BATCH_SIZE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			c.BackgroundTask.VideoPollingBatchSize = parsed
		}
	}
	if value := os.Getenv("VIDEO_GENERATION_TIMEOUT_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			c.BackgroundTask.VideoGenerationTimeoutSeconds = parsed
		}
	}
	if value := os.Getenv("VIDEO_RECOVERY_LOOKBACK_HOURS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			c.BackgroundTask.VideoRecoveryLookbackHours = parsed
		}
	}
	if value := os.Getenv("VIDEO_POLLING_LEASE_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			c.BackgroundTask.VideoPollingLeaseSeconds = parsed
		}
	}
}

func (c *Config) IsVideoPollingEnabled() bool {
	return c == nil || c.BackgroundTask.VideoPollingEnabled == nil || *c.BackgroundTask.VideoPollingEnabled
}
