# Agent Note: content R1保存、目录与不可变历史证明

Status: implemented

Task: INK-163
Scope: content R1保存、目录与不可变历史证明

## Problem

目录操作可能只改position却递增章revision；把历史证明等同正文版本会缺失可验证的旧锚点。保存过程中先写正文后写版本，一旦后者失败必须全部回滚。 已检索既有平台原子边界与身份锁序笔记，保留其默认行为，本篇补充领域落地约束。

## Decision

全书scope锁稳定目录成员，正文/元数据版本/writing_event/计数/proof及事件均同UoW。每个章revision追加历史proof；每个不可变version另有revision=1的chapter_version投影。时钟在应用处UTC且截为PG微秒，避免PUT与GET展示不同持久时间。

## Alternatives considered

只在正文变化时记录proof会丢失重排历史；版本写失败后补偿正文无法保持原子。采用PG事务与独立proof语义。

## Consequences

同书写串行换取稳定完整集合；version属于哪个chapter必须由content权威读取证明，不能只用同书hash推断。

## Verification

主独立go run ./cmd/acceptance/ink163退出0，12组/27操作完整Schema；故障wrapper实际看到tx内正文已写，再证明9表指纹回滚。

详见 [任务验收](../../../../docs/acceptance-INK-163.md)。
