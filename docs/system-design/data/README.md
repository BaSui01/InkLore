# R1数据设计入口

本目录SQL是目标DDL，未在本轮工作数据库执行。禁止把这些文件直接当无审查生产迁移；实施阶段生成版本化迁移并在隔离PG17实例验收。

| 顺序 | 文件 | Owner与核心聚合 |
| --- | --- | --- |
| 1 | [platform.sql](platform.sql) | 身份、scope/version证明、文件、幂等、队列、outbox、审计 |
| 2 | [content.sql](content.sql) | 作品、卷、章、版本、写作事件、导入导出job快照 |
| 3 | [outline.sql](outline.sql) | 大纲节点、节拍、场景、伏笔与事件 |
| 4 | [worldview.sql](worldview.sql) | 世界修订、实体、关系、只追加账本、可清除payload、heads |
| 5 | [inspiration.sql](inspiration.sql) | 来源/分段、分析/报告、文风版本、卡片/采纳 |
| 6 | [compliance.sql](compliance.sql) | 规则版本、规则、报告、问题/决定、审阅、净化 |
| 7 | [agent.sql](agent.sql) | 定义/工具/工作流版本、运行/节点/执行/审批/澄清问题/产物/事件；R1.x增量升级见000010 |
| 8 | [llm.sql](llm.sql) | Provider/模型/渠道/Key、策略/定价、预算/预留/usage |
| 9 | [permissions.sql](permissions.sql) | runtime与retention最小权限；最后执行 |

同域复合FK证明tenant与聚合绑定；跨业务域仅UUID，通过Port/授权投影验证。平台表可被域FK引用，但绝不允许应用代码跨查另一业务域私表。根身份与技术系统表例外见11。

迁移：R1基线迁移000001..000009；ADR-0027向前增量依次为`migrations/r1/000010_llm_policy_task_kinds.sql`与`migrations/r1/000011_agent_clarification.sql`；ADR-0029的`migrations/r1/000012_platform_user_root_username.sql`只加可空唯一username列，保留既有邮箱身份与数据。000010给旧不可变策略版本补空task_kinds（不推断历史映射）；000011增加Agent状态/event约束并创建Question表/索引，保留既有Run/event；downgrade不支持，应用回退需保留扩展schema，不能删除既有身份或Agent历史。

权限：业务删除先soft-delete，runtime无物理DELETE；retention专用身份按到期与tenant清理，账本event永不能DELETE，只清payload。只追加业务表runtime无UPDATE；触发器加第二层保护。恢复旧备份前应用删除tombstone，不能使被清除正文复活。

删除顺序：先授权tombstone和任务取消→擦除run/报告/来源中的复制正文→清引用产物与版本payload→清可删除子记录→删除已无引用文件。ledger事件保留安全机器证明。不能因FK阻挡就CASCADE删除整条审计链。所有purge写安全审计并可按tenant检查。

验收：空库DDL顺序成功、FK错租户/错run拒绝、版本写失败回滚、不可变表UPDATE/DELETE被拒、查询EXPLAIN走目标索引、清理仅到期scope。静态脚本只检查表Owner与引用存在，不能冒充上述PG执行结果。
