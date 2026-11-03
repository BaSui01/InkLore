# Agent Note: INK-206 / INK-211 首页与自动登录验收

Status: implemented

Task: INK-206
Scope: 原 docs/acceptance-INK-206.md 的历史材料迁移与链接维护

## Problem

阶段性交付证明、契约差异、外部 UI 调研等历史记录混放在 docs 读者文档目录，容易让读者误认成最新有效的规范或功能事实。

## Decision

将原始材料迁入工程记忆，保留其来源、历史约束、验证记录和未完成边界。这是记录整理，不代表原记录涉及的能力已在当前云端完成验收。

## Alternatives considered

继续置于 docs 可维持旧入口但扩大文档噪声；删除历史材料会丢失决策依据与复现信息。因此保留其原有正文并收敛为 notes 记录。

## Consequences

原路径不再作为权威入口；引用必须更新。外部研究中的 UI 功能分析仅作为参考，不对第三方功能或当前 InkLore 的实现状态背书。

## Verification

保留源文档文本，按工程笔记结构调整标题和相对链接；禁用的表意图符号按仓库笔记规范去除。本轮没有重跑源记录中的历史命令。

### 历史原始资料

### INK-206 / INK-211 首页与自动登录验收

日期：2026-09-20。任务状态以 [backlog](../../../../docs/backlog.md) 为准。

### 交付范围

参考用户指定马良首页的文学排版、浅纸色、暖金色按钮、双栏首屏和轻卡片，制作 InkLore 创作者首页。页面使用自身品牌与原创纸页/钢笔矢量构图，没有复制参考站的图片、业务记录或统计。局部首页视觉按用户本次明确指令实现；不改变全局设计 token、管理端主题、HTTP 契约和领域边界，不引入依赖。

入口 `http://localhost:29000/#/home` 使用当前本地环境覆盖端口，保留 Creator 登录保护。默认双端端口配置、管理端和编辑器均未由本任务调整。首页从原侧栏/章节状态壳中独立呈现，书架查询、创建及进入指定作品都使用真实 Content API；没有模型选项占位、预填样例剧情、模拟生成或虚构统计。

### 运行路径

- CreatorApp → HomeRoute → HomeHeader / BookCard / CreateNovelDialog / HomeShowcase。
- useHomeBookshelf → TanStack Query → 共享 ApiClient + 当前 AuthSession → GET `/v1/content/novels?limit=12&status=active|archived&cursor=...`。
- 创建表单 → ApiClient POST `/v1/content/novels`，提交 title/summary/category/target_words；同载荷重试复用幂等键，双击使用同步锁；成功用返回 ID 调用既有编辑器。
- 列表按主体作用域和状态隔离。关键词仅搜索已加载数据，界面明确范围；字数明确为当前列表累计值，不能解释为全站或今日统计。
- INK-211：AuthSession.bootstrap 共用一个在途 Promise，内部 refresh 使用既有单飞机制，finally 释放以允许后续重试。原本 React StrictMode 双初始化导致 refresh 200/401、会话被重放保护吊销；修复后一次 session-csrf 与一次 refresh。
- 本批未修改 Go 包、服务端启动、数据库结构、领域装配或依赖；服务端依赖运行路径不受本批源码影响。

### 浏览器与接口证据

Chrome DevTools MCP 连接真实本地 Vite 与已运行的 29080 后端。数据库只读查询确认已有 creator 身份和 bcrypt 哈希；未尝试还原哈希，未修改原账号密码。以独立本地验收账号登录，全部记录都属于该验收主体。第一次初始化失败产生的验收账号仍保留；成功主体留有一部验收作品。密码只在注册调用内存生成，不写到源码、文档或浏览器持久存储。

| 操作 | 观察结果 |
| --- | --- |
| 首次空书架 | GET 200，展示专属空态，没有预置作品 |
| 输入灵感后打开创建弹窗 | 简介完整带入；焦点在书名；原生 modal dialog 阻止背景交互 |
| 空书名提交 | 浏览器 required 拒绝，焦点保留书名，无创建请求 |
| 填书名/题材/目标并提交 | POST 201；进入返回 ID 的编辑器；编辑器标题与创建作品一致，详情/卷/章节 GET 200 |
| 重新加载首页 | session-csrf 200、refresh 200 各一次；书架 GET 200；真实作品仍存在。StrictMode 首次中止的一次只读 GET 后正常重发，不再有重复刷新吊销 |
| 真实游标分页 | 通过 API 在验收主体新增 12 部明确标注的临时验收作品，第一页展示 12 张；点击加载更多后 13 张、13 个唯一标题，末页入口消失 |
| 指定作品打开 | 点击非首项“书架分页验收 01”，编辑器展示对应标题，没有跳到第一部 |
| 分页验收清理 | 12 部分页记录均经真实 DELETE API 软删除；保留最初用于截图的一部验收作品 |
| 搜索未匹配 | 展示搜索专属空态；清除后恢复真实卡片；没有把过滤结果当全库搜索 |
| 刷新故障 | 临时仅对当前标签页 GET 书架注入 HTTP 500，旧作品卡片保留，展示 stale/错误码/重试；恢复原生 fetch 后刷新成功 |
| 首次读取故障 | 在同样 500 注入下重新挂载首页，显示 error，既不显示空书架也不显示虚假 0 统计；恢复后可重试 |
| 离线 | DevTools Offline 下 navigator.onLine=false；离线横条显示，创建和刷新禁用；恢复网络后回到可用态 |
| 中文浅色桌面 | 1440×1000，完整首屏、书架、功能区可见；锚点避开固定导航 |
| 中文手机 | 390×844 和 320×740；document/home scrollWidth 等于视口宽度，无整页横向溢出 |
| 英文深色手机 | 320×740；真实切换语言和主题，无裸露 home.* key、无横向溢出 |

故障注入只用于验证错误处理，没有给产品注入模拟成功响应；结束后恢复原生 fetch 与网络环境。未调用模型、不验证生成质量；未把现有编辑器、其他业务页面或完整 R1 功能算作本轮验收。屏幕阅读器、真机软键盘和全站 WCAG 审计未执行。

### 截图

- [桌面首屏](../../../../docs/evidence/INK-206/home-desktop.png)
- [真实书架](../../../../docs/evidence/INK-206/bookshelf.png)
- [功能与指南](../../../../docs/evidence/INK-206/features.png)
- [320 手机](../../../../docs/evidence/INK-206/home-mobile-320.png)
- [英文深色手机](../../../../docs/evidence/INK-206/home-mobile-dark-en.png)
- [刷新失败保留作品](../../../../docs/evidence/INK-206/refresh-error.png)

截图由 Chrome DevTools MCP 获取，再将工具返回的图像字节存入本工作区；未通过截屏修改业务数据。

### 命令证据

- `go build ./...`：退出 0。
- `npm --prefix frontend run build`：Creator/Admin 两个生产构建均退出 0。
- `node scripts/agent-notes.mjs check`、`node scripts/check-project-skills.mjs`：通过。
- 本批目标文件 `git diff --check`：通过。未新增或修改测试文件。
- 新首页 TSX 的静态文案都使用 t；本批文件 Emoji 扫描无命中，单文件小于 50×1024 bytes。
- `go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts`：退出 1，旧解析器误把导出标识 zhCN/enUS 当成词条，已独立登记 INK-212，不改检查器掩盖结果。
- 补充验证：用当前 TypeScript transpileModule 将 locales 转为临时 CommonJS 模块，读取实际组合后的 zhCN/enUS，1222 对 1222 keys、missing=[]、extra=[]；新首页 117 个字面引用全部有双语词条。临时目录完成后删除。词条机械对齐通过不等于全文翻译质量审计。

INK-206 保持 review：页面与本轮行为验收已交付，原 i18n 检查器误报仍由 INK-212 独立处理。INK-211 登录修复具有真实浏览器复现与恢复证据。
