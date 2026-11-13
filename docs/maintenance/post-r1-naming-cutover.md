# ADR-0060 后续运维：R1 命名收敛的在线切换指引

本文承接 [ADR-0060](../decisions/0060-retire-stage-naming.md)，给出两处**必须由维护者在目标环境执行**的切换步骤。二者都不在代码侧自动完成，且都以「不中断、可回滚」为前提。

## 1. 扩展流历史表改名（必需）

代码自 ADR-0060 起不再读取旧表名 `schema_migrations_post_r1`；任何**既有数据库**（本地、Supabase staging、CI 一次性库外的持久环境）在部署新版本前必须完成一次幂等改名：

```sql
-- 幂等：表不存在或已改名时无副作用
DO $$
BEGIN
  IF to_regclass('public.schema_migrations_post_r1') IS NOT NULL
     AND to_regclass('public.schema_migrations_extensions') IS NULL THEN
    ALTER TABLE public.schema_migrations_post_r1 RENAME TO schema_migrations_extensions;
    RAISE NOTICE 'renamed schema_migrations_post_r1 -> schema_migrations_extensions';
  ELSE
    RAISE NOTICE 'no rename needed';
  END IF;
END
$$;
```

先核对再执行：

```sql
SELECT to_regclass('public.schema_migrations_post_r1')     AS legacy_table,
       to_regclass('public.schema_migrations_extensions')  AS current_table;
SELECT count(*) FROM schema_migrations;                 -- 主流：应为 17
SELECT count(*) FROM schema_migrations_post_r1;          -- 扩展流：应为 7（改名后表名同步替换）
```

- **未执行会怎样**：服务启动时扩展流 `Up` 会在**新表**上重新应用 7 条迁移；若那些 DDL 非幂等，会 fail-closed 拒绝启动（错误为 `extension migration refused`）。改名可完全避免该路径。
- **回滚**：`ALTER TABLE schema_migrations_extensions RENAME TO schema_migrations_post_r1;`（仅在回退到旧版本时需要）。
- 主流 17 条**不受影响**：其 version/name/checksum 与文件逐条相同，无需任何数据库操作。

## 2. 准入/限流 Redis 键前缀改名（可选，需 drain-and-cutover）

现状：`inklore:r1:llm:{tenant}:` 与 `inklore:r1:login:` 前缀**有意保留**（见 ADR-0060 保留项）。原因：这些键是**跨版本协调状态**（RPM 计数、并发租约、尝试标记、熔断状态），滚动升级期间新旧进程必须共享同一命名空间，否则各自计数会突破供应商限流。

若要改名（去掉 `r1` 段），必须满足「新旧进程不共存」或「一次切换」条件，推荐按下列任一方式执行：

### 方式 A：先排空再切换（推荐，最简单）

1. 部署窗口内先把服务副本缩容到 **0**（Render 可暂停实例；自建环境停止进程）；
2. 确认无存活旧进程：`redis-cli --scan --pattern 'inklore:r1:*' | head`，并在监控上确认无新写入；
3. 发布新版本（新前缀）；
4. 旧键会在 TTL 到期后自然消失，也可显式清理：`redis-cli --scan --pattern 'inklore:r1:llm:*' | xargs -r redis-cli DEL`（登录限流键 TTL 更短，通常无需清理）。

### 方式 B：过渡期双读（需要代码改动，成本更高）

新代码读取时「先新键、未命中则读旧键」，写入统一用新键；观察一个完整 TTL 周期（RPM 窗口 + 熔断冷却）后再移除旧键读取分支。此方式允许滚动升级，但需要改动准入与限流两处键访问逻辑，并补一轮灰度验证。

> 无论采用哪种方式，都必须**同时**改 `gatewayPrefix`（`internal/llm/infrastructure/gateway_admission.go`）与登录限流键（`internal/platform/identity/limiter.go`），并确保二者在同一个发布单元内切换。

## 3. 执行检查清单

| 步骤 | 命令/判据 | 期望 |
|---|---|---|
| 1 | `go run ./cmd/migrate -cmd status` | `[main]` 17 条、`[extensions]` 7 条均为「已应用」 |
| 2 | 第 1 节 SQL 的先核对查询 | 旧表存在、新表不存在 |
| 3 | 执行第 1 节幂等改名 | `renamed ...` |
| 4 | 再跑 `migrate -cmd status` | 全部「已应用」，零新应用 |
| 5 | `curl -i /healthz` | 200 |
| 6 | 只在采用第 2 节时执行 | 按方式 A 或 B 的顺序操作 |

## 4. 为什么这两件事不放进代码

- 表改名是**数据库侧状态迁移**：代码无法在启动前安全地自证「旧表存在且新表不存在」并自动改名（多实例并发启动会互相竞争），因此交由维护者在维护窗口执行一次。
- Redis 前缀改名是**滚动升级协调问题**，不是命名问题；在代码里静默改名会直接放大超限风险（这正是 PR #140 审查中被指出的 P1 项）。
