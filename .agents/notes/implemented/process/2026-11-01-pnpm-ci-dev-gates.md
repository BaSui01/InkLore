# Agent Note: pnpm 锁文件与 dev CI 门禁保持一致

Status: implemented

Task: INK-248
Scope: .github/workflows/ci.yml 与开发验证指南

## Problem

前端已有 pnpm 工作区和 pnpm-lock.yaml，聚合构建脚本也调用 pnpm；CI 却读取不存在的 package-lock.json 并执行 npm ci。push 只监听 main，无法覆盖用户授权的 dev 合入后检查。验证指南还引用旧 i18n 路径。

## Decision

CI 先安装固定 pnpm 11.25.0，再配置 Node 22 的 pnpm 缓存，使用现有锁文件进行 frozen-lockfile 安装并保留双端构建。不改依赖或锁文件。push 增加 dev，保留 main 和已有 PR 触发；不更改权限、凭据、保护或门禁内容。版本选择采用当前环境可执行的 pnpm 版本，官方兼容表支持 Node 22；不跟随 latest 漂移。

## Alternatives considered

- 新建 npm 锁文件：会制造第二套依赖事实且聚合脚本仍要求 pnpm，不采用。
- 不固定 pnpm 版本：每次安装可能改变工具链行为，不能作为可复现安装条件。
- 仅修安装命令：缓存路径仍错误，dev 合入后也无对应 push 检查，因此同时修正同一门禁链路。

## Consequences

配置与现有包管理器一致，但 frozen-lockfile 是否接受当前锁文件、依赖下载和双端构建是否成功仍须真实执行。Node 22 的远端运行未由本地 Node 24 代替。未改 Go 包、依赖或运行接线，不新增测试文件。已检索活跃 notes；既有 i18n 检查修复与工程记忆门禁决定保持适用，本次不改历史封印。

## Verification

工作流 YAML 与关键字段、原有 jobs 和门禁保留情况及变更空白进行本地静态检查；这只验证配置结构，不证明 GitHub Actions 实际执行。当前没有完整本地 checkout，未执行 pnpm frozen 安装、Go build/vet、双端构建、全仓 Skills/notes 或业务验收。任务保持 review，PR 保持 draft；最终同一 head SHA 的完整 CI 以及 dev push 触发由独立验收阶段核实。
