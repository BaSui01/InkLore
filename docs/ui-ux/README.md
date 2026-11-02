# INK-259 · 双端 UI/UX 优化实施包

> 状态：设计提案 / review；基于 dev 2026-11-03 快照 185d0a6。本文档是现有代码的界面优化实施建议，不是第二套冻结契约；没有声称高保真图片、浏览器截图、响应式交互原型或前后端验收已经完成。

## 1. 目标与边界

InkLore 同时提供独立的 Creator Studio（3000）和 Admin Console（3001）。优化目标不是合并入口或更换业务模型，而是让作者更容易完成写作、保存、审阅，让管理员更安全地判断模型容量、配置渠道、管理运行风险。两端只共享基础控件和语义 Token；页面、鉴权 audience、导航、应用状态和构建产物继续隔离。

本包面向 1440×1024 桌面基准设计，并按照现有统一断点完成 320 CSS px 至宽屏的重排；提供深色与浅色语义主题。前端显示语言来自国际化词条，不引入 UI 写死中文、业务静态假数据、虚构进度或原本不存在的后端接口。

## 2. 权威资料与使用顺序

1. [工程规则](../../AGENTS.md)及[唯一设计入口](../system-design/00-INDEX.md)。
2. [全局契约基线](../system-design/00-CONTRACT-BASELINE.md)与[OpenAPI 操作目录](../system-design/api/README.md)。
3. [创作者功能 UI 合同](../system-design/15-CREATOR-FUNCTION-UX-SPEC.md)、[管理端功能 UI 合同](../system-design/16-ADMIN-FUNCTION-UX-SPEC.md)。
4. [视觉与无障碍合同](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)、[复用和状态合同](../system-design/20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)。
5. [运行代码事实](../architecture.md)、[任务及验收状态](../backlog.md)。

存在冲突时以上方冻结契约和代码实证为准。本包的版式草案、信息层级与实施优先级属于非冻结设计提案；不能以“UI 优化”为由改写 15/16/17/20、OpenAPI、权限或实时协议。拟改变业务契约时先提交独立差异与正式裁决。

## 3. 分册与交付清单

| 文档 | 可操作产出 | 尚未代表 |
| --- | --- | --- |
| [管理端页面蓝图](admin-console-plan.md) | 已有导航盘点、概览/渠道/智能体/审计交互和逐步改进方案 | A01–A17 全部实现 |
| [创作者端页面蓝图](creator-studio-plan.md) | 书架、写作三栏、版本恢复、AI 建议采纳与创作链路 | C01–C19 全部实现 |
| [组件与验收清单](component-system-and-acceptance.md) | 源码复用映射、主题/响应式/安全/无障碍规范、PR 分批清单 | 已运行浏览器/CI 验收 |
| [视觉稿制作交接](visual-handoff.md) | 后续高保真图片及原型的明确输入、输出、审核标准 | 已生成、已上传或已确认设计图片 |

## 4. 现状快照

- Admin 已有路由：/dashboard、/channels、/models、/users、/agents、/audit、/project-logs；由 [admin/navigation.ts](../../frontend/apps/admin/src/navigation.ts) 和 [admin/App.tsx](../../frontend/apps/admin/src/App.tsx) 直接确认。
- Creator 已有路由：首页 /、/editor、/agent-guide、/outline、/worldview、/analysis，以及鉴权路由；由 [creator/navigation.ts](../../frontend/apps/creator/src/navigation.ts) 和 [creator/App.tsx](../../frontend/apps/creator/src/App.tsx) 直接确认。
- 已存在工作区包：contracts、transport、recovery、i18n、ui、run-ui；基于 [frontend/packages](../../frontend/packages) 的实际代码。现有组件不等于 20 号分册所有目标组件都已落地。
- 当前页面采用自定义 History API 路由；冻结目标采用 React Router 7。迁移需要单独验证 deep link、权限、returnTo 与刷新恢复，不可在本设计 PR 中假称已完成。
- AGENTS 的禁止规则继续有效：无 Emoji UI、无第二套框架、无固定样本业务数据、无未验收的数据库写入/工具执行入口。

## 5. 实施顺序

第一阶段：设计和证据对齐。对两端各选一个主场景，先完成桌面关键页面与其 loading/empty/error/offline/stale/partial/forbidden/conflict 的表现，核对真实字段和权限；在正式修改前提供高保真截图并让使用者选择视觉方向。

第二阶段：共享控件与双端应用壳。只在 frontend/packages/ui 中实现基础语义组件，保持各 app 独立的壳、菜单和认证状态；每个 PR 仅覆盖可独立验收的一组组件。

第三阶段：主路径改造。优先 Creator 首页→开书→写作→保存/恢复→审阅，以及 Admin 概览→渠道→Key 引用/模型→审计。其他页面按依赖逐批实施，不一次性重写全部路由。

第四阶段：逐角色、逐断点进行浏览器、键盘、中文输入法、真实 API、租户隔离和故障恢复验收。安全与权限验收通过后，才能推进非草稿实现 PR；与仓库的分支保护及人工审查规则保持一致。

## 6. 对当前 PR 的完成声明

本 PR 只增加 UI/UX 规划文档、文档导航及 backlog/changelog 记录；不修改 React、Go、SQL、冻结设计分册、Actions 或任何生产资源。真实高保真图片和交互原型属于后续单独交付；不得把本 PR 标记为“前端优化已上线”。