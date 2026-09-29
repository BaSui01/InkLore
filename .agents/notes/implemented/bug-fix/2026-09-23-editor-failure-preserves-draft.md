# Agent Note: 编辑器失败保留作者草稿与同键写入边界

Status: implemented

Task: INK-221
Scope: frontend/apps/creator/src/routes/EditorRoute.tsx、editorDraft.ts/editorRun.ts，transport SDK 与编辑器词条

## Problem

旧 EditorRoute 的五个读取请求分别 `.catch(() => []/null)`，故障会清空目录、误显示可创建的空章；GET 正文异常只写控制台，切章时迟到响应可盖掉别章。保存失败后仍显示“上次保存”，网络结果未知时下次重发会生成新的幂等键；返回数据缺 revision 还以本地 `+1` 假定成功。旧 AI 运行无论成功或失败都在定时器里插入伪文本。已核对 [INK-220笔记](2026-09-23-transport-safe-error-propagation.md) 与14/20分册，独立补足作者侧错误语义。

## Decision

五资源列表同时读取并逐项落实成功数据；失败仅记录结构化错误并保留旧快照，章节成功列表才可进入空态。章节 GET 以请求序号及所选 ID 屏蔽迟到回包，未核对正文 ID/修订前禁编辑保存。当前 dirty、在途写、未知响应时不切章或清空正文，页内返回阻止并提示，离页刷新交给 beforeunload；资源 403/404 不再渲染原正文，可下载仍在内存的本地草稿。

每个保存意图固定 `chapterId/body/expectedRevision/idempotencyKey/generation`。SDK 接收显式幂等键，只有同键同体可重放未知结果；并行保存单飞，200 回包的 ID、revision 与服务端 LF 正文不一致也保持未确认状态。用户在写入期间的新编辑归下一意图；400/422 实际更正后可新建操作，409 不自动覆盖。ErrorPresenter 显示持续失败/安全诊断，受理 AI run 不等于产物采纳；删除固定文案伪造正文。编辑错误双语词条单独切片，控制手写文件大小。

## Alternatives considered

- 保留 `.catch(() => [])` 并在页面外围添一次 toast：后续逻辑仍收到伪真空列表且可清稿，放弃。
- 保存失败自动按新 UUID 和当前草稿重写：响应丢失可能重复写/重复计费，版本冲突也不能自动覆盖，放弃。
- GET 失败后让 TipTap 显示前一章正文但把标题切到新章：会错误归属并可能将前一章写进新章，放弃。
- 自动塞入 AI 结果近似文案：没有真实 artifact 与作者确认，放弃。

## Consequences

异常不再假装成功，代价是保存冲突必须由作者核对。已彻底移除 TipTap 富文本框架及其 HTML 数据流，正文编辑收敛为原生多行文本框 PlainTextEditor，符合 00契约基线§2 与 03分册§2 的纯文本 LF 规范和 Unicode 码点字数统计；409 版本冲突时弹出 ConflictResolutionModal 基于 DiffView 进行三方核对，允许作者选择采用服务器版本或以本地草稿覆盖提交。

## Verification

`npm run build:creator && npm run build:admin`、`go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`（R1–R5 全部通过）、`go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts`（945/945 完全一致）、notes/skills 检查退出 0。EditorRoute.tsx 尺寸 42,269 字节，PlainTextEditor 5,079 字节，ConflictResolutionModal 3,938 字节，editorText 1,663 字节，全部严格小于 50KB 上限。本地 PostgreSQL 17 已应用全部 000001~000009 迁移；启动真实 cmd/server（端口 29080）探测 /healthz 与 /readyz 均返回 200 OK 并通过数据库就绪检查；真实 HTTP 注入验证 401、404、405（带 Allow: GET, OPTIONS）及 400 格式错误均返回标准 R1 错误封包（含 code/message_key/details/request_id/trace_id）；双端 Vite 启动后使用内嵌浏览器实际访问管理端登录页与创作者端首页并求值校验码点计算、LF 换行与安全诊断格式。未新增或修改任何测试文件，未执行任何 Git 操作。