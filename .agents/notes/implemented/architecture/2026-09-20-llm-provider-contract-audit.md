# Agent Note: LLM 供应商协议来源核验

Status: implemented

Task: INK-217
Scope: docs/llm-provider-specifications.md, docs/backlog.md, CHANGELOG.md

## Problem

原 LLM 供应商规范把项目 R1 的公开 Provider 枚举、内部传输模式和候选 OpenAI 兼容供应商混为一谈，并包含没有官方来源支撑的模型版本、OpenAI Responses WebSocket、Gemini Interactions 请求字段和 Imagen predict 路径。其示例会直接误导适配器的序列化、工具续调和流式终态处理。

已检索 INK-205 的运行链笔记和 R1 网关设计。R1 公开枚举固定为 openai/deepseek/anthropic/gemini/ollama；OpenAI Responses 只能作为 openai 内部运行模式，不能借文档扩展数据库或管理 API 合同。

## Decision

将规范改为来源驱动的上游协议核验基线：逐项链接 OpenAI、DeepSeek、Anthropic、Gemini、Ollama、Z.AI 与 Kimi 官方文档，并按端点、鉴权、请求形状、工具续调、SSE 与 usage 分开记录。模型名不再作为“最新”目录或能力判断，统一要求从本地 Model Catalog 和已验证渠道能力读取。

Gemini Interactions 保留为真实但内部的传输合同，明确它不能与 generateContent 互换字段。GLM 与 Kimi 只被记录为候选 openai 兼容 Channel，未被虚构为新的 R1 Provider。OpenAI 图片 generations/edits 和当前 Gemini 图片合同单独说明，避免将文本模型接口当成图片编辑协议。

## Alternatives considered

- 只替换明显虚构的模型名称：未采用。端点、请求字段和流事件仍会误导实现，且下一次模型发布会重现漂移。
- 以当前代码路径覆盖官方合同：未采用。代码保留了已退出的 Imagen predict 与非 multipart 的图生图近似实现，不能反过来定义外部真相。
- 在本轮同步修复图片适配器：未采用。用户授权的是资料搜集与文档修正；图片路径更改涉及模型能力、安全策略和真实凭据验收，已登记为 INK-218。

## Consequences

适配器维护者能够从同一文档找到每个已声明端点的官方来源和不可混用的对象模型。文档不再承诺供应商动态模型目录、账户限额或未用真实凭据验证的功能。INK-218 留下两项明确实现差距：Gemini Imagen predict 已退出，以及 OpenAI 图生图应使用 multipart edits 而不是 prompt 摘要。

本变更不改变 R1 枚举、HTTP 对外合同、数据库、运行路径或 ProviderRouter 的实际调用行为，故不产生 ADR。

## Verification

- 使用供应商官方页面在 2026-09-20 逐端点复核，并在规范中保留 24 个 HTTPS 来源链接；本地 Markdown 链接格式检查通过。
- `go build ./...`：退出码 0。
- `node scripts/agent-notes.mjs check`：30 篇笔记、结构与链接校验通过。
- `git diff --check -- docs/llm-provider-specifications.md docs/backlog.md .agents/notes/implemented/architecture/2026-09-20-llm-provider-contract-audit.md`：退出码 0。
- 目标变更只涉及规范、看板、更新日志和工程笔记；无 Go/TS 运行路径变更。
- 未验证：没有生产凭据、区域资格或模型账户权限；未对任何付费上游发起生成请求。
