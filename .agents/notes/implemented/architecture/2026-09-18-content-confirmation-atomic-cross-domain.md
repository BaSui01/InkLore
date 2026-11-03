# Agent Note: content确认与worldview事实的同事务边界

Status: implemented

Task: INK-166
Scope: internal/content/application/r1 confirmation、internal/app/adapters/r1_world_fact.go、internal/worldview/application/facts.go

## Problem

章节确认同时改变content章节状态和worldview账本。旧content域不能直接依赖worldview写模型；若只按novel/hash传来源，无法证明版本UUID属于指定章节；若把事实提交放在独立事务，章节与事实会出现半提交。

既有content验收覆盖27个正文/目录操作，确认是后续独立操作，不能把它混入原任务的覆盖断言。

## Decision

content定义自己的WorldFactPort和typed ConfirmationSource/Confirm/FactProposal值。app adapter在锁定content权威章节与版本后转换到worldview Source/CheckFacts，并在原UoW调用CommitFacts。worldview验证chapter历史revision/hash、chapter_version revision=1、novel/hash和live授权；version与chapter的最终对应由content权威读取保证。

确认顺序保持scope→novel→chapter→worldview；空提议仍递增world revision但不追加ledger event。worldview失败、audit/outbox失败、或任何content更新失败都会通过嵌套UoW rollback-only传播。

transport允许仅由schema声明的指针字段使用null；正文、revision和ID等非nullable字段仍严格拒绝缺失/null。

## Alternatives considered

直接在content导入worldview application写类型会突破业务域边界；只保存world revision再异步确认会留下章节已确认但事实未提交的窗口；先单独预检后写入会在并发world revision变化时产生过期许可；这些方案均未采用。

## Consequences

确认依赖app组合根的显式adapter，content不会独立启动成可确认状态；worldview仍无公开跨域写入口。确认操作作为INK-166单独覆盖，INK-163继续保持27操作范围。nullable presence规则变为指针类型的通用安全例外，需要未来新指针字段继续在Schema和typed结构中明确约束。

## Verification

主代理运行：

- go build ./...：退出0。
- go run ./cmd/acceptance/ink166：退出0，9组全部PASS，确认操作Schema覆盖1。
- go run ./cmd/acceptance/ink163：退出0，12组及27个原有操作通过。
- go run ./cmd/acceptance/ink164：退出0，10组及29操作通过。
- go run ./cmd/acceptance/ink165：退出0，16组及25操作通过。
- INK-160/161回归继续退出0。

详见 .agents/notes/implemented/testing/2026-09-18-acceptance-ink-166.md。
