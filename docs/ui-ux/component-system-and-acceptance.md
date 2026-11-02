# 共享组件、主题 Token 与验收矩阵

> INK-259 · 设计实施检查表。唯一数值与行为来源：[17](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md) 和 [20](../system-design/20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)。本文不新增设计令牌或另一个 UI 框架。

## 1. 当前工作区与演进原则

当前 [frontend/packages/ui](../../frontend/packages/ui/src) 已有 Button、Card、Badge、Input、Modal、ConfirmDialog、Pagination、DiffView、ErrorPresenter、PageGate、StatusBanner、ThemeProvider 和认证组件；[packages/run-ui](../../frontend/packages/run-ui/src) 有 RunInspector、ApprovalCard。transport、contracts、i18n 和 recovery 已作为独立共享包存在。上述目录存在不代表每个契约特性都实现完整。

正式 20 号分册中的 SchemaForm、DataTable、FilePicker、EditorController、EventTimeline、UsageSummary 等是需要按源码逐项核对的目标组件；没有实际源码/行为时不得标记“已复用”。

| 类型 | 放置边界 | 组件建议 | 不允许 |
| --- | --- | --- | --- |
| 基础控件 | packages/ui | Button、Field、Modal/Drawer、ConfirmDialog、StatusBanner、ErrorPresenter | 内含业务 API、后端权限判定或虚构数据 |
| 表格与选择 | packages/ui | Table/Pagination、FilterBar、EmptyState、Tree 基础行为 | 总数未知显示 0、全局假全选 |
| 运行可视化 | packages/run-ui | RunInspector、ApprovalCard、EventTimeline、DiffView 组合 | 请求 Creator 正文到 Admin、自动二次执行 Run |
| 作者工作流 | apps/creator | EditorController、RecoveryRepository、正文与结构域适配器 | 从 Creator 导入 Admin 组件或绕过 Content Owner |
| 资源治理 | apps/admin | ChannelForm、PolicyMatrix、AgentDefinition、AuditTable | 管理端读取作者草稿/完整 prompt/Secret |
| 外部边界 | contracts/transport/i18n | 生成类型、ApiClient、StreamClient、国际化 | UI 里拼猜字段、硬编码示例资源、跨租户缓存 |

项目冻结目标采用 React19、React Router7、TanStack Query5、Ajv8、antd6 和 Lucide；当前 package.json 及代码实现与冻结目标存在差距。增加依赖或替换组件框架须先核对冻结治理与 ADR，不因为设计图看起来更漂亮就擅自安装第二套组件库。

## 2. 主题约定

主题由 (appKind, mode) 选择，mode=light 或 dark，默认可跟随系统，保存的只是非敏感用户偏好。严格复用已冻结语义名：

- Creator light：bg.canvas #F6F3ED；bg.paper #FFFCF5。
- Admin light：bg.canvas #F5F7FA；bg.paper #FFFFFF。
- Creator dark：bg.canvas #171A20；bg.paper #20242C。
- Admin dark：bg.canvas #111827；bg.paper #1F2937。
- 两端语义前景、边框、主动作、focus、状态色按 [17 §2](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md) 表，不复制出新的平行 CSS Token。

| 度量 | 冻结要求 |
| --- | --- |
| spacing | 4 / 8 / 12 / 16 / 24 / 32 / 48 |
| radius | 4 / 8 / 12 |
| 导航 | 展开 224 / 折叠 64 |
| 控件与行 | 控件 40；表格行通常 48；可触及区域至少 44×44 |
| 弹窗与详情 | 确认 440、普通 640、比较 960；Drawer 通常 480 |
| 字体与正文 | UI 16 / 1.5，作者正文默认 18 / 1.8，正文可用宽 640–800 |
| 焦点 | 2px 轮廓及 2px offset，forced-colors 下保留系统可见焦点 |
| 降低动画 | reduced-motion 时动画 0；仅给必要状态提示 |

组件只读取语义 Token；不能散布固定 #hex 或用颜色独自表达是否启用、警告、增删变化。保留项目字体栈与中英文长度差异。

## 3. 唯一响应式矩阵

| 视口宽度 | 共通处理 | Creator | Admin |
| --- | --- | --- | --- |
| >= 1440 | 导航展开、边距 32 | 结构 + 纯文本正文 + 上下文检查器 | 保留关键表列、详情抽屉 |
| 1200–1439 | 导航折叠 64、边距 24 | 结构 + 正文，右侧浮层 | 次要列移到详情 |
| 768–1199 | 导航抽屉、边距 20 | 单主区，结构与助手互斥抽屉 | 过滤器换行、表单列数自适应 |
| 480–767 | 导航抽屉、边距 16 | 正文满宽、工具条换行 | 表格局部滚动或语义卡片 |
| 320–479 | 导航抽屉、边距 12 | 正文满宽、检查器全屏 | 一列表单、错误摘要常驻 |

遵守 17 的唯一断点，不新加 1280 / 960 特殊编辑器断点。触控与 400% 缩放需确保主操作可达且没有全页横向滚动。

## 4. 状态与 API 反馈模式

所有页面继承 15/16 所规定的 loading、empty、error、offline、stale、partial、forbidden、conflict；成功且 items 为空才可显示 empty。HTTP 401 按会话刷新/失效合同处理；合法但跨 scope 的资源按 404 不泄存在性；403 仅在权限可公开断言时显示。所有写操作需合规使用幂等与 expected_revision，服务端 409 不能自动强写覆盖。

表单规则：label/aria-describedby、Schema 校验、错误摘要和焦点跳转；无权/缺 Schema 时禁止提交，保存失败不清脏数据。网络错误展示安全 error code 与 request_id，复制的诊断字段受限；不要把原始请求体、prompt 或 Cookie 放入通知。

异步规则：Run 状态从后端读；SSE 只用于生成与 Agent 流，WS 只用于低频终态通知；网络重订阅不重复创建 run，不给屏幕阅读器逐 token 播报。章节保存反馈只基于服务端确认，不用前端计时器填充进度条。

## 5. 设计验收可复现检查

| 场景 | 观察证据 | 拒收条件 |
| --- | --- | --- |
| 角色隔离 | Creator 与 Admin 不同端口、不同 audience/菜单/会话；逐角色浏览器检查 | 管理端露出正文、作者端可进运维、仅隐藏按钮代替权限 |
| 数据真实性 | 比对页面已加载数/Revision/API 响应和错误态 | 空数据样本、假实时图、未加载值等于 0 |
| 保存可靠性 | 人工触发 409/500、网络断开、刷新恢复与快速切章 | 覆盖本地稿、跨章串稿、失败显示已保存 |
| 安全治理 | operator/auditor、跨租户、SecretRef/审计脱敏的真实接口结果 | 明文 Key、完整 prompt、管理端代批 |
| 无障碍 | 键盘巡航、ARIA、图表等价表、IME、屏幕阅读器、200/400% 缩放 | 仅颜色标状态、焦点丢失、不可访问的图操作 |
| 主题与国际化 | Creator/Admin × light/dark；zh-CN 与 en-US；WCAG2.2 AA | 裸文案、按钮裁切、强制中文时区外的日期口径 |
| 响应式 | 1440、1200、768、480、320 CSS px 截图与操作录制 | 页面水平溢出、底部操作条遮挡、触控目标不足 |
| 后端集成 | 操作 ID、tenant/scope、重试/幂等和冻结 revision | 只靠构建通过就宣称用户流程验收 |

按仓库规则不创建 *_test.go 或 *.test.ts。后续实现 PR 可用既有构建与架构门禁、真实浏览器、隔离 PG/Redis 手工可复现流程提供证据；未经执行不得填写“通过”。

## 6. PR 组织方式

- 文档 PR（本次）：只提交规划、证据边界和导航。
- 共享视觉组件 PR：独立修改 packages/ui 与 i18n，先核对依赖版本/冻结规定。
- Creator 核心路径 PR：书架和编辑器分别提交，保存及恢复需真实验收。
- Admin 核心路径 PR：概览与渠道/Key 分别提交；工具只读边界不得回退。
- 后续页面 PR：依据 15/16 和 backlog 单独登记 ID，再按实际契约补齐。

每个实现 PR 说明“变更前行为、来源操作、权威 Owner、错误和权限、桌面/手机截图、实测与未覆盖项”。不可为通过 CI 擅改冻结 API、仓库架构红线或安全门禁。