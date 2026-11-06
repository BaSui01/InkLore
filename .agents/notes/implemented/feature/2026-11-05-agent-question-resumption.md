# Agent Note: 澄清问题与原 Run 的持久恢复

Status: implemented

Task: #104
Scope: Agent Question、Answer CAS、Job 租约、checkpoint、HTTP/SSE、显式工具种子

## Problem

基线 `219d981` 的 Question 表和 API/SSE 契约已有，但 Run 仓储、应用、路由和 Worker 没有相应实现。旧 SQL 把“不是 open”等同于 answered_at 非空，导致未回答问题不能合法取消或过期。节点每次从 ready 恢复都会增加 attempt，8 轮澄清会在第 5 次 Claim 时触发数据库技术重试上限。

## Decision

把 `ask_user` 注册为现有冻结工具目录中的调度控制能力。必须通过显式、严格校验的 Admin seed manifest 注册，再由不可变 definition 工具版本声明启用；模型不能附加租户、策略、工具权限。Scheduler 在含澄清能力的 DAG 批次串行执行节点，确保释放同一 Job 时没有同批其他节点仍执行外部 IO。

Question 使用既有表，最多 8 题、同时 1 题 open，响应限 string enum、array enum、bounded string。题面/答案受 4 KiB 限制；等待取 15 分钟与原 Run deadline 的较早值。问题、node checkpoint、Run 状态、策略版本 pin、SSE 事件和 Job 挂起在同一 UoW 提交。Job 锁先于 Run 锁，worker mutation 核对 lease epoch、fence、当前状态及有效租约。

回答走现有 HTTP Idempotency Executor；校验本人、租户、问题 revision/version、冻结 definition/policy、source revision/hash。唯一成功者把已规范化答案作为不可信上下文存入原 node checkpoint，唤醒原 Job，不新建 Run。过期/源变更使用明确的 committed 409，保证终态与幂等响应同事务；相同 key 回放不再次 CAS。原 Run deadline 不因回答重置，模型调用沿用该绝对截止时间。

`GET Run` 增加 `pending_questions`，与 Run/Nodes 在只读一致快照中读取。Question GET 与 SSE 只投影公开字段，永不返回原始答案。客户端重连使用既有 Last-Event-ID。

新增 `post_r1/000005_agent_question_runtime.sql` 修正未回答终态约束，不修改已应用 SQL。ready 的正常续跑保留 attempt，重新 Claim 仍递增 fence 并绑定新 Job epoch。

## Alternatives considered

没有创建第二种运行器，也没有把澄清复用为工具副作用 Approval。重新创建 Run 会丢失上下文及冻结预算，也无法满足幂等；没有采用。无事务的“先写答案再唤醒”会留下无法恢复的中间状态；没有采用。结果未知的模型请求不会因 Worker 租约回收而重发，终止为 execution_outcome_unknown。

## Consequences

需先应用新迁移。HTTP 错误依赖 #2 的 Question 错误目录注册。源契约增加 `Run.pending_questions`，统一 SDK 由集成分支生成；Creator/Admin UI 在 #106 接线。`ModelGenerateInput.ExecutionKey` 使用持久 `node ID/iteration`，供 #103 对齐真实预算和调用标识。

默认 seed 配置不自动启用新能力；显式 manifest 可使用 `domain.ClarificationInputSchema` / `ClarificationOutputSchema` 的精确 schema，并在 definition.tool_keys 声明 `ask_user`。重复 seed 保留相同工具版本，不恢复被 operator 禁用的 head。

## Verification

- `go build ./...`、`go vet ./internal/agent/... ./internal/app ./cmd/admin`、`go test ./internal/agent/... ./internal/app ./cmd/admin`、`go run ./cmd/archguard`、`node docs/system-design/tools/check-design.cjs`、`node scripts/check-platform-contracts.mjs`、`node scripts/check-project-skills.mjs` 通过。
- 真实数据库命令：在隔离 PG17/Redis7、完整迁移后设置 `AGENT_QUESTION_PG_TEST=1`，运行 `go test ./internal/app ./cmd/admin -run 'TestQuestion|TestClarification' -count=1 -v`。
- 实测：12 并发 Answer 只有 1 次成功；旧 revision、非法选项、跨租户/同租户非 Owner 被拒；原 Key 回放无额外事件/运行/模型轮次；重建 JobsAdapter 后原 Job/Run 续跑；HTTP SSE Last-Event-ID 重连；原始答案不出现在投影与 SSE；取消、Worker/Answer 过期、源变更 committed 409；8 轮上限且技术 attempt 不增加；撤销 Session 后拒绝读答；合法显式 seed 首次/重放/禁用保留。
- 注入问题 pin 持久化失败：Question、checkpoint、question.required 均回滚，Job 保留旧租约；模拟进程死亡后由真实 Worker 回收租约，结果未知请求不重发。
- 运行环境使用真实 PG17 和平台 Identity/Scope/Jobs/HTTP/SSE；模型输入为确定性测试 fixture，源业务对象使用真实 Scope 投影 fixture。真实付费模型、实际 Content 正文/事实端到端、浏览器 UI、Redis 故障恢复均 `NOT_VERIFIED`，不以本测试代替 #103/#105/#106/#107 验收。
- 历史迁移外层事务问题由 #105 独立修复；本轮完整迁移成功不作为其事务原子性证据。上述故障注入验证的是本轮应用 UoW 原子性。
