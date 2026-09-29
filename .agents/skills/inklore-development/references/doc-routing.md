# 按任务加载文档

相对链接从本文件计算；命令工作目录均为仓库根。先读 AGENTS、R1设计入口与基线、architecture实施差距和目标backlog，再按本表下钻。

| 问题 | 权威入口 | 使用方式 |
| --- | --- | --- |
| 工程红线与冲突 | [AGENTS](../../../../AGENTS.md)、[ADR 索引](../../../../docs/decisions/README.md) | 核对 Accepted / Proposed / 替代范围，不能只看编号新旧 |
| 工程记忆与被否方案 | [notes 规约](../../../notes/README.md) | 按分类、机制检索活跃/否决笔记；历史追溯才读取 archived |
| 当前代码接线 | [architecture](../../../../docs/architecture.md) | 对照源代码及本轮命令，历史数量不复制到技能 |
| 任务与完成定义 | [backlog](../../../../docs/backlog.md)、[管理规约](../../../../docs/project-management.md)、[冻结实施计划](../../../../docs/system-design/18-DELIVERY-AND-ACCEPTANCE-PLAN.md) | backlog 唯一任务状态；实施计划仅正式R1.x变更时修订 |
| 重构目标 | [设计索引](../../../../docs/system-design/00-INDEX.md)、[R1 基线](../../../../docs/system-design/00-CONTRACT-BASELINE.md) | 读目标域分册；设计冻结不等于代码完成 |
| 后端域实现 | [领域详细设计目录](../../../../docs/system-design/domains/) | 按 content/outline/worldview/inspiration/compliance/agent/llm 选择对应分册，继续读 DDL 与契约 |
| REST 与持久化 | [持久化设计](../../../../docs/system-design/11-DATA-PERSISTENCE-AND-API-SPEC.md)、[契约入口](../../../../docs/system-design/api/README.md) | 同时核对实际 handler、迁移与组合根；新重构契约按 R1 指向的 api/data 文件读取 |
| SSE / WS | [有效ADR索引](../../../../docs/decisions/README.md)、[实时任务与恢复](../../../../docs/system-design/21-REALTIME-JOBS-AND-RECOVERY.md) | 与目标基线和已运行端点逐项对照，不混用 POST 生成与 GET 订阅 |
| 创作者页面 | [Creator UX](../../../../docs/system-design/15-CREATOR-FUNCTION-UX-SPEC.md) | 连同目标领域、真实 API、权限和未保存状态一起阅读 |
| 管理页面 | [Admin UX](../../../../docs/system-design/16-ADMIN-FUNCTION-UX-SPEC.md) | 核对配置落库、生效、脱敏与审计，不读取作者私有正文 |
| UI 组件及错误 | [设计系统](../../../../docs/system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)、[错误恢复](../../../../docs/system-design/14-ERROR-HANDLING-AND-RECOVERY.md) | 覆盖 loading/empty/error/disabled、键盘、i18n 与输入保留 |
| 运维及交付 | [运维设计](../../../../docs/system-design/13-DEVOPS-OBSERVABILITY-AND-OPERATIONS.md)、[验收计划](../../../../docs/system-design/18-DELIVERY-AND-ACCEPTANCE-PLAN.md) | 设计 SLO 与实际演练结果分别记录 |

## 防止文档漂移

不同问题各有归属：规则在 AGENTS、现状在 architecture、目标在 system-design、架构理由在 decisions、实现因果在 notes、执行在 backlog、版本交付在 CHANGELOG。技能只保留阅读路线与操作流程，不复制 API 清单、表结构、包数量和任务状态。

先搜索机制名、错误码、端口名与代码入口注释，再读命中的决策全文。历史归档仅在追溯理由时读取；目录不存在时检索当前索引，不凭空建一个旧目录。文档迁移同步入站链接，运行 node scripts/check-project-skills.mjs。
