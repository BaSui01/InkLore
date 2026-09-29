# Agent Note: 合规质检与文风检查真实接线及净化采纳闭环

Status: implemented

Task: INK-225
Scope: cmd/admin/, frontend/apps/creator/src/routes/, frontend/packages/contracts/, frontend/packages/transport/, frontend/packages/i18n/

## Problem

此前编辑器中存在以下缺口与占位：
1. `EditorRoute.tsx` 顶部操作栏中的“文风检查”按钮存在 `alert("正在连接 compliance 审核接口...")` 假占位，未对接后端合规域真实 API；
2. 前端 SDK 缺少对 `/v1/compliance/scans` 和 `/v1/compliance/reports` 的封装，创作者无法直观获知当前章节正文的 AI 味评分（AIFlavorScore）、违规问题列表与改写建议；
3. 新建租户缺少预置已发布的出版级合规规则集（`compliance_rule_sets`），导致调用扫描时报错规则快照不可用。

## Decision

1. 在 `cmd/admin/seed.go` 中为租户初始化注入已发布的出版级网文行文与去AI味规则集（包含常见AI腔调修辞、生硬过渡词、违禁词等），预计算 SHA-256 规则集校验和；
2. 在 `@inklore/contracts` 和 `@inklore/transport` 补齐 `ComplianceReport`、`ComplianceIssue`、`CreateScanInput` 及 `api.compliance.scan` / `getReport` / `listIssues` / `ignoreIssue` 真实客户端接口；
3. 落地独立的 `ComplianceReviewModal.tsx` 抽屉/模态框组件（严格 <= 50KB 铁律）：
   - 动态调用真实后端 `/v1/compliance/scans` 计算正文 SHA-256 并提交快速规则匹配；
   - 结构化呈现 AI 味指数仪表评分（0~100 分）、风险数与详细问题列表；
   - 支持作者针对单项问题点击“采纳建议”，基于 Unicode 码点精确定位并替换草稿正文并触发保存（严禁静默覆盖正文）；
4. 在 `EditorRoute.tsx` 彻底移除 `alert` 假实现，挂载真实合规检查弹窗与采纳联动；
5. 补充中英文纯净国际化词条，`cmd/i18ncheck` 100% 对齐。

## Alternatives considered

- 静默自动替换正文敏感词：严重违反“Agent 绝非事实权威，所有修改必须由作者显式审阅确认”原则，坚决放弃。
- 将抽屉所有代码堆在 `EditorRoute.tsx` 中：会导致该单文件体积超出 50KB（51,200 bytes）红线，故拆分为独立的 `ComplianceReviewModal.tsx`。

## Consequences

创作者端编辑器所有操作按钮 100% 接入真实后端，彻底实现零 alert 占位、零假数据，合规质检与去 AI 味形成“正文扫描 -> 仪表评分 -> 风险呈现 -> 逐条审阅采纳”的完整闭环。

## Verification

全量编译、双端构建、`cmd/archguard` 架构红线校验、`cmd/i18ncheck` 双语词条对齐及工程笔记校验全绿通过。
