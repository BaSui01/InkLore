package config

import (
	"InkLore/internal/platform/logger"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadEnvFile 将本地 .env 文件中尚未存在的键加载到进程环境。
func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			logger.Warn("env file line has invalid format", "line_number", lineNum)
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("设置环境变量 %q 失败: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 .env 文件失败: %w", err)
	}
	logger.Info("env file loaded")
	return nil
}

// loadFromEnv 使用系统环境变量覆盖文件配置。
func (c *Config) loadFromEnv() error {
	if envKey := os.Getenv("ENCRYPTION_KEY"); envKey != "" {
		c.Security.EncryptionKey = envKey
	}
	// ── 认证 Token/Session TTL 配置 ──
	// AUTH_ACCESS_TOKEN_TTL：前端 session_id Cookie 存活时间（秒）。
	// 默认 1800s（30 分钟），频繁过期会导致前端触发 SSO 重定向。
	// 推荐生产环境设为 28800（8 小时）以减少频繁认证跳转。
	if v := os.Getenv("AUTH_ACCESS_TOKEN_TTL"); v != "" {
		if ttl, err := strconv.Atoi(v); err == nil && ttl > 0 {
			c.Auth.AccessTokenTTL = ttl
		}
	}
	// AUTH_REFRESH_TOKEN_TTL：refresh_token Cookie 存活时间（秒）。
	// 默认 2592000s（30 天）。
	if v := os.Getenv("AUTH_REFRESH_TOKEN_TTL"); v != "" {
		if ttl, err := strconv.Atoi(v); err == nil && ttl > 0 {
			c.Auth.RefreshTokenTTL = ttl
		}
	}
	// AUTH_SESSION_TTL：opaque session 在 Redis 的存活时间（秒）。
	// 默认 86400s（24 小时）。
	if v := os.Getenv("AUTH_SESSION_TTL"); v != "" {
		if ttl, err := strconv.Atoi(v); err == nil && ttl > 0 {
			c.Auth.SessionTTL = ttl
		}
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		c.Redis.Password = redisPassword
	}
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		c.Redis.Host = redisHost
	}
	if redisPort := os.Getenv("REDIS_PORT"); redisPort != "" {
		if port, err := strconv.Atoi(redisPort); err == nil {
			c.Redis.Port = port
		}
	}
	if redisDB := os.Getenv("REDIS_DB"); redisDB != "" {
		if db, err := strconv.Atoi(redisDB); err == nil {
			c.Redis.DB = db
		}
	}
	if appName := os.Getenv("APP_NAME"); appName != "" {
		c.App.Name = appName
	}
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		c.App.Env = appEnv
	}
	if appHost := os.Getenv("APP_HOST"); appHost != "" {
		c.App.Host = appHost
	}
	if appDebug := os.Getenv("APP_DEBUG"); appDebug != "" {
		c.App.Debug = appDebug == strTrue || appDebug == "1" || strings.EqualFold(appDebug, "yes")
	}
	if appPort := os.Getenv("APP_PORT"); appPort != "" {
		if port, err := strconv.Atoi(appPort); err == nil {
			c.App.Port = port
		}
	}
	if timeout := os.Getenv("APP_HTTP_READ_TIMEOUT_SECONDS"); timeout != "" {
		if seconds, err := strconv.Atoi(timeout); err == nil {
			c.App.HTTPReadTimeoutSeconds = seconds
		}
	}
	if timeout := os.Getenv("APP_HTTP_WRITE_TIMEOUT_SECONDS"); timeout != "" {
		if seconds, err := strconv.Atoi(timeout); err == nil {
			c.App.HTTPWriteTimeoutSeconds = seconds
		}
	}
	if timeout := os.Getenv("APP_HTTP_IDLE_TIMEOUT_SECONDS"); timeout != "" {
		if seconds, err := strconv.Atoi(timeout); err == nil {
			c.App.HTTPIdleTimeoutSeconds = seconds
		}
	}
	if tlsCertFile := os.Getenv("APP_TLS_CERT_FILE"); tlsCertFile != "" {
		c.App.TLSCertFile = tlsCertFile
	}
	if tlsKeyFile := os.Getenv("APP_TLS_KEY_FILE"); tlsKeyFile != "" {
		c.App.TLSKeyFile = tlsKeyFile
	}
	if adminEnabled := os.Getenv("ADMIN_SERVER_ENABLED"); adminEnabled != "" {
		c.AdminServer.Enabled = adminEnabled == strTrue || adminEnabled == "1" || strings.EqualFold(adminEnabled, "yes")
	}
	if adminHost := os.Getenv("ADMIN_SERVER_HOST"); adminHost != "" {
		c.AdminServer.Host = adminHost
	}
	if adminPort := os.Getenv("ADMIN_SERVER_PORT"); adminPort != "" {
		if port, err := strconv.Atoi(adminPort); err == nil {
			c.AdminServer.Port = port
		}
	}
	if cookieName := os.Getenv("ADMIN_SESSION_COOKIE_NAME"); cookieName != "" {
		c.AdminServer.SessionCookieName = cookieName
	}
	if gatewayKeys := os.Getenv("GATEWAY_API_KEYS"); gatewayKeys != "" {
		c.Gateway.AllowedAPIKeys = splitCSV(gatewayKeys)
	}
	if defaultPolicyID := os.Getenv("GATEWAY_DEFAULT_VIDEO_URL_POLICY_ID"); defaultPolicyID != "" {
		c.Gateway.DefaultVideoURLPolicyID = strings.TrimSpace(defaultPolicyID)
	}
	if uploadMaxBytes := os.Getenv("GATEWAY_VIDEO_UPLOAD_MAX_BYTES"); uploadMaxBytes != "" {
		if value, err := strconv.ParseInt(uploadMaxBytes, 10, 64); err == nil {
			c.Gateway.VideoUploadMaxBytes = value
		}
	}
	if downloadMaxBytes := os.Getenv("GATEWAY_VIDEO_DOWNLOAD_MAX_BYTES"); downloadMaxBytes != "" {
		if value, err := strconv.ParseInt(downloadMaxBytes, 10, 64); err == nil {
			c.Gateway.VideoDownloadMaxBytes = value
		}
	}
	if downloadTimeout := os.Getenv("GATEWAY_VIDEO_DOWNLOAD_TIMEOUT_SECONDS"); downloadTimeout != "" {
		if value, err := strconv.Atoi(downloadTimeout); err == nil {
			c.Gateway.VideoDownloadTimeoutSecs = value
		}
	}
	if allowedExts := os.Getenv("GATEWAY_ALLOWED_VIDEO_EXTENSIONS"); allowedExts != "" {
		c.Gateway.AllowedVideoExtensions = splitCSV(allowedExts)
	}
	if allowedMIMETypes := os.Getenv("GATEWAY_ALLOWED_VIDEO_MIME_TYPES"); allowedMIMETypes != "" {
		c.Gateway.AllowedVideoMIMETypes = splitCSV(allowedMIMETypes)
	}
	if rawPolicies := os.Getenv("GATEWAY_VIDEO_URL_POLICIES_JSON"); rawPolicies != "" {
		policies, err := parseGatewayVideoURLPolicies(rawPolicies)
		if err != nil {
			return err
		}
		c.Gateway.VideoURLPolicies = policies
	}
	if dbDriver := os.Getenv("DATABASE_DRIVER"); dbDriver != "" {
		c.Database.Driver = dbDriver
	}
	if dbDSN := os.Getenv("DATABASE_DSN"); dbDSN != "" {
		c.Database.DSN = dbDSN
	}
	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		c.Database.Path = dbPath
	}
	if queueMaxSize := os.Getenv("QUEUE_MAX_SIZE"); queueMaxSize != "" {
		if v, err := strconv.Atoi(queueMaxSize); err == nil {
			c.Queue.MaxSize = v
		}
	}
	if queueWorkers := os.Getenv("QUEUE_WORKERS"); queueWorkers != "" {
		if v, err := strconv.Atoi(queueWorkers); err == nil {
			c.Queue.Workers = v
		}
	}
	if queueTaskTimeout := os.Getenv("QUEUE_TASK_TIMEOUT"); queueTaskTimeout != "" {
		if v, err := strconv.Atoi(queueTaskTimeout); err == nil {
			c.Queue.TaskTimeout = v
		}
	}
	if legacyTenantID := firstNonEmptyEnv("MEDIA_LEGACY_TENANT_ID", "LEGACY_TENANT_ID"); legacyTenantID != "" {
		c.LegacyTenant.TenantID = strings.TrimSpace(legacyTenantID)
	}
	c.loadModelRuntimeEnv()
	if imageGenerationTimeout := os.Getenv("IMAGE_GENERATION_TIMEOUT_SECONDS"); imageGenerationTimeout != "" {
		if v, err := strconv.Atoi(imageGenerationTimeout); err == nil {
			c.BackgroundTask.ImageGenerationTimeoutSeconds = v
		}
	}
	if statsAggregationInterval := os.Getenv("STATS_AGGREGATION_INTERVAL_MINUTES"); statsAggregationInterval != "" {
		if v, err := strconv.Atoi(statsAggregationInterval); err == nil {
			c.BackgroundTask.StatsAggregationIntervalMinutes = v
		}
	}
	c.loadVideoPollingEnv()
	if value := os.Getenv("GENERATION_TERMINAL_WRITER_ENABLED"); value != "" {
		c.BackgroundTask.GenerationTerminalWriterEnabled = parseBoolEnv(value)
	}
	if value := os.Getenv("GENERATION_TERMINAL_PROCESSOR_SHADOW_ENABLED"); value != "" {
		c.BackgroundTask.GenerationTerminalProcessorShadowEnabled = parseBoolEnv(value)
	}
	if value := os.Getenv("GENERATION_TERMINAL_PROCESSOR_APPLY_ENABLED"); value != "" {
		c.BackgroundTask.GenerationTerminalProcessorApplyEnabled = parseBoolEnv(value)
	}
	if value := os.Getenv("GENERATION_DURABLE_RETRY_PUBLISHER_ENABLED"); value != "" {
		c.BackgroundTask.GenerationDurableRetryPublisherEnabled = parseBoolEnv(value)
	}
	if value := os.Getenv("GENERATION_FENCING_COMPATIBLE_WRITE_ENABLED"); value != "" {
		c.BackgroundTask.GenerationFencingCompatibleWriteEnabled = parseBoolEnv(value)
	}
	if value := os.Getenv("GENERATION_FENCING_ENFORCE_ENABLED"); value != "" {
		c.BackgroundTask.GenerationFencingEnforceEnabled = parseBoolEnv(value)
	}
	if value := os.Getenv("GENERATION_HISTORY_APPLY_ENABLED"); value != "" {
		c.BackgroundTask.GenerationHistoryApplyEnabled = parseBoolEnv(value)
	}
	if statsAggregationLookbackDays := os.Getenv("STATS_AGGREGATION_LOOKBACK_DAYS"); statsAggregationLookbackDays != "" {
		if v, err := strconv.Atoi(statsAggregationLookbackDays); err == nil {
			c.BackgroundTask.StatsAggregationLookbackDays = v
		}
	}
	if statsRetentionDays := os.Getenv("STATS_RETENTION_DAYS"); statsRetentionDays != "" {
		if v, err := strconv.Atoi(statsRetentionDays); err == nil {
			c.BackgroundTask.StatsRetentionDays = v
		}
	}
	if inputImageOriginalCacheTTLHours := os.Getenv("INPUT_IMAGE_ORIGINAL_CACHE_TTL_HOURS"); inputImageOriginalCacheTTLHours != "" {
		if v, err := strconv.Atoi(inputImageOriginalCacheTTLHours); err == nil {
			c.BackgroundTask.InputImageOriginalCacheTTLHours = v
		}
	}
	if inputImageOriginalCacheCleanupInterval := os.Getenv("INPUT_IMAGE_ORIGINAL_CACHE_CLEANUP_INTERVAL_MINUTES"); inputImageOriginalCacheCleanupInterval != "" {
		if v, err := strconv.Atoi(inputImageOriginalCacheCleanupInterval); err == nil {
			c.BackgroundTask.InputImageOriginalCacheCleanupIntervalMinutes = v
		}
	}
	if inputImageUploadSessionTTLHours := os.Getenv("INPUT_IMAGE_UPLOAD_SESSION_TTL_HOURS"); inputImageUploadSessionTTLHours != "" {
		if v, err := strconv.Atoi(inputImageUploadSessionTTLHours); err == nil {
			c.BackgroundTask.InputImageUploadSessionTTLHours = v
		}
	}
	if inputImageUploadCleanupInterval := os.Getenv("INPUT_IMAGE_UPLOAD_CLEANUP_INTERVAL_MINUTES"); inputImageUploadCleanupInterval != "" {
		if v, err := strconv.Atoi(inputImageUploadCleanupInterval); err == nil {
			c.BackgroundTask.InputImageUploadCleanupIntervalMinutes = v
		}
	}
	if inputImageUploadMaxActivePerUser := os.Getenv("INPUT_IMAGE_UPLOAD_MAX_ACTIVE_PER_USER"); inputImageUploadMaxActivePerUser != "" {
		if v, err := strconv.Atoi(inputImageUploadMaxActivePerUser); err == nil {
			c.BackgroundTask.InputImageUploadMaxActivePerUser = v
		}
	}
	if interval := os.Getenv("MASTER_SKU_DAILY_SNAPSHOT_INTERVAL_MINUTES"); interval != "" {
		if value, err := strconv.Atoi(interval); err == nil && value > 0 {
			c.BackgroundTask.MasterSKUDailySnapshotIntervalMinutes = value
		}
	}
	if enabled := os.Getenv("MEDIA_STORAGE_RECONCILE_ENABLED"); enabled != "" {
		if value, err := strconv.ParseBool(enabled); err == nil {
			c.BackgroundTask.MediaStorageReconcileEnabled = &value
		}
	}
	if interval := os.Getenv("MEDIA_STORAGE_RECONCILE_INTERVAL_MINUTES"); interval != "" {
		if value, err := strconv.Atoi(interval); err == nil {
			c.BackgroundTask.MediaStorageReconcileIntervalMinutes = value
		}
	}
	if batchSize := os.Getenv("MEDIA_STORAGE_RECONCILE_BATCH_SIZE"); batchSize != "" {
		if value, err := strconv.Atoi(batchSize); err == nil {
			c.BackgroundTask.MediaStorageReconcileBatchSize = value
		}
	}
	if apiDefaultTimeout := os.Getenv("API_DEFAULT_TIMEOUT"); apiDefaultTimeout != "" {
		if v, err := strconv.Atoi(apiDefaultTimeout); err == nil {
			c.API.DefaultTimeout = v
		}
	}
	if apiMaxRetries := os.Getenv("API_MAX_RETRIES"); apiMaxRetries != "" {
		if v, err := strconv.Atoi(apiMaxRetries); err == nil {
			c.API.MaxRetries = v
		}
	}
	if apiRetryDelay := os.Getenv("API_RETRY_DELAY"); apiRetryDelay != "" {
		if v, err := strconv.Atoi(apiRetryDelay); err == nil {
			c.API.RetryDelay = v
		}
	}
	if imagesPath := os.Getenv("STORAGE_IMAGES_PATH"); imagesPath != "" {
		c.Storage.ImagesPath = imagesPath
	}
	if thumbCachePath := os.Getenv("STORAGE_THUMBNAIL_CACHE_PATH"); thumbCachePath != "" {
		c.Storage.ThumbnailCachePath = thumbCachePath
	}
	if downloadTimeout := os.Getenv("STORAGE_DOWNLOAD_TIMEOUT"); downloadTimeout != "" {
		if v, err := strconv.Atoi(downloadTimeout); err == nil {
			c.Storage.DownloadTimeout = v
		}
	}
	if allowPrivate := firstNonEmptyEnv("STORAGE_ALLOW_PRIVATE_IPS", "ALLOW_PRIVATE_IPS"); allowPrivate != "" {
		c.Storage.AllowPrivateIPs = parseBoolEnv(allowPrivate)
	}
	// ALLOW_MEMORY_STORE 必须独立判断：它曾经被嵌在上面的 if 块里，导致
	// 「不设 ALLOW_PRIVATE_IPS 时该开关永远读不到」——一个静默失效的安全开关
	// 比没有开关更危险（INK-108 实测发现）。
	if allowMemory := strings.TrimSpace(os.Getenv("ALLOW_MEMORY_STORE")); allowMemory != "" {
		c.Storage.AllowMemoryStore = parseBoolEnv(allowMemory)
	}
	if enabled := firstNonEmptyEnv("STORAGE_OBJECT_ENABLED", "OBJECT_STORAGE_ENABLED", "S3_ENABLED", "AUTH_CENTER_RUNTIME_S3_ENABLED", "AUTH_STORAGE_ENABLED"); enabled != "" {
		c.Storage.ObjectStorage.Enabled = parseBoolEnv(enabled)
	}
	if storageType := firstNonEmptyEnv("AUTH_STORAGE_TYPE"); storageType != "" {
		normalizedType := strings.ToLower(strings.TrimSpace(storageType))
		switch normalizedType {
		case "s3", "minio", "aws":
			// s3 类取值归一化为 Provider="s3"（MinIO/AWS S3 语义），
			// 仅当 Provider 为空时写入，保留下方 STORAGE_OBJECT_PROVIDER 等显式覆盖的优先级。
			c.Storage.ObjectStorage.Enabled = true
			if c.Storage.ObjectStorage.Provider == "" {
				c.Storage.ObjectStorage.Provider = "s3"
			}
		case "oss", "aliyun":
			// oss/aliyun 归一化为 Provider="aliyun"，保持现状语义。
			c.Storage.ObjectStorage.Enabled = true
			if c.Storage.ObjectStorage.Provider == "" {
				c.Storage.ObjectStorage.Provider = "aliyun"
			}
		}
	}
	if provider := firstNonEmptyEnv("STORAGE_OBJECT_PROVIDER", "OBJECT_STORAGE_PROVIDER", "S3_PROVIDER", "AUTH_STORAGE_PROVIDER"); provider != "" {
		c.Storage.ObjectStorage.Provider = provider
	}
	if provider := firstNonEmptyEnv("ALIYUN_OSS_PROVIDER"); provider != "" {
		c.Storage.ObjectStorage.Provider = provider
	}
	if endpoint := firstNonEmptyEnv("STORAGE_OBJECT_ENDPOINT", "OBJECT_STORAGE_ENDPOINT", "S3_ENDPOINT", "AWS_ENDPOINT_URL", "AUTH_CENTER_RUNTIME_S3_ENDPOINT", "AUTH_STORAGE_S3_ENDPOINT", "ALIYUN_OSS_ENDPOINT"); endpoint != "" {
		c.Storage.ObjectStorage.Endpoint = endpoint
	}
	if publicBaseURL := firstNonEmptyEnv("STORAGE_OBJECT_PUBLIC_BASE_URL", "OBJECT_STORAGE_PUBLIC_BASE_URL", "S3_PUBLIC_BASE_URL", "AUTH_CENTER_RUNTIME_S3_PUBLIC_BASE_URL", "AUTH_STORAGE_BASE_URL"); publicBaseURL != "" {
		c.Storage.ObjectStorage.PublicBaseURL = publicBaseURL
	}
	if region := firstNonEmptyEnv("STORAGE_OBJECT_REGION", "OBJECT_STORAGE_REGION", "AWS_REGION", "AWS_DEFAULT_REGION", "AUTH_CENTER_RUNTIME_S3_REGION", "AUTH_STORAGE_S3_REGION", "ALIYUN_OSS_REGION"); region != "" {
		c.Storage.ObjectStorage.Region = region
	}
	if bucket := firstNonEmptyEnv("STORAGE_OBJECT_BUCKET", "OBJECT_STORAGE_BUCKET", "S3_BUCKET", "AWS_BUCKET", "AUTH_CENTER_RUNTIME_S3_BUCKET", "AUTH_STORAGE_S3_BUCKET", "ALIYUN_OSS_BUCKET"); bucket != "" {
		c.Storage.ObjectStorage.Bucket = bucket
	}
	if accessKey := firstNonEmptyEnv("STORAGE_OBJECT_ACCESS_KEY_ID", "OBJECT_STORAGE_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID", "AUTH_CENTER_RUNTIME_S3_ACCESS_KEY_ID", "AUTH_STORAGE_S3_ACCESS_KEY", "ALIYUN_ACCESS_KEY_ID"); accessKey != "" {
		c.Storage.ObjectStorage.AccessKeyID = accessKey
	}
	if secretKey := firstNonEmptyEnv("STORAGE_OBJECT_SECRET_ACCESS_KEY", "OBJECT_STORAGE_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", "AUTH_CENTER_RUNTIME_S3_SECRET_ACCESS_KEY", "AUTH_STORAGE_S3_SECRET_KEY", "ALIYUN_ACCESS_KEY_SECRET"); secretKey != "" {
		c.Storage.ObjectStorage.SecretAccessKey = secretKey
	}
	if sessionToken := firstNonEmptyEnv("STORAGE_OBJECT_SESSION_TOKEN", "OBJECT_STORAGE_SESSION_TOKEN", "AWS_SESSION_TOKEN", "AUTH_CENTER_RUNTIME_S3_SESSION_TOKEN"); sessionToken != "" {
		c.Storage.ObjectStorage.SessionToken = sessionToken
	}
	if rootPath := firstNonEmptyEnv("STORAGE_OBJECT_ROOT_PATH", "OBJECT_STORAGE_ROOT_PATH", "S3_ROOT_PATH", "AUTH_CENTER_RUNTIME_S3_ROOT_PATH"); rootPath != "" {
		c.Storage.ObjectStorage.RootPath = rootPath
	}
	if forcePathStyle := firstNonEmptyEnv("STORAGE_OBJECT_FORCE_PATH_STYLE", "OBJECT_STORAGE_FORCE_PATH_STYLE", "S3_FORCE_PATH_STYLE", "AUTH_CENTER_RUNTIME_S3_FORCE_PATH_STYLE", "AUTH_STORAGE_S3_PATH_STYLE"); forcePathStyle != "" {
		c.Storage.ObjectStorage.ForcePathStyle = parseBoolEnv(forcePathStyle)
	}
	if disableSSL := firstNonEmptyEnv("STORAGE_OBJECT_DISABLE_SSL", "OBJECT_STORAGE_DISABLE_SSL", "S3_DISABLE_SSL", "AUTH_CENTER_RUNTIME_S3_DISABLE_SSL"); disableSSL != "" {
		c.Storage.ObjectStorage.DisableSSL = parseBoolEnv(disableSSL)
	}
	if preserveUploads := firstNonEmptyEnv("STORAGE_OBJECT_PRESERVE_UPLOADS_BEHAVIOR", "OBJECT_STORAGE_PRESERVE_UPLOADS_BEHAVIOR", "S3_PRESERVE_UPLOADS_BEHAVIOR"); preserveUploads != "" {
		c.Storage.ObjectStorage.PreserveUploads = parseBoolEnv(preserveUploads)
	}
	if allowDirectKey := firstNonEmptyEnv("OSS_ALLOW_DIRECT_KEY", "STORAGE_OBJECT_ALLOW_DIRECT_KEY", "OBJECT_STORAGE_ALLOW_DIRECT_KEY"); allowDirectKey != "" {
		c.Storage.ObjectStorage.AllowDirectKey = parseBoolEnv(allowDirectKey)
	}
	if roleARN := firstNonEmptyEnv("ALIYUN_OSS_ROLE_ARN", "STORAGE_OBJECT_ROLE_ARN", "OBJECT_STORAGE_ROLE_ARN"); roleARN != "" {
		c.Storage.ObjectStorage.RoleARN = roleARN
	}
	if stsEndpoint := firstNonEmptyEnv("ALIYUN_STS_ENDPOINT", "STORAGE_OBJECT_STS_ENDPOINT", "OBJECT_STORAGE_STS_ENDPOINT"); stsEndpoint != "" {
		c.Storage.ObjectStorage.STSEndpoint = stsEndpoint
	}
	if stsTTL := firstNonEmptyEnv("ALIYUN_OSS_STS_TTL_SECONDS", "STORAGE_OBJECT_STS_TTL_SECONDS", "OBJECT_STORAGE_STS_TTL_SECONDS"); stsTTL != "" {
		if v, err := strconv.ParseInt(stsTTL, 10, 64); err == nil {
			c.Storage.ObjectStorage.STSRoleSessionTTLSeconds = v
		}
	}
	if directUpload := firstNonEmptyEnv("ALIYUN_OSS_FRONTEND_DIRECT_UPLOAD", "STORAGE_OBJECT_FRONTEND_DIRECT_UPLOAD", "OBJECT_STORAGE_FRONTEND_DIRECT_UPLOAD"); directUpload != "" {
		c.Storage.ObjectStorage.FrontendDirectUploadEnabled = parseBoolEnv(directUpload)
	}
	if cookieSecure := os.Getenv("COOKIE_SECURE"); cookieSecure != "" {
		c.Auth.CookieSecure = cookieSecure == strTrue || cookieSecure == "1"
	}
	if redisEnabled := os.Getenv("REDIS_ENABLED"); redisEnabled != "" {
		c.Redis.Enabled = redisEnabled == strTrue || redisEnabled == "1"
	}
	if remoteURL := firstNonEmptyEnv("GARMENT_STYLE_LIBRARY_URL", "STYLE_LIBRARY_REMOTE_URL"); remoteURL != "" {
		c.StyleLibrary.RemoteURL = remoteURL
	}
	if remoteToken := firstNonEmptyEnv("GARMENT_ACCOUNT_API_TOKEN", "GARMENT_ACCTOKEN", "STYLE_LIBRARY_REMOTE_TOKEN"); remoteToken != "" {
		c.StyleLibrary.RemoteToken = remoteToken
	}
	if timeout := firstNonEmptyEnv("GARMENT_STYLE_LIBRARY_TIMEOUT_SECONDS", "STYLE_LIBRARY_REMOTE_TIMEOUT_SECONDS"); timeout != "" {
		if v, err := strconv.Atoi(timeout); err == nil {
			c.StyleLibrary.RemoteTimeout = v
		}
	}
	if pageSize := firstNonEmptyEnv("GARMENT_STYLE_LIBRARY_PAGE_SIZE", "STYLE_LIBRARY_REMOTE_PAGE_SIZE"); pageSize != "" {
		if v, err := strconv.Atoi(pageSize); err == nil {
			c.StyleLibrary.RemotePageSize = v
		}
	}
	if retentionDays := os.Getenv("SOFT_DELETE_RETENTION_DAYS"); retentionDays != "" {
		if v, err := strconv.Atoi(retentionDays); err == nil {
			c.SoftDelete.RetentionDays = v
		}
	}
	if cdnURL := os.Getenv("CDN_URL"); cdnURL != "" {
		c.CDNUrl = cdnURL
	} else if c.CDN.Enabled && c.CDN.StaticURL != "" {
		c.CDNUrl = c.CDN.StaticURL
	}
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		c.Auth.JWTSecret = jwtSecret
	}
	if opaqueEnabled := firstNonEmptyEnv("AUTH_OPAQUE_SESSION_ENABLED", "OPAQUE_SESSION_ENABLED"); opaqueEnabled != "" {
		c.Auth.OpaqueSessionEnabled = parseBoolEnv(opaqueEnabled)
	}
	if v := firstNonEmptyEnv("AUTH_CENTER_ISSUER_URL", "SSO_ISSUER"); v != "" {
		c.SSO.Issuer = v
		c.AuthCenter.IssuerURL = v
	}
	if v := firstNonEmptyEnv("AUTH_CENTER_CLIENT_ID", "SSO_CLIENT_ID"); v != "" {
		c.SSO.ClientID = v
		c.AuthCenter.ClientID = v
	}
	if v := firstNonEmptyEnv("AUTH_CENTER_CLIENT_SECRET", "SSO_CLIENT_SECRET"); v != "" {
		c.SSO.ClientSecret = v
		c.AuthCenter.ClientSecret = v
	}
	if v := os.Getenv("SSO_TENANT_ID"); v != "" {
		c.SSO.TenantID = strings.TrimSpace(v)
	} else if v := os.Getenv("AUTH_CENTER_TENANT_ID"); v != "" {
		c.SSO.TenantID = strings.TrimSpace(v)
	}
	if v := firstNonEmptyEnv("AUTH_CENTER_REDIRECT_URI", "SSO_REDIRECT_URI"); v != "" {
		c.SSO.RedirectURI = v
		c.AuthCenter.RedirectURI = v
	}
	if v := firstNonEmptyEnv("AUTH_CENTER_INTERNAL_SERVICE_TOKEN", "AUTH_CENTER_AUTHORIZATION_SERVICE_TOKEN"); v != "" {
		c.AuthCenter.InternalServiceToken = v
	}
	if v := os.Getenv("SSO_ADMIN_REDIRECT_URI"); v != "" {
		c.SSO.AdminRedirectURI = v
	}
	if v := os.Getenv("SSO_SCOPES"); v != "" {
		c.SSO.Scopes = strings.Fields(v)
	}
	if v := os.Getenv("SSO_FRONTEND_URL"); v != "" {
		c.SSO.FrontendURL = v
		c.Auth.FrontendURL = v
	}
	if v := os.Getenv("SSO_ADMIN_FRONTEND_URL"); v != "" {
		c.SSO.AdminFrontendURL = v
	}
	if c.SSO.Issuer != "" && c.SSO.ClientID != "" {
		c.SSO.Enabled = true
	}
	if v := os.Getenv("AUTH_CENTER_BASE_URL"); v != "" {
		c.AuthCenter.BaseURL = v
	}
	if c.AuthCenter.IssuerURL == "" && c.AuthCenter.BaseURL != "" {
		c.AuthCenter.IssuerURL = c.AuthCenter.BaseURL
	} else if c.AuthCenter.BaseURL == "" && c.AuthCenter.IssuerURL != "" {
		c.AuthCenter.BaseURL = c.AuthCenter.IssuerURL
	}
	if c.SSO.Issuer == "" && c.AuthCenter.IssuerURL != "" {
		c.SSO.Issuer = c.AuthCenter.IssuerURL
	} else if c.AuthCenter.IssuerURL == "" && c.SSO.Issuer != "" {
		c.AuthCenter.IssuerURL = c.SSO.Issuer
	}
	if c.AuthCenter.ClientID == "" && c.SSO.ClientID != "" {
		c.AuthCenter.ClientID = c.SSO.ClientID
	} else if c.SSO.ClientID == "" && c.AuthCenter.ClientID != "" {
		c.SSO.ClientID = c.AuthCenter.ClientID
	}
	if c.AuthCenter.ClientSecret == "" && c.SSO.ClientSecret != "" {
		c.AuthCenter.ClientSecret = c.SSO.ClientSecret
	} else if c.SSO.ClientSecret == "" && c.AuthCenter.ClientSecret != "" {
		c.SSO.ClientSecret = c.AuthCenter.ClientSecret
	}
	if c.AuthCenter.RedirectURI == "" && c.SSO.RedirectURI != "" {
		c.AuthCenter.RedirectURI = c.SSO.RedirectURI
	} else if c.SSO.RedirectURI == "" && c.AuthCenter.RedirectURI != "" {
		c.SSO.RedirectURI = c.AuthCenter.RedirectURI
	}
	if c.App.Env == "development" && os.Getenv("COOKIE_SECURE") == "" && strings.HasPrefix(strings.ToLower(c.SSO.Issuer), "http://") {
		c.Auth.CookieSecure = false
	}
	if v := os.Getenv("AUTH_CENTER_ORG_SYNC_MODE"); v != "" {
		c.AuthCenter.OrgSync.Mode = v
	}
	if v := os.Getenv("AUTH_CENTER_ORG_SNAPSHOT_PATH"); v != "" {
		c.AuthCenter.OrgSync.SnapshotPath = v
	}
	if v := os.Getenv("AUTH_CENTER_ORG_SNAPSHOT_TOKEN"); v != "" {
		c.AuthCenter.OrgSync.SnapshotToken = v
	}
	if v := os.Getenv("AUTH_CENTER_SCIM_TOKEN"); v != "" {
		c.AuthCenter.OrgSync.SCIMToken = v
	}
	if v := os.Getenv("AUTH_CENTER_ORG_SYNC_INTERVAL"); v != "" {
		c.AuthCenter.OrgSync.Interval = v
	}
	if v := os.Getenv("AUTH_CENTER_ORG_SYNC_MAX_STALENESS"); v != "" {
		c.AuthCenter.OrgSync.MaxStaleness = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_MODE"); v != "" {
		c.AuthCenter.Authorization.Mode = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_SERVICE_TOKEN"); v != "" {
		c.AuthCenter.Authorization.ServiceToken = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_ALLOW_LEGACY_BODY_CONTEXT"); v != "" {
		c.AuthCenter.Authorization.AllowLegacyBodyContext = strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_CHECK_PATH"); v != "" {
		c.AuthCenter.Authorization.CheckPath = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_SCOPE_PATH"); v != "" {
		c.AuthCenter.Authorization.ScopePath = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_VERSIONS_PATH"); v != "" {
		c.AuthCenter.Authorization.VersionsPath = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_TIMEOUT"); v != "" {
		c.AuthCenter.Authorization.Timeout = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_CACHE_TTL"); v != "" {
		c.AuthCenter.Authorization.CacheTTL = v
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_INVALIDATION_WEBHOOK_ENABLED"); v != "" {
		c.AuthCenter.Authorization.InvalidationWebhookEnabled = strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	}
	if v := os.Getenv("AUTH_CENTER_AUTHORIZATION_SHADOW_PERMISSIONS"); v != "" {
		c.AuthCenter.Authorization.ShadowPermissions = splitCSV(v)
	}
	if v := os.Getenv("AUTH_CENTER_USER_ROLES_ENABLED"); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			c.AuthCenter.UserRoles.Enabled = true
		case "0", "false", "no", "off":
			c.AuthCenter.UserRoles.Enabled = false
		}
	}
	if v := os.Getenv("AUTH_CENTER_USER_ROLES_SERVICE_TOKEN"); v != "" {
		c.AuthCenter.UserRoles.ServiceToken = v
	}
	if v := os.Getenv("AUTH_CENTER_USER_ROLES_PATH_TEMPLATE"); v != "" {
		c.AuthCenter.UserRoles.PathTemplate = v
	}
	if v := os.Getenv("AUTH_CENTER_USER_ROLES_TIMEOUT"); v != "" {
		c.AuthCenter.UserRoles.Timeout = v
	}
	c.applyAuthCenterUserRolesDefaults()
	if klingEnabled := os.Getenv("KLING_CALLBACK_ENABLED"); klingEnabled != "" {
		c.Callback.KlingEnabled = klingEnabled == strTrue || klingEnabled == "1"
	}
	if mediaEnabled := os.Getenv("MEDIA_CALLBACK_ENABLED"); mediaEnabled != "" {
		c.Callback.MediaEnabled = mediaEnabled == strTrue || mediaEnabled == "1"
	}
	if c.Callback.KlingEnabled {
		if klingSecret := os.Getenv("KLING_CALLBACK_SECRET"); klingSecret != "" {
			c.Callback.KlingSecret = klingSecret
		}
	}
	if c.Callback.MediaEnabled {
		if mediaSecret := os.Getenv("MEDIA_CALLBACK_SECRET"); mediaSecret != "" {
			c.Callback.MediaSecret = mediaSecret
		}
		if allowIPs := os.Getenv("MEDIA_CALLBACK_ALLOWED_IPS"); allowIPs != "" {
			c.Callback.MediaAllowedIPs = splitCSV(allowIPs)
		}
	}
	if c.Callback.KlingEnabled {
		if allowIPs := os.Getenv("KLING_CALLBACK_ALLOWED_IPS"); allowIPs != "" {
			c.Callback.KlingAllowedIPs = splitCSV(allowIPs)
		}
	}
	if trustedProxies := os.Getenv("CALLBACK_TRUSTED_PROXY_CIDRS"); trustedProxies != "" {
		c.Callback.TrustedProxyCIDRs = splitCSV(trustedProxies)
	}
	if trustedProxies := os.Getenv("API_TRUSTED_PROXY_CIDRS"); trustedProxies != "" {
		c.RateLimit.TrustedProxyCIDRs = splitCSV(trustedProxies)
	}
	if verifyEnabled := os.Getenv("CALLBACK_SIGNATURE_VERIFY"); verifyEnabled != "" {
		c.Callback.EnableSignatureVerify = verifyEnabled == strTrue || verifyEnabled == "1"
	}
	if allowedOrigins := os.Getenv("ALLOWED_ORIGINS"); allowedOrigins != "" {
		c.Security.AllowedOrigins = splitCSV(allowedOrigins)
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		c.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		c.Logging.Format = logFormat
	}
	if logOutput := os.Getenv("LOG_OUTPUT"); logOutput != "" {
		c.Logging.Output = logOutput
	}
	if logFilePath := os.Getenv("LOG_FILE_PATH"); logFilePath != "" {
		c.Logging.FilePath = logFilePath
	}
	if logErrorFilePath := os.Getenv("LOG_ERROR_FILE_PATH"); logErrorFilePath != "" {
		c.Logging.ErrorFilePath = logErrorFilePath
	}
	if sampleEvery := os.Getenv("HTTP_HOMEPAGE_SUCCESS_LOG_SAMPLE_EVERY"); sampleEvery != "" {
		if value, err := strconv.Atoi(sampleEvery); err == nil && value > 0 {
			c.Logging.HTTPHomepageSuccessSampleEvery = value
		}
	}
	if wsReadBuf := os.Getenv("WEBSOCKET_READ_BUFFER_SIZE"); wsReadBuf != "" {
		if v, err := strconv.Atoi(wsReadBuf); err == nil {
			c.WebSocket.ReadBufferSize = v
		}
	}
	if wsWriteBuf := os.Getenv("WEBSOCKET_WRITE_BUFFER_SIZE"); wsWriteBuf != "" {
		if v, err := strconv.Atoi(wsWriteBuf); err == nil {
			c.WebSocket.WriteBufferSize = v
		}
	}
	if wsPing := os.Getenv("WEBSOCKET_PING_PERIOD"); wsPing != "" {
		if v, err := strconv.Atoi(wsPing); err == nil {
			c.WebSocket.PingPeriod = v
		}
	}
	if bcryptCost := os.Getenv("BCRYPT_COST"); bcryptCost != "" {
		if v, err := strconv.Atoi(bcryptCost); err == nil {
			c.Security.BcryptCost = v
		}
	}
	if dbMaxOpen := os.Getenv("DB_MAX_OPEN_CONNS"); dbMaxOpen != "" {
		if v, err := strconv.Atoi(dbMaxOpen); err == nil {
			c.Database.MaxOpenConns = v
		}
	}
	if dbMaxIdle := os.Getenv("DB_MAX_IDLE_CONNS"); dbMaxIdle != "" {
		if v, err := strconv.Atoi(dbMaxIdle); err == nil {
			c.Database.MaxIdleConns = v
		}
	}
	if dbConnLifetime := os.Getenv("DB_CONN_MAX_LIFETIME"); dbConnLifetime != "" {
		if v, err := strconv.Atoi(dbConnLifetime); err == nil {
			c.Database.ConnMaxLifetime = v
		}
	}
	if dbConnIdleTime := os.Getenv("DB_CONN_MAX_IDLE_TIME"); dbConnIdleTime != "" {
		if v, err := strconv.Atoi(dbConnIdleTime); err == nil {
			c.Database.ConnMaxIdleTime = v
		}
	}
	if dbSlowThreshold := os.Getenv("DB_SLOW_THRESHOLD_MS"); dbSlowThreshold != "" {
		if v, err := strconv.Atoi(dbSlowThreshold); err == nil {
			c.Database.SlowThresholdMs = v
		}
	} else if dbSlowThresholdLegacy := os.Getenv("DATABASE_SLOW_THRESHOLD_MS"); dbSlowThresholdLegacy != "" {
		if v, err := strconv.Atoi(dbSlowThresholdLegacy); err == nil {
			c.Database.SlowThresholdMs = v
		}
	}

	// 设计考量 (Task 1.5 唯一配置源收敛)：
	// 将原先散落在业务代码各处的 os.Getenv 统一提升至 Config 统一加载，
	// 确保所有配置项强类型化并在启动期完成校验与默认值应用。
	if enabled := os.Getenv("CUTOUT_ENABLED"); enabled != "" {
		c.Cutout.Enabled = strings.EqualFold(strings.TrimSpace(enabled), "true") || strings.TrimSpace(enabled) == "1"
	}
	if workerURL := os.Getenv("CUTOUT_WORKER_URL"); workerURL != "" {
		c.Cutout.WorkerURL = strings.TrimSpace(workerURL)
	}
	if model := os.Getenv("CUTOUT_MODEL"); model != "" {
		c.Cutout.Model = strings.TrimSpace(model)
	}
	if token := os.Getenv("CUTOUT_AUTH_TOKEN"); token != "" {
		c.Cutout.AuthToken = strings.TrimSpace(token)
	}
	if timeout := os.Getenv("CUTOUT_TIMEOUT_SECONDS"); timeout != "" {
		if v, err := strconv.Atoi(timeout); err == nil && v > 0 {
			c.Cutout.TimeoutSeconds = v
		}
	}

	if appID := os.Getenv("FEISHU_APP_ID"); appID != "" {
		c.FeishuSync.AppID = strings.TrimSpace(appID)
	}
	if appSecret := os.Getenv("FEISHU_APP_SECRET"); appSecret != "" {
		c.FeishuSync.AppSecret = strings.TrimSpace(appSecret)
	}
	if sheetURL := os.Getenv("FEISHU_SHEET_URL"); sheetURL != "" {
		c.FeishuSync.SheetURL = strings.TrimSpace(sheetURL)
	}
	if sheetName := os.Getenv("FEISHU_SHEET_NAME"); sheetName != "" {
		c.FeishuSync.SheetName = strings.TrimSpace(sheetName)
	}
	if sheetKind := os.Getenv("FEISHU_SHEET_KIND"); sheetKind != "" {
		c.FeishuSync.SheetKind = strings.TrimSpace(sheetKind)
	}

	if raw := os.Getenv("AI_RATE_LIMIT_CAPACITY"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			c.RateLimit.AICapacity = v
		}
	}
	if raw := os.Getenv("AI_RATE_LIMIT_PER_MINUTE"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			c.RateLimit.AIPerMinute = v
		}
	}

	if key := os.Getenv("MCP_SECRET_KEY"); key != "" {
		c.MCPServer.SecretKey = strings.TrimSpace(key)
	}

	if v := os.Getenv("SSO_ADMIN_TENANT_ID"); v != "" {
		c.SSO.AdminTenantID = strings.TrimSpace(v)
	}

	if v := firstNonEmptyEnv("FIRECRAWL_API_KEYS", "FIRECRAWL_API_KEY"); v != "" {
		c.WebSearch.FirecrawlAPIKeys = strings.TrimSpace(v)
	}
	if v := os.Getenv("FIRECRAWL_BASE_URL"); v != "" {
		c.WebSearch.FirecrawlBaseURL = strings.TrimSpace(v)
	}
	if v := os.Getenv("TAVILY_API_KEYS"); v != "" {
		c.WebSearch.TavilyAPIKeys = strings.TrimSpace(v)
	}
	if v := os.Getenv("SEARXNG_URL"); v != "" {
		c.WebSearch.SearxngURLs = strings.TrimSpace(v)
	}
	return nil
}
