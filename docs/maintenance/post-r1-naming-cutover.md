# ADR-0060 后续运维：R1 命名收敛的在线切换指引

本文承接 [ADR-0060](../decisions/0060-retire-stage-naming.md)，给出两处**必须由维护者在目标环境执行**的切换步骤。二者都不在代码侧自动完成，且都以「不中断、可回滚」为前提。

## 0. 先确定目标 schema

迁移连接实际使用的 schema 决定了下面所有 SQL 的限定名：

| 环境 | 历史表所在 schema | 依据 |
|---|---|---|
| 本地开发 / 一次性 CI 库 | `public` | `docker-compose.dev.yml` 与 CI 默认库 |
| Supabase staging | `inklore_ci` | [migrations/extensions/README.md](../../migrations/extensions/README.md) 要求执行身份把 `search_path` 设为 `inklore_ci,extensions,public`；`.github/workflows/isolated-integration.yml` 以该配置跑迁移 |

**不要凭直觉写 `public.`**：在 staging 上那样写会漏掉真正的旧表，并误报「无需改名」。

## 1. 扩展流历史表改名（既有库必需）

> ⚠️ **必须先改名，再跑任何 `migrate` 子命令。**
> `cmd/migrate -cmd status` 会经 `StatusIn → AppliedRecordsIn → EnsureVersionTableIn` 对目标表执行
> `CREATE TABLE IF NOT EXISTS`。若先跑 status，新表会被**提前创建**，下面的守卫条件随之失效，
> 改名被跳过；随后的部署会对新表重放扩展流迁移，遇到非幂等的 DDL（如
> `000001_platform_mail_settings` 的 `CREATE TABLE`）即 fail-closed 拒绝启动。

### 步骤 1：直接检查旧表（不要跑 status）

```sql
-- 把 :schema 替换为第 0 节确定的 schema（public 或 inklore_ci）
SELECT to_regclass(':schema.schema_migrations_post_r1')    AS legacy_table,
       to_regclass(':schema.schema_migrations_extensions') AS current_table,
       (SELECT count(*) FROM :schema.schema_migrations_post_r1) AS legacy_rows;   -- 期望 7
```

判据：`legacy_table` 非空、`current_table` 为空、`legacy_rows` 为 7。

### 步骤 2：用同一 schema 执行幂等改名

```sql
-- :schema 同上；重复执行无副作用
DO $$
BEGIN
  IF to_regclass(':schema.schema_migrations_post_r1') IS NOT NULL
     AND to_regclass(':schema.schema_migrations_extensions') IS NULL THEN
    ALTER TABLE :schema.schema_migrations_post_r1 RENAME TO schema_migrations_extensions;
    RAISE NOTICE 'renamed schema_migrations_post_r1 -> schema_migrations_extensions';
  ELSE
    RAISE NOTICE 'no rename needed';
  END IF;
END
$$;
```

### 步骤 3：改名完成后才跑 status 复验

```bash
go run ./cmd/migrate -cmd status
```

期望：`[main]` 17 条与 `[extensions]` 7 条全部「已应用」，且**零新应用**。

### 回滚

仅在回退到旧版本时需要：

```sql
ALTER TABLE :schema.schema_migrations_extensions RENAME TO schema_migrations_post_r1;
```

> 主流 17 条**不受影响**：其 version/name/checksum 与文件逐条相同，无需任何数据库操作。

## 2. 准入/限流 Redis 键前缀改名（默认不改）

现状是有意保留 `inklore:r1:llm:{tenant}:` 与 `inklore:r1:login:`（见 ADR-0060 保留项）。原因不只是「命名」：

- `gatewayAcquireLua`（`internal/llm/infrastructure/gateway_admission.go`）在**单个命名空间内原子地**检查并更新 RPM 计数、并发租约与尝试标记；
- `loginWindow`（`internal/platform/identity/limiter.go`）同样维护**单一**尝试集合。

因此**跨代拆分成两个命名空间必然各自计数**：「读新键未命中再读旧键」这类回退读**无法收敛**——新实例写入后不再看旧键，旧实例也永远看不到新写入，双方都会放行，限额实际被放大。共存期间必须共享同一权威命名空间。

### 可选做法（按推荐度）

**A. 保持现状（推荐，也是 ADR-0060 的选择）**

不改前缀 → 无需任何操作，跨版本兼容性最佳。

**B. 一次性切换（需要真正的 drain + 等待协调窗口）**

1. 把服务副本缩容到 **0** 并确认旧进程全部退出；
2. **等待 ≥ 最长协调窗口**（不是「副本为 0」就够）：
   - 网关 RPM 窗口 **60 秒**；
   - 限流/熔断期限最长 **300 秒**；
   - 登录尝试窗口 **60 秒**；
   → 建议等待 **300 秒 + 余量（例如 5–10 分钟）**；
   - 原因：`gatewayRecoveryLua` 在新命名空间没有 PostgreSQL inflight 预约时会**立即**把新 guard 判定为就绪，**不会**继承旧命名空间的 RPM 与冷却状态，所以状态交接必须靠「窗口耗尽」而不是靠进程不共存；
3. 确认旧键不再增长（例如 `redis-cli --scan --pattern 'inklore:r1:*'` 连续观察计数稳定）；
4. 再发布新前缀版本——必须在**同一发布单元**内同时改 `gatewayPrefix` 与登录限流键；
5. 旧键随 TTL 自然消失；如需立即清理：`redis-cli --scan --pattern 'inklore:r1:llm:*' | xargs -r redis-cli DEL`。

**C. 允许跨代并存（不建议）**

若业务要求滚动升级期间新旧进程长期共存，则必须设计**原子过渡协议**：两代共享同一权威键（例如都读写同一个不含版本段的键），或实现「读旧+读新取更严格者、写双方」的双向收敛逻辑，并单独验证。当前代码未实现该协议，不要按回退读的方式上线。

## 3. 执行检查清单

| # | 动作 | 判据 |
|---|---|---|
| 1 | 确定目标 schema（第 0 节） | 与迁移连接一致 |
| 2 | 直接查旧表与新表（**不要跑 status**） | 旧表在、新表不在、`legacy_rows = 7` |
| 3 | 执行幂等改名 SQL（同 schema） | `renamed ...` |
| 4 | `go run ./cmd/migrate -cmd status` | `[main]` 17 条、`[extensions]` 7 条均「已应用」，零新应用 |
| 5 | `curl -i /healthz` | 200 |
| 6 | 仅当采用第 2 节方式 B | 缩容 → **等待 ≥300 秒** → 确认旧键不增长 → 发布新前缀 |

## 4. 为什么这两件事不放进代码

- **表改名**：代码无法在启动前安全地自证「旧表存在且新表不存在」并自动改名——多实例并发启动会互相竞争，且 `status`/`up` 自身会创建新表（见第 1 节的顺序要求）。因此交由维护者在维护窗口执行一次。
- **前缀改名**：这是滚动升级的**状态交接**问题而非命名问题。在代码里静默改名会直接放大超限风险（PR #140 审查中指出的 P1 项）；可行路径只有「保持共享命名空间」或「采用真正的 drain + 窗口等待」。
