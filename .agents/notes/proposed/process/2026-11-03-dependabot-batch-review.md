# Agent Note: Dependabot 五项补丁整合与双分支安全验收

Status: proposed

Task: INK-261
Scope: go.mod、go.sum、frontend/package.json、frontend/pnpm-lock.yaml、dev 与 private 发布

## Problem

Dependabot 在仓库默认 private 分支分别建立 #49–#53 五项补丁，覆盖网络传输与前端构建/Markdown 渲染依赖。每个 bot PR 具备源码锁文件修改，但缺少仓库要求的业务影响、工程决策与独立运行验收。直接顺序合并 private 可能令 dev 的依赖图落后、多个 pnpm 锁文件反复冲突，而且会降低合并审核的可复现性。

## Proposal

在 dev 独立分支整合以下官方 Dependabot PR 的变动并留存出处：
- #49 github.com/quic-go/quic-go v0.59.0 → v0.59.1
- #50 golang.org/x/net v0.54.0 → v0.55.0
- #51 vite v7.3.6 → v7.3.7；frontend/package.json 将 vite 下限指定为 ^7.3.7
- #52 source-map-js v1.2.1 → v1.2.2
- #53 markdown-it v14.3.2 → v14.3.3

所有包与完整 integrity/hash 取自 bot 的经过审核的提交。用共同旧锁文件为三份 pnpm 差异定位非重叠变更，再合成为一个最终锁文件。仅允许四个 manifest/lockfile 和 backlog、architecture、CHANGELOG、工程笔记变动，不对无关组件进行版本更新。

先运行 Go 模块校验、Go build/vet/archguard、pnpm frozen-lockfile、Creator/Admin 双端构建、Skills/notes、GitHub PR 必需 CI 与临时 PostgreSQL/Redis/API 冒烟。通过后以人工审阅合入 dev，再通过独立小范围发布 PR 更新 private。不得直接绕过受保护分支或解除源 Dependabot PR 的阻止标签。依赖进入 private 且逐字节核对后，原 bot PR 可作为 superseded 关闭。

## Alternatives considered

- 将五个 bot PR 直接合并 private：开发分支落后且缺正式交付证据，拒绝。
- 一次性大范围升级到所有依赖的最新版：会引入未审查的跨主版本变化，拒绝。
- 跳过锁文件重生成与 frozen 校验：可能导致前端包解析不可复现，拒绝。
- 为消除安全告警自动合并所有 PR：与代码、锁文件需人工治理的规则冲突，拒绝。

## Risks

QUIC/HTTP3 trailer、HTTP 头与网络解析变更可能影响实际请求。Vite 工具链变化影响打包和调试、markdown-it 影响不可信 Markdown 内容的 XSS 及 CSP；需要真实运行与前端内容安全验收。编译、依赖完整性和 CI 全绿不能取代浏览器或供应商网络层验证。

## Acceptance criteria

- 合并后版本恰好为上面五项指定的目标版本，所有差异能追溯 bot 提交，不修改其他依赖。
- go mod verify/build/vet/archguard、pnpm frozen lockfile、两端构建、i18n、Skills/notes 和 GitHub CI 全绿。
- 临时 PG/Redis 与真实 API 正常，且 PR 最新 SHA 与报告相同；独立审阅后合入 dev。
- Private 发布走另一条受保护 PR，未经发布不能关闭源 Dependabot PR；已有风险不因版本变化视为全部解决。
