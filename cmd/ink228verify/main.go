// Command ink228verify runs behavioral probes only against an ephemeral CI database.
// No real LLM network calls, user credentials, or production resources are used.
package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	agentinfra "InkLore/internal/agent/infrastructure"
	llminfra "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/uow"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func failIfUnsafe() (string, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if os.Getenv("INK228_CI_ONLY") != "1" || dsn == "" {
		return "", fmt.Errorf("requires INK228_CI_ONLY=1 and an isolated DATABASE_DSN")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" {
		return "", fmt.Errorf("requires PostgreSQL URI")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || (host != "127.0.0.1" && host != "localhost") || port != "5432" {
		return "", fmt.Errorf("database must be local disposable PostgreSQL on port 5432")
	}
	if u.Path != "/inklore_ci" || u.Query().Get("sslmode") != "disable" {
		return "", fmt.Errorf("only isolated inklore_ci database is allowed")
	}
	return dsn, nil
}

func require(ok bool, format string, args ...any) error {
	if !ok {
		return fmt.Errorf(format, args...)
	}
	return nil
}

func run() error {
	dsn, err := failIfUnsafe()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("isolated database unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(12)
	sqlDB.SetMaxIdleConns(4)

	var actualDB string
	if err := db.Raw("SELECT current_database()").Scan(&actualDB).Error; err != nil {
		return err
	}
	if err := require(actualDB == "inklore_ci", "connected to incorrect database"); err != nil {
		return err
	}
	manager, err := uow.New(db)
	if err != nil {
		return err
	}
	store, err := llminfra.NewStore(manager)
	if err != nil {
		return err
	}
	runRepo, err := agentinfra.NewRunRepository(manager)
	if err != nil {
		return err
	}
	govRepo, err := agentinfra.NewRepository(manager)
	if err != nil {
		return err
	}
	jobs, err := agentinfra.NewJobsAdapter(nil, manager)
	if err != nil {
		return err
	}
	resolveChannel, err := llminfra.NewTenantChannelResolver(manager)
	if err != nil {
		return err
	}
	a, b, err := seedFixtures(ctx, db)
	if err != nil {
		return fmt.Errorf("fixture setup: %w", err)
	}
	if err := checkTenantRoutes(ctx, db, resolveChannel, store, a, b); err != nil {
		return fmt.Errorf("tenant isolation: %w", err)
	}
	fmt.Println("PASS: real PostgreSQL two-tenant routes, frozen versions and fail-closed credentials")
	if err := checkMockUpstream(ctx, db, resolveChannel, a, b); err != nil {
		return fmt.Errorf("mock upstream: %w", err)
	}
	fmt.Println("PASS: real loopback HTTP egress used tenant-specific model aliases and keys")
	if err := checkConcurrentRuns(ctx, db, manager, store, runRepo, govRepo, jobs, a, b); err != nil {
		return fmt.Errorf("concurrency: %w", err)
	}
	fmt.Println("PASS: real RunService atomic Run/Job, FOR SHARE publish race and stable frozen policies")
	fmt.Println("INK228_REAL_PG_PASS")
	return nil
}

func main() {
	if err := run(); err != nil {
		// Print no connection strings, secrets, or input bodies.
		msg := err.Error()
		if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
			msg = strings.ReplaceAll(msg, dsn, "[redacted]")
		}
		fmt.Fprintln(os.Stderr, "INK228_REAL_PG_FAIL:", msg)
		os.Exit(1)
	}
}
