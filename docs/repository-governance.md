# 仓库治理与自动合并

## 分支职责

- `private`：稳定主分支。所有变更必须经 PR 与 CI，不由自动化替代发布审查。
- `dev`：持续集成分支。适合已完成评审、满足门禁的日常改动。
- `feat/*`、`fix/*`、`docs/*`、`chore/*`、`review/*`：短期任务分支。合并 PR 后 GitHub 自动删除来源分支。
- 无活动 PR 的旧分支只有在其提交已进入稳定保留的分支、确认无其他引用后才能清理。未合并分支一律保留待人工判定。

## 自动合并策略

仓库开启 GitHub 原生 auto-merge 与合并后删除来源分支。运行在默认分支 `private` 的可信 `pull_request_target` 工作流只评估 **同仓库、目标为 `dev`、非 Draft** 的 PR；不会 checkout 或执行 PR 代码。

- **普通文档自动化**：仅 `README.md`、`CONTRIBUTING.md`、开发指南/项目管理入口等明确白名单文件可以自动开启 auto-merge。
- **普通代码需授权**：其他非敏感变更必须由维护者添加 `auto-merge-approved` 标签；仅 CI 通过不足以授权自动合并。
- **始终人工**：包含 `SECURITY.md`、本地环境/凭据文件、`.github/`、`AGENTS.md`、ADR、契约资产、迁移、平台安全与底层 LLM/Agent 基础设施的 PR；即便加授权标签也不会自动合并。
- **强制阻止**：`manual-review` 或 `do-not-merge` 标签、跨仓库来源、Draft、文件过多或目标非 dev。
- **动态撤销**：PR 改为 Draft、修改目标、添加阻止标签或新增敏感路径时撤销此前开启的 auto-merge。陈旧 head 事件不允许撤销新版本的决定，重命名同时检查原路径和目标路径。

自动合并只在 GitHub 要求的最新三项 CI、PR 无冲突且 Review 对话解决后执行；不得用机器人绕过保护或把代码构建当作业务验收。目标 `private` 的发布 PR 必须人工审查。

## 仓库设置

`private` 和 `dev` 要求 PR、最新 CI、解决 Review 对话；禁止强推、删除。检查项为项目 Skills/notes、后端构建/静态检查、前端构建。

GitHub 仓库开启 `allow_auto_merge`、`delete_branch_on_merge` 和 `allow_update_branch`。此项是 GitHub 本身的设置，跟工作流脚本独立生效。

## 边界

- PR 的合并不等于业务 Issue 已验收，任务状态和验收证据统一维护在 GitHub Issues。
- Agent 任务恢复、冻结策略、跨租户路由、Provider Key 及数据库迁移需额外安全验收。
- 不主动改默认分支、历史提交、已接受的 ADR 或现有持久化数据。

## GitHub 功能整合

协作看板、依赖安全、定时只读 Insights、Wiki 首次发布与 GitHub Security 的使用步骤统一见 [GitHub 功能与运维入口](github-platform.md)。

GitHub Project 展示已有 Issues 和 Milestones，不再与文档维护两套状态。Dependabot 与定时健康报表以默认分支中的配置为准；凭据和历史风险按实际安全任务持续处理。
