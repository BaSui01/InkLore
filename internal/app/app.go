package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"InkLore/internal/agent/application"
	"InkLore/internal/platform/config"
	"InkLore/internal/platform/database"
	"InkLore/internal/platform/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// App 是进程内唯一组合根：负责把各域的端口与适配器装配成可运行的服务。
//
// 装配原则（见 AGENTS.md 第 1 节）：跨域调用只在此处通过 Port + adapters 连接，
// 任何业务域都不得自行导入其它域的实现。
type App struct {
	platform             *Platform
	domains              *DomainRuntime
	config               *config.Config
	db                   *gorm.DB
	router               *gin.Engine
	agentScheduler       *application.Scheduler
	workerCancel         context.CancelFunc
	workerDone           <-chan struct{}
	activateAgentTools   func()
	deactivateAgentTools func()
}

// ServerTimeouts 汇总 HTTP 服务器超时设置。
type ServerTimeouts struct {
	Read  time.Duration
	Write time.Duration
	Idle  time.Duration
}

// New 装载配置并装配应用。
func New() (*App, error) {
	cfg, err := config.Bootstrap(config.DefaultPath)
	if err != nil {
		return nil, fmt.Errorf("装载配置失败: %w", err)
	}
	return NewWithConfig(cfg)
}

// migrateOnStart 决定是否在启动期自动执行迁移。
//
// 开发环境默认开启（冷启动即可建表，便于本地起服务）；生产环境默认关闭，
// 由部署流程显式执行 migrate，避免多实例并发启动时互相抢迁移。
func migrateOnStart(cfg *config.Config) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MIGRATE_ON_START"))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return !cfg.IsProduction()
	default:
		return !cfg.IsProduction()
	}
}

// DB 暴露数据库句柄，供后续任务的 SQL 仓储装配使用。
func (a *App) DB() *gorm.DB {
	return a.db
}

// Config 暴露运行配置。
func (a *App) Config() *config.Config {
	return a.config
}

// Address 返回 HTTP 监听地址：HTTP_ADDR 环境变量优先，其次为配置 app.host/app.port。
func (a *App) Address() string {
	if address := strings.TrimSpace(os.Getenv("HTTP_ADDR")); address != "" {
		return address
	}
	host := strings.TrimSpace(a.config.App.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	port := a.config.App.Port
	if port == 0 {
		port = 8080
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// ServerTimeouts 返回 HTTP 服务器超时设置。
//
// 注意：Write 超时**不用于** http.Server，因为章节生成走 SSE 长连接，
// 设置写超时会在流式中途切断响应；SSE 的超时由业务层的请求上下文控制。
func (a *App) ServerTimeouts() ServerTimeouts {
	return ServerTimeouts{
		Read:  secondsOr(a.config.App.HTTPReadTimeoutSeconds, 120),
		Write: 0,
		Idle:  secondsOr(a.config.App.HTTPIdleTimeoutSeconds, 120),
	}
}

func secondsOr(seconds int, fallback int) time.Duration {
	if seconds <= 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

// Handler 返回 HTTP 处理器。
func (a *App) Handler() http.Handler {
	return a.router
}

// Router 返回 Gin 引擎。
func (a *App) Router() *gin.Engine {
	return a.router
}

// Runtime returns assembled internal platform and domain services.
func (a *App) Runtime() (*Platform, *DomainRuntime) {
	return a.platform, a.domains
}

// Close 释放应用持有的外部资源。
func (a *App) Close() error {
	if a.deactivateAgentTools != nil {
		a.deactivateAgentTools()
	}
	if a.workerCancel != nil {
		a.workerCancel()
		<-a.workerDone
	}
	if a.platform != nil && a.platform.Redis != nil {
		_ = a.platform.Redis.Close()
	}
	logErr := logger.Close()
	dbErr := database.Close(a.db)
	if dbErr != nil {
		return dbErr
	}
	return logErr
}
