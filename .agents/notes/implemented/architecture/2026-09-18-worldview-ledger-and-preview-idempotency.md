# Agent Note: worldview R1事实账本、预检与维护边界

Status: implemented

Task: INK-165
Scope: worldview R1事实账本、预检与维护边界

## Problem

预检既要RR一致性，也必须遵守幂等入口。若先在缓存外执行旧expected_revision预检，再查缓存，合法旧键会被后来的world修订错误阻断。只追加账本还要允许到期擦除私密payload。 已检索既有平台原子边界与身份锁序笔记，保留其默认行为，本篇补充领域落地约束。

## Decision

ExecuteReadConsistent从根启动RR，预检与技术幂等记录同一事务；不把物理只读模式用于需要写缓存的事务。world行总序锁后整批inspect，无部分写。盐+规范化payload digest与不可变events分离；维护明确要求当前授权callback，正常服务未配时失败关闭。维护动作列入显式安全审计名单，系统清理使用nil actor。

## Alternatives considered

缓存外预计算会破坏重放；逐条边检查边写会留下部分事实；直接改删ledger破坏可追溯性，均未采用。

## Consequences

正常写加载本书状态，最大容量性能留后续验证；外部不能直接构造可信确认来源，后续content适配核查version归属。新增公开错误码仍需独立裁决。

## Verification

主独立go run ./cmd/acceptance/ink165的16组/25操作通过，含独立digest、PG禁止event UPDATE/DELETE、并发及外层回滚、真实审计失败、维护重建。30天擦除补充证据见验收记录。

详见 [任务验收](../testing/2026-09-18-acceptance-ink-165.md)。
