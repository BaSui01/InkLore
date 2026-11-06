# Agent Note: 按当前确认正文累计作品写作进度

Status: implemented

Task: #118
Scope: content 确认正文统计、作品目标日期、Creator 进度与目标设置

## Problem

原实现把草稿保存/回滚计入 `content_novels.word_count` 与 `content_writing_events`，而确认章节不会调整统计。删除章节扣减当前草稿字数，无法表达“上一确认版本仍生效、新草稿尚未接受”。作品只有目标字数，没有目标完结日期或可用的写作进度界面。

## Decision

- 累计以未删除章节的 `confirmed_version_id` 指向正文为准。Unicode 非空白码点沿用 `Words`/`WritingDelta`；草稿保存、回滚、导入和复制等待作者确认。
- 确认、删除、恢复与事实提交/章节版本处于同一 UoW。以 `(tenant_id, chapter_id, chapter_revision)` 唯一约束记录转换；同一确认版本可以多次删除恢复，每次只影响对应转换。命令重放复用既有幂等结果。
- 日统计复用原事件表，仅读取 `confirm/delete/restore` 来源。新增和减少分别累计，净增可为负，当前累计不得为负。保留响应 `save_count` 字段名，其含义在契约中明确为确认/删除/恢复次数。
- 应用没有用户/租户时区配置；沿用 `timeutil` 的 `Asia/Shanghai` 自然日，不假设不存在的配置。目标完结日期使用 PostgreSQL `DATE` 和 API `YYYY-MM-DD`，不是时间点。
- 迁移 `post_r1/000004_content_writing_goals.sql` 按当前确认版本覆盖回填累计，重复执行不累加。保留旧草稿事件，但不据此虚构历史确认日，也不把回填归入今天。
- Creator 新建可选日期，书架/工作台展示当前确认字数和目标，超过 100% 保留真实百分比及超出字数。工作台显示当天增减，并提供带 CAS 的目标设置。请求中断/无效成功体保留目标输入与幂等 key。

## Alternatives considered

继续按草稿保存统计会与 Issue 明确的“当前确认正文”口径冲突。每日重算或独立统计服务增加延迟和基础设施，不适合已有同事务写入路径。把旧草稿事件转换成确认事件无法恢复真实确认日期，因此只回填累计。每天新增单独聚合表没有当前查询需求，继续复用带索引的事件表。

## Consequences

旧作品升级后累计可能减少，这是从草稿总量纠正为确认正文总量。每日净增包含删除扣减与恢复增加；它不等于输入字符次数。工作草稿仍显示自身字数，但不会改变作品已确认累计。目标日期可留空或清除，目标字数 0 代表未设置。

协调复用 #119 的三个精确 `content.job.created/committed/cancelled` 审计动作与 ApiClient 的成功 JSON 读取修复，避免复制/导入提交在审计阶段整体回滚，或截断响应被当作成功。

## Verification

- `go build ./...`、`go vet ./...`、`gofmt`、`go run ./cmd/archguard -reachability` 通过。
- `pnpm --dir frontend run build` 完成 Creator/Admin 两端；`i18ncheck` 通过。生成的 content Go/TypeScript 契约与机器契约同步。
- `E2E_ALLOW_DISPOSABLE_DB=1 python3 <isolated-runner> --repo <worktree> --label writing118 --api -- python3 scripts/ci/content_writing_acceptance.py` 在隔离 PostgreSQL 17.11/Redis 7.4.2 与真实 API 上通过。请求和断言数据见 [验收记录](../../reference/evidence/issue-118/writing-acceptance.json)。新库 21 条迁移成功，迁移命令重放无新增；升级 SQL 再执行两次仍保持累计 8、当天新增 22/减少 14、9 条确认/状态事件。
- 真实行为包括 Unicode、草稿排除、相同文本再次确认、确认/删除/恢复重放、并发 CAS、回滚待确认、世界观 CAS 失败、统计 INSERT 故障后的全事务回滚、日期校验与清除、跨租户拒绝、复制累计为零。导入统计通过持久化预览夹具再调用真实 CommitImport；完整文件上传/解析链属于 #119，不将该夹具冒充完整导入验收。旧草稿事件不混入新口径；PG 北京午夜两侧分组经隔离事务验证。
- `node --check scripts/ci/content_writing_browser.cjs` 通过。该脚本覆盖真实登录、160%/200% 进度、目标保存与刷新、丢失/截断/空响应同 key 恢复和空态。当前执行环境 Chromium 启动即 `TargetClosedError`，页面行为为 **NOT_VERIFIED**；交由现有 GitHub Chromium CI 执行并保存截图。
- 基线静态契约检查仍报 `idempotency missing llm_fetch_upstream_models` 与 `Error catalog implementation drift`；本变更不修改相应 LLM/错误目录文件，主代理另行修复，不能将这两个门禁记为通过。
