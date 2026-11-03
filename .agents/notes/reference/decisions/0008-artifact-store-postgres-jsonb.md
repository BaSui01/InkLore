# ADR-0008：Agent 制品库统一落 PostgreSQL JSONB，不引入 MongoDB

- **状态**：Accepted
- **日期**：2026-09-17
- **决策者**：架构负责人（AI 协作）
- **关联任务**：INK-020、INK-045、INK-046
- **替代关系**：无

## 背景（Context）

移植进本项目 `internal/agent/infrastructure/artifactstore` 的制品库代码只提供了一个 `MongoStore`
实现（`Store` 是端口接口），但两份编排文件 `docker-compose.dev.yml` / `docker-compose.prod.yml`
实际启动的服务只有 `pgvector/pgvector:pg16` 与 `redis:7-alpine`，**没有任何 mongodb 服务**；
`MongoStore` 本身从未被组合根实例化（没有任何装配点向它传入 Mongo 客户端）。

> **勘误（2026-09-17，同日复盘）**：本文初稿曾称 `artifactstore` 是「不可达包」，实测有误 ——
> `agent/infrastructure/persistence/agent_payload_externalizer.go` 导入了该包，因此它在运行路径上
> （`go list -deps ./cmd/server` 可比对确认），只是从未被实例化，且该外部化器把
> `artifactstore.BackendMongo` 硬编码为存储后端标签。**决策结论不变**，删除范围以 INK-046 为准。

约束与相关红线：

- `AGENTS.md` 第 1 节要求模块化单体 + 三个业务域，**不拆微服务**；引入第二种文档数据库等于
  增加一张必须独立运维、备份、监控的数据平面。
- 租户隔离模式（INK-010 / INK-044）建立在「每条 SQL 都带 `tenant_id` 作用域」之上；
  MongoDB 没有等价的强制作用域机制，需要用应用层代码重造一套。
- 制品的数据形态是「JSON 文档 + 少量长文本」，单条通常远小于 1MB，
  不需要 MongoDB 的分片、Change Stream 或 GridFS。
- 数据库已启用 `pgvector` 与 `pg_trgm`，JSONB 与 GIN 索引开箱可用。

## 决策（Decision）

InkLore 的 Agent 制品库**只使用 PostgreSQL**：制品正文以 JSONB 落表，配 `tenant_id` 作用域与
`content_hash` 去重；大二进制交给对象存储（后续任务）。**不引入 MongoDB**，
`MongoStore` 作为待删除代码登记为 INK-046。

具体做法：

1. 制品相关表（`agent_artifacts`、`artifact_refs` 等）纳入 INK-045 的统一建表迁移，
   字段必须包含 `tenant_id`、`type`、`source`、`content_hash`、`payload JSONB`、
   `payload_size_bytes`、`retention_expires_at`。
2. `artifactstore.Store` 端口保持稳定；删除 `MongoStore` 与 `BackendMongo` 常量的动作随 INK-046 执行。
3. `docker-compose.*.yml` **保持不引入 mongodb**（与本决策一致，无需修改编排文件）。
4. 超过阈值（暂定 4MB）的二进制**不内联入库**，在对象存储任务落地前明确报错，而不是静默截断。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 引入 MongoDB（沿用现有 `MongoStore`） | 文档模型贴合、移植代码可直接用 | 新增一个数据平面；无强制 tenant 作用域；备份/恢复/监控/CI 都要再来一套 | 收益仅是少写一层 JSONB 映射，代价是长期运维面与租户隔离风险 |
| 引入 MongoDB 但仅开发环境启用 | 保留迁移代码 | 开发与生产存储不一致，问题推迟到上线才暴露 | 违反环境等价原则 |
| **PostgreSQL JSONB（采纳）** | 复用既有库、租户作用域模式、迁移工具链与备份流程 | 超大单文档与高频原子更新不如文档数据库自然 | — |
| 文件系统存制品 + DB 存索引 | 大制品成本低 | 与容器化多实例、备份、回滚冲突 | 部署复杂度反而更高 |

## 后果（Consequences）

**正面**

- 数据平面收敛为 PostgreSQL + Redis 两个组件，编排、备份、CI 与生产部署无需新增依赖。
- 制品与作品、事实、运行记录同库，可在同一只读事务内完成「运行记录 + 制品 + 引用」的一致性读取。
- 租户隔离直接沿用既有 `tenant_id` 作用域模式，不需要第二套隔离实现。

**负面 / 代价**

- 必须在 INK-045 里认真设计 JSONB 表结构与索引（如 `(tenant_id, type, created_at DESC)`、
  `content_hash` 唯一约束），不能依赖文档数据库的 schema-less 兜底。
- 单条超大制品不适合内联，必须等对象存储任务；在此之前超限写入明确失败。
- `mongo_store.go` 等文件在被删除前属于「已知待删代码」，可达性统计会继续把它们计入不可达。

**需要长期承担的义务**

- 新增制品类型时必须同时给出 `tenant_id` 作用域与容量上限。
- 若未来确需文档数据库，必须新增 ADR 替代本文件并给出迁移与回滚方案，不得修改本文正文。

## 验证方式（Verification）

1. `grep -rn "mongo" docker-compose.dev.yml docker-compose.prod.yml` 无输出（确认没有 mongodb 服务）。
2. INK-045 落地后，`migrations/` 中的制品表带 `tenant_id`、`content_hash`、`payload JSONB` 列。
3. INK-046 完成后 `internal/agent/infrastructure/artifactstore/mongo_store.go` 不存在，
   且 `agent_payload_externalizer.go` 不再引用 `artifactstore.BackendMongo`；`go build ./...` 通过。
4. 超限制品写入返回明确错误（随对象存储任务验证）。