# Agent Note: 系统预置智能体种子与创作端人机闭环审阅

Status: implemented

Task: INK-223
Scope: cmd/admin/ (seed.go, main.go)、frontend/apps/creator/src/routes/ (ArtifactReviewModal.tsx, editorRun.ts, EditorRoute.tsx) 与 transport/contracts SDK

## Problem

此前数据库中虽然具备完整的 Agent 与工作流表结构，但表中完全缺少系统预置的 LLM Task Policy、Agent 定义与 Kahn DAG 工作流种子数据，导致创作者端调度智能体时报错策略不存在或配置未启用；同时，创作者端在触发划词润色与扩写时缺少可视化的产物差异审阅面板，违反了 AGENTS.md 中“Agent 绝非事实 Owner，无权私自篡改正文，所有建议必须由作者确认采纳后方可落库”的核心工程原则。

## Decision

1. 管理端命令行新增 `admin seed` 播种引擎（`cmd/admin/seed.go`）：
   - 为租户注入预置 Provider（`openai`）、Model（`gpt-4o-mini`，128k 上下文）；
   - 注入核心任务策略 `llm_task_policies`（`inline_polish` 正文划词润色、`novel_architect` 小说架构推演、`content_polish` 章节深度润色）；
   - 注入核心 Agent Definitions（`inline_polisher`、`novel_architect`、`worldview_extractor`）；
   - 编排并注入标准 Kahn DAG 工作流（`workflow_inline_polish`），符合有向无环图拓扑校验。
2. 创作者端打通智能体调度与人机闭环审阅（Human-in-the-Loop）：
   - `api.agent` 补齐目录查询（`listCatalogWorkflows`, `listCatalogDefinitions`）与产物查询；
   - `editorRun.ts` 动态匹配当前租户启用的定义与工作流，构造标准 `CreateRunInput` 提交真实后端；
   - 落地 `ArtifactReviewModal.tsx`，基于 `@inklore/ui` 的 `DiffView` 展示选区原文与 AI 建议文本的双向对比，由作者决定是否采纳替换并自动触发草稿保存。

## Alternatives considered

- 允许 Agent 直接修改正文草稿：违背 AGENTS.md 事实单一来源原则与作者主权红线，放弃。
- 在前端写死固定的 Agent UUID：租户隔离要求不同租户拥有独立的配置版本与权限作用域，不能硬编码跨租户 UUID，放弃。

## Consequences

系统在部署和启动后具备开箱即用的预置小说智能体矩阵，划词润色与扩写形成了“作者划词 -> 调度工作流 -> 差异审阅 -> 确认采纳”的完整合规闭环。

## Verification

执行 `go run ./cmd/admin seed` 实证全量 5 个已有租户的 Provider、Model、Policy、Agent 与 DAG 工作流全部播种入库；启动真实后端服务验证 `/v1/agents/catalog/workflows` 返回真实数据与 401 鉴权守卫；双端生产构建、`go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`（R1–R5 全绿）、`go run ./cmd/i18ncheck`（945/945 对齐）及 `node scripts/agent-notes.mjs check` 校验通过。未新增或修改任何测试文件，未执行任何未授权 Git 命令。
