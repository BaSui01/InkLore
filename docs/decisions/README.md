# 架构决策记录（ADR）

本目录原位保存选择依据。R1施工设计以 [00基线](../system-design/00-CONTRACT-BASELINE.md) 和有效替代关系为准；治理见 [12分册](../system-design/12-ENGINEERING-GOVERNANCE.md)。旧ADR未被替代条款仍有效，原文不改写。

## 索引

| ADR | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0001](./0001-modular-monolith-domain-first.md) | 采用模块化单体与 Domain-First 分层（第 2 条业务域清单已被 ADR-0014 替代） | Accepted | 2026-09-16 |
| [0002](./0002-realtime-sse-and-ws.md) | SSE 与 WebSocket 职责隔离 | Accepted | 2026-09-16 |
| [0003](./0003-beijing-time-contract.md) | 全栈北京时间契约 | Accepted | 2026-09-16 |
| [0004](./0004-secrets-only-from-env.md) | Provider 密钥只从环境变量注入 | Accepted | 2026-09-16 |
| [0005](./0005-no-fabrication-without-provider.md) | 未配置 Provider 时默认拒绝生成，离线演示需显式开启 | Accepted | 2026-09-17 |
| [0006](./0006-platform-token-and-auth-mode.md) | 平台自签 HS256 令牌与认证默认策略 | Accepted | 2026-09-17 |
| [0007](./0007-chapter-confirmation-and-fact-versioning.md) | 章节确认制与事实版本化 | Accepted | 2026-09-17 |
| [0008](./0008-artifact-store-postgres-jsonb.md) | Agent 制品库统一落 PostgreSQL JSONB，不引入 MongoDB | Accepted | 2026-09-17 |
| [0009](./0009-principal-context-and-approval-authorization.md) | 认证主体经请求上下文传递，审批以「本人审批本人」为当前模型 | Accepted | 2026-09-17 |
| [0010](./0010-embedding-dimension-contract.md) | 嵌入维度契约：向量列维度固定，模型原生维度由上游按声明截断 | Accepted | 2026-09-17 |
| [0011](./0011-production-config-delivery.md) | 生产配置装载：镜像只带模板，真实配置由部署侧只读挂载 | Accepted | 2026-09-17 |
| [0012](./0012-database-as-single-authoritative-store.md) | 数据库是唯一权威存储，内存档位需显式开启 | Accepted | 2026-09-17 |
| [0013](./0013-task-level-model-policies.md) | 任务级模型策略与模型解析顺序（显式 > 策略 > 环境默认） | Accepted | 2026-09-17 |
| [0014](./0014-domain-boundary-expansion-to-seven-contexts.md) | 业务限界上下文由 3 个扩张为 7 个（Supersedes ADR-0001 第 2 条） | Accepted | 2026-09-18 |
| [0015](./0015-compatible-errors-and-write-preconditions.md) | 旧接口兼容提案；整体重构不采用，替代为0016 | Rejected | 2026-09-18 |
| [0016](./0016-r1-reconstruction-design-freeze.md) | R1完整重构设计定稿；文档位置条款由0017替代 | Accepted | 2026-09-18 |
| [0017](./0017-preserve-decision-records-and-single-design-entry.md) | 设计单入口、ADR原位与执行记录分离 | Accepted | 2026-09-18 |
| [0018](./0018-r1-minimal-contract-errata.md) | R1.1四项最小契约勘误 | Accepted | 2026-09-18 |
| [0019](./0019-r1-conditional-download-erratum.md) | 条件下载304声明单项补齐 | Accepted | 2026-09-18 |
| [0020](./0020-gin-http-convergence.md) | Gin HTTP 入口与严格请求契约收敛 | Accepted | 2026-09-20 |
| [0025](./0025-image-adapter-current-provider-contract.md) | 图片适配器按当前供应商合同 fail-closed | Accepted | 2026-09-20 |
| [0026](./0026-llm-channel-protocol-and-secret-resolution.md) | LLM 渠道协议与凭据解析收敛 | Accepted | 2026-09-20 |
| [0027](./0027-proposed-creator-task-policy-catalog.md) | Creator 安全任务策略目录、Run 版本绑定与中途澄清 | Accepted | 2026-09-24 |
| [0028](./0028-password-minimum-length.md) | 密码最低长度调整为六个字符 | Accepted | 2026-09-27 |
| [0029](./0029-root-username-admin-login.md) | 管理员邮箱与root用户名双凭据登录 | Accepted | 2026-09-27 |
| [0030](./0030-global-root-administrator.md) | root全局管理员与Creator双权限 | Accepted | 2026-09-28 |
| [0031](./0031-creator-root-username-login.md) | Creator 登录支持 root 用户名 | Accepted | 2026-09-28 |
| [0032](./0032-agent-guided-novel-creation.md) | Agent 引导式创建小说 | Accepted | 2026-09-28 |
| [0033](./0033-postgresql-safe-project-logs.md) | PostgreSQL 安全项目日志只读检索投影 | Proposed | 2026-09-28 |
| [0034](./0034-agent-tool-execution-status-and-safe-retirement.md) | Agent 工具执行状态显式暴露与历史定义安全退役（工具配置展示部分由 ADR-0035 提议替代） | Proposed | 2026-09-29 |
| [0035](./0035-agent-builtin-readonly-tool-catalog.md) | Agent 工具只读内置目录，不开放在线配置 | Proposed | 2026-09-29 |

## 约定

- 文件名：`NNNN-kebab-title.md`，编号四位、单调递增、**永不复用**。
- 状态：`Proposed` → `Accepted` / `Rejected`；被替代时标为 `Superseded by ADR-XXXX`。
- **已接受的 ADR 不修改正文**。需要改变结论时新增一条 ADR 并在旧条目顶部加替代说明。
- 判断标准：任何影响分层、域边界、存储选型、全局契约或新增对外依赖的改动，都必须有对应 ADR。

## 何时需要写 ADR

需要：拆分/合并域、引入新框架或驱动、改变数据落库引擎、改变通信协议归属、定义全局时间/密钥/鉴权契约。

不需要：域内重构、修 bug、加一个工具函数、调整日志文案。

- [ADR-0021：R1.x认证扩展](0021-r1-auth-extensions.md)：Accepted；部分替代ADR-0016公开注册限制。

- [ADR-0022：PostgreSQL 17基线](0022-postgresql-17-baseline.md)：Accepted；部分替代ADR-0016数据库主版本。

- [ADR-0023：Creator普通路径路由](0023-creator-history-routing.md)：Accepted；用户要求普通路径，约定History导航与静态托管回退。

- [ADR-0024：R1完整版本化迁移链收敛](0024-r1-complete-migration-stream.md)：Accepted；补齐Agent、Inspiration、Compliance与Permissions正式迁移。

- [ADR-0025：图片适配器按当前供应商合同 fail-closed](0025-image-adapter-current-provider-contract.md)：Accepted；移除退出 Imagen predict、OpenAI 图生图改为 multipart edits、Gemini 不默认放开安全阈值。

- [ADR-0026：LLM 渠道协议与凭据解析收敛](0026-llm-channel-protocol-and-secret-resolution.md)：Accepted；区分公开 Provider 与内部 transport，并使数据库 SecretRef fail-closed 解析。

- [ADR-0037：源码文件大小上限调整为500KiB](0037-source-file-size-limit-500kib.md)（Accepted，用户明确裁决，仅替代旧源码大小条款）。

- [ADR-0038：Supabase 测试库使用私有 schema 隔离](0038-supabase-isolated-staging-schema.md)（Proposed，限 INK-255 云端验收，不变更 R1 冻结迁移原文）。
