# Agent Note: R1身份吊销提交与跨租户成员锁序

Status: implemented

Task: INK-161
Scope: identity、membership、audit、cursor与R1 HTTP/部署bootstrap的无冲突部分

## Problem

已检索[平台原子命令笔记](2026-09-18-platform-atomic-command-boundary.md)，沿用[ADR-0016](../../reference/decisions/0016-reconstruction-design-freeze.md)的同PG事务和[ADR-0017](../../reference/decisions/0017-preserve-decision-records-and-single-design-entry.md)的事实入口。旧会话与旧路由不满足双aud、refresh重放吊销、当前成员权限、同租户成员管理和隐私审计要求。

refresh已使用token必须401，但把该错误直接返回Within会回滚吊销。两个租户operator互为目标用户时，只统一tenant优先仍不足：每个事务先锁自己账号再锁对方账号会形成环。

## Decision

identity只信HS256签名中的定位信息，访问时重读PG用户、成员与会话；JWT角色不作为授权权威。refresh在根UoW锁身份和token，重放先吊销并写审计，callback返回nil提交后才返回公开401；审计或PG故障仍回滚并返回真实失败。

成员目标写先锁tenant，再按UUID排序锁actor与target用户，再做实时身份授权与成员CAS。幂等回调首先使用AuthorizeMember，不能先普通identity.Authorize占actor共享锁。普通身份链按tenant→user→membership→session→refresh；非锁token定位只用来获锁，获锁后重验当前值。

session-csrf的正常同源GET常没有Origin：只在Fetch Metadata为same-origin且Host或浏览器Referer精确绑定所选aud Origin时允许恢复；已提供错误Origin绝不走此特例。refresh/logout仍要求Origin与CSRF。ws-ticket使用冻结audience DTO并核对JWT audience；无Cookie重复logout保持幂等。

审计只收有限字段和值，不序列化身份/密码请求对象，调用方在同UoW映射已验证tenant/actor并写入。游标签名绑定主体、tenant、aud、权限revision、操作、筛选和排序，24小时过期。

## Alternatives considered

- 重放时直接返回401给事务回调：会撤销安全吊销，未采用。
- 只锁tenant、actor优先锁user：跨tenant用户可共享，存在互锁，改为排序锁定两用户。
- 所有恢复GET都要求Origin：阻断正常同源代理页面刷新，已用真实HTTP反例确认并修正。
- 缺Origin即放行，或只信任Referer：不能保持aud与站点边界，未采用；Fetch Metadata约束同时包住Host和Referer分支。
- 审计接受任意map再靠调用方脱敏：容易混入密码/正文，改为封闭字段和值白名单。
- 改冻结文件补缺失错误码：用户明确选择保持冻结，故接口暂缓验收，未自行勘误。

## Consequences

bcrypt12在幂等外层UoW内仍可能延长持锁时间；未暴露可信hash字段给HTTP。Redis故障只能以每进程严格滑窗兜底，跨实例聚合限流在Redis故障时没有全局精确计数保证。已登录会话不依赖Redis读取。

同源恢复要求代理保留可信Host或浏览器Referer与Fetch Metadata；前端应继续只在内存存access/CSRF。物理会话/审计清理、生产TLS以及浏览器UI不因后端HTTP验证而完成。审计白名单后续域接入要显式扩展，不允许任意新字段直通。

## Verification

[INK-161证据](../testing/2026-09-18-acceptance-ink-161.md)：全仓构建、真实PG16/Redis7/loopback HTTP的17组场景、bootstrap CLI和INK-160回归。覆盖token轮换/重放提交、双aud、CSRF恢复、成员权限与跨租户重置、并发最后operator、跨租户排序锁、refresh/disable竞争、审计隐私与游标绑定、Redis故障兜底。

本轮补充：沿冻结基线既定jsonschema/v6引入v6.0.3，验收命令预编译完整响应并拒绝网络引用；19组验收覆盖13已挂操作成功响应，另4组真实代理/Cookie容器HTTP通过。封闭对象反例验证校验器确能拒错，不把冻结成功顶层开放属性误报为关闭。初次代理脚本失败因PowerShell保留跨端Origin，清理每次自定义头后通过，没有放宽后端授权。浏览器fixture只在独立验收二进制，随机密码留服务端、令牌只浏览器内存、诊断只输出布尔；实际Chrome连接器不可用，未运行真实浏览器。

没有新增禁用测试文件；未声称真实浏览器或生产TLS通过。完整任务状态只认backlog，待裁决接口见[INK-178](2026-09-18-contract-differences-ink-178.md)。
