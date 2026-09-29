package config

// Config 应用配置
type Config struct {
	Auth           AuthConfig           `yaml:"auth"`
	AuthCenter     AuthCenterConfig     `yaml:"auth_center"`
	SSO            SSOConfig            `yaml:"sso"`
	App            AppConfig            `yaml:"app"`
	Logging        LoggingConfig        `yaml:"logging"`
	CDN            CDNConfig            `yaml:"cdn"`
	CDNUrl         string               `yaml:"-"`
	Security       SecurityConfig       `yaml:"security"`
	Gateway        GatewayConfig        `yaml:"gateway"`
	Storage        StorageConfig        `yaml:"storage"`
	StyleLibrary   StyleLibraryConfig   `yaml:"style_library"`
	SoftDelete     SoftDeleteConfig     `yaml:"soft_delete"`
	Callback       CallbackConfig       `yaml:"callback"`
	AgentAuxiliary AgentAuxiliaryConfig `yaml:"agent_auxiliary"`
	Database       DatabaseConfig       `yaml:"database"`
	Redis          RedisConfig          `yaml:"redis"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	BackgroundTask BackgroundTaskConfig `yaml:"background_task"`
	API            APIConfig            `yaml:"api"`
	WebSocket      WebSocketConfig      `yaml:"websocket"`
	Queue          QueueConfig          `yaml:"queue"`
	ModelRuntime   ModelRuntimeConfig   `yaml:"model_runtime"`
	// Cutout 抠图与图像分割外部服务配置
	Cutout CutoutConfig `yaml:"cutout"`
	// FeishuSync 飞书表格款式数据同步配置
	FeishuSync FeishuSyncConfig `yaml:"feishu_sync"`
	// MCPServer 内部 MCP 基础安全配置
	MCPServer MCPServerConfig `yaml:"mcp_server"`
	// WebSearch 外部网络检索（Firecrawl / Tavily / Searxng）配置
	WebSearch WebSearchConfig `yaml:"web_search"`
	// LegacyTenant 是历史素材（tenant_id 为空）回填的兜底租户；启动校验非空。
	LegacyTenant LegacyTenantConfig `yaml:"legacy_tenant"`
	// AdminServer 系统超级管理员物理隔离端口服务配置
	AdminServer AdminServerConfig `yaml:"admin_server"`
}

// AdminServerConfig 系统超管运维端独立服务配置（物理隔离）
type AdminServerConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	Host                    string `yaml:"host"`
	Port                    int    `yaml:"port"`
	HTTPReadTimeoutSeconds  int    `yaml:"http_read_timeout_seconds"`
	HTTPWriteTimeoutSeconds int    `yaml:"http_write_timeout_seconds"`
	HTTPIdleTimeoutSeconds  int    `yaml:"http_idle_timeout_seconds"`
	SessionCookieName       string `yaml:"session_cookie_name"`
}

// LegacyTenantConfig 记录无法按 auth_center user identity 映射的历史素材归属租户。
type LegacyTenantConfig struct {
	TenantID string `yaml:"tenant_id"`
}

// AppConfig 应用基础配置
type AppConfig struct {
	Name                    string `yaml:"name"`
	Env                     string `yaml:"env"`
	Host                    string `yaml:"host"`
	LogLevel                string `yaml:"log_level"`
	TLSCertFile             string `yaml:"tls_cert_file"`
	TLSKeyFile              string `yaml:"tls_key_file"`
	Port                    int    `yaml:"port"`
	HTTPReadTimeoutSeconds  int    `yaml:"http_read_timeout_seconds"`
	HTTPWriteTimeoutSeconds int    `yaml:"http_write_timeout_seconds"`
	HTTPIdleTimeoutSeconds  int    `yaml:"http_idle_timeout_seconds"`
	Debug                   bool   `yaml:"debug"`
	// AllowStressBypass 仅保留旧配置结构兼容；认证中间件不再读取或允许该旁路。
	AllowStressBypass bool `yaml:"allow_stress_bypass"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	// Path 是 SQLite 测试夹具路径；runtime 不读取。
	Path string `yaml:"path"`
	// DSN 是 PostgreSQL 连接串，仅 driver=postgres 时生效。
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime int    `yaml:"conn_max_idle_time"`
	SlowThresholdMs int    `yaml:"slow_threshold_ms"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	// AllowMemoryStore 显式开启「无数据库」的内存仓储档位（INK-108 / ADR-0012）。
	//
	// 默认 false：数据库是唯一权威存储，未配置 DSN 时进程直接拒绝启动
	// （含开发环境）。只有显式把它设为 true 才允许降级为内存仓储，
	// 避免任何人把「数据没落库」当成正常运行。
	AllowMemoryStore bool `yaml:"allow_memory_store"`

	ImagesPath         string   `yaml:"images_path"`
	ThumbnailCachePath string   `yaml:"thumbnail_cache_path"`
	AllowedTypes       []string `yaml:"allowed_types"`

	AllowedDomains  []string            `yaml:"allowed_domains"`
	BlockedIPRanges []string            `yaml:"blocked_ip_ranges"`
	AllowPrivateIPs bool                `yaml:"allow_private_ips"`
	ObjectStorage   ObjectStorageConfig `yaml:"object_storage"`
	MaxFileSize     int64               `yaml:"max_file_size"`
	DownloadTimeout int                 `yaml:"download_timeout"`
}

// ObjectStorageConfig S3 兼容对象存储配置
type ObjectStorageConfig struct {
	SessionToken                string `yaml:"session_token"`
	RoleARN                     string `yaml:"role_arn"`
	Endpoint                    string `yaml:"endpoint"`
	PublicBaseURL               string `yaml:"public_base_url"`
	Region                      string `yaml:"region"`
	Bucket                      string `yaml:"bucket"`
	AccessKeyID                 string `yaml:"access_key_id"`
	SecretAccessKey             string `yaml:"secret_access_key"`
	Provider                    string `yaml:"provider"`
	RootPath                    string `yaml:"root_path"`
	STSEndpoint                 string `yaml:"sts_endpoint"`
	STSRoleSessionTTLSeconds    int64  `yaml:"sts_role_session_ttl_seconds"`
	ForcePathStyle              bool   `yaml:"force_path_style"`
	AllowDirectKey              bool   `yaml:"allow_direct_key"`
	PreserveUploads             bool   `yaml:"preserve_uploads_behavior"`
	DisableSSL                  bool   `yaml:"disable_ssl"`
	Enabled                     bool   `yaml:"enabled"`
	FrontendDirectUploadEnabled bool   `yaml:"frontend_direct_upload_enabled"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret          string `yaml:"jwt_secret"`
	CookieName         string `yaml:"cookie_name"`
	RefreshCookieName  string `yaml:"refresh_cookie_name"`
	CookieDomain       string `yaml:"cookie_domain"`
	FrontendURL        string `yaml:"frontend_url"`
	SessionTTL         int    `yaml:"session_ttl"`
	SessionIdleTimeout int    `yaml:"session_idle_timeout"`
	AccessTokenTTL     int    `yaml:"access_token_ttl"`
	RefreshTokenTTL    int    `yaml:"refresh_token_ttl"`
	CookieSecure       bool   `yaml:"cookie_secure"`
	CookieHttpOnly     bool   `yaml:"cookie_http_only"`
	// OpaqueSessionEnabled 为 true 时，服务端保存真实 access/refresh token，cookie 仅写 opaque session id。
	OpaqueSessionEnabled bool `yaml:"opaque_session_enabled"`
}

// SSOConfig 单点登录（OIDC）配置
type SSOConfig struct {
	Issuer           string   `yaml:"issuer"`
	ClientID         string   `yaml:"client_id"`
	ClientSecret     string   `yaml:"client_secret"`
	TenantID         string   `yaml:"tenant_id"`
	AdminTenantID    string   `yaml:"admin_tenant_id"`
	RedirectURI      string   `yaml:"redirect_uri"`
	FrontendURL      string   `yaml:"frontend_url"`
	AdminRedirectURI string   `yaml:"admin_redirect_uri"`
	AdminFrontendURL string   `yaml:"admin_frontend_url"`
	Scopes           []string `yaml:"scopes"`
	Enabled          bool     `yaml:"enabled"`
}

// AuthCenterConfig 用户中心/认证中心服务端集成配置
type AuthCenterConfig struct {
	BaseURL              string                        `yaml:"base_url"`
	IssuerURL            string                        `yaml:"issuer_url"`
	ClientID             string                        `yaml:"client_id"`
	ClientSecret         string                        `yaml:"client_secret"`
	InternalServiceToken string                        `yaml:"internal_service_token"`
	RedirectURI          string                        `yaml:"redirect_uri"`
	OrgSync              AuthCenterOrgSyncConfig       `yaml:"org_sync"`
	Authorization        AuthCenterAuthorizationConfig `yaml:"authorization"`
	UserRoles            AuthCenterUserRolesConfig     `yaml:"user_roles"`
}

// AuthCenterUserRolesConfig 控制管理端用户列表的认证中心角色 live 查询，仅用于展示。
type AuthCenterUserRolesConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ServiceToken string `yaml:"service_token"`
	PathTemplate string `yaml:"path_template"`
	Timeout      string `yaml:"timeout"`
}

// AuthCenterOrgSyncConfig 控制从用户中心到本地组织投影的只读同步
type AuthCenterOrgSyncConfig struct {
	// Mode 支持 disabled、snapshot 和 scim。
	Mode          string `yaml:"mode"`
	SnapshotPath  string `yaml:"snapshot_path"`
	SnapshotToken string `yaml:"snapshot_token"`
	SCIMToken     string `yaml:"scim_token"`
	Interval      string `yaml:"interval"`
	MaxStaleness  string `yaml:"max_staleness"`
}

// StyleLibraryConfig 款号主数据外部同步配置
type StyleLibraryConfig struct {
	RemoteURL      string `yaml:"remote_url"`
	RemoteToken    string `yaml:"remote_token"`
	RemoteTimeout  int    `yaml:"remote_timeout"`
	RemotePageSize int    `yaml:"remote_page_size"`
}

// SoftDeleteConfig 软删除回收站可见保留期配置
type SoftDeleteConfig struct {
	// RetentionDays 控制用户回收站可见窗口；非法值回退为 30 天。
	RetentionDays int `yaml:"retention_days"`
}

// SoftDeleteRetentionDays 返回归一化后的回收站可见天数。
func (c *Config) SoftDeleteRetentionDays() int {
	if c == nil || c.SoftDelete.RetentionDays <= 0 {
		return 30
	}
	return c.SoftDelete.RetentionDays
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EncryptionKey  string   `yaml:"encryption_key"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	BcryptCost     int      `yaml:"bcrypt_cost"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	MaxSize     int `yaml:"max_size"`
	Workers     int `yaml:"workers"`
	TaskTimeout int `yaml:"task_timeout"`
}

// WebSocketConfig WebSocket 配置
type WebSocketConfig struct {
	ReadBufferSize  int `yaml:"read_buffer_size"`
	WriteBufferSize int `yaml:"write_buffer_size"`
	PingPeriod      int `yaml:"ping_period"`
}

// APIConfig 上游 API 调用配置
type APIConfig struct {
	DefaultTimeout int `yaml:"default_timeout"`
	MaxRetries     int `yaml:"max_retries"`
	RetryDelay     int `yaml:"retry_delay"`
}

// RateLimitConfig controls trusted ingress proxies for client-IP rate limiting.
// Forwarded headers are ignored unless the TCP peer matches this allowlist.
type RateLimitConfig struct {
	TrustedProxyCIDRs []string `yaml:"trusted_proxy_cidrs"`
	AICapacity        int      `yaml:"ai_capacity"`
	AIPerMinute       int      `yaml:"ai_per_minute"`
}

// CutoutConfig 抠图与图像分割外部服务配置
type CutoutConfig struct {
	Enabled        bool   `yaml:"enabled"`
	WorkerURL      string `yaml:"worker_url"`
	Model          string `yaml:"model"`
	AuthToken      string `yaml:"auth_token"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// FeishuSyncConfig 飞书表格款式数据同步配置
type FeishuSyncConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	SheetURL  string `yaml:"sheet_url"`
	SheetName string `yaml:"sheet_name"`
	SheetKind string `yaml:"sheet_kind"`
}

// MCPServerConfig 内部 MCP 基础安全与鉴权配置
type MCPServerConfig struct {
	SecretKey string `yaml:"secret_key"`
}

// WebSearchConfig 外部网络搜索（Firecrawl / Tavily / Searxng）强类型配置
type WebSearchConfig struct {
	FirecrawlAPIKeys string `yaml:"firecrawl_api_keys"`
	FirecrawlBaseURL string `yaml:"firecrawl_base_url"`
	TavilyAPIKeys    string `yaml:"tavily_api_keys"`
	SearxngURLs      string `yaml:"searxng_urls"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level                          string `yaml:"level"`
	Format                         string `yaml:"format"`
	Output                         string `yaml:"output"`
	FilePath                       string `yaml:"file_path"`
	ErrorFilePath                  string `yaml:"error_file_path"`
	HTTPHomepageSuccessSampleEvery int    `yaml:"http_homepage_success_sample_every"`
}

// AuxiliaryTaskConfig 是「辅助任务 + 辅助模型」的绑定配置（INK-050）。
//
// 模型只按 model_key 声明：运行时到 Provider 注册表（llm_models / llm_providers）
// 解析出模型 ID 与供应商，保证「配置里写的模型一定是管理端真实存在且启用的模型」，
// 而不是在上下文压缩运行到一半才发现模型不存在。
type AuxiliaryTaskConfig struct {
	Enabled      bool    `yaml:"enabled"`
	ModelKey     string  `yaml:"model_key"`
	SystemPrompt string  `yaml:"system_prompt"`
	UserPrompt   string  `yaml:"user_prompt"`
	MaxTokens    int     `yaml:"max_tokens"`
	Temperature  float64 `yaml:"temperature"`
}

// AgentAuxiliaryConfig 汇总各辅助任务的绑定；目前只有上下文压缩在用。
type AgentAuxiliaryConfig struct {
	Compression AuxiliaryTaskConfig `yaml:"compression"`
}
