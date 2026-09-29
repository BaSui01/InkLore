# Agent Note: outline R1历史锚点、JCS与只读一致快照

Status: implemented

Task: INK-164
Scope: outline R1历史锚点、JCS与只读一致快照

## Problem

节奏聚合多个查询必须来自同一快照；普通事务内临时升级隔离级别无效。当前owner已证明可见后，错误书目标/伪造历史hash属于输入约束，不应混同未授权404。 已检索既有平台原子边界与身份锁序笔记，保留其默认行为，本篇补充领域落地约束。

## Decision

WithinReadSnapshot只在根启动REPEATABLE READ READ ONLY，身份/scope读在该上下文不加行锁；数据库拒写。普通Within默认仍READ COMMITTED；Recoverable继承模式。先完成可见性授权，再将已授权目标书不匹配和不存在revision/hash组合映射422。Node按冻结JCS计算语义hash并记录完整码点长度。

## Alternatives considered

持共享锁替代冻结明确要求的RR被放弃；任意404转422会泄露未授权资源，因此只转换已授权后的约束失败。

## Consequences

一致快照拒绝从普通外层事务升级；调用者必须在服务最外层选择边界。父deleted仍阻断子trash，tombstone proof可写不代表可读。

## Verification

6组platformextensions真实PG验证RR/强制只读/旧缓存重放/删除边界；outline独立10组与29操作完整Schema通过，含特殊Unicode独立JCS、锚点、故障回滚。

详见 [任务验收](../../../../docs/acceptance-INK-164.md)。
