# Agent Note: 传输错误与诊断不再伪装成功

Status: implemented

Task: INK-220
Scope: frontend/apps/admin、frontend/packages/transport/ui/recovery 与既有 Go HTTP/SSE/WS 边界

## Problem

管理端十处 GET wrapper 以 catch console.warn 返回空列表、null 或空分页，使 500/403/网络失败看起来是业务成功空态；流解析和会话/页面 console 还曾输出原始异常或 SSE/WS 原帧。用户输入、密钥、正文可能被第三方异常消息夹带。对照 [Gin 入口安全笔记](../simplification/2026-09-20-gin-transport-security.md)，后端现行白名单日志与流后不写 JSON 的约束继续有效。

## Decision

GET wrapper 不再吞错，ApiClient 的结构化错误原样抛给调用页。五个管理读页面在成功时才替换其本地快照，失败保留旧数据/审计分页游标，以 ErrorPresenter 持久显示与重试。共享诊断只输出固定事件名、已校验格式的 code/status/request_id/trace_id，不序列化原始事件、异常、topic 或用户输入；没有 transport 依赖的恢复包只打固定事件名。ErrorPresenter 不使用不可信 Error.message。管理会话仅从原 Cookie 尝试恢复，不保留源码中的默认口令与自动登录；管理注册等无真实授权的假入口关闭。渠道模型/密钥步骤失败时停止后续写，禁止绑定其他模型 ID 冒充成功。

后端 HTTP Contract/Logger/Recovery/Response 是已装配共享基座，仍只在普通 JSON 路径和 SSE/WS 升级之前写 JSON；已建 SSE 与已升级 WS 不套统一 JSON。未新增公开错误码、表、依赖或更改 Accepted ADR。

## Alternatives considered

- 让管理 API 自行 warn 并返回空值：实现省事但让读取失败被 UI 当作可提交的权威空数据，放弃。
- 控制台仅遮盖已知字段但继续输出整个 Error 或 event.data：异常嵌套对象和上游文本难以穷尽脱敏，放弃。
- 让 SSE/WS 共用 HTTP 的 JSON 封包：升级后协议不同，不能补发 HTTP 状态，放弃。

## Consequences

可见失败不再吞掉数据，状态由页面负责；并行 GET 整组失败时保留整组旧快照，不声称部分 GET 已入页面（后续若按资源分别展示可采用 allSettled 和独立状态）。删除固定管理员凭据会要求真实操作员显式登录。Creator 编辑器仍有其旧有的多资源 `.catch(() => [])` 占位行为，不属于本次 admin wrapper 修复，须另行登记处理；管理端与 Creator 完整实网故障验收需可用身份/依赖服务。

## Verification

`npm run build:admin` 初次失败暴露未引用默认凭据与 JSX unknown 错误，修正后 `npm run build:creator && npm run build:admin` 均退出 0。`go build ./...`、`go vet ./...`、`go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts`（945/945）、`go run ./cmd/archguard -reachability`（R1–R5）及 notes/skills check 全部退出 0。隔离静态护栏核对 10 个管理 GET 无 catch 假空、5 页有 ErrorPresenter、SSE/WS 无原帧日志、应用 TS 文件不超 50KB、无默认管理员口令。没有执行真实登录态 500/403/断流故障注入，也不将编译当作该行为的运行时证据。没有执行任何 Git 操作，没有新增/修改测试文件。