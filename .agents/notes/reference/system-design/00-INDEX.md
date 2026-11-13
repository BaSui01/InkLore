> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# InkLore · 系统设计入口

**状态：持续维护的设计基线。初始设计任务为 INK-141。** 这是唯一目标设计入口。具体实现与验收状态以[任务看板](../planning/backlog.md)和[实施事实](../implementation/architecture-history.md)为准，不把旧实现限制、历史done或设计文档存在当作当前契约验收。分册之间无“临时兼容版”关系；字段、状态和权限必须统一。

先读[契约基线](00-CONTRACT-BASELINE.md)，再按角色读以下资产。全局选择及取舍记录在[ADR-0016](../decisions/0016-reconstruction-design-freeze.md)，以前ADR仅按其未替代范围适用，不另建决策库。现行入口不以发行代号命名；历史ADR文件名、迁移标识和验收编号保留追溯用途。

## 1. 开发阅读路径

| 角色/工作 | 必读 | 产出核对 |
| --- | --- | --- |
| 产品/功能 | [01需求](01-REQUIREMENTS-SPECIFICATION.md)、15/16页面 | FR-01..25全部可追踪，无假功能 |
| 后端领域 | [02架构](02-SYSTEM-ARCHITECTURE.md)、[03目录](03-DIRECTORY-STRUCTURE-AND-MODULARITY.md)、对应domain detail | Owner/Port/事务/状态/失败补偿 |
| API联调 | [API目录](../../../../docs/system-design/api/README.md)、[OpenAPI](../../../../docs/system-design/api/openapi.json)、[14错误](14-ERROR-HANDLING-AND-RECOVERY.md) | 请求响应Schema、权限、错误、幂等 |
| 数据库 | [11数据](11-DATA-PERSISTENCE-AND-API-SPEC.md)、[DDL目录](../../../../docs/system-design/data/README.md) | 租户FK/索引/唯一/CAS/清理 |
| 前端 | [15作者](15-CREATOR-FUNCTION-UX-SPEC.md)、[16管理](16-ADMIN-FUNCTION-UX-SPEC.md)、[17视觉](17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)、[20复用](20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md) | 页面→控制器→API→数据Owner→状态与验收 |
| 执行/运维 | [21实时与恢复](21-REALTIME-JOBS-AND-RECOVERY.md)、[13运维](13-DEVOPS-OBSERVABILITY-AND-OPERATIONS.md) | lease/fence/终态、预算、备份恢复 |
| 实施负责人 | [18实施验收](18-DELIVERY-AND-ACCEPTANCE-PLAN.md)、[12治理](12-ENGINEERING-GOVERNANCE.md)、[19清理](19-LEGACY-CLEANUP-AND-FREEZE.md) | 分阶段门禁、任务、证据、冻结变更 |

## 2. 七域

| 域 | 功能规格 | 实现细节 |
| --- | --- | --- |
| content | [03内容](03-DOMAIN-CONTENT-SPEC.md) | [服务/仓储/事务](domains/03-DOMAIN-CONTENT-DETAIL-SPEC.md) |
| outline | [04大纲](04-DOMAIN-OUTLINE-SPEC.md) | [服务/状态/查询](domains/04-DOMAIN-OUTLINE-DETAIL-SPEC.md) |
| worldview | [05世界观](05-DOMAIN-WORLDVIEW-SPEC.md) | [账本/投影/确认](domains/05-DOMAIN-WORLDVIEW-DETAIL-SPEC.md) |
| inspiration | [06拆书灵感](06-DOMAIN-INSPIRATION-SPEC.md) | [分析/文风/采纳](domains/06-DOMAIN-INSPIRATION-DETAIL-SPEC.md) |
| agent | [07执行工作流](07-DOMAIN-AGENT-WORKFLOW-SPEC.md) | [TAO/工具/审批](domains/07-DOMAIN-AGENT-DETAIL-SPEC.md) |
| compliance | [08质检](08-DOMAIN-COMPLIANCE-SPEC.md) | [规则/报告/净化](domains/08-DOMAIN-COMPLIANCE-DETAIL-SPEC.md) |
| llm | [09网关](09-DOMAIN-LLM-GATEWAY-SPEC.md) | [调度/预算/结构化](domains/09-DOMAIN-LLM-GATEWAY-DETAIL-SPEC.md) |

公共技术能力见[10平台](10-PLATFORM-AND-SHARED-SPEC.md)，依赖版本/命名见[02T](02-TECH-STACK-AND-NAMING-CONVENTIONS.md)。

## 3. 机器资产与唯一来源

- api/源JSON是唯一字段/操作定义；openapi.json和generated/由tools/build-contracts.cjs编译，不手改生成视图。所有生成引用保持源Schema，不flatten不同域同名类型。
- data/*.sql是唯一物理结构；旧Markdown DDL与旧docs/api已删除，不能从历史文件复制表或错误码。
- [需求映射](implementation/requirements-map.md)将FR关联operationId；[错误登记](../../../../docs/system-design/api/error-catalog.json)由x-errors生成，UI词条不得随意改code。
- 设计检查工具见tools/check-design.cjs。freeze-manifest.json记录文件SHA256用于漂移检测，不是第三方认证或历史防篡改封印。
- 运行路径与验收证据是实施事实，记录在docs/architecture.md与docs/backlog.md；它们不维护第二套目标架构。

## 5. 契约增量：Agent 策略绑定与中途澄清

用户于2026-09-24批准且仅批准 ADR-0027 两项：Creator安全任务策略目录与CreateRun不可变策略版本绑定；Agent中途澄清完整持久交互。具体API/DDL/SSE状态、错误、兼容窗口、迁移及验收均见 [ADR-0027](../decisions/0027-proposed-creator-task-policy-catalog.md)、[基线](00-CONTRACT-BASELINE.md)、[Agent工作流](07-DOMAIN-AGENT-WORKFLOW-SPEC.md)、[Agent详细设计](domains/07-DOMAIN-AGENT-DETAIL-SPEC.md)、[实施计划](18-DELIVERY-AND-ACCEPTANCE-PLAN.md)。改变仅限上述范围，不推翻本索引其他冻结决策；实现仍以 backlog 为准。
关键实现决策已具体到接口/SQL/Port/状态/组件；开发时不再自行发明新协议。若遇设计缺陷必须提交契约差异（原因、影响、兼容/迁移、验收）再同步全部契约。禁止为了“永不修改”而保留已知矛盾，亦禁止用“以后再说”掩盖缺失设计。TTS/协同/支付属于未来需求，不因去除发行标签而纳入当前范围。

## 6. 全局密码长度调整

ADR-0028按用户明确要求将新密码下限调整为6个Unicode字符；72 UTF-8字节上限、bcrypt cost 12、身份/租户/角色边界与无内置口令约束不变。同步契约见[基线](00-CONTRACT-BASELINE.md)、[Creator](15-CREATOR-FUNCTION-UX-SPEC.md)、[Admin](16-ADMIN-FUNCTION-UX-SPEC.md)及[platform API Schema](../../../../docs/system-design/api/platform-schemas.json)，运行验收以INK-232为准。

## 7. 管理员root登录名

ADR-0029为platform用户增加可空唯一username；管理员可用邮箱或用户名root登录，root只绑定operator账户并仍受租户成员授权限制。新增`000012_platform_user_root_username.sql`，由部署CLI绑定已有邮箱或在首次bootstrap时设置root；不创建跨租户super_admin、不内置密码。实现与迁移验收以INK-233为准。

## 8. Creator 用户名登录

ADR-0031扩展唯一保留名`root`，允许以用户名和密码进入Creator audience。登录仍须存在活动creator membership，Creator资源保留tenant与owner scope；用户名不授予Admin audience。注册、找回密码和邮箱管理仍使用邮箱。实现与运行验收以INK-235为准。

## 9. Agent 引导创建小说

ADR-0032新增Agent-owned持久化小说创建引导session：Agent使用租户版`novel_architect`与真实task policy，经固定`ask_user`工具逐轮提问并提供建议，最后提出可编辑作品元数据；只有作者明确确认才经Content API创建作品。创建前不伪造Novel资源，模型/渠道不可用时真实失败。实现与运行验收以INK-236为准。
