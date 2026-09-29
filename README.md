# InkLore 墨境

模块化单体AI小说创作平台。当前工作阶段是 **R1完整重构设计已定稿、业务实现待按契约重构**。

设计唯一入口：[docs/system-design/00-INDEX.md](docs/system-design/00-INDEX.md)。先读 [00全局契约](docs/system-design/00-CONTRACT-BASELINE.md)，再读领域功能、SQL、OpenAPI和双端UI分册。不要把旧代码行为当作新接口标准。

| 入口 | 内容 |
| --- | --- |
| [AGENTS](AGENTS.md) | 工程红线与协作规则 |
| [OpenAPI](docs/system-design/api/openapi.json) | 类型化接口、权限、幂等、错误 |
| [DDL](docs/system-design/data/README.md) | 七域与平台表、约束和执行顺序 |
| [创作者端](docs/system-design/15-CREATOR-FUNCTION-UX-SPEC.md) / [管理端](docs/system-design/16-ADMIN-FUNCTION-UX-SPEC.md) | 页面字段、状态、操作、异常恢复 |
| [复用设计](docs/system-design/20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md) | 组件与控制器输入输出、缓存、状态所有权 |
| [实施与验收](docs/system-design/18-DELIVERY-AND-ACCEPTANCE-PLAN.md) | 分阶段任务与端到端门禁 |
| [看板](docs/backlog.md) / [实现事实](docs/architecture.md) | 实际进度与证据，不是第二套设计 |
| [决策](docs/decisions/README.md) / [开发维护](docs/development-guide.md) | 取舍依据、Skills和工程记忆 |

文档检查：

```powershell
node docs/system-design/tools/build-contracts.cjs
node docs/system-design/tools/check-design.cjs
go build ./...
```

生成OpenAPI入口和目录只编译设计资产，不启动服务、不执行数据库迁移。真实PG/Redis/模型及浏览器验收按18逐项执行。开发目标是两个独立应用Creator3000与Admin3001、API8080；当前代码是否达标以实际验收记录为准。
