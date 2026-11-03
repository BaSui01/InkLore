# Agent Note: 上游模型目录探测的最小权限与可信出口

Status: proposed

Task: INK-262
Scope: internal/llm/infrastructure/upstream_models.go、internal/llm/application/catalog.go、internal/llm/transport/http/handler.go、internal/platform/httpguard/httpguard.go

## Problem

CodeQL 在上游模型目录的四处 `http.Client.Do` 处报告 Critical `go/request-forgery`。原代码将调用者的 `base_url` 直接传给默认 HTTP Client，并允许默认重定向和环境 Proxy；且 `api_key` 若恰好符合大写环境变量格式，就会调用 `os.Getenv` 将实际服务端凭据加载到用户自定上游请求。读取认证路径允许 creator/auditor 角色进入目录查询，需明确隔离。不能仅根据 CodeQL 分级认定已有外部利用事件。

## Proposal

- 将 `POST /v1/llm/upstream-models` 与 Application Catalog 的目录探测均限定为 Admin operator 写级权限，避免非授权的只读角色向任意上游发出请求。
- 用户传入的 `api_key` 只当作明示字面值，不再作为环境变量名从服务端 Secret 读取；已有 Channel/SecretRef 正式配置链路不受此辅助目录查询修改。
- 复用 `platform/httpguard.ValidateURL` 和 `NewClient`：要求自定义端点为无 Query/Fragment/Userinfo 的公网 HTTPS，DNS 地址在验证和拨号阶段拒绝保留/私有 IP；禁止环境代理绕过受保护拨号器、禁用所有 provider 重定向，限制响应与超时。
- 保留唯一受控本地回退：operator 选择 Ollama 且完全未提供 BaseURL 时，使用原先固定的 localhost:11434，并禁用任何 redirect。用户手写 URL（即使协议标为 Ollama）不继承本地例外。
- 异常日志不能记录用户完整 URL 或上游响应正文，以避免附带 Key/query 文本进入审计。

## Alternatives considered

- 仅用 `url.Parse` 或 URL 字符串白名单：挡不住 DNS rebinding、IP literal 或重定向到内网。
- 一律禁止 localhost：能够缩小出口，但会直接破坏现有文档允许的本地 Ollama 基础接入；需另行由产品/部署策略正式裁决，不能隐式抹去。
- 保留任意私有 URL 并依赖调用者有 Admin 权限：被盗用的 operator Session 与出错的客户端重定向仍可能产生 SSRF，不能替代网络出口验证。
- 让环境变量代理执行 DNS/HTTP 连接：代理可代替被防护的客户端解析私有目标，因此本次为模型目录探测设置 `DisableProxy`，不改变其他 MCP/HTTP Client 默认代理行为。

## Acceptance criteria

- 构建、vet、archguard、Skills/notes、GitHub 必需 CI 与 CodeQL 针对最新 PR HEAD 完成真实检查，不通过不得合入。
- 在隔离环境内对 `GET /models`、`/api/tags` 等供应商 API 运行正反例；先证明 operator 合法访问公开 HTTPS 或默认本地 Ollama，creator/auditor 越权拒绝，日志不输出完整 URL/响应或任何 Secret。
- 对 loopback/IP literal/私网/metadata/特殊 IPv6/无协议、userinfo、URL Query、http 远端、DNS 地址切换、301/302 至其他域以及启用系统 HTTP_PROXY 的环境分别证明失败关闭。
- 复核 Gemini Key query 路径，以及 custom provider 私有网关的兼容范围；原有明确要求内网模型路由的场景须单独安全部署策略/ADR，不能仅靠禁用守卫来恢复。
- 运行时密钥和生产数据库一律不用于测试；没有隔离网络端点时保留未测，状态保持 review/blocked。

## Risks

- 这是目录辅助查询的安全硬化，可能拒绝从公网提供纯 HTTP 或从私网提供的自定义 Ollama/LLM 模型目录；若真实产品依赖，必须先给出受限 allowlist 和隔离网络策略，不应关闭安全守卫。
- `httpguard` 现有默认代理行为未改，新增 `DisableProxy` 只在本目录调用中开启；需要验证不同部署环境对代理的依赖。
- CodeQL 静态数据流不一定识别自定义 Sanitizer；只有实际独立动态验证与安全审查均通过，才能认定漏洞关闭。

## Follow-up hardening: error and key disclosure

- Upstream catalog errors now expose only the HTTP status; untrusted upstream response bodies are not forwarded to application error handling.
- Request construction, DNS/network and malformed endpoint failures return URL-free errors to avoid leaking full request URLs or user-provided secrets.
- Gemini discovery uses the existing `x-goog-api-key` header exclusively; the credential is no longer appended as `?key=` to the target URL.
- This is defense in depth on top of the SSRF guard. Isolated redirect/DNS/HTTP_PROXY probes, CodeQL and browser/operator validation remain independent release gates.

## Isolated negative probes added to INK-228 CI

The existing `cmd/ink228verify/upstream.go` now checks that explicit custom catalog URLs for OpenAI, Anthropic, Gemini and Ollama cannot contact its own local HTTP or HTTPS listener, that reserved IPv4/IPv6 IP categories are rejected without a socket, and that a synthetic DNS answer containing both public and private IPs fails closed. `cmd/ink228verify/main.go` runs the probes during disposable PG17/Redis7 validation. These tests never dial real metadata/private addresses. They do not yet exercise live DNS rebinding at connection time, redirect response variants, or a real public HTTPS Gemini endpoint. Those remain manual/security Review items.
