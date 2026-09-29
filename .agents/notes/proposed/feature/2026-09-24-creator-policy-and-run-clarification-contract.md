# Agent Note: Creator策略版本绑定与可恢复中途澄清

Status: proposed

Task: INK-227
Scope: docs/system-design/api、agent R1.x DDL及Run/SSE恢复契约

## Problem

Creator提交自由task_key使可见目录选择与Run绑定脱离；仅在agent_runs保存policy_version_id不能证明Worker使用该版本。approval.required只代表工具副作用授权，若把信息澄清塞入审批会让权限含义与恢复状态错误耦合。Run需能跨离页、重连与worker崩溃保存等待问题、唯一回答与恢复checkpoint。

## Proposal

依据Accepted ADR-0027，以Agent消费方Port读取LLM安全策略投影，Creator提交不可变policy version并由服务端派生task_key，Worker每次Gateway调用消费冻结版本。新增PG持久Question、waiting_clarification及question.required/question.answered，问题/Run/Node/job/outbox原子变更；Answer按本人scope、source binding、Schema、Idempotency-Key与CAS恢复同Run。设计交付和实现验收任务分离。

## Alternatives considered

直接暴露Admin策略API减少重复查询层，但泄露路由/模型治理信息且越过audience边界。只用task_key重查当前头不增加调用字段，却会让历史Run漂移，所以不采用。仅发SSE不存Question避免DDL，但断流/退出后不能可靠恢复，也不能并发消费答案。创建新Run续问实现直观，却可能重新执行已完成副作用并丢失checkpoint，故继续原Run。

## Acceptance criteria

创建Run并发策略更新只能绑定已核验当前版本或完全不创建；Worker真实模型调用使用冻结policy version。断开Creator连接并重开后可GET Question/Run与Last-Event-ID恢复；同题并发只一答成功、幂等重放不二次唤醒；权限撤回/取消/超时/source变更CAS收敛且持久事件原子提交。回答不覆盖正文/事实，未知副作用不重发。当前设计检查不代替PG/HTTP/Worker/UI验收。

## Risks

安全Schema子集必须在服务端严格拒绝远程引用、权限/配置字段和危险结构；题面/答案仍是用户不可信输入。题目等待计入30分钟Run期限，撤权后不允许本人恢复。向前迁移不支持删除Question/事件的down，应用回退须兼容扩展schema。旧task_key兼容仅限单一滚动发布窗口，需由后续实现节点明确sunset证据。
