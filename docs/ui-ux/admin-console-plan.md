# Admin Console · 管理端体验优化蓝图

> INK-259 · 规划稿，不修改后端协议。对应正式目标：[16 管理端](../system-design/16-ADMIN-FUNCTION-UX-SPEC.md)；通用视觉与状态：[17](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)、[20](../system-design/20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)。

## 1. 实际页面与目标覆盖

当前应用位于 [frontend/apps/admin](../../frontend/apps/admin)，路由由 [navigation.ts](../../frontend/apps/admin/src/navigation.ts) 和 [App.tsx](../../frontend/apps/admin/src/App.tsx) 直接分派。以下“已有”只代表有页面代码，不意味着对应 API、角色和真实场景已经全部验收。

| 现有路径 / 组件 | 当前用途 | 对应冻结功能 | 本轮设计重点 |
| --- | --- | --- | --- |
| /dashboard · LlmDashboardRoute | LLM 调度概览 | A01 | 真实采样范围、数据新鲜度、容量未知、分区失败 |
| /channels · ChannelsRoute | 渠道和关联 Key 管理 | A03/A04，关联 A02 | 列表→详情→版本化编辑；凭据引用脱敏 |
| /models · ModelConfigRoute | 模型和能力管理 | A05，关联 A07 | 能力标签、价格版本与预算交互分离 |
| /users · UserManagementRoute | 本租户成员和规则 | A14，关联 A07 | 权限边界、敏感动作确认及失败回填 |
| /agents · AgentGovernanceRoute | 定义、工作流、内置工具 | A08–A10 | 定义/工作流分组；内置工具只读 |
| /audit · AuditTrailRoute | 安全审计 | A15 | 筛选、北京时间、脱敏详情、诊断复制 |
| /project-logs · ProjectLogsRoute | 安全项目日志 | INK-241 实施增量 | 只读日志查询、真实错误码与 request_id |

A02 独立供应商、A06 策略、A11 MCP、A12 运行审计、A13 审批审计、A16 系统观测、A17 质检规则等是正式目标功能，但当前没有对应独立路由；不能仅凭界面蓝图声称已完成。优先复用现有已连接能力，后续按 [backlog](../backlog.md) 的依赖和证据独立实现。

## 2. 目标体验：更快判断“现在是否能运行”

管理端是决策工作台而不是卡片陈列馆。用户打开概览应先看到：当前租户、采样时间、真实渠道/预算/用量概况、需要人工处理的失败；再通过一条明确的导航进入渠道或审计。没有后台指标时要显示“未知/无法加载”，不能造绿色健康灯或成功率。

### 概览屏信息布局（1440×1024 结构线框，非像素级视觉稿）

~~~text
┌────────────────┬───────────────────────────────────────────────────────┐
│ 墨境 · 管理     │ 当前页面 / 模型概览     当前租户 · 当前身份 · 刷新     │
│                ├───────────────────────────────────────────────────────┤
│ 模型概览       │ 页面标题和范围：已加载数据 / 最近观测时间              │
│ 渠道与凭据     │                                                       │
│ 模型配置       │ 预算摘要    渠道容量摘要    用量摘要（分别标明来源）   │
│ 成员管理       │                                                       │
│ Agent 治理     │ 渠道状态表（真实状态、采样时点、unknown / degraded）   │
│ 安全审计       │                                                       │
│ 项目日志       │ 用量列表 / 可访问数据表（仅当前已加载样本）           │
│                │                                                       │
│ 主题 / 语言    │ 局部错误、过期提醒、诊断标识与安全重试                 │
└────────────────┴───────────────────────────────────────────────────────┘
~~~

- 桌面主导航宽度、折叠值、边距和断点全部取自 [17 视觉规范](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)，不把以上文字图当固定 px 布局。
- 首屏只放最关键的可核对摘要，不堆叠“全部功能”卡片。主动作是“刷新”，次动作是进入渠道或查看用量。
- A01 数据以 /v1/llm/channels、/v1/llm/usage、/v1/llm/budget 为权威。展示 breaker_state、capacity_observed_at、inflight、remaining_rpm、spent/reserved/remaining 等已有字段；不存在独立 GetStatus 汇总时不拼造成功百分比。
- “已加载 N 条用量”与是否还有下一页必须明确。只对已返回数据画图，并附同权限表格；未获取全量不得标“总量”。
- 独立数据区各自 loading/error/retry；预算失败不令渠道表变成空数据。

## 3. 重点场景：渠道与凭据

| 步骤 | 用户可见行为 | 后端或保护措施 |
| --- | --- | --- |
| 进入渠道 | 搜索、筛选与分页；区分启用配置和实时运行状态 | 读取 /v1/llm/channels；显示容量观察时间 |
| 选择一行 | 右侧详情区域展示配置版本和可见字段 | 不把 Key 或 Secret 原文送入 UI |
| 编辑渠道 | 使用现有协议与名称、权重、限额字段；保留 0 与空值差异 | 按 A03 Schema 校验，带 expected_revision + Idempotency-Key |
| 管理 Key | 在渠道范围内展开“引用列表/添加引用” | 只从服务端允许的 SecretRef 目录选择；无自由粘贴明文 Key 控件 |
| 保存冲突 | 409 显示本地草稿与最新修订，给重载及再次确认 | 禁止静默覆盖；保存失败保留输入 |
| 删除或停用 | 危险动作解释影响、确认后执行；失败保留行 | 服务端引用约束和当前权限为最终裁决 |

目前 [ChannelsRoute](../../frontend/apps/admin/src/routes/ChannelsRoute.tsx) 已存在渠道与 Key CRUD 视图。优化应先提取可复用的 FilterBar、状态行、详情抽屉和敏感操作确认，而不是重做资源 API。凭据健康字段只反映服务端回传结果；不能做不存在的“单渠道测试”。

## 4. 重点场景：智能体治理与运行风险

- 定义和工作流使用同一资源管理布局，但各自有独立保存、版本和草稿状态。工作流图应有键盘等价节点表；不把画布当唯一编辑方式。
- 代码内置工具页严格**只读**，仅使用 /v1/agents/builtin-tools；这是 INK-247 的现行限制。不可恢复过时的在线创建/删除/启停工具入口，也不能以配置存在宣称可执行。
- 对运行审计仅使用脱敏 Admin 能力，不借 Creator 的 artifact、events、approvals 或取消接口。Admin 无权替作者回答澄清、批准工具副作用或查看正文。
- 审计与项目日志分区展示 code、request_id、trace_id 与安全变化摘要；不复制原始 Query、正文、完整 prompt、Secret、Cookie 或参数。
- 对 operator 展示授权操作，对 auditor 显示只读内容；前端隐藏按钮不取代后端 403 和 tenant/scope 保护。

## 5. 视觉体系与响应式

Admin 采用 [17 的 Admin 语义主题](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)：冷灰 canvas、克制的 surface、可辨识的状态颜色、2px 焦点环。列表默认 48px 行高、40px 控件高，抽屉按既定 480px 或 640px 适配；小屏优先“对象/状态/操作”关键列，次要元数据进详情，禁横向溢出整页。

管理端也必须完整实现 loading、empty、error、offline、stale、partial、forbidden、conflict。操作等待时只禁对应写入口，不冻结无关可读区；空数据仅在真正成功返回空集合时出现。

## 6. 建议拆分与实际验收

1. **Admin-01 概览真实数据层级**：验证渠道采样过期、Redis 容量未知、预算失败、用量部分分页、auditor 只读；审查 WCAG2.2 AA 与窄屏。
2. **Admin-02 渠道/Key 编辑体验**：验证 409、引用受限、Key 不泄漏、启停和删除失败后保留数据、审计关联；不触碰真实密钥。
3. **Admin-03 Agent/审计一致性**：验证只读内置工具、Definition/Workflow 版本状态、事件安全摘要、时间与 request_id 复制；不增加管理端代答/代批。
4. **Admin-04 按契约补齐剩余页面**：每个单独排期并取得 OpenAPI/权限与浏览器证据，不能用导航占位冒充完成。

后续实现 PR 必须写清具体页面→operationId→权限→Owner→验收案例，并区分静态构建成功与真实 PG/Redis/API/浏览器链路成功。