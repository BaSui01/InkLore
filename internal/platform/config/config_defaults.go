package config

import (
	"os"
	"strings"
	"time"
)

// setDefaults 设置未显式配置项的运行时默认值。
func (c *Config) setDefaults() {
	if c.App.Port == 0 {
		c.App.Port = 8080
	}
	if c.App.Host == "" {
		c.App.Host = "0.0.0.0"
	}
	if c.App.Env == "" {
		c.App.Env = "development"
	}
	if c.App.HTTPReadTimeoutSeconds == 0 {
		c.App.HTTPReadTimeoutSeconds = 120
	}
	if c.App.HTTPWriteTimeoutSeconds == 0 {
		// 覆盖配置的 330 秒同步上游窗口及多轮 Agent 响应，并为响应写入留出余量。
		c.App.HTTPWriteTimeoutSeconds = 600
	}
	if c.App.HTTPIdleTimeoutSeconds == 0 {
		c.App.HTTPIdleTimeoutSeconds = 120
	}
	if c.AdminServer.Port == 0 {
		c.AdminServer.Port = 5003
	}
	if c.AdminServer.Host == "" {
		c.AdminServer.Host = "127.0.0.1"
	}
	if c.AdminServer.HTTPReadTimeoutSeconds == 0 {
		c.AdminServer.HTTPReadTimeoutSeconds = c.App.HTTPReadTimeoutSeconds
	}
	if c.AdminServer.HTTPWriteTimeoutSeconds == 0 {
		c.AdminServer.HTTPWriteTimeoutSeconds = c.App.HTTPWriteTimeoutSeconds
	}
	if c.AdminServer.HTTPIdleTimeoutSeconds == 0 {
		c.AdminServer.HTTPIdleTimeoutSeconds = c.App.HTTPIdleTimeoutSeconds
	}
	if c.AdminServer.SessionCookieName == "" {
		c.AdminServer.SessionCookieName = "sys_admin_session"
	}
	if !c.AdminServer.Enabled && os.Getenv("ADMIN_SERVER_ENABLED") != "false" {
		c.AdminServer.Enabled = true
	}
	if c.Database.Driver == "" {
		c.Database.Driver = strPostgres
	}
	if c.Database.Path == "" {
		c.Database.Path = "./data/db/app.db"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 80
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 40
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 300
	}
	if c.Database.ConnMaxIdleTime == 0 {
		c.Database.ConnMaxIdleTime = 300
	}
	if c.Database.SlowThresholdMs == 0 {
		c.Database.SlowThresholdMs = 200
	}
	if c.Storage.ImagesPath == "" {
		c.Storage.ImagesPath = "data/images"
	}
	if c.Storage.MaxFileSize == 0 {
		c.Storage.MaxFileSize = 50 * 1024 * 1024
	}
	if len(c.Storage.AllowedTypes) == 0 {
		c.Storage.AllowedTypes = []string{"image/png", "image/jpeg", "image/webp"}
	}
	if c.Storage.DownloadTimeout == 0 {
		c.Storage.DownloadTimeout = 30
	}
	if len(c.Storage.AllowedDomains) == 0 {
		c.Storage.AllowedDomains = []string{"*"}
	}
	if len(c.Storage.BlockedIPRanges) == 0 {
		c.Storage.BlockedIPRanges = []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "::1/128", "fc00::/7"}
	}
	if c.Storage.ObjectStorage.Region == "" {
		c.Storage.ObjectStorage.Region = "us-east-1"
	}
	if c.Storage.ObjectStorage.PreserveUploads || !c.Storage.ObjectStorage.Enabled {
		c.Storage.ObjectStorage.PreserveUploads = true
	}
	if c.Storage.ObjectStorage.Enabled && strings.EqualFold(strings.TrimSpace(c.Storage.ObjectStorage.Provider), "aliyun") && c.Storage.ObjectStorage.Region == "us-east-1" {
		c.Storage.ObjectStorage.Region = "oss-cn-hangzhou"
	}
	if c.WebSearch.FirecrawlBaseURL == "" {
		c.WebSearch.FirecrawlBaseURL = "https://api.firecrawl.dev/v1"
	}
	if c.WebSearch.SearxngURLs == "" {
		c.WebSearch.SearxngURLs = "https://searx.be,https://searxng.site,https://search.ononoki.org"
	}
	if c.Auth.SessionTTL == 0 {
		c.Auth.SessionTTL = 86400
	}
	if c.Auth.AccessTokenTTL == 0 {
		c.Auth.AccessTokenTTL = 1800
	}
	if c.Auth.RefreshTokenTTL == 0 {
		c.Auth.RefreshTokenTTL = 2592000
	}
	if c.Auth.CookieName == "" {
		c.Auth.CookieName = "session_id"
	}
	if c.Auth.RefreshCookieName == "" {
		c.Auth.RefreshCookieName = "refresh_token"
	}
	if c.AuthCenter.OrgSync.Mode == "" {
		c.AuthCenter.OrgSync.Mode = "disabled"
	}
	if c.AuthCenter.OrgSync.SnapshotPath == "" {
		c.AuthCenter.OrgSync.SnapshotPath = "/api/org/snapshot"
	}
	if c.AuthCenter.OrgSync.Interval == "" {
		c.AuthCenter.OrgSync.Interval = "10m"
	}
	if c.AuthCenter.OrgSync.MaxStaleness == "" {
		c.AuthCenter.OrgSync.MaxStaleness = "30m"
	}
	if c.AuthCenter.Authorization.Mode == "" {
		c.AuthCenter.Authorization.Mode = "off"
	}
	if c.AuthCenter.Authorization.CheckPath == "" {
		c.AuthCenter.Authorization.CheckPath = "/api/v2/authorization/check"
	}
	if c.AuthCenter.Authorization.ScopePath == "" {
		c.AuthCenter.Authorization.ScopePath = "/api/v2/authorization/scope"
	}
	if c.AuthCenter.Authorization.VersionsPath == "" {
		c.AuthCenter.Authorization.VersionsPath = "/api/v2/authorization/versions"
	}
	if c.AuthCenter.Authorization.Timeout == "" {
		c.AuthCenter.Authorization.Timeout = "3s"
	}
	if c.AuthCenter.Authorization.CacheTTL == "" {
		c.AuthCenter.Authorization.CacheTTL = "30s"
	}
	c.applyAuthCenterUserRolesDefaults()
	if c.StyleLibrary.RemoteTimeout == 0 {
		c.StyleLibrary.RemoteTimeout = 15
	}
	if c.StyleLibrary.RemotePageSize == 0 {
		c.StyleLibrary.RemotePageSize = 100
	}
	if c.Security.BcryptCost == 0 {
		c.Security.BcryptCost = 10
	}
	if c.Queue.MaxSize == 0 {
		c.Queue.MaxSize = 1000
	}
	if c.Queue.Workers == 0 {
		c.Queue.Workers = 5
	}
	if c.Queue.TaskTimeout < 0 {
		c.Queue.TaskTimeout = 0
	} else if c.Queue.TaskTimeout == 0 {
		c.Queue.TaskTimeout = 1800
	}
	if c.WebSocket.ReadBufferSize == 0 {
		c.WebSocket.ReadBufferSize = 1024
	}
	if c.WebSocket.WriteBufferSize == 0 {
		c.WebSocket.WriteBufferSize = 1024
	}
	if c.WebSocket.PingPeriod == 0 {
		c.WebSocket.PingPeriod = 54
	}
	if c.API.DefaultTimeout == 0 {
		c.API.DefaultTimeout = 120
	}
	if c.API.MaxRetries == 0 {
		c.API.MaxRetries = 3
	}
	if c.API.RetryDelay == 0 {
		c.API.RetryDelay = 5
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "stdout"
	}
	if c.Logging.HTTPHomepageSuccessSampleEvery <= 0 {
		c.Logging.HTTPHomepageSuccessSampleEvery = 100
	}
	if c.Redis.Host == "" {
		c.Redis.Host = "localhost"
	}
	if c.Redis.Port == 0 {
		c.Redis.Port = 6379
	}
	if c.Redis.PoolSize == 0 {
		c.Redis.PoolSize = 20
	}
	if c.Redis.MinIdleConns == 0 {
		c.Redis.MinIdleConns = 10
	}
	if c.Redis.MaxRetries == 0 {
		c.Redis.MaxRetries = 3
	}
	if c.Redis.DialTimeout == 0 {
		c.Redis.DialTimeout = 3 * time.Second
	}
	if c.Redis.ReadTimeout == 0 {
		c.Redis.ReadTimeout = 2 * time.Second
	}
	if c.Redis.WriteTimeout == 0 {
		c.Redis.WriteTimeout = 2 * time.Second
	}
	c.setModelRuntimeDefaults()
	if c.AgentAuxiliary.Compression.MaxTokens <= 0 {
		c.AgentAuxiliary.Compression.MaxTokens = 1024
	}
	if c.AgentAuxiliary.Compression.Temperature <= 0 {
		c.AgentAuxiliary.Compression.Temperature = 0.2
	}
	if strings.TrimSpace(c.AgentAuxiliary.Compression.SystemPrompt) == "" {
		c.AgentAuxiliary.Compression.SystemPrompt = "你是长篇小说创作系统的上下文压缩助手，负责把前序对话压缩成不丢关键事实的交接文档。"
	}
	if strings.TrimSpace(c.AgentAuxiliary.Compression.UserPrompt) == "" {
		c.AgentAuxiliary.Compression.UserPrompt = "以下是需要压缩的前序对话（JSON 数组）：\n{{.Conversation}}\n\n请输出结构化交接文档，必须保留：核心目标、已完成工作、关键约束、未决事项。{{.Instruction}}"
	}
	if c.Callback.TimestampTolerance == 0 {
		c.Callback.TimestampTolerance = 300
	}
	if c.BackgroundTask.ImageGenerationTimeoutSeconds <= 0 {
		c.BackgroundTask.ImageGenerationTimeoutSeconds = 1800
	}
	if c.BackgroundTask.StatsAggregationIntervalMinutes <= 0 {
		c.BackgroundTask.StatsAggregationIntervalMinutes = 15
	}
	c.setVideoPollingDefaults()
	if c.BackgroundTask.StatsAggregationLookbackDays <= 0 {
		c.BackgroundTask.StatsAggregationLookbackDays = 7
	}
	if c.BackgroundTask.StatsRetentionDays <= 0 {
		c.BackgroundTask.StatsRetentionDays = 180
	}
	if c.SoftDelete.RetentionDays <= 0 {
		c.SoftDelete.RetentionDays = 30
	}
	if c.BackgroundTask.InputImageOriginalCacheTTLHours <= 0 {
		c.BackgroundTask.InputImageOriginalCacheTTLHours = 24
	}
	if c.BackgroundTask.InputImageOriginalCacheCleanupIntervalMinutes <= 0 {
		c.BackgroundTask.InputImageOriginalCacheCleanupIntervalMinutes = 24 * 60
	}
	if c.BackgroundTask.InputImageUploadSessionTTLHours <= 0 {
		c.BackgroundTask.InputImageUploadSessionTTLHours = 2
	}
	if c.BackgroundTask.InputImageUploadCleanupIntervalMinutes <= 0 {
		c.BackgroundTask.InputImageUploadCleanupIntervalMinutes = 60
	}
	if c.BackgroundTask.InputImageUploadMaxActivePerUser <= 0 {
		c.BackgroundTask.InputImageUploadMaxActivePerUser = 80
	}
	if c.BackgroundTask.MasterSKUDailySnapshotIntervalMinutes <= 0 {
		c.BackgroundTask.MasterSKUDailySnapshotIntervalMinutes = 60
	}
	if c.BackgroundTask.MediaStorageReconcileEnabled == nil {
		enabled := true
		c.BackgroundTask.MediaStorageReconcileEnabled = &enabled
	}
	if c.BackgroundTask.MediaStorageReconcileIntervalMinutes <= 0 {
		c.BackgroundTask.MediaStorageReconcileIntervalMinutes = 1
	}
	if c.BackgroundTask.MediaStorageReconcileBatchSize <= 0 || c.BackgroundTask.MediaStorageReconcileBatchSize > 200 {
		c.BackgroundTask.MediaStorageReconcileBatchSize = 100
	}
	if c.BackgroundTask.MatrixDrawReconcileIntervalSeconds <= 0 {
		c.BackgroundTask.MatrixDrawReconcileIntervalSeconds = 60
	}
	if c.BackgroundTask.MatrixDrawReconcileEvery <= 0 {
		c.BackgroundTask.MatrixDrawReconcileEvery = 10
	}
}
