# Agent Note: R1命令在同一PG事务内授权、执行与缓存结果

Status: implemented

Task: INK-160
Scope: internal/platform/uow、scope、idempotency与internal/app的R1公共装配

## Problem

R1要求本地业务写、scope、revision证明和幂等结果原子提交，且同键执行中应及时返回409。PG中未提交的processing行对其他事务不可见，单纯INSERT ON CONFLICT会等待；先独立提交claim又会留下业务回滚后仍占键的半状态。旧启动链还会加载旧迁移、旧路由和.env，不应被当作R1运行基座。

改前检索既有notes和决策后，以[ADR-0016](../../reference/decisions/0016-reconstruction-design-freeze.md)的同PG UnitOfWork/新库/固定DAG及[ADR-0017](../../reference/decisions/0017-preserve-decision-records-and-single-design-entry.md)的记录位置为准。此笔记补充已接受架构的实现细节，不作新契约裁决。

## Decision

命令先非事务验证当前身份与可见性；进入根UoW后先对五元幂等键争取pg_try_advisory_xact_lock，再持锁验证身份和scope，再查缓存和执行。锁身份与HMAC载荷hash分离，因此密钥轮换不会改变互斥键。claim、业务、结果均在该事务中提交。过期completed在同一行原子重置，processing绝不自动抢占。

普通Within共享连接并保留rollback-only，不能吞掉嵌套错误提交半账。确定性409/422需要缓存拒绝结果，因此增设明确Recoverable边界：同连接savepoint、独立私有cause状态，局部回滚后外层可提交错误结果；取消、panic和savepoint失败仍污染所有祖先。显式Commit=true公共409可提交规定的失效状态，不放宽普通错误。

scope按父作品→子资源加锁，写命令第一次授权即FOR UPDATE，避免两个共享锁升级死锁。历史revision/hash验证仍先持有当前权限与父作品可读证明，不将“当前hash已变化”等同历史证明无效。

R1使用独立迁移embed流和新组合根，当前仅暴露已验收技术探针。旧构造器保留未调用代码，供后续退役任务处理；不挂旧路由形成兼容API。BootstrapR1只消费当前运行配置和进程环境，不读旧.env或要求未接线的AES密钥。

## Alternatives considered

- 先提交processing：会打破业务和幂等原子性，故未选。
- 仅用唯一索引冲突：能保证唯一，但等待未提交行，不能实现执行中及时409，故增加事务advisory try-lock。
- 对所有嵌套失败放宽rollback-only：调用者吞错可能提交半账，故只允许明确的Recoverable边界。
- 缓存确定性错误时手工SQL savepoint而共享cause：SQL虽回滚，根仍被嵌套Within标记失败，最终错误缓存也回滚，故隔离私有状态并传播真正致命条件。
- 让新旧迁移/路由并存：会在R1新库创建旧表并暴露不一致协议，故单独接线；旧库转换仍留INK-177。

## Consequences

同作品写入会在父scope上串行，换取固定锁序与删除/授权一致性。事务上下文不得并发或逃逸；授权闭包由可信transport/application装配，必须绑定Request主体和资源。上层必须正确区分业务拒绝与基础设施故障，不能把未知数据库失败伪装为可缓存409。

不读取.env意味着原来的隐式开发启动习惯改变，必须显式注入环境变量；一次验收中旧Bootstrap确实覆盖显式DSN，但迁移流校验在DDL提交前拒绝混库，随后独立加载器和专用数据库验证完成。

缓存重放只保证已登记本地事务结果，不授权自动重发外部模型/工具副作用。人为遗留processing保持409并需人工诊断。数据库权限收紧、后台清理和各域行为不因基础表存在而完成。

## Verification

[INK-160验收记录](../testing/2026-09-18-acceptance-ink-160.md)给出真实PG16和loopback HTTP命令。全仓go build、26组验收、archguard、冻结源一致性、设计、技能、notes及diff检查作为本任务门禁。验收包含同连接/同txid、根回滚、Recoverable确定性隔离及致命传播、双连接CAS、scope历史证明、非阻塞并发claim、重放撤权、TTL、JSON/错误/启动故障。

API/DDL冻结源未修改；发现后续操作不一致记入[INK-178](2026-09-18-contract-differences-ink-178.md)，不在本实现中自行裁定。未验收账号、领域业务、浏览器、模型、生产切换或恢复容量。
