# 按任务读取资料

先读当前用户要求和 AGENTS，再定位真实源码。下表提供导航；历史材料只帮助理解原因，不自动成为现行约束。

| 需要确认什么 | 入口 | 使用方式 |
| --- | --- | --- |
| 当前工程要求 | [AGENTS](../../../../AGENTS.md)、[开发维护指南](../../../../docs/development-guide.md) | 核对当前授权、边界与验证方式 |
| 当前接线与模块职责 | [architecture](../../../../docs/architecture.md)、[组合根](../../../../internal/app/) | 沿实际调用链读取，文档与源码冲突时说明差异 |
| 当前任务和依赖 | [GitHub Issues](https://github.com/BaSui01/InkLore/issues)、[Milestones](https://github.com/BaSui01/InkLore/milestones)、[PR](https://github.com/BaSui01/InkLore/pulls) | 复用已有工作，任务状态不再维护多份副本 |
| API、错误与事件 | [机器契约入口](../../../../docs/system-design/00-INDEX.md)、[api](../../../../docs/system-design/api/) | 同时核对实际路由、解码、授权、调用方与生成资产 |
| 数据与升级 | [DDL 参考](../../../../docs/system-design/data/)、[迁移](../../../../migrations/) | 迁移与真实数据库结果证明升级行为，不改写已执行历史 |
| Creator/Admin 与共享能力 | [双端源码](../../../../frontend/apps/)、[共享包](../../../../frontend/packages/) | 查真实 API、状态控制器、错误恢复与权限 |
| 本地运行和部署准备 | [本地环境](../../../../docs/local-environment.md)、[仓库治理](../../../../docs/repository-governance.md)、[部署配置](../../../../deploy/) | 读取当前命令与目标环境；操作仍需相应授权 |
| 工程因果与否决方案 | [notes 规则](../../../notes/README.md)、[历史 ADR](../../../notes/reference/decisions/README.md) | 按机制检索；核实历史假设是否仍成立 |
| 旧规划与验收 | [旧 backlog](../../../notes/reference/planning/backlog.md)、[旧架构记录](../../../notes/reference/implementation/architecture-history.md)、[历史设计](../../../notes/reference/system-design/) | 仅追溯，不作为当前进度或永久施工目标 |

不复制完整 API 清单、表结构、包数量或任务状态到技能。路径迁移时同步受影响的入站链接并执行技能校验；读取历史文件不等于采纳其规则。
