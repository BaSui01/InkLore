# Agent Note: R1.1 identity completion and bounded audit export

Status: implemented

Task: INK-161
Scope: identity password changes, audit export, app HTTP methods, acceptance

## Problem

INK-161原冻结源缺改密幂等头、导出422和405错误定义。无冲突部分完成后，用户明确批准四项最小勘误；正式边界见ADR-0018，笔记不授予额外契约修改权。

## Decision

改密幂等授权先按tenant共享锁、user更新锁、membership/session授权顺序；只查当前权限，不把旧密码或CAS当缓存重放前置。实际命令校验当前密码与数据库user revision，bcrypt更新、revision递增、跨tenant/aud全部会话吊销、审计与幂等记录在同一root UoW。旧会话不能重放，新会话可重放已完成结果，不再次吊销或清除新Cookie。成功首次仅清当前aud Cookie，避免清除另一端可能属于其他账号的Cookie。

导出一次查询最多10001条，检查31天与90天可见边界，全部safeAudit并编码完成才发送NDJSON文件头。超过10000明确422；不流式输出部分后才发现超限。405 Allow反映注册路由与全局中间件OPTIONS。

## Alternatives considered

- 继续冻结：已由用户具体批准最小修正替代，不可自行扩大批准范围。
- 普通共享授权后升级user锁：并发改密有升级死锁风险，采用专用预授权。
- 先检查旧密码再查幂等缓存：会阻断合法新会话对原命令的重放。
- 缓存JSON按字节比对：冻结存储是JSONB，会合法重排空白/字段；验证完整JSON语义并保持原request_id/trace_id。
- 查询前发送导出头：无法保证超限返回完整JSON错误。

## Consequences

密码hash计算在事务持锁中，吞吐成本明确；没有预哈希客户端接口。导出占用最多10000条的有界内存。没有产品前端或真实Chrome证据，后续UI与最终端到端验收仍须执行。

## Verification

最终INK-161真实PG/Redis/HTTP 24组通过，包括注入审计失败回滚密码、revision与全部session，两会话并发改密恰一200一401。15操作逐响应Schema、NDJSON10000正例/10001反例、32天拒绝、敏感字段不泄露；405 Allow与OPTIONS实测。INK-160 26组回归通过。原manifest已另存；仅8冻结源/派生资产变化，DDL不变，错误95→97。详docs/acceptance-INK-161.md。
