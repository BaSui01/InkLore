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
- **始终人工**：包含 `SECURITY.md`、本地环境/凭据文件、`.github/`、`AGENTS.md`、ADR、冻结设计、迁移、平台安全与底层 LLM/Agent 基础设施的 PR；即便加授权标签也不会自动合并。
- **强制阻止**：`manual-review` 或 `do-not-merge` 标签、跨仓库来源、Draft、文件过多或目标非 dev。
- **动态撤销**：PR 改为 Draft、修改目标、添加阻止标签或新增敏感路径时撤销此前开启的 auto-merge。陈旧 head 事件不允许撤销新版本的决定，重命名同时检查原路径和目标路径。

自动合并只在 GitHub 要求的最新三项 CI、PR 无冲突且 Review 对话解决后执行；不得用机器人绕过保护或把代码构建当作业务验收。目标 `private` 的发布 PR 必须人工审查。

## dev → private：每 20 个提交的自动同步

默认分支 `private` 上的 `.github/workflows/dev-promotion.yml` 监听 `dev` push 后成功结束的 CI，并每小时定时补偿，也支持手动触发。GitHub Compare `private...dev` 的 `ahead_by` 达到 **20** 时创建一次受保护同步 PR。

- **始终保留 dev**：PR 源是从当时 `dev` SHA 创建的 `release/dev-<sha>` 临时快照分支，合并后只删除快照分支。
- **Merge Commit**：保留 Git 提交祖先，使成功发布后 `ahead_by` 正常归零或减少；不能使用 squash/rebase 代替。
- **完全自动的条件**：`private` 没有额外分叉提交（`behind_by=0`）、变更没有命中高风险目录、最新 dev 三项必要 CI 全绿，且 GitHub 要求的 PR 检查、合并状态及 Review 对话也满足时才启用 auto-merge。
- **失败关闭**：有分叉、迁移/密钥/身份/Agent-LLM 底层/工作流等敏感变化或检查缺失时，只提交带 `manual-review` 的 release PR；不强推、不跳过保护或私自合并。
- **不重复发布**：已有尚未结束的 `release/dev-*` PR 时不创建第二份；下一次仅在旧 PR 完成后再次按阈值判断。
- 由 `GITHUB_TOKEN` 自动创建 PR 时，GitHub 可能不会触发新的 `pull_request` 工作流；无法取得所需检查时必须阻塞，不得把 dev 的既往检查冒充 PR 的完整独立验收。

此动作只同步 Git 历史，不发布 Tag、不部署服务、不迁移数据库，也不因 CI 全绿而将未验收的业务 Issue 自动标 done。详见 [INK-256 工程记忆](../.agents/notes/implemented/process/2026-11-03-dev-private-20-commit-promotion.md)。

## 仓库设置

`private` 和 `dev` 要求 PR、最新 CI、解决 Review 对话；禁止强推、删除。检查项为项目 Skills/notes、后端构建/静态检查、前端构建。

GitHub 仓库开启 `allow_auto_merge`、`delete_branch_on_merge` 和 `allow_update_branch`。此项是 GitHub 本身的设置，跟工作流脚本独立生效。

## 边界

- PR 的合并不等于业务 Issue 已验收，Issue 仍按 `docs/backlog.md` 的状态管理。
- Agent 任务恢复、冻结策略、跨租户路由、Provider Key 及数据库迁移需额外安全验收。
- 不主动改默认分支、历史提交、已接受的 ADR 或现有持久化数据。
