# Agent Note: 创作者端核心创作链贯通与硬编码消除

Status: implemented

Task: INK-222
Scope: frontend/apps/creator/src/routes/ (OutlineRoute, WorldviewRoute, AnalysisRoute, EditorRoute, ConfirmChapterModal) 与 transport/contracts SDK

## Problem

创作者端 OutlineRoute、WorldviewRoute 与 AnalysisRoute 此前长期遗留写死的韩立、绝境深渊、落云宗图谱及40章模拟采样曲线等静态假数据，严重违背了 AGENTS.md 核心架构红线第6条（零硬编码铁律）；且 EditorRoute 中仅有正文保存，缺少小说创作最关键的“确认本章正文并跨域提交世界观事实账本 (ConfirmChapter)”的前端闭环。

## Decision

彻底消除这三个页面的全部静态硬编码数组，全面对接后端真实 REST API：
1. OutlineRoute：基于当前作品 ID 并行加载真实章节列表、大纲节点与细分场景（api.outline.listScenes / listNodes），支持在页面创建真实章节和大纲场景；空作品展示真实空态引导，支持点击跳转对应章节编辑器。
2. WorldviewRoute：基于真实角色（api.worldview.listCharacters）、世界观词条（api.worldview.listLore）与图谱拓扑数据（api.worldview.getGraph）动态构建 Cytoscape 力导向关系网络；支持创建新角色档案落库。
3. AnalysisRoute：接入 api.outline.getPacing 读取真实章节张力与节奏诊断数据，根据实际采样点动态绘制 SVG 曲线，无数据时展示优雅引导。
4. EditorRoute 与 ConfirmChapterModal：新增章节确认定稿弹窗，获取当前世界观修订版本号（api.worldview.getWorldRevision），在同一原子事务中调用 api.content.confirmChapter，将章节状态从 draft 置为 confirmed 并将事实写入账本。

## Alternatives considered

- 保留少量默认大纲卡片作为新手引导：违背零硬编码红线，容易导致测试数据与真实数据混淆，放弃。
- 章节确认仅发送客户端本地推算版本号：后端具有严格的 expected_revision 与 expected_world_revision CAS 校验，必须从权威接口获取并校验，放弃。

## Consequences

创作者端彻底消除了所有假数据，大纲、设定图谱与正文编辑正式形成数据互通闭环；全部手写代码均严格控制在 50KB 以内，保持零 Emoji 与纯净 i18n 规范。

## Verification

`npm --prefix frontend run build:creator` 与 `build:admin` 生产构建 100% 成功；`go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`（R1–R5 全部通过）、`go run ./cmd/i18ncheck`（945/945 完全一致）以及 `node scripts/agent-notes.mjs check` 校验通过。未新增或修改任何测试文件，未执行任何未授权 Git 命令。
