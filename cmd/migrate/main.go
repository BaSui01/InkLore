// Command migrate 执行 InkLore 的数据库迁移。
//
// 用法：
//
//	go run ./cmd/migrate -cmd up
//	go run ./cmd/migrate -cmd status
//	go run ./cmd/migrate -cmd down -n 1
//	DATABASE_DSN is injected through the process environment; target an isolated database.
//
// 迁移脚本通过 embed 打包进二进制，无需依赖运行目录下的 SQL 文件。
//
// 退出码：0 成功；1 执行失败；2 参数或配置错误。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"InkLore/internal/platform/config"
	"InkLore/internal/platform/database"
	"InkLore/migrations"
	"InkLore/migrations/extensions"
)

func main() {
	configPath := flag.String("config", config.DefaultPath, "配置文件路径")
	dsnOverride := flag.String("dsn", "", "覆盖 PostgreSQL DSN（默认读取配置或 DATABASE_DSN 环境变量）")
	command := flag.String("cmd", "up", "执行动作：up | down | status")
	steps := flag.Int("n", 1, "down 回滚的迁移条数")
	timeout := flag.Duration("timeout", 60*time.Second, "整体执行超时")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg, err := config.Bootstrap(*configPath)
	if err != nil {
		fail(2, "装载配置失败: %v", err)
	}

	options := database.OptionsFromConfig(cfg)
	if *dsnOverride != "" {
		options.DSN = *dsnOverride
	}
	if !options.Enabled() {
		fail(2, "未配置数据库连接串：请设置环境变量 DATABASE_DSN，或使用 -dsn 参数")
	}

	db, err := database.Open(ctx, options)
	if err != nil {
		fail(1, "PostgreSQL unavailable; check connection configuration")
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "migrate: 关闭数据库连接失败: %v\n", closeErr)
		}
	}()

	switch *command {
	case "up":
		result, upErr := database.Up(ctx, db, migrations.FS(), migrations.Dir)
		if upErr != nil {
			fail(1, "migration refused; verify database identity, migration checksums and PostgreSQL prerequisites")
			return
		}
		for _, label := range result.Skipped {
			fmt.Printf("  跳过   %s（此前已应用）\n", label)
		}
		for _, label := range result.Applied {
			fmt.Printf("  已应用 %s\n", label)
		}
		fmt.Printf("up 完成（主流）：新应用 %d 条，跳过 %d 条\n", len(result.Applied), len(result.Skipped))

		postResult, postErr := database.UpExtensions(ctx, db, extensions.FS(), extensions.Dir)
		if postErr != nil {
			if errors.Is(postErr, database.ErrLegacyExtensionHistory) {
				fail(1, "%v", postErr)
				return
			}
			fail(1, "extension migration refused; verify database identity, migration checksums and PostgreSQL prerequisites")
			return
		}
		for _, label := range postResult.Skipped {
			fmt.Printf("  跳过   %s（此前已应用）\n", label)
		}
		for _, label := range postResult.Applied {
			fmt.Printf("  已应用 %s\n", label)
		}
		fmt.Printf("up 完成（扩展流）：新应用 %d 条，跳过 %d 条\n", len(postResult.Applied), len(postResult.Skipped))

	case "down":
		reverted, downErr := database.Down(ctx, db, migrations.FS(), migrations.Dir, *steps)
		if downErr != nil {
			fail(1, "回滚失败: %v", downErr)
		}
		for _, label := range reverted {
			fmt.Printf("  已回滚 %s\n", label)
		}
		fmt.Printf("down 完成：回滚 %d 条\n", len(reverted))

	case "status":
		rows, statusErr := database.Status(ctx, db, migrations.FS(), migrations.Dir)
		if statusErr != nil {
			fail(1, "读取迁移状态失败: %v", statusErr)
		}
		fmt.Println("[main]")
		for _, row := range rows {
			state := "未应用"
			if row.Applied {
				state = "已应用 " + row.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("  %s_%s  %s\n", row.Version, row.Name, state)
		}
		postRows, postStatusErr := database.StatusExtensions(ctx, db, extensions.FS(), extensions.Dir)
		if postStatusErr != nil {
			fail(1, "读取扩展迁移状态失败: %v", postStatusErr)
		}
		fmt.Println("[extensions]")
		for _, row := range postRows {
			state := "未应用"
			if row.Applied {
				state = "已应用 " + row.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("  %s_%s  %s\n", row.Version, row.Name, state)
		}

	default:
		fail(2, "未知动作 %q（支持 up | down | status）", *command)
	}
}

func fail(code int, format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(code)
}
