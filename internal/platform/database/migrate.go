package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	// migrationTable 记录冻结 R1 流已应用的迁移版本，是幂等执行的唯一依据。
	migrationTable = "schema_migrations"
	// downMarker 分隔同一个迁移文件中的升级段与降级段。
	downMarker = "-- +migrate Down"
)

// createMigrationTableSQLFor 为指定历史表生成建表语句。
// 表结构使用 TIMESTAMPTZ，遵循全栈北京时间契约；独立迁移流使用独立历史表，
// 避免污染冻结 R1 流的既定历史。
func createMigrationTableSQLFor(table string) string {
	return "CREATE TABLE IF NOT EXISTS " + table + " (" +
		"version TEXT PRIMARY KEY, " +
		"name TEXT NOT NULL, " +
		"checksum TEXT NOT NULL, " +
		"applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())"
}

// Migration 是一条解析后的迁移脚本。
type Migration struct {
	Version  string
	Name     string
	Up       string
	Down     string
	Checksum string
}

// HasDown 表示该迁移定义了降级段，可以被回滚。
func (m Migration) HasDown() bool {
	return strings.TrimSpace(m.Down) != ""
}

// Label 返回用于日志与输出的标识，例如 000001_init。
func (m Migration) Label() string {
	return m.Version + "_" + m.Name
}

// AppliedRecord 是版本表中的一行。
type AppliedRecord struct {
	Version   string
	Name      string
	Checksum  string
	AppliedAt time.Time
}

// UpResult 描述一次向上迁移的结果。
type UpResult struct {
	Applied []string
	Skipped []string
}

// StatusRow 描述一条迁移脚本当前的应用状态。
type StatusRow struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt time.Time
}

// LoadMigrations 读取迁移目录下的全部脚本，按版本号升序返回。
//
// 约定：文件名形如 NNNNNN_描述.sql。CRLF 会被归一为 LF 后再计算校验和，
// 保证在不同操作系统上得到一致的校验和。
func LoadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	if fsys == nil {
		return nil, fmt.Errorf("迁移文件系统为空")
	}
	if dir == "" {
		dir = "."
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败 dir=%s: %w", dir, err)
	}

	result := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version, name, ok := parseMigrationFileName(entry.Name())
		if !ok {
			continue
		}
		relative := path.Join(dir, entry.Name())
		raw, readErr := fs.ReadFile(fsys, relative)
		if readErr != nil {
			return nil, fmt.Errorf("读取迁移文件失败 file=%s: %w", relative, readErr)
		}
		content := strings.ReplaceAll(string(raw), "\r\n", "\n")
		up, down := splitDownSection(content)
		result = append(result, Migration{
			Version:  version,
			Name:     name,
			Up:       up,
			Down:     down,
			Checksum: checksumOf(content),
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("迁移目录 %s 下没有找到合法的 NNNNNN_*.sql 脚本", dir)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	for i := 1; i < len(result); i++ {
		if result[i].Version == result[i-1].Version {
			return nil, fmt.Errorf("迁移版本号重复: %s", result[i].Version)
		}
	}
	return result, nil
}

// EnsureVersionTable 创建默认（冻结 R1 流）迁移版本表（幂等）。
func EnsureVersionTable(ctx context.Context, db *gorm.DB) error {
	return EnsureVersionTableIn(ctx, db, migrationTable)
}

// EnsureVersionTableIn 为指定迁移流创建历史表（幂等）。
func EnsureVersionTableIn(ctx context.Context, db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := db.WithContext(ctx).Exec(createMigrationTableSQLFor(table)).Error; err != nil {
		return fmt.Errorf("创建迁移版本表失败: %w", err)
	}
	return nil
}

// AppliedRecords 返回冻结 R1 流已应用的迁移版本 → 记录。
func AppliedRecords(ctx context.Context, db *gorm.DB) (map[string]AppliedRecord, error) {
	return AppliedRecordsIn(ctx, db, migrationTable)
}

// AppliedRecordsIn 返回指定迁移流已应用的版本 → 记录。
func AppliedRecordsIn(ctx context.Context, db *gorm.DB, table string) (map[string]AppliedRecord, error) {
	if err := EnsureVersionTableIn(ctx, db, table); err != nil {
		return nil, err
	}
	rows := make([]AppliedRecord, 0, 8)
	if err := db.WithContext(ctx).
		Table(table).
		Select("version, name, checksum, applied_at").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取迁移版本表失败: %w", err)
	}
	indexed := make(map[string]AppliedRecord, len(rows))
	for _, row := range rows {
		indexed[row.Version] = row
	}
	return indexed, nil
}

// Up 应用全部未执行的迁移。
//
// 幂等：已应用且校验和一致的迁移直接跳过；已应用但校验和变化则报错，
// 防止历史迁移被静默篡改导致各环境库结构不一致。
func Up(ctx context.Context, db *gorm.DB, fsys fs.FS, dir string) (UpResult, error) {
	return UpIn(ctx, db, fsys, dir, migrationTable)
}

// UpIn 把迁移应用到指定流的历史表，供冻结 R1 之外的扩展流复用同一套
// 幂等、校验和与并发锁语义。
func UpIn(ctx context.Context, db *gorm.DB, fsys fs.FS, dir, table string) (UpResult, error) {
	var result UpResult
	if db == nil {
		return result, fmt.Errorf("database is required")
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(731160)).Error; err != nil {
			return err
		}
		var err error
		result, err = upLocked(ctx, tx, fsys, dir, table)
		return err
	})
	if err != nil {
		return UpResult{}, err
	}
	return result, err
}

func upLocked(ctx context.Context, db *gorm.DB, fsys fs.FS, dir, table string) (UpResult, error) {
	result := UpResult{Applied: []string{}, Skipped: []string{}}

	migrations, err := LoadMigrations(fsys, dir)
	if err != nil {
		return result, err
	}
	applied, err := AppliedRecordsIn(ctx, db, table)
	if err != nil {
		return result, err
	}

	known := make(map[string]Migration, len(migrations))
	for _, m := range migrations {
		known[m.Version] = m
	}
	for version, existing := range applied {
		m, ok := known[version]
		if !ok || existing.Name != m.Name || existing.Checksum != m.Checksum {
			return result, fmt.Errorf("migration stream mismatch at version %s; use a separate R1 database", version)
		}
	}
	for _, m := range migrations {
		if existing, ok := applied[m.Version]; ok {
			if existing.Checksum != m.Checksum {
				return result, fmt.Errorf(
					"迁移 %s 在应用后被修改（库中校验和 %s，当前文件 %s）；请新增迁移脚本而不是改动已应用的历史脚本",
					m.Label(), shortHash(existing.Checksum), shortHash(m.Checksum))
			}
			result.Skipped = append(result.Skipped, m.Label())
			continue
		}
		if err := applyUp(ctx, db, m, table); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, m.Label())
	}
	return result, nil
}

func applyUp(ctx context.Context, db *gorm.DB, m Migration, table string) error {
	body, err := migrationBody(m.Up)
	if err != nil {
		return fmt.Errorf("迁移 %s 事务边界无效: %w", m.Label(), err)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(body) != "" {
			if err := tx.Exec(body).Error; err != nil {
				return fmt.Errorf("执行迁移 %s 失败: %w", m.Label(), err)
			}
		}
		if err := tx.Exec(
			"INSERT INTO "+table+" (version, name, checksum) VALUES (?, ?, ?)",
			m.Version, m.Name, m.Checksum,
		).Error; err != nil {
			return fmt.Errorf("登记迁移 %s 失败: %w", m.Label(), err)
		}
		return nil
	})
}

// Down 按版本号从新到旧回滚最近 steps 条已应用迁移。
//
// 未定义降级段的迁移会被拒绝回滚，而不是产生半吊子状态。
func Down(ctx context.Context, db *gorm.DB, fsys fs.FS, dir string, steps int) ([]string, error) {
	return DownIn(ctx, db, fsys, dir, steps, migrationTable)
}

// DownIn 按版本号从新到旧回滚指定流最近 steps 条已应用迁移。
func DownIn(ctx context.Context, db *gorm.DB, fsys fs.FS, dir string, steps int, table string) ([]string, error) {
	if steps <= 0 {
		return nil, fmt.Errorf("回滚步数必须大于 0")
	}
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	var reverted []string
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(731160)).Error; err != nil {
			return err
		}
		var err error
		reverted, err = downLocked(ctx, tx, fsys, dir, steps, table)
		return err
	})
	if err != nil {
		return nil, err
	}
	return reverted, nil
}

func downLocked(ctx context.Context, db *gorm.DB, fsys fs.FS, dir string, steps int, table string) ([]string, error) {
	reverted := make([]string, 0, steps)
	migrations, err := LoadMigrations(fsys, dir)
	if err != nil {
		return reverted, err
	}
	applied, err := AppliedRecordsIn(ctx, db, table)
	if err != nil {
		return reverted, err
	}
	if len(applied) == 0 {
		return reverted, nil
	}

	byVersion := make(map[string]Migration, len(migrations))
	for _, m := range migrations {
		byVersion[m.Version] = m
	}

	versions := make([]string, 0, len(applied))
	for version := range applied {
		versions = append(versions, version)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))

	for i := 0; i < steps && i < len(versions); i++ {
		version := versions[i]
		m, ok := byVersion[version]
		if !ok {
			return reverted, fmt.Errorf("已应用的迁移 %s 在脚本目录中不存在，无法回滚", version)
		}
		if record := applied[version]; record.Name != m.Name || record.Checksum != m.Checksum {
			return reverted, fmt.Errorf("migration stream mismatch at version %s", version)
		}
		if !m.HasDown() {
			return reverted, fmt.Errorf("迁移 %s 未定义降级段（%s），拒绝回滚", m.Label(), downMarker)
		}
		if err := applyDown(ctx, db, m, table); err != nil {
			return reverted, err
		}
		reverted = append(reverted, m.Label())
	}
	return reverted, nil
}

func applyDown(ctx context.Context, db *gorm.DB, m Migration, table string) error {
	body, err := migrationBody(m.Down)
	if err != nil {
		return fmt.Errorf("迁移 %s 降级事务边界无效: %w", m.Label(), err)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(body).Error; err != nil {
			return fmt.Errorf("执行 %s 降级段失败: %w", m.Label(), err)
		}
		if err := tx.Exec("DELETE FROM "+table+" WHERE version = ?", m.Version).Error; err != nil {
			return fmt.Errorf("删除迁移记录 %s 失败: %w", m.Label(), err)
		}
		return nil
	})
}

// Status 返回冻结 R1 流全部迁移脚本的应用状态。
func Status(ctx context.Context, db *gorm.DB, fsys fs.FS, dir string) ([]StatusRow, error) {
	return StatusIn(ctx, db, fsys, dir, migrationTable)
}

// StatusIn 返回指定流全部迁移脚本的应用状态。
func StatusIn(ctx context.Context, db *gorm.DB, fsys fs.FS, dir, table string) ([]StatusRow, error) {
	migrations, err := LoadMigrations(fsys, dir)
	if err != nil {
		return nil, err
	}
	applied, err := AppliedRecordsIn(ctx, db, table)
	if err != nil {
		return nil, err
	}
	known := make(map[string]Migration, len(migrations))
	for _, m := range migrations {
		known[m.Version] = m
	}
	for version, record := range applied {
		m, ok := known[version]
		if !ok || record.Name != m.Name || record.Checksum != m.Checksum {
			return nil, fmt.Errorf("migration stream mismatch at version %s", version)
		}
	}
	rows := make([]StatusRow, 0, len(migrations))
	for _, m := range migrations {
		row := StatusRow{Version: m.Version, Name: m.Name}
		if record, ok := applied[m.Version]; ok {
			row.Applied = true
			row.AppliedAt = record.AppliedAt
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseMigrationFileName(base string) (version, name string, ok bool) {
	if !strings.HasSuffix(base, ".sql") {
		return "", "", false
	}
	stem := strings.TrimSuffix(base, ".sql")
	index := strings.Index(stem, "_")
	if index <= 0 || index == len(stem)-1 {
		return "", "", false
	}
	version = stem[:index]
	for _, r := range version {
		if r < '0' || r > '9' {
			return "", "", false
		}
	}
	return version, stem[index+1:], true
}

func splitDownSection(content string) (up, down string) {
	index := strings.Index(content, downMarker)
	if index < 0 {
		return content, ""
	}
	return content[:index], content[index+len(downMarker):]
}

func checksumOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func shortHash(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
