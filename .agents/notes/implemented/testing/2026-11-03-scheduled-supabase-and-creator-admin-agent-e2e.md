# Agent Note: 保护分支定期只读监控与隔离浏览器端到端门禁

Status: implemented

Task: INK-270
Scope: .github/workflows/supabase-staging-preflight.yml、creator-admin-agent-e2e.yml、scripts/ci 验收脚本

## Problem

此前只在手动 workflow_dispatch 对 Supabase 进行了只读连接测试；默认保护分支一旦长期没有人手动操作，就无法及时发现网络、迁移或权限变化。当前临时 PostgreSQL/Redis 门禁只检验 Creator 注册与基础健康接口，尚未贯穿 Creator 作品、Admin 治理、Agent 权限与浏览器真实路由。

## Decision

在默认 private 分支上以 UTC cron 安排每日 Supabase 受限只读预检（03:17），并保持既有 Environment 保护和零业务写入；GitHub 定时事件仅默认分支有效。另每周一、周四 02:43 UTC 自动运行一次性 PG17/Redis7 + Go API + 双端生产构建 + Chromium 门禁。PR/保护分支 push 同时运行，所有随机 CI 身份仅存在于一次性 runner，绝不使用远程共享库或付费模型密钥。浏览器使用 pinned Playwright 只在 /tmp 安装，项目依赖文件不变。

API 验收只断言真实资源的 PostgreSQL 持久化、租户隔离、Admin 身份与定义写入、Creator 安全目录，以及 Agent 拒绝无权限 Run 时无副作用。使用真实 HttpOnly Cookie 恢复受保护的前端路径，不通过 localStorage 伪造已登录状态。

## Alternatives considered

- 在真实 Supabase 中创建供 E2E 使用的用户/小说：只读账号不能写，扩大现有云端数据暴露面，拒绝。
- 在外部 PR 上下文中读取生产模型 Secret：不安全且可能产生费用，拒绝。
- 仅 curl /healthz 和静态截图便宣称 Agent 全流程通过：不覆盖身份和租户边界，拒绝。
- 持续保留长期开机测试服务器：免费托管不保证资源且产生额外运维，不必要。

## Consequences

Schedule 是最佳努力触发，可能延迟，不能作为严格 SLA。E2E 会拉取依赖与 Chromium，首次 GitHub Actions 执行可能较慢。将全功能 Agent Worker、真实上游模型调用及恢复场景与无模型密钥的 Agent 治理验收明确区分；未验证不得写为 done。

## Verification

目前已完成只读调度、临时服务和验收脚本的实现；以 PR 最新头的 GitHub CI、E2E 浏览器真实运行结论、发布后的默认分支 push、以及手动 Supabase preflight 为最终证据。计划任务实际是否自动触发须到下一次 cron 再复查，本地静态校验不能代替远程执行。
