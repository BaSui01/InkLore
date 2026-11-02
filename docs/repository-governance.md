# 仓库治理与自动合并

## 分支职责

- `private`：稳定主分支。所有变更必须经 PR 与 CI，不由自动化替代发布审查。
- `dev`：持续集成分支。适合已完成评审、满足门禁的日常改动。
- `feat/*`、`fix/*`、`docs/*`、`chore/*`、`review/*`：短期任务分支。合并 PR 后 GitHub 自动删除来源分支。
- 无活动 PR 的旧分支只有在其提交已进入稳定保留的分支、确认无其他引用后才能清理。未合并分支一律保留待人工判定。

## 自动合并策略

开启仓库级 auto-merge，使用 GitHub 分支保护检查作为最终合并门槛。自动启用只针对 **同仓库、目标为 dev、非 Draft** 的 PR，且不得带有 `manual-review` / `do-not-merge` 标签。

发现以下路径变化时不自动启用，必须人工审阅：

- `migrations/`、`docs/system-design/`、`docs/decisions/`
- `AGENTS.md`、`.github/workflows/`、`.github/CODEOWNERS`
- 身份认证、授权、加密、支付、计费相关敏感目录

自动启用 auto-merge 不代表跳过 CI 或立即合并。GitHub 仅在最新 head 所要求的检查均通过、没有冲突且满足分支规则后执行合并。新提交会重新检查。禁止管理员绕过规则、禁止 Draft 自动合并。

向 `private` 的发布 PR 必须人工决策，不纳入此自动规则。

## 仓库设置

`private` 和 `dev` 要求 PR、最新 CI、解决 Review 对话；禁止强推、删除。检查项为项目 Skills/notes、后端构建/静态检查、前端构建。

GitHub 仓库开启 `allow_auto_merge`、`delete_branch_on_merge` 和 `allow_update_branch`。此项是 GitHub 本身的设置，跟工作流脚本独立生效。

## 边界

- PR 的合并不等于业务 Issue 已验收，Issue 仍按 `docs/backlog.md` 的状态管理。
- Agent 任务恢复、冻结策略、跨租户路由、Provider Key 及数据库迁移需额外安全验收。
- 不主动改默认分支、历史提交、已接受的 ADR 或现有持久化数据。
