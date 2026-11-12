# Agent Note: 认证扩展不能绕过严格契约

Status: implemented

Task: INK-209
Scope: platform identity、app identity HTTP、认证源Schema

## Problem

Gin完整启动因未冻结认证路由缺失操作映射而失败。已有注册接受8字节密码，随后登录要求12字符；重置令牌提前删除使数据库事务失败无法重试。没有投递器却返回sent=true。

## Decision

按[ADR-0021](../../reference/decisions/0021-auth-extensions.md)保留用户明确要求的扩展。注册与重置复用ValidatePassword；令牌claim排除并发，事务成功才消费，失败释放claim。找回开发响应明确sent=false，非开发在没有投递器时503。

## Alternatives considered

删除扩展能继续保持原R1，但用户选择正式修改契约。跳过路由检查会保留双重输入规则，不采用。为邮件和令牌引入新持久化方案涉及额外供应商与迁移，本轮仅明确现有开发实现边界。

## Consequences

注册得到的密码满足登录规则，事务失败不丢失重试资格。内存令牌重启失效、跨实例不共享；生产邮件与多副本恢复尚未交付，不以本次契约纳入宣称完成。

## Verification

本轮代码修改后go build ./...通过。机读生成、冻结检查、11输入与5个Gin请求、build/vet/护栏/双端构建通过；事务行为仍在INK-209验收；当前dbx已连通系统PG17.11，但独立建库被MCP高风险SQL策略拒绝；不能复用此前摘要中的PG证据冒充本轮验证。
