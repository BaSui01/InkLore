# ADR-0061：PostgreSQL 18 运行基线

- 状态：Accepted
- 日期：2026-11-13
- 关联任务：无既有 INK 编号；依据用户指令「更新 pgsql 最新版本」（2026-11-13）
- 决策者：BaSui01
- 替代范围：仅替代 ADR-0022 的「PostgreSQL 主版本 + 新部署 Compose 镜像」裁决。PostgreSQL 单一数据权威、Redis 职责、迁移校验和保护、领域边界全部原样保留；ADR-0022 不重写（Accepted 表示当时的决策）。

## 背景

ADR-0022（2026-09-20）依据当时系统实例 PostgreSQL 17.11，把运行基线定为 17，并规定「新部署 Compose 使用 pgvector/pgvector:pg17」。

2026-11-13 用户要求把 PostgreSQL 更新到最新稳定版。核实结果：PostgreSQL 最新**稳定**版本为 18.6（19 仍为 RC，未 GA），pgvector 官方镜像 `pg18` 可用（本机实际拉取 `pgvector/pgvector:0.8.7-pg18`）。本地开发编排与 4 个 CI 工作流已切到 18，但部署模板 `docker-compose.prod.yml` 仍停留 `pg17`，形成「CI 已按 18 验证、部署模板还是 17」的版本分叉，也让模板失去 CI 迁移验收结论的适用性。

## 决策

1. **运行基线更新为 PostgreSQL 18**（本机验收精确版本 18.6，`select version()` = `PostgreSQL 18.6 (Debian 18.6-1.pgdg12+2) on x86_64-pc-linux-gnu`）。本地开发、CI 一次性实例、部署模板统一使用 `pgvector/pgvector:pg18`。
2. **迁移策略沿用「禁止原卷直接更换主版本启动」**：跨主版本必须先 `pg_dump` 逻辑备份，再恢复到新数据卷。PG18+ 镜像的 `PGDATA` 是 `/var/lib/postgresql/18/docker`，因此宿主挂载点必须是 `/var/lib/postgresql`（沿用 `/var/lib/postgresql/data` 会被镜像判为未使用挂载并拒绝启动）。
3. **卷命名区分环境**：本地开发 `ai-novel-postgres18`；生产编排 `ai-novel-postgres18-prod`。两个 compose 文件位于同一检出目录、Compose 项目名同为 `inklore`，同名卷会让两个编排共用同一个 Docker 卷（Redis 卷 `ai-novel-redis` 已是此情形），生产必须区分，避免本地把生产库指向开发库。
4. **不改变迁移身份**：已应用迁移 SQL 的字节、`schema_migrations` 17 条与 `schema_migrations_extensions` 7 条的 version/name/checksum 全部不变；主版本升级只影响服务端二进制与数据目录布局，不触及迁移校验和保护。
5. **仅替换 ADR-0022 的主版本裁决**：其余内容（PG 单一权威、Redis 仅缓存与协调、扩展流独立历史表）继续有效。

## 备选

- **保持 PostgreSQL 17**：不满足用户指令，且 CI 已按 18 验证，部署模板会长期与验证版本分叉。未选。
- **prod 只换镜像、沿用旧卷名**：旧 PG17 卷会被 PG18 挂载，PG18 在卷内新建 `18/docker` 集群而旧数据仍在 `data/`，表现为「启动成功、实为空库」的静默后果。未选。
- **prod 复用 dev 的卷名 `ai-novel-postgres18`**：同一检出目录下两个编排共用同一卷，本地执行生产编排会挂到开发库。未选。
- **为 prod 设独立 Compose 项目名**：会连带改容器名、网络名与 Redis 卷名，对既有部署等价于换库换名，风险大于收益。未选。
- **关闭分支保护强推重写远程历史**：不必要，本变更按正常 PR 流程演进。未选。

## 影响

- **收益**：本地开发、CI、部署模板三处数据库主版本一致；CI 的真实迁移验收结论可直接适用于部署模板。
- **成本**：任何仍在 PG17 的环境必须在维护窗口执行 `pg_dump` → 恢复到新卷 → 切换；旧卷在切换完成并核对业务数据前不删除。
- **兼容性**：应用侧无协议变化（DSN 形状、pgvector 维度、扩展集合不变）；已应用迁移与校验和不变，本地实测迁移零新应用。
- **失败恢复**：升级异常时回退到旧卷 + 旧镜像（PG17）即可恢复；新卷在新集群初始化后未承载业务数据，可直接丢弃重建。
- **未验证边界**：`docker-compose.prod.yml` 没有真实生产部署实例可用，其变更属模板一致性修订，生产侧标注 `NOT_VERIFIED`。

## 验证与实施边界

本机实测（2026-11-13，Windows + Docker Desktop）：

- `docker ps`：`inklore-postgres-1` = `pgvector/pgvector:pg18`（healthy）、`inklore-redis-1` = `redis:7-alpine`（healthy）；`docker volume ls` 仅 `inklore_ai-novel-postgres18` 与 `inklore_ai-novel-redis`（旧 PG17 卷已不存在）。
- `psql -Atc 'select version();'` → `PostgreSQL 18.6 (Debian 18.6-1.pgdg12+2) on x86_64-pc-linux-gnu, compiled by gcc (Debian 12.2.0-14+deb12u1) 12.2.0, 64-bit`；库 `ai_novel`：`schema_migrations` 17 条、`schema_migrations_extensions` 7 条、public 85 张表。
- 三端健康检查：`29080/healthz`、`29000/healthz`、`29001/healthz` 全部 200。
- GitHub Actions：PR head 上「PostgreSQL 18 + Redis 7 真实迁移验收」「后端（构建/vet/格式/架构红线）」「前端（类型检查/构建）」「项目 Skills 与工程记忆校验」全部 SUCCESS。

实施边界：生产环境（或任何仍有 PG17 数据的环境）的实际升级必须在维护窗口由维护者执行 dump/restore；本轮不接触任何生产库，也不改动已应用迁移。

## 后续与关联

- 替代对象：[ADR-0022：PostgreSQL 17 运行基线](../../.agents/notes/reference/decisions/0022-postgresql-17-baseline.md)。
- 编排：[docker-compose.dev.yml](../../docker-compose.dev.yml)、[docker-compose.prod.yml](../../docker-compose.prod.yml)。
- 本地环境说明：[docs/local-environment.md](../local-environment.md)。
- 工程笔记：[2026-11-13-postgresql-18-baseline.md](../../.agents/notes/implemented/architecture/2026-11-13-postgresql-18-baseline.md)。
- 交付：[PR #147](https://github.com/BaSui01/InkLore/pull/147)。
