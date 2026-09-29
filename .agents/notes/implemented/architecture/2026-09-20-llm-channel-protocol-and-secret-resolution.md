# Agent Note: LLM 渠道协议与凭据解析收敛

Status: implemented

Task: INK-218
Scope: internal/llm/infrastructure, internal/app, frontend/apps/admin/src/routes/ChannelModal.tsx, .env.example

## Problem

供应商协议核验后，图片适配仍包含已关闭的 Gemini Imagen `:predict` 调用，OpenAI 图生图把参考图描述拼入 prompt 而不是上传二进制。环境渠道仅有公开 provider，选路结果丢失内部 transport；数据库渠道即使已经在目录登记也会被 ProviderRouter 预先判成未配置。

管理端允许浏览器直接录入 API Key，并把 `secret_store` locator 当作凭据值传给上游。该行为违反 ADR-0004 的密钥注入边界。改前检索了 INK-217 的协议核验笔记、INK-205 真实协议运行链笔记以及 ADR-0004/0005/0016；它们仍适用，本记录实现 ADR-0026 的补充裁决。

## Decision

`LLM_CHANNELS` 将 R1 公开 `provider` 与基础设施内部 `protocol` 分开解析。provider 只允许五个 R1 值；OpenAI 仅可选择 `openai`/`openai_responses`，Gemini 仅可选择 `gemini`/`gemini_interactions`/`gemini_openai`。routing.Channel 带回已选 transport，且模型必须在该渠道登记，不能回退到无关渠道。

ProviderRouter 设置数据库解析器后把数据库目录作为真实配置来源；运行时只从已部署环境变量解析数据库 Key。Secret Store driver 未实现时，Catalog 指纹计算和实际调用都 fail closed，绝不把 locator 当作 API Key。管理端改为只输入环境变量名。

图片适配器按 transport 显式分派。OpenAI 图像编辑使用 multipart `image[]` 和 base64 data URI 解码后的真实二进制；无文件授权解析器时拒绝持久化引用。Imagen 名称、Gemini Interactions 与 Gemini OpenAI 兼容模式的图片请求在没有独立官方合同适配器前明确失败。Gemini `generateContent` 请求不再默认发送 `BLOCK_NONE` 安全覆盖。

## Alternatives considered

- 继续以模型名、代理 URL 推断协议：配置较少，但会在兼容 API 和模型别名场景误分派，未采用。
- 让浏览器提交 API Key 或将 Secret Store locator 作为临时 Key：看似便于配置，但会破坏密钥边界，未采用。
- 直接把 Gemini Interactions 图片能力接入：官方推荐但其 input/output 对象与现有 generateContent 适配不同，需要单独能力、输出和实网验收，未在本任务猜测实现。

## Consequences

渠道 transport、模型映射和密钥来源可追溯且确定；OpenAI 图像编辑不再是文字近似。现有 `openai_compatible` 环境渠道配置需要改为 `openai`，并在非默认 transport 时写出 `protocol`。当前部署只能使用环境变量 SecretRef，直到提供同步、受控的 Secret Store adapter。

图片生成仍不是 R1 公共能力。本次没有添加 API、DDL、管理端 Provider 枚举或图片产品入口；所有未验收图片 transport 都明确拒绝。

## Verification

- 2026-09-20 通过 Firecrawl 读取 OpenAI Images edits、Gemini 图片生成和 Gemini Safety Settings 官方页面；OpenAI 规定 `POST /v1/images/edits` 使用 multipart `image[]`，Gemini 文档推荐 Interactions 且明确每请求 safety override 的含义。
- `go build ./...`：退出码 0。
- `go vet ./...`：退出码 0。
- `go run ./cmd/archguard -reachability`：五项架构红线通过；可达性仅报告既有 16 个不可达包。
- `npm run build:admin`：退出码 0。
- `node scripts/check-project-skills.mjs`、`node scripts/agent-notes.mjs check`、`git diff --check`：退出码 0。
- 未验证：没有可脱敏的上游 API Key、已发布数据库渠道或供应商计费授权；未执行真实文生图、图像编辑、Gemini 安全拦截或 Secret Store 调用，不能将本地构建视为上游实网验收。
