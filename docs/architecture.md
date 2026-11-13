# 当前架构与能力边界

依据 dev 提交 `60279c40f5c81982c69d035c0c886931d22334cd` 的源码审计。
本次治理只调整规则、资料、辅助检查与内部命名，未修复下列业务缺口。
旧逐任务记录已归档至 [历史实施记录](../.agents/notes/reference/implementation/architecture-history.md)。

## 实际装配

| 层次 | 入口 | 当前职责 |
| --- | --- | --- |
| 进程与组合根 | cmd/server、internal/app | 配置、HTTP、领域装配、Worker 生命周期 |
| 技术基座 | internal/platform | 身份、事务、幂等、作业、文件、事件和日志 |
| 创作领域 | internal/content、outline、worldview | 作品与章节、大纲、设定和事实账本 |
| 创作辅助 | internal/inspiration、compliance | 拆书、建议、扫描和审阅 |
| 持久 Agent | internal/agent/application/scheduler.go | PostgreSQL Run/Node/Job、TAO、工具和产物 |
| 模型调用 | internal/llm、internal/app/adapters | 目录、租户渠道解析、Provider 和生成 |
| 双端界面 | frontend/apps/creator、frontend/apps/admin | 作者与管理台，共享 contracts/transport/UI |

模块化单体和领域边界是当前实现。调整依据职责、调用关系与维护成本，不要求固定领域数量或空目录。

## 已实现部分与缺口

| 能力 | 源码判断 | 边界 |
| --- | --- | --- |
| 作品、章节、大纲、角色与设定 | 存在真实路由、服务和存储 | Creator 部分字段、翻页和作品交接不完整 |
| Run、作业租约和 SSE | 存在持久实现 | 模型上下文和前端恢复仍有接线缺口 |
| Provider 与租户渠道选择 | 存在真实实现 | 预算、限流、费用组件未完整接入 GenerateChat |
| 版本、CAS 与章节确认 | 存在领域实现 | 世界观修订查询路径、revision 和事实提案有断点 |
| 导入导出 | 存在任务和服务 | Content 未装配 Files/Reviews，可出现无文件产物的成功状态 |
| 通用澄清 | 存在迁移和生成契约 | 当前 Run 应用、路由和问答 UI 不完整 |
| 请求 Schema | 存在注册表接口 | Initialize / Validate / CheckRoutes 当前为空实现 |

重点证据：[组合根](../internal/app/domains.go)、[调度器](../internal/agent/application/scheduler.go)、[上下文适配](../internal/app/adapters/domain_services.go)、[Schema 注册表](../internal/platform/requestschema/registry.go)、[Content 任务](../internal/content/application/jobs.go)、[前端 SDK](../frontend/packages/transport/src/api/sdk.ts)。

完整能力矩阵、调用链和 Issue 映射见 [源码审计](../.agents/notes/reference/audits/2026-11-03-source-baseline.md)。
以上是源码结论，未将历史记录或本次静态检查当成真实模型、数据库和浏览器业务验收。

## 数据、安全与验证

PostgreSQL 是业务持久化来源，Redis 承担部分运行时协调。迁移历史位于 migrations/（唯一迁移流），已应用脚本的版本、文件名与校验和保持不变，结构变化一律追加新迁移。

保留租户授权、事实所有权、事务、版本快照、CAS、幂等、审批、租约和敏感信息保护。
退役设计冻结不影响这些运行约束。

编译与类型检查证明可构建；静态语义检查验证部分契约关系；隔离迁移验证特定数据库行为。
用户操作、API、数据、Worker 和模型结果需对应实测，任务状态以 GitHub 为准。
