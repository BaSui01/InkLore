// Package database 提供 PostgreSQL 连接装配与迁移执行能力。
//
// 定位：属于微内核技术基座（internal/platform），不承载任何业务语义；
// 业务域只通过组合根拿到 *gorm.DB，不直接依赖本包的具体实现细节。
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"InkLore/internal/platform/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	defaultMaxOpenConns    = 40
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 10 * time.Minute
	defaultSlowThreshold   = 200 * time.Millisecond
	defaultPingTimeout     = 5 * time.Second
)

// Options 是建立 PostgreSQL 连接所需的参数。
type Options struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	SlowThreshold   time.Duration
	Debug           bool
}

// OptionsFromConfig 将配置中心的数据库配置映射为连接参数。
func OptionsFromConfig(cfg *config.Config) Options {
	if cfg == nil {
		return Options{}
	}
	opts := Options{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Database.ConnMaxLifetime) * time.Second,
		ConnMaxIdleTime: time.Duration(cfg.Database.ConnMaxIdleTime) * time.Second,
		SlowThreshold:   time.Duration(cfg.Database.SlowThresholdMs) * time.Millisecond,
		Debug:           cfg.App.Debug,
	}
	if cfg.Logging.Level == "debug" {
		opts.Debug = true
	}
	return opts
}

// normalize 补齐零值，保证调用方只填 DSN 也能得到合理连接池。
func (o Options) normalize() Options {
	if o.MaxOpenConns <= 0 {
		o.MaxOpenConns = defaultMaxOpenConns
	}
	if o.MaxIdleConns <= 0 {
		o.MaxIdleConns = defaultMaxIdleConns
	}
	if o.MaxIdleConns > o.MaxOpenConns {
		o.MaxIdleConns = o.MaxOpenConns
	}
	if o.ConnMaxLifetime <= 0 {
		o.ConnMaxLifetime = defaultConnMaxLifetime
	}
	if o.ConnMaxIdleTime <= 0 {
		o.ConnMaxIdleTime = defaultConnMaxIdleTime
	}
	if o.SlowThreshold <= 0 {
		o.SlowThreshold = defaultSlowThreshold
	}
	return o
}

// Enabled 表示是否配置了数据库连接串。
func (o Options) Enabled() bool {
	return o.DSN != ""
}

// Open 建立 PostgreSQL 连接、配置连接池并执行一次 Ping 校验。
//
// 失败时返回的错误包含可读原因，调用方应据此拒绝启动，而不是继续以无数据库状态运行。
func Open(ctx context.Context, opts Options) (*gorm.DB, error) {
	if !opts.Enabled() {
		return nil, fmt.Errorf("PostgreSQL DSN 未配置（请设置环境变量 DATABASE_DSN 或 database.dsn）")
	}
	opts = opts.normalize()

	// SQL parameters and PostgreSQL error detail can contain private payloads.
	// Emit only safe operation metadata; never interpolate driver errors or SQL.
	gormLog := safeSQLLogger{level: gormlogger.Warn, slow: opts.SlowThreshold}

	db, err := gorm.Open(postgres.Open(opts.DSN), &gorm.Config{
		Logger:                 gormLog,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("打开 PostgreSQL 连接失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(opts.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("PostgreSQL 连接不可用（Ping 失败）: %w", err)
	}

	return db, nil
}

// Close 关闭底层连接池。
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping 探测数据库可用性，供就绪探针使用。
func Ping(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()
	return sqlDB.PingContext(pingCtx)
}

// Stats 返回连接池统计，便于观测。
func Stats(db *gorm.DB) (sql.DBStats, error) {
	if db == nil {
		return sql.DBStats{}, fmt.Errorf("数据库未初始化")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return sql.DBStats{}, err
	}
	return sqlDB.Stats(), nil
}
