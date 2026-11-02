# Agent Note: pnpm 锁文件与 dev CI 门禁保持一致

Status: implemented

Task: INK-248
Scope: Makefile、.github/workflows/ci.yml 与开发验证指南

## Problem

前端已有 pnpm 工作区和 pnpm-lock.yaml，聚合构建脚本也调用 pnpm；CI 却读取不存在的 package-lock.json 并执行 npm ci。push 只监听 main，无法覆盖用户授权的 dev 合入后检查。验证指南还引用旧 i18n 路径。

## Decision

CI 先安装固定 pnpm 11.25.0，再配置 Node 22 的 pnpm 缓存，使用现有锁文件进行 frozen-lockfile 安装并保留双端构建。不改依赖或锁文件。push 增加 dev，保留 main 和已有 PR 触发；不更改权限、凭据、保护或门禁内容。版本选择采用当前环境可执行的 pnpm 版本，官方兼容表支持 Node 22；不跟随 latest 漂移。

dot 云端复用 Makefile 的 make ci，顺序执行 Skills、历史 notes、格式、全包 Go build、vet、可达性护栏、i18n、冻结依赖安装和双端构建。保持原 workflow，不依赖它提供唯一验收证据。BUILDVCS 默认 auto；仅来源已核验的无 Git 快照显式传 BUILDVCS=false，不自动降级。notes --base 仍要求真实 Git 历史，缺失必须失败。

## Alternatives considered

- 新建 shell/CI 框架：已有 Makefile 可复用，不增加第二套编排。
- 新建 npm 锁文件：会制造第二套依赖事实且聚合脚本仍要求 pnpm，不采用。
- 不固定 pnpm 版本：每次安装可能改变工具链行为，不能作为可复现安装条件。
- 仅修安装命令：缓存路径仍错误，dev 合入后也无对应 push 检查，因此同时修正同一门禁链路。

## Consequences

配置与现有包管理器一致，本轮 dot 快照已真实完成 frozen-lockfile 安装和双端构建，但不能据此声称全部门禁通过。Node 22 的远端运行未由本地 Node 24 代替。未改 Go 包、依赖或运行接线，不新增测试文件。已检索活跃 notes；既有 i18n 检查修复与工程记忆门禁决定保持适用，本次不改历史封印。

## Verification

2026-11-01，基于 b8cf9bfc7b1681d41412045d79ed48ecd3d43c68 的 960 blob 校验快照，在独立副本应用本次 Makefile 草稿，由 dot 本地执行；未使用 Codex 任务或依赖 Actions 结果。

- `make build-check vet BUILDVCS=false`：通过（Go 1.27.1）。
- `make project-skills i18n-check BUILDVCS=false`：通过，920 个 i18n key 一致。
- `make frontend-install PNPM='pnpm --store-dir /workspace/shared/inklore-env/pnpm-store'`：冻结安装通过。
- `make frontend-build PNPM='pnpm --config.store-dir=/workspace/shared/inklore-env/pnpm-store'`：Creator/Admin 均通过。Node 24.22.1、pnpm 11.25.0，不代表 Node 22 验证。
- `make ci BUILDVCS=false NOTES_BASE=6cdc9cede365031361de5c49e031dda094df74f1`：退出 2，快照没有真实 Git 历史，notes --base 拒绝，未伪造基线。
- 单项 fmt-check、guard-report 均非零，保留来源快照已有格式/架构护栏失败，不在本任务修复或降低门禁。
- Makefile diff 空白检查与配方解析通过。完整业务请求、DB、浏览器及独立验收未完成。

本地证据：`/workspace/shared/inklore-ci-local-draft/actual-*.log`；来源快照报告 `/workspace/shared/inklore-env/PR5-VALIDATION.md`。这些是本轮 dot 工作区证据路径，不是仓库可下载附件或未来环境保证。完整门禁未全过，INK-248 保持 review、PR 保持 draft，不合并。
