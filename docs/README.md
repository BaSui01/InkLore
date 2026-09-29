# InkLore 文档入口

当前项目按 **R1完整设计重构**，先完成设计再实施。唯一施工入口是 [system-design/00-INDEX.md](system-design/00-INDEX.md)，旧产品分析、旧API与重复DDL文档已按清理清单移除。

- [冻结契约与范围](system-design/00-CONTRACT-BASELINE.md)：路由、ID、错误、并发、事务、权限、实时和复用规则。
- [机读API](system-design/api/openapi.json) / [API目录](system-design/api/README.md) / [真实DDL设计](system-design/data/README.md)。
- [作者功能UI/UX](system-design/15-CREATOR-FUNCTION-UX-SPEC.md)、[管理端UI/UX](system-design/16-ADMIN-FUNCTION-UX-SPEC.md)、[复用与状态](system-design/20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)。
- [实施计划](system-design/18-DELIVERY-AND-ACCEPTANCE-PLAN.md) / [文档清理](system-design/19-LEGACY-CLEANUP-AND-FREEZE.md) / [LLM供应商官方协议规范](llm-provider-specifications.md)。
- [任务看板](backlog.md)、[代码实施现状](architecture.md)、[决策理由](decisions/README.md)只记录执行事实与选择依据，不形成第二套目标设计。
- [开发维护指南](development-guide.md)编排仓库Skills；新任务必须先读R1基线，不沿旧包路径猜接口。
- [本地环境与数据库配置](local-environment.md)：记录开发机 PostgreSQL 17、Redis 7、统一随机密码与 DBX 客户端连接。
- [长期工程记忆](../.agents/notes/README.md)保存实现约束、真实备选和故障根因；[INK-143验收](notes-delivery-INK-143.md)记录笔记与归档门禁验证。

“冻结”表示开发不再临时发明核心契约；发现真实缺陷按R1.x变更流程统一修订。文档完成不等于重构代码完成。
