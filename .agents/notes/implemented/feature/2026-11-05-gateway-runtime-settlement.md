# Agent Note: 真实派发的预算、容量与费用恢复

Status: implemented

Task: #103
Scope: LLM Gateway runtime、Agent model adapter、费用回执与应用装配

## Problem

供应商路由已有协议实现，但租户实际生成没有完整使用预算预占、Redis 容量控制与持久执行记录。进程在外部调用后失联时，重复派发可能重复计费；Agent 失败终止也可能丢失已产生的费用回执。

## Decision

真实 Chat 派发绑定租户、冻结策略版本、稳定执行键和请求摘要。PostgreSQL 记录派发阶段与账单预占，Redis 执行容量准入；Redis 不可用时拒绝生成，不降级为无约束调用。上游结果、实际或保守估计用量和费用回执经事务结算；不完整流、超时或结果未知仍保留可追溯账务，不能把未知派发当作安全重试。

PR #125 复核发现两个失败边界：OpenAI usage 已到达但缺少 `[DONE]` 时，decoder 的成功尾部回调未将实际用量传回结算；Redis 准入成功后 MarkDispatch 拒绝，普通 lease release 仍保留 24 小时 attempt 标记。decoder 现在在所有返回路径交付已接收的最终 usage，工具结果仍仅在流完整成功时交付。撤销使用 ReleasePrepared，在 PG 锁下验证 reservation 的完整身份、原 revision、未 dispatch 状态及空 dispatch 元数据，先提交释放，再用原 Redis token 原子撤回 attempt、RPM 与 lease。普通完成释放继续保留防重标记；过期 owner、已 dispatch、已结算及无法证明的状态均不能清除标记。

Agent 将真实回执与 usage.updated 事件同事务去重保存，保留 Estimated 标记。终止 Run 不清空已存在的回执；租约恢复只查询原操作的账务，等待结算，不再次调用模型或工具。组合根注入实际预算与准入服务，预算崩溃恢复使用独立、限时循环，避免慢模型阻塞回收。

凭据只接受合法部署引用，拒绝明文、旧加密 locator 和缺少密钥配置的写入。`LLM_SECRET_BINDINGS` 明确授权供应商、引用类型/名称/版本，创建、轮换和真实派发都重新校验；不能借环境引用读取 JWT 等非 LLM 秘密。历史敏感值固定掩码，不为接口显示而解密。Admin 临时探测密钥不会暗中持久化，也不会吞掉保存失败。

## Alternatives considered

只在 UI 显示预算无法约束实际调用；只用进程内幂等无法跨重启；恢复时无条件重新调用无法保证不重复收费，均不采用。没有响应用量时不宣称精确账单，明确记录估计并保留后续调整来源。

## Consequences

追加 000006 迁移，不修改已应用迁移。部署需要 PostgreSQL 与 Redis；缺少预算、定价、容量或凭据时返回明确失败。外部供应商不保证恰好一次，系统将已派发但不可确认的操作保留为未知状态，阻止隐式重发。

## Verification

- 质检改写的执行键绑定已持久化的 rewrite ID 与轮次；同报告下独立改写不再共享一次计费派发。已配置模型但生成失败时，使用限时独立清理事务持久化 failed 并返回原错误，不再隐式变成规则改写成功。真实规则创建流程还修复了 INSERT 的 13 列仅有 11 个占位符的问题。`AGENT_QUESTION_PG_TEST=1 go test -race ./internal/app -run TestComplianceRewriteOperationPG -count=1 -v` 在 PG17/Redis7 下通过（19.001 秒），实际建立规则集、发布、扫描、两次独立改写及成功/HTTP 500 路径：两次派发/两张回执，同一改写重放零额外派发，生成失败没有候选正文或成功补丁。

- Go build、vet、archguard、契约与生成类型检查通过。
- 隔离 PG17 / Redis7 的 LLM 和 Agent `go test -race` 通过，含容量、预算、实际结算、估计用量、失败收费、回执去重及在用量事件/产物写入前注入数据库故障后的恢复。
- 原 Question / Answer CAS / 同 Run 恢复回归通过；组合后的 app 测试 102 秒完成。
- 凭据真实 HTTP / PostgreSQL / Redis 验收覆盖创建、轮换、幂等重放、审计/Outbox、未授权环境变量、跨供应商/租户、版本不匹配及撤销后拒绝派发；合法绑定经本地真实 HTTP 协议调用并产生费用回执。
- PR #125 修复在隔离 PG17 / Redis7 下执行 `INKLORE_GATEWAY_INTEGRATION=1 go test -race ./internal/llm/infrastructure -run 'GatewayRuntimePostgres|TestChatStream' -count=1 -v -timeout=180s` 通过（14.314 秒）：TLS 流在 usage 后断开仍按实际输入/输出/缓存计算 19 微元，未知结果不能提交工具或再次派发；MarkDispatch 配置失效零派发零收费，原操作恢复后恰好一次调用；旧 PG revision/Redis token 与 dispatched/settled 边界不能撤销新 owner。既有预算、容量、租户、取消、恢复、费用原子性、凭据、超时和租约丢失回归保持通过。
- HTTP 协议 fixture 只证明协议与账务行为。缺少外部供应商凭据，真实外部模型生成仍为 NOT_VERIFIED。
