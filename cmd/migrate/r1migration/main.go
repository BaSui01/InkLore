package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	legacyDSN := flag.String("legacy-dsn", "", "旧库 PostgreSQL DSN 连接串（可选）")
	r1DSN := flag.String("r1-dsn", "", "目标 R1 PostgreSQL DSN 连接串（可选）")
	dryRun := flag.Bool("dry-run", true, "只读演练模式，不落盘写入物理数据库（默认开启）")
	useSample := flag.Bool("sample", true, "使用内置多租户与全量小说实体旧库演练样本数据")
	verifyOnly := flag.Bool("verify-only", false, "仅执行既有 R1 数据账本与外键一致性校验")
	reportPath := flag.String("report", "docs/system-design/implementation/r1-migration-report.json", "输出机器检查报告路径")
	checkBundle := flag.Bool("check-bundle", true, "执行前端双端 (:3000 Creator / :3001 Admin) 资源指纹核对")
	checkDeadCode := flag.Bool("check-deadcode", true, "执行旧无用代码清理清单与无反向依赖合规检查")
	repoRoot := flag.String("root", ".", "代码仓库根目录路径")
	timeout := flag.Duration("timeout", 120*time.Second, "全流程超时限制")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	opts := PipelineOptions{
		LegacyDSN:     *legacyDSN,
		R1DSN:         *r1DSN,
		DryRun:        *dryRun,
		UseSample:     *useSample,
		VerifyOnly:    *verifyOnly,
		ReportPath:    *reportPath,
		CheckBundle:   *checkBundle,
		CheckDeadCode: *checkDeadCode,
		RepoRoot:      *repoRoot,
	}

	runner := NewPipelineRunner(opts)
	report, err := runner.Execute(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "migration runner execution failed: %v\n", err)
		os.Exit(1)
	}

	if len(report.Errors) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "migration verification failed with %d error(s)\n", len(report.Errors))
		os.Exit(1)
	}

	os.Exit(0)
}
