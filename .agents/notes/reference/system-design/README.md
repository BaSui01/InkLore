# 历史系统设计

这里保留迁移前的设计、实施顺序、冻结清单和检查输出。

旧 R1 / R1.x、固定七域、固定任务顺序和设计哈希不再约束当前开发。
原文中的“唯一入口”“必须冻结”“仅正式 R1.x 变更”属于历史表述，不是当前指令。

现行入口为 [AGENTS](../../../../AGENTS.md) 与 [当前架构](../../../../docs/architecture.md)。
工具消费的 OpenAPI / DDL 仍位于 [docs/system-design](../../../../docs/system-design/00-INDEX.md)。

freeze-manifest.json 保留旧清单原貌，没有重算哈希掩盖漂移。
implementation 报告只代表原执行时刻，不是本次验收。
