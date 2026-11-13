# Agent Note: 收口 PostgreSQL 18 运行基线

Status: implemented

Task: https://github.com/BaSui01/InkLore/pull/147
Scope: `docker-compose.prod.yml`、`docker-compose.dev.yml`、`deploy/render/README.md`、`docs/decisions/0061-postgresql-18-baseline.md`

## Problem

用户要求把 PostgreSQL 更新到最新稳定版（核实：18.6 为最新稳定，19 仍为 RC）。此前只完成了本地开发编排与 4 个 CI 工作流的升级，遗留三类问题：

1. **部署模板与验证版本分叉**：`docker-compose.prod.yml` 仍是 `pgvector/pgvector:pg17`，而 CI 已按 18 跑真实迁移验收——模板上线的那一天，数据库主版本与验证版本不一致。
2. **与已接受 ADR 冲突但未记录**：ADR-0022（Accepted）明文裁决「新部署 Compose 使用 pgvector/pgvector:pg17」。既不能改写已接受 ADR，也不能让替代关系无声发生。
3. **两处陈述已失真**：`docker-compose.dev.yml` 头注释称「旧 PG17 卷保留在磁盘」（实测 `docker volume ls` 只剩 `inklore_ai-novel-postgres18` 与 `inklore_ai-novel-redis`）；`deploy/render/README.md` 称 CI 套件为 `PG17/Redis7`（工作流实际已是 18）。

## Decision

- **基线**：运行基线更新为 PostgreSQL 18（本机 `select version()` = `PostgreSQL 18.6 (Debian 18.6-1.pgdg12+2)`）；本地开发、CI 一次性实例、部署模板统一 `pgvector/pgvector:pg18`。
- **模板**：`docker-compose.prod.yml` 镜像改 `pg18`，挂载点由 `/var/lib/postgresql/data` 改为 `/var/lib/postgresql`（PG18+ 镜像 `PGDATA=/var/lib/postgresql/18/docker`，沿用旧挂载点会被镜像判为未使用挂载并拒绝启动），头注释改为「既有 PG17 卷须先 `pg_dump` 再恢复到新卷」。
- **卷命名区分环境**：dev `ai-novel-postgres18` / `ai-novel-redis`；prod `ai-novel-postgres18-prod` / `ai-novel-redis-prod`。两文件同处一个检出目录、Compose 项目名同为 `inklore`，同名卷会让两个编排共用同一 Docker 卷（本地执行生产编排会挂到开发库，限流计数与并发租约等协调状态也会混用），生产必须用不同卷名。
- **不改迁移身份**：已应用迁移 SQL 字节、`schema_migrations` 17 条与 `schema_migrations_extensions` 7 条的 version/name/checksum 全部不变；主版本升级只影响服务端与数据目录布局。
- **记录替代关系**：新增 ADR-0061 记录新的运行基线并声明「仅替代 ADR-0022 的主版本 + Compose 镜像裁决」，ADR-0022 原文保留。
- **文档更正**：dev 编排头注释改为「旧 PG17 卷已随清理删除」；`deploy/render/README.md` 的 CI 套件描述改为 `PG18/Redis7`。

## Alternatives considered

- **保持 PostgreSQL 17**：不满足用户指令，且 CI 已按 18 验证，模板会长期与验证版本分叉。未选。
- **只换 prod 镜像、沿用旧卷名 `ai-novel-postgres`**：旧 PG17 卷会被 PG18 挂载，PG18 在卷内另建 `18/docker` 集群，表现为「启动成功、实为空库」的静默后果。未选。
- **prod 复用 dev 卷名 `ai-novel-postgres18`**：同一检出目录下两编排共用同一卷，本地跑生产编排会挂到开发库。未选（本 PR 的交付实现即为规避此点）。
- **给 prod 设独立 Compose 项目名**：会连带改容器名、网络名与 Redis 卷名，对既有部署等价于换库换名。未选。
- **改写 ADR-0022 的 PG17 裁决**：违背「已接受 ADR 保留原样、长期变化记录替代范围」。未选。
- **force push 重写远程历史**：毫无必要，正常 PR 演进即可。未选。

## Consequences

- 三处数据库主版本一致，CI 的真实迁移验收结论可直接适用于部署模板。
- 任何仍在 PG17 的环境必须一次性 dump/restore 到新卷；旧卷在核对业务数据前不删除，因此升级失败可回退（旧卷 + 旧镜像）。
- 生产编排的 Redis 卷改为 `ai-novel-redis-prod` 后，dev 与 prod 不再共用任何数据卷。代价：既有 prod Redis 卷不再被读取，切换后限流窗口与并发租约等协调状态重置（该卷只承载缓存与协调状态），旧卷保留在磁盘可回退。
- 容器名与网络仍由 Compose 项目名决定（`inklore-postgres-1`、`inklore_default` 等），同目录同时运行两个编排仍会撞容器名与端口。彻底隔离需为 prod 设独立项目名（会让既有部署重新指向新卷新容器），ADR-0061 已将该项列为未选并记录该暴露面。
- 生产侧没有可用的部署实例与凭据，模板修订无法在真实生产验证，按仓库规则标注 `NOT_VERIFIED`。
- 已应用迁移与校验和未变，所有既有库的 `Up/Verify` 行为不受影响。

## Verification

- `docker compose -f docker-compose.dev.yml config --quiet` 退出 0；`docker compose -f docker-compose.prod.yml config --quiet`（注入占位 `POSTGRES_PASSWORD`、32 字节 `ENCRYPTION_KEY`）退出 0；两个编排声明的数据卷不再重名（dev `ai-novel-postgres18` / `ai-novel-redis`，prod `ai-novel-postgres18-prod` / `ai-novel-redis-prod`）。
- `node scripts/agent-notes.mjs check --base origin/dev`：115 notes，`base archive seals unchanged: 0`，结构与链接一致。
- `node scripts/check-project-skills.mjs`（12 文件 / 51 链接）、`node docs/system-design/tools/check-design.cjs`（77 tables，issues/warnings 空）、`node scripts/check-platform-contracts.mjs`（102 映射）全部通过。
- 运行证据：`docker ps` 中 `inklore-postgres-1` 使用 `pgvector/pgvector:pg18`（healthy）；`psql -Atc 'select version();'` 输出上述 18.6 字符串；库 `ai_novel` 为 `schema_migrations` 17 条 + `schema_migrations_extensions` 7 条、public 85 表；`29080/29000/29001` 的 `/healthz` 全部 200。
- 未验证：生产环境真实升级（无实例/凭据）、PG17 卷的实际 dump/restore 演练、既有 prod Redis 卷切到新卷后的状态重置影响（无实例可演练）。
