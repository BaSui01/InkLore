package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GatewayConfig API gateway external endpoint configuration.
type GatewayConfig struct {
	VideoURLPolicies         []GatewayVideoURLPolicyConfig `yaml:"video_url_policies"`
	AllowedVideoExtensions   []string                      `yaml:"allowed_video_extensions"`
	AllowedVideoMIMETypes    []string                      `yaml:"allowed_video_mime_types"`
	AllowedAPIKeys           []string                      `yaml:"allowed_api_keys"`
	DefaultVideoURLPolicyID  string                        `yaml:"default_video_url_policy_id"`
	VideoUploadMaxBytes      int64                         `yaml:"video_upload_max_bytes"`
	VideoDownloadMaxBytes    int64                         `yaml:"video_download_max_bytes"`
	VideoDownloadTimeoutSecs int                           `yaml:"video_download_timeout_seconds"`
}

type GatewayVideoURLPolicyConfig struct {
	AllowedCIDRs        []string `json:"allowed_cidrs" yaml:"allowed_cidrs"`
	AllowedHosts        []string `json:"allowed_hosts" yaml:"allowed_hosts"`
	AllowedSchemes      []string `json:"allowed_schemes" yaml:"allowed_schemes"`
	ID                  string   `json:"id" yaml:"id"`
	DownloadTimeoutSecs int      `json:"download_timeout_seconds" yaml:"download_timeout_seconds"`
	MaxSizeBytes        int64    `json:"max_size_bytes" yaml:"max_size_bytes"`
	AllowLinkLocal      bool     `json:"allow_link_local" yaml:"allow_link_local"`
	AllowLoopback       bool     `json:"allow_loopback" yaml:"allow_loopback"`
	AllowPrivateIPs     bool     `json:"allow_private_ips" yaml:"allow_private_ips"`
	AllowPublicIPs      bool     `json:"allow_public_ips" yaml:"allow_public_ips"`
}

func parseGatewayVideoURLPolicies(raw string) ([]GatewayVideoURLPolicyConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var policies []GatewayVideoURLPolicyConfig
	if err := json.Unmarshal([]byte(raw), &policies); err != nil {
		return nil, fmt.Errorf("parse gateway video url policies: %w", err)
	}
	for i := range policies {
		policies[i].ID = strings.TrimSpace(policies[i].ID)
	}
	return policies, nil
}

// CDNConfig CDN 配置
type CDNConfig struct {
	StaticURL  string `yaml:"static_url"`
	UploadsURL string `yaml:"uploads_url"`
	Enabled    bool   `yaml:"enabled"`
}

// CallbackConfig 第三方回调配置
type CallbackConfig struct {
	// Kling 回调签名密钥（HMAC-SHA256），建议从环境变量 KLING_CALLBACK_SECRET 读取
	KlingSecret string `yaml:"kling_secret"`
	// 通用媒体回调签名密钥（HMAC-SHA256），建议从环境变量 MEDIA_CALLBACK_SECRET 读取
	MediaSecret string `yaml:"media_secret"`
	// Kling 回调允许的 IP 白名单（可选，为空则不验证 IP）
	KlingAllowedIPs []string `yaml:"kling_allowed_ips"`
	// 通用媒体回调允许的 IP 白名单（可选，为空则不验证 IP）
	MediaAllowedIPs []string `yaml:"media_allowed_ips"`
	// 可信反向代理 CIDR；只有直连来源命中时才接受代理 IP 头
	TrustedProxyCIDRs []string `yaml:"trusted_proxy_cidrs"`
	// 时间戳容差（秒），防止重放攻击，默认 300 秒
	TimestampTolerance int `yaml:"timestamp_tolerance"`
	// KlingEnabled controls registration and validation of the Kling inbound callback.
	KlingEnabled bool `yaml:"kling_enabled"`
	// MediaEnabled controls registration and validation of generic media callbacks.
	MediaEnabled bool `yaml:"media_enabled"`
	// 是否启用签名验证（生产环境建议启用）
	EnableSignatureVerify bool `yaml:"enable_signature_verify"`
}

// BackgroundTaskConfig 后台任务配置
type BackgroundTaskConfig struct {
	// 同步图片生成任务默认总截止时间（秒），默认 600
	ImageGenerationTimeoutSeconds int `yaml:"image_generation_timeout_seconds"`
	// 异步视频后台轮询是否启用，默认启用
	VideoPollingEnabled *bool `yaml:"video_polling_enabled"`
	// 异步视频轮询扫描间隔（秒），默认 3
	VideoPollingIntervalSeconds int `yaml:"video_polling_interval_seconds"`
	// 单轮领取任务数，默认 50
	VideoPollingBatchSize int `yaml:"video_polling_batch_size"`
	// 异步视频默认总截止时间（秒），默认 1800
	VideoGenerationTimeoutSeconds int `yaml:"video_generation_timeout_seconds"`
	// 假完成任务恢复回看窗口（小时），默认 24
	VideoRecoveryLookbackHours int `yaml:"video_recovery_lookback_hours"`
	// 多实例轮询租约时长（秒），默认 60
	VideoPollingLeaseSeconds int `yaml:"video_polling_lease_seconds"`
	// 统计日报聚合扫描间隔（分钟），默认 15
	StatsAggregationIntervalMinutes int `yaml:"stats_aggregation_interval_minutes"`
	// 统计日报回看窗口（天），每次重算最近 N 天，默认 7
	StatsAggregationLookbackDays int `yaml:"stats_aggregation_lookback_days"`
	// 统计日报保留天数，默认 180
	StatsRetentionDays int `yaml:"stats_retention_days"`
	// 输入参考图原图缓存保留时长（小时），默认 24
	InputImageOriginalCacheTTLHours int `yaml:"input_image_original_cache_ttl_hours"`
	// 输入参考图原图缓存清理间隔（分钟），默认 1440（24小时）
	InputImageOriginalCacheCleanupIntervalMinutes int `yaml:"input_image_original_cache_cleanup_interval_minutes"`
	// 输入参考图上传会话保留时长（小时），默认 2
	InputImageUploadSessionTTLHours int `yaml:"input_image_upload_session_ttl_hours"`
	// 输入参考图上传会话清理间隔（分钟），默认 60
	InputImageUploadCleanupIntervalMinutes int `yaml:"input_image_upload_cleanup_interval_minutes"`
	// 单用户最多允许同时存在的参考图上传会话数，默认 80
	InputImageUploadMaxActivePerUser int `yaml:"input_image_upload_max_active_per_user"`
	// 分片上传每片大小（MB），默认 1（0 表示使用内置默认值）
	InputImageUploadChunkSizeMB int `yaml:"input_image_upload_chunk_size_mb"`
	// 单张参考图允许的最大体积（MB），默认 30（0 表示使用内置默认值）
	InputImageUploadMaxFileSizeMB int `yaml:"input_image_upload_max_file_size_mb"`
	// 权威 SKU 日快照补齐巡检间隔（分钟），默认 60。
	MasterSKUDailySnapshotIntervalMinutes int `yaml:"master_sku_daily_snapshot_interval_minutes"`
	// 素材物理对象七天隔离回收；仅处理已进入账本的对象。
	MediaStorageReconcileEnabled         *bool `yaml:"media_storage_reconcile_enabled"`
	MediaStorageReconcileIntervalMinutes int   `yaml:"media_storage_reconcile_interval_minutes"`
	MediaStorageReconcileBatchSize       int   `yaml:"media_storage_reconcile_batch_size"`

	// 矩阵绘图自动化：事件消费/对账扫描间隔（秒），默认 60
	MatrixDrawReconcileIntervalSeconds int `yaml:"matrix_draw_reconcile_interval_seconds"`
	// 矩阵绘图自动化：每 N 次扫描执行一次全量对账兜底，默认 10
	MatrixDrawReconcileEvery int `yaml:"matrix_draw_reconcile_every"`

	// Wave 0 generation-state gates are independent and default off. Later waves
	// may read them, but this contract-prep wave does not activate behavior.
	GenerationTerminalWriterEnabled          bool `yaml:"generation_terminal_writer_enabled"`
	GenerationTerminalProcessorShadowEnabled bool `yaml:"generation_terminal_processor_shadow_enabled"`
	GenerationTerminalProcessorApplyEnabled  bool `yaml:"generation_terminal_processor_apply_enabled"`
	GenerationDurableRetryPublisherEnabled   bool `yaml:"generation_durable_retry_publisher_enabled"`
	GenerationFencingCompatibleWriteEnabled  bool `yaml:"generation_fencing_compatible_write_enabled"`
	GenerationFencingEnforceEnabled          bool `yaml:"generation_fencing_enforce_enabled"`
	GenerationHistoryApplyEnabled            bool `yaml:"generation_history_apply_enabled"`

	// 渠道健康检测
	ChannelAutoDisable ChannelAutoDisableConfig `yaml:"channel_auto_disable"`
}

type GenerationStateFeatureGates struct {
	TerminalWriter          bool
	TerminalProcessorShadow bool
	TerminalProcessorApply  bool
	DurableRetryPublisher   bool
	FencingCompatibleWrite  bool
	FencingEnforce          bool
	HistoryApply            bool
}

func (c *Config) GenerationStateGates() GenerationStateFeatureGates {
	if c == nil {
		return GenerationStateFeatureGates{}
	}
	background := c.BackgroundTask
	return GenerationStateFeatureGates{
		TerminalWriter:          background.GenerationTerminalWriterEnabled,
		TerminalProcessorShadow: background.GenerationTerminalProcessorShadowEnabled,
		TerminalProcessorApply:  background.GenerationTerminalProcessorApplyEnabled,
		DurableRetryPublisher:   background.GenerationDurableRetryPublisherEnabled,
		FencingCompatibleWrite:  background.GenerationFencingCompatibleWriteEnabled,
		FencingEnforce:          background.GenerationFencingEnforceEnabled,
		HistoryApply:            background.GenerationHistoryApplyEnabled,
	}
}

// ChannelAutoDisableConfig 渠道自动禁用配置（yaml 兜底；运行时以 system_configs 为准）
type ChannelAutoDisableConfig struct {
	// 是否启用
	Enabled bool `yaml:"enabled"`
	// 检查间隔（分钟），默认 5
	CheckIntervalMinutes int `yaml:"check_interval_minutes"`
	// 统计窗口（分钟），默认 10
	WindowMinutes int `yaml:"window_minutes"`
	// 窗口内失败次数阈值，默认 5
	FailureThreshold int `yaml:"failure_threshold"`
	// 计入失败的上游状态码，默认 429,403,502,503
	CountedStatusCodes []int `yaml:"counted_status_codes"`
	// 最短冷却（分钟），默认 15
	MinCooldownMinutes int `yaml:"min_cooldown_minutes"`
	// 是否计入超时/无状态码传输错误，默认 true
	CountTransportErrors *bool `yaml:"count_transport_errors"`
	// 兼容旧配置：失败率阈值（0-1），默认不再作为主判定
	ErrorRateThreshold float64 `yaml:"error_rate_threshold"`
	// 兼容旧配置：最小调用量
	MinCalls int `yaml:"min_calls"`
}
