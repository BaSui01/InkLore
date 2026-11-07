package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"InkLore/internal/platform/audit"
	"InkLore/internal/platform/captcha"
	"InkLore/internal/platform/config"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/database"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/files"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/mail"
	"InkLore/internal/platform/membership"
	"InkLore/internal/platform/middleware"
	"InkLore/internal/platform/notifications"
	"InkLore/internal/platform/observability"
	"InkLore/internal/platform/requestschema"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"InkLore/internal/platform/version"
	"InkLore/migrations/r1"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Platform owns the R1 shared dependencies. Domain ports are wired only by app.
type Platform struct {
	Files                      *files.Service
	Notifications              *notifications.Service
	Captcha                    *captcha.Service
	Observability              *observability.Service
	UoW                        *uow.Manager
	Scope                      *scope.Authorizer
	Idempotency                *idempotency.Executor
	Identity                   *identity.Service
	Mail                       *mail.Service
	Members                    *membership.Service
	Audit                      *audit.Store
	ProjectLogs                *logger.ProjectLogStore
	Cursor                     *cursor.Codec
	Redis                      *redis.Client
	CreatorOrigin, AdminOrigin string
}

// NewWithConfig builds only the R1 runtime. Retired routes are not mounted.
func NewWithConfig(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if err := requestschema.Initialize(); err != nil {
		return nil, err
	}
	origins, err := platformConfig()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.OptionsFromConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("R1 PostgreSQL unavailable")
	}
	success := false
	defer func() {
		if !success {
			_ = database.Close(db)
		}
	}()
	if migrateOnStart(cfg) {
		if _, err = database.Up(ctx, db, r1.FS(), r1.Dir); err != nil {
			return nil, err
		}
	} else {
		if err := database.Verify(ctx, db, r1.FS(), r1.Dir); err != nil {
			return nil, err
		}
	}
	manager, err := uow.New(db)
	if err != nil {
		return nil, err
	}
	idem, err := idempotency.New(manager, []byte(os.Getenv("IDEMPOTENCY_HASH_KEY")))
	if err != nil {
		return nil, err
	}
	platform := &Platform{UoW: manager, Scope: scope.New(manager), Idempotency: idem, Audit: audit.New(manager), ProjectLogs: logger.NewProjectLogStore(manager), CreatorOrigin: origins[0], AdminOrigin: origins[1], Captcha: captcha.NewService()}
	platform.Cursor, err = cursor.New([]byte(os.Getenv("CURSOR_SIGNING_KEY")))
	if err != nil {
		return nil, err
	}
	var limiter identity.LoginLimiter
	redisAddress := os.Getenv("REDIS_ADDR")
	if redisAddress == "" && cfg.Redis.Enabled {
		redisAddress = cfg.Redis.GetRedisAddr()
	}
	if redisAddress != "" {
		platform.Redis = redis.NewClient(&redis.Options{Addr: redisAddress, Password: os.Getenv("REDIS_PASSWORD"), DB: cfg.Redis.DB, DialTimeout: 500 * time.Millisecond, ReadTimeout: 500 * time.Millisecond, WriteTimeout: 500 * time.Millisecond, MaxRetries: -1})
		limiter = identity.NewRedisLoginLimiter(platform.Redis)
		defer func() {
			if !success {
				_ = platform.Redis.Close()
			}
		}()
	}
	platform.Identity, err = identity.New(manager, identity.Options{JWTKey: []byte(os.Getenv("JWT_SIGNING_KEY")), CSRFKey: []byte(os.Getenv("CSRF_KEY")), Limiter: limiter, OnEvent: identityAudit(platform.Audit)})
	if err != nil {
		return nil, err
	}
	mailOptions, err := mail.EnvironmentOptions()
	if err != nil {
		return nil, err
	}
	mailOptions.Identity = platform.Identity
	mailOptions.OnEvent = mailAudit(platform.Audit)
	platform.Mail, err = mail.New(manager, mailOptions)
	if err != nil {
		return nil, err
	}
	if err = platform.Identity.ConfigureEmail(platform.Mail, []byte(os.Getenv("MAIL_OTP_HASH_KEY"))); err != nil {
		return nil, err
	}
	platform.Members = membership.New(manager, platform.Identity)
	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.RedirectFixedPath = false
	engine.HandleMethodNotAllowed = true
	if err := engine.SetTrustedProxies(cfg.RateLimit.TrustedProxyCIDRs); err != nil {
		return nil, fmt.Errorf("invalid API trusted proxy CIDRs: %w", err)
	}
	engine.Use(middleware.Contract(), middleware.Logger(), middleware.Recovery(), middleware.ContractCORS(origins))
	engine.NoRoute(func(c *gin.Context) { response.Fail(c, platformerr.Contract(404, "platform.resource_not_found", nil)) })
	engine.NoMethod(func(c *gin.Context) {
		middleware.CompleteAllow(c)
		response.Fail(c, platformerr.Contract(405, "platform.method_not_allowed", nil))
	})
	engine.GET("/healthz", func(c *gin.Context) { response.OK(c, http.StatusOK, gin.H{"status": "ok", "version": version.Version}) })
	engine.GET("/readyz", func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ping(pingCtx, db); err != nil {
			response.Fail(c, platformerr.Contract(503, "platform.dependency_unavailable", nil))
			return
		}
		state := "ready"
		if platform.Redis != nil {
			redisCtx, redisCancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
			defer redisCancel()
			if err := platform.Redis.Ping(redisCtx).Err(); err != nil {
				state = "degraded"
			}
		}
		response.OK(c, http.StatusOK, gin.H{"state": state, "database": "ok"})
	})
	application := &App{config: cfg, db: db, router: engine, platform: platform}
	application.registerIdentityRoutes(engine)
	application.registerMailRoutes(engine)
	application.registerPasswordRoute(engine)
	application.registerMembershipRoutes(engine)
	application.registerAuditRoutes(engine)
	application.registerProjectLogRoutes(engine)
	application.registerAuditExportRoute(engine)
	if err = application.registerDomainRoutes(engine); err != nil {
		return nil, err
	}
	if err = application.registerPlatformServices(engine); err != nil {
		return nil, err
	}
	routes := make([]requestschema.Route, 0, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes = append(routes, requestschema.Route{Method: route.Method, Path: route.Path})
	}
	if err := requestschema.CheckRoutes(routes); err != nil {
		return nil, err
	}
	if err := logger.EnableProjectLogs(db); err != nil {
		return nil, err
	}
	application.startAgentWorker()
	success = true
	return application, nil
}

func platformConfig() ([]string, error) {
	seen := map[string]bool{}
	for _, name := range []string{"JWT_SIGNING_KEY", "CSRF_KEY", "IDEMPOTENCY_HASH_KEY", "CURSOR_SIGNING_KEY"} {
		key := os.Getenv(name)
		if len([]byte(key)) < 32 {
			return nil, fmt.Errorf("%s requires at least 32 bytes from environment", name)
		}
		if seen[key] {
			return nil, fmt.Errorf("R1 signing keys must be independent")
		}
		seen[key] = true
	}
	origins := []string{}
	for _, name := range []string{"CREATOR_ORIGIN", "ADMIN_ORIGIN"} {
		raw := os.Getenv(name)
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(raw, "*") {
			return nil, fmt.Errorf("%s requires an exact HTTP origin", name)
		}
		origins = append(origins, raw)
	}
	if origins[0] == origins[1] {
		return nil, fmt.Errorf("Creator and Admin require distinct origins")
	}
	return origins, nil
}
