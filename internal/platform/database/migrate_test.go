package database

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMigrationWrapperPreservesSQLAndChecksum(t *testing.T) {
	script := "-- historical wrapper\nBEGIN;\nDO $$ BEGIN PERFORM 1; END $$;\nSELECT 'COMMIT;';\nCOMMIT; -- end\n"
	body, err := migrationBody(script)
	if err != nil || !strings.Contains(body, "DO $$ BEGIN PERFORM 1; END $$;") || !strings.Contains(body, "SELECT 'COMMIT;';") {
		t.Fatalf("SQL body changed: %v", err)
	}
	loaded, err := LoadMigrations(fstest.MapFS{"000001_probe.sql": {Data: []byte(script)}}, ".")
	if err != nil || len(loaded) != 1 || loaded[0].Checksum != checksumOf(script) || loaded[0].Up != script {
		t.Fatal("normalizing execution must not change migration history")
	}
	for _, script := range []string{"BEGIN;\nSELECT 1;", "SELECT 1;\nCOMMIT;", "BEGIN; SELECT 1; COMMIT;"} {
		if _, err := migrationBody(script); err == nil {
			t.Fatal("ambiguous transaction wrapper accepted")
		}
	}
}

// Only explicit disposable local databases are accepted. No production/staging
// DSN is discovered implicitly from application configuration.
func migrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("INKLORE_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set INKLORE_MIGRATION_TEST_DSN to an isolated local PostgreSQL")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil || (cfg.Host != "127.0.0.1" && cfg.Host != "localhost" && cfg.Host != "::1") || (cfg.Database != "inklore_ci" && cfg.Database != "inklore_e2e") {
		t.Fatal("migration tests require a disposable local inklore_ci/inklore_e2e database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	base, err := Open(ctx, Options{DSN: dsn})
	if err != nil {
		t.Fatal("cannot connect to isolated migration database")
	}
	schema := "migration_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	connection := stdlib.OpenDB(*cfg)
	connection.SetMaxOpenConns(4)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("cannot open isolated schema")
	}
	t.Cleanup(func() {
		_ = connection.Close()
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		pool, _ := base.DB()
		_ = pool.Close()
	})
	return db
}

func TestMigrationPostgresAtomicFailure(t *testing.T) {
	db := migrationTestDB(t)
	scripts := fstest.MapFS{
		"000001_first.sql": {Data: []byte("BEGIN;\nCREATE TABLE probe (id int);\nINSERT INTO probe VALUES (1);\nCOMMIT;\n")},
		"000002_fail.sql":  {Data: []byte("BEGIN;\nINSERT INTO probe VALUES (2);\nSELECT 1/0;\nCOMMIT;\n")},
	}
	result, err := Up(context.Background(), db, scripts, ".")
	if err == nil || len(result.Applied) != 0 {
		t.Fatal("failed stream reported committed migrations")
	}
	var absent bool
	if err := db.Raw("SELECT to_regclass('probe') IS NULL AND to_regclass('schema_migrations') IS NULL").Scan(&absent).Error; err != nil || !absent {
		t.Fatal("DDL, inserted rows or history escaped rollback")
	}
}

func TestMigrationPostgresHistoryFailure(t *testing.T) {
	db := migrationTestDB(t)
	if err := EnsureVersionTable(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE schema_migrations ADD CHECK (version <> '000001')").Error; err != nil {
		t.Fatal(err)
	}
	scripts := fstest.MapFS{"000001_probe.sql": {Data: []byte("BEGIN;\nCREATE TABLE probe (id int);\nCOMMIT;\n")}}
	if _, err := Up(context.Background(), db, scripts, "."); err == nil {
		t.Fatal("history failure accepted")
	}
	var absent bool
	if err := db.Raw("SELECT to_regclass('probe') IS NULL").Scan(&absent).Error; err != nil || !absent {
		t.Fatal("DDL committed without its history row")
	}
}

func TestMigrationPostgresConcurrencyAndReplay(t *testing.T) {
	db := migrationTestDB(t)
	scripts := fstest.MapFS{
		"000001_probe.sql": {Data: []byte("BEGIN;\nCREATE TABLE probe (id int PRIMARY KEY);\nINSERT INTO probe VALUES (1);\nCOMMIT;\n")},
		"000002_next.sql":  {Data: []byte("SELECT pg_sleep(0.1);\nINSERT INTO probe VALUES (2);\n")},
	}
	var results [2]UpResult
	var errors [2]error
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		workers.Go(func() {
			<-start
			results[i], errors[i] = Up(context.Background(), db, scripts, ".")
		})
	}
	close(start)
	workers.Wait()
	if errors[0] != nil || errors[1] != nil || len(results[0].Applied)+len(results[1].Applied) != 2 || len(results[0].Skipped)+len(results[1].Skipped) != 2 {
		t.Fatalf("concurrent migrations did not serialize: %v", errors)
	}
	var rows int64
	if err := db.Table("probe").Count(&rows).Error; err != nil || rows != 2 {
		t.Fatal("concurrent execution duplicated or lost writes")
	}
	replay, err := Up(context.Background(), db, scripts, ".")
	if err != nil || len(replay.Applied) != 0 || len(replay.Skipped) != 2 {
		t.Fatal("migration replay changed persisted history")
	}
	scripts["000001_probe.sql"] = &fstest.MapFile{Data: append([]byte("-- changed\n"), scripts["000001_probe.sql"].Data...)}
	if _, err := Up(context.Background(), db, scripts, "."); err == nil {
		t.Fatal("changed historical checksum accepted")
	}
}

func TestMigrationPostgresDownRollback(t *testing.T) {
	db := migrationTestDB(t)
	scripts := fstest.MapFS{
		"000001_first.sql": {Data: []byte("BEGIN;\nCREATE TABLE probe (id int);\nINSERT INTO probe VALUES (1);\nCOMMIT;\n-- +migrate Down\nBEGIN;\nDROP TABLE probe;\nSELECT 1/0;\nCOMMIT;\n")},
		"000002_next.sql":  {Data: []byte("INSERT INTO probe VALUES (2);\n-- +migrate Down\nBEGIN;\nDELETE FROM probe WHERE id=2;\nCOMMIT;\n")},
	}
	if _, err := Up(context.Background(), db, scripts, "."); err != nil {
		t.Fatal(err)
	}
	reverted, err := Down(context.Background(), db, scripts, ".", 2)
	if err == nil || len(reverted) != 0 {
		t.Fatal("failed down stream reported reverted migrations")
	}
	for _, table := range []string{"probe", "schema_migrations"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 2 {
			t.Fatal("partial down changes escaped rollback")
		}
	}
	scripts["000002_next.sql"] = &fstest.MapFile{Data: append([]byte("-- changed\n"), scripts["000002_next.sql"].Data...)}
	if _, err := Down(context.Background(), db, scripts, ".", 1); err == nil {
		t.Fatal("down accepted modified migration history")
	}
}

func TestMigrationPostgresDownSuccess(t *testing.T) {
	db := migrationTestDB(t)
	scripts := fstest.MapFS{"000001_probe.sql": {Data: []byte("BEGIN;\nCREATE TABLE probe (id int);\nCOMMIT;\n-- +migrate Down\nBEGIN;\nDROP TABLE probe;\nCOMMIT;\n")}}
	if _, err := Up(context.Background(), db, scripts, "."); err != nil {
		t.Fatal(err)
	}
	reverted, err := Down(context.Background(), db, scripts, ".", 1)
	if err != nil || len(reverted) != 1 {
		t.Fatal("valid wrapped migration did not roll back", err)
	}
	var absent bool
	if err := db.Raw("SELECT to_regclass('probe') IS NULL AND NOT EXISTS (SELECT 1 FROM schema_migrations)").Scan(&absent).Error; err != nil || !absent {
		t.Fatal("DDL and history did not roll back together")
	}
	if replay, err := Up(context.Background(), db, scripts, "."); err != nil || len(replay.Applied) != 1 {
		t.Fatal("rolled back migration cannot be applied again")
	}
}
