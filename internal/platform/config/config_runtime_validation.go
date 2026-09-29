package config

import (
	"fmt"
	"os"
	"strings"
)

// validate verifies the complete runtime configuration.
func (c *Config) validate() error {
	if err := c.validateEncryptionKey(); err != nil {
		return err
	}
	if err := c.validateApp(); err != nil {
		return err
	}
	if err := c.validateDatabase(); err != nil {
		return err
	}
	if err := c.validateObjectStorage(); err != nil {
		return err
	}
	if err := c.validateCallbacks(); err != nil {
		return err
	}
	if err := c.validateAuthCenterAuthorization(); err != nil {
		return err
	}
	// LEGACY_TENANT_ID（legacy_tenant.tenant_id）不在此处启动必填：
	// 它是 cmd/legacy-tenant-backfill 一次性回填命令的参数，服务运行不依赖；
	// 回填命令入口（cmd/legacy-tenant-backfill/main.go）自身校验非空。
	if err := c.validateCookiePrefixContract(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateCookiePrefixContract() error {
	domain := strings.TrimSpace(c.Auth.CookieDomain)
	for _, cookieName := range []string{c.Auth.CookieName, c.Auth.RefreshCookieName} {
		name := strings.TrimSpace(cookieName)
		if strings.HasPrefix(name, "__Host-") {
			if domain != "" {
				return fmt.Errorf("__Host- Cookie %q 禁止配置 cookie_domain", name)
			}
			if !c.Auth.CookieSecure {
				return fmt.Errorf("__Host- Cookie %q 必须启用 cookie_secure", name)
			}
		}
	}
	return nil
}
func (c *Config) validateCallbacks() error {
	if !c.IsProduction() {
		return nil
	}
	if !c.Callback.EnableSignatureVerify {
		return fmt.Errorf("生产环境必须启用回调签名验证")
	}
	if c.Callback.KlingEnabled && strings.TrimSpace(c.Callback.KlingSecret) == "" {
		return fmt.Errorf("生产环境启用 Kling 回调时必须配置 KLING_CALLBACK_SECRET")
	}
	if c.Callback.MediaEnabled && strings.TrimSpace(c.Callback.MediaSecret) == "" {
		return fmt.Errorf("生产环境启用媒体回调时必须配置 MEDIA_CALLBACK_SECRET")
	}
	return nil
}
func (c *Config) validateObjectStorage() error {
	osCfg := c.Storage.ObjectStorage
	if !osCfg.Enabled {
		return nil
	}
	if strings.TrimSpace(osCfg.Bucket) == "" {
		return fmt.Errorf("对象存储已启用但 bucket 未配置")
	}
	if strings.TrimSpace(osCfg.Endpoint) == "" {
		return fmt.Errorf("对象存储已启用但 endpoint 未配置")
	}
	if strings.TrimSpace(osCfg.AccessKeyID) == "" || strings.TrimSpace(osCfg.SecretAccessKey) == "" {
		return fmt.Errorf("对象存储已启用但访问密钥未配置完整")
	}
	// RoleARN 仅用于阿里云 OSS STS 直传：Provider 显式为 s3/minio/aws（MinIO/AWS S3）时
	// OSS 直传本就禁用（OSS 专属能力），不要求 RoleARN；其余取值（空值/aliyun/oss，默认即 OSS 语义）维持必填。
	provider := strings.ToLower(strings.TrimSpace(osCfg.Provider))
	if provider != "s3" && provider != "minio" && provider != "aws" &&
		osCfg.FrontendDirectUploadEnabled && strings.TrimSpace(osCfg.RoleARN) == "" {
		return fmt.Errorf("前端直传已启用但对象存储 role_arn 未配置")
	}
	return nil
}
func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", strTrue, "yes", "y", "on":
		return true
	default:
		return false
	}
}

// validateEncryptionKey enforces the 32-byte AES-256 key contract and production hardening.
func (c *Config) validateEncryptionKey() error {
	key := c.Security.EncryptionKey
	if key == "" {
		return fmt.Errorf("加密密钥未配置\n" + "  请通过以下方式之一设置:\n" + "  1. 环境变量: export ENCRYPTION_KEY=\"your-32-byte-key-here-123456\"\n" + "  2. 配置文件: security.encryption_key: \"your-32-byte-key-here-123456\"\n" + "  \n" + "  生成安全密钥:\n" + "    Linux/Mac: openssl rand -base64 32 | head -c 32\n" + "    Windows:   certutil -encode -f random.bin key.txt && type key.txt")
	}
	keyLen := len(key)
	if keyLen != 32 {
		return fmt.Errorf("加密密钥长度无效: 需要 32 字节,当前 %d 字节\n"+"  AES-256 要求: 密钥必须是 32 字节 (256 bits)\n"+"  生成32字节密钥:\n"+"    openssl rand -base64 32 | head -c 32", keyLen)
	}
	weakKeys := []string{"12345678901234567890123456789012", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "00000000000000000000000000000000", "your-32-byte-encryption-key-here", "your-32-byte-key-here-123456789", "your-32-byte-key-here-12345678"}
	for _, weak := range weakKeys {
		if key == weak {
			return fmt.Errorf("检测到弱加密密钥: 不能使用示例密钥或简单密钥\n" + "  请使用强随机密钥,生成命令:\n" + "    Linux/Mac: openssl rand -base64 32 | head -c 32\n" + "    Windows:   certutil -encode -f random.bin key.txt\n" + "  然后设置环境变量:\n" + "    export ENCRYPTION_KEY=\"生成的密钥\"")
		}
	}
	if c.IsProduction() {
		if os.Getenv("ENCRYPTION_KEY") == "" {
			return fmt.Errorf("生产环境安全警告: 必须通过环境变量 ENCRYPTION_KEY 设置加密密钥\n" + "  原因: 配置文件可能被提交到版本控制系统\n" + "  当前: 密钥从配置文件读取 (不安全)\n" + "  \n" + "  解决方案:\n" + "    1. 生成安全密钥: openssl rand -base64 32 | head -c 32\n" + "    2. 设置环境变量: export ENCRYPTION_KEY=\"生成的密钥\"\n" + "    3. 从配置文件移除: 删除 security.encryption_key 配置项")
		}
		if !c.Auth.CookieSecure {
			return fmt.Errorf("生产环境安全警告: 必须启用 auth.cookie_secure = true\n" + "  原因: 防止 Cookie 在非 HTTPS 连接中传输\n" + "  当前: cookie_secure = false (不安全)\n" + "  \n" + "  解决方案:\n" + "    1. 在配置文件中设置: auth.cookie_secure: true\n" + "    2. 确保使用 HTTPS 部署")
		}
	}
	return nil
}

// validateApp verifies the application port and environment name.
func (c *Config) validateApp() error {
	if c.App.Port < 1 || c.App.Port > 65535 {
		return fmt.Errorf("应用端口无效: %d (必须在 1-65535 之间)", c.App.Port)
	}
	for name, seconds := range map[string]int{
		"http_read_timeout_seconds":  c.App.HTTPReadTimeoutSeconds,
		"http_write_timeout_seconds": c.App.HTTPWriteTimeoutSeconds,
		"http_idle_timeout_seconds":  c.App.HTTPIdleTimeoutSeconds,
	} {
		if seconds <= 0 {
			return fmt.Errorf("应用 %s 无效: %d (必须大于 0)", name, seconds)
		}
	}
	validEnvs := map[string]bool{"development": true, "production": true, "test": true}
	if !validEnvs[c.App.Env] {
		return fmt.Errorf("应用环境无效: %s\n"+"  支持的环境: development, production, test\n"+"  当前: %s", c.App.Env, c.App.Env)
	}
	return nil
}

// validateDatabase verifies the PostgreSQL runtime DSN and pool constraints.
func (c *Config) validateDatabase() error {
	driver := strings.ToLower(strings.TrimSpace(c.Database.Driver))
	switch driver {
	case "":
		c.Database.Driver = strPostgres
	case strSqlite, "sqlite3":
		return fmt.Errorf("数据库驱动无效: %s (runtime 只支持 postgres；sqlite 仅测试夹具显式使用)", c.Database.Driver)
	case strPostgres, "postgresql", "pgsql", "pg":
		c.Database.Driver = strPostgres
	default:
		return fmt.Errorf("数据库驱动无效: %s (runtime 只支持 postgres；sqlite 仅测试夹具显式使用)", c.Database.Driver)
	}
	if c.Database.Driver == strPostgres && c.Database.DSN == "" {
		return fmt.Errorf("PostgreSQL DSN 未配置")
	}
	if c.Database.MaxOpenConns < 1 {
		return fmt.Errorf("数据库最大连接数无效: %d (必须 >= 1)", c.Database.MaxOpenConns)
	}
	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("数据库最大空闲连接数无效: %d (必须 >= 0)", c.Database.MaxIdleConns)
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("数据库配置错误: 最大空闲连接数 (%d) 不能大于最大连接数 (%d)", c.Database.MaxIdleConns, c.Database.MaxOpenConns)
	}
	return nil
}
