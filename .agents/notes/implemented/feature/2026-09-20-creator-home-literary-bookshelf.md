# Agent Note: 文学首页与真实书架的状态边界

Status: implemented

Task: INK-206
Scope: frontend/apps/creator/src/routes/HomeRoute.tsx、components/home、domains/content/useHomeBookshelf.ts 与共享首页词条

## Problem

用户要求参考马良写作完善自己的首页。原首页混用侧栏工作台壳、硬编码模型与预填剧情，生成按钮实际只创建元数据；列表异常被清空后与成功空书架无法区分。原 SDK 列表丢弃游标并有隐式登录，不适合作为新首页数据边界。

改前检索 notes 的首页/HomeRoute/书架/creator，没有既有首页设计决策；Vite 过期缓存笔记仅与开发服务相关，仍适用但不覆盖本次页面。设计依据为 [作者页](../../reference/system-design/15-CREATOR-FUNCTION-UX-SPEC.md)、[共享状态](../../reference/system-design/20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)，本次用户明确指定的参考视觉限定在首页。

## Decision

首页使用独立顶部导航与局部暖色变量，宋体品牌标题、纸页钢笔构图和作品卡片；不改共享 token 或另一端。首页文案通过双语词典提供。插画只表达写作意象，不表达运行进度、样例正文或虚构字数。

书架使用当前 AuthSession 构造共享 ApiClient，以主体和状态作为 Query key，保留 next_cursor；不调用 SDK 隐式登录方法。首页显示的是已加载数量、当前列表字数和北京时间更新时间。搜索明确为已加载数据。500 首次失败是 error，刷新失败保留旧快照，403/404 收起旧卡片；离线禁远端写入。

创意开书将用户输入带入原生 dialog，校验码点长度和目标整数，真实创建后用返回 ID 进入编辑器。原生 dialog 提供模态焦点行为；写命令用同步锁禁止双击，并为相同载荷保留幂等键。没有接线的模型选择与伪生成承诺从首页移除。

## Alternatives considered

直接复制参考站统计与示例书目可以快速补满视觉，但会把演示误作业务事实，违反真实数据要求，因此不采用。保留原工作台壳有较小改动量，但章节模式、保存状态与首页语境不符，无法满足用户要求的完整首页。

扩充现有 SDK 能统一方法入口，但会同时触及其隐式登录问题及其他调用者；本批复用已有 ApiClient，相关 SDK 缺陷单独登记 INK-207。公开官网首页会方便未登录浏览，但会改变当前身份入口，本批保留登录保护。

## Consequences

收益是视觉完整、空态与异常可区分、真实创建和书架连续；代价是首页持有独立 QueryProvider，离开页面会重新读取书架。加载字数不能当作全库统计；概念功能介绍不能代替那些页面的重新验收。

本地验收使用独立主体，避免操作用户原作品；留下一个截图用验收作品及验收账号，临时分页作品已走 API 软删除。生成质量、真机软键盘和全站无障碍仍不在本轮证据范围。

## Verification

[验收记录](../testing/2026-09-20-acceptance-ink-206.md)含 Chrome 截图、POST 201、刷新留存、13 条真实分页、指定作品进入、500 首次/刷新失败和离线恢复。go build 与双端生产构建通过，真实组合双语词典 key 一致。原 i18n 检查器的导出名误报登记 INK-212，任务状态保持 review；notes、skills 与定向 diff 校验通过。无新依赖、Go 接线或测试文件。
