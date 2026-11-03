# LLM 供应商官方协议与契约规范

本文档是 InkLore 上游模型协议的核验基线，不是模型目录、价格表或对外 API 契约。它用于指导 `internal/llm/infrastructure` 的请求序列化、响应解构、上游 SSE 解析和用量审计。项目自己的 Provider、Model、Channel、Policy、预算和错误契约，以 [LLM Gateway R1 设计](system-design/09-DOMAIN-LLM-GATEWAY-SPEC.md) 与机读 API/DDL 为准。

最后核验：2026-09-20。本次补充核验 Gemini Interactions 在 2026 年 6 月成为 GA 后的推荐地位、旧 schema 的退出，以及其当前流式 steps 合同。所有外部链接均为供应商官方开发者文档。供应商可在不通知本仓库的情况下变更模型标识、预览状态、限额和单模型能力；禁止把“当前示例模型”或字符串前缀当成永久能力判定规则。

## 1. 适用范围与证据

### 1.1 InkLore 公开边界

R1 的 Provider 公开枚举只有 `openai`、`deepseek`、`anthropic`、`gemini` 与 `ollama`。这五个值的管理 API、数据库与策略能力由 R1 契约定义，不能因本文出现某个供应商就扩展枚举。

`openai_responses`、`gemini_interactions` 与 `gemini_openai` 是当前基础设施中可识别的内部传输模式，不是新的 R1 Provider 值。特别是 OpenAI Responses 通过 `LLM_OPENAI_API=responses` 作为 `openai` 的内部选择；任何将它添加为管理端 Provider 值的改动，均属于冻结契约变更，必须先走 ADR 与机读契约更新。

GLM/Z.AI 与 Kimi/Moonshot 的接口在第 10 节保留为已核验的候选兼容协议。当前 R1 枚举没有 `glm` 或 `kimi`，不能据此宣称项目已经有相应的原生 Provider 接入或完成生产验收。

### 1.2 官方来源清单

| 协议或能力 | 官方来源 | 本文使用范围 |
| --- | --- | --- |
| OpenAI Chat Completions | [Chat API reference](https://developers.openai.com/api/reference/resources/chat/) | 请求、SSE、工具调用、`stream_options` |
| OpenAI Responses | [Create a response](https://developers.openai.com/api/reference/resources/responses/methods/create/) / [streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events/) | `input`/`output` 模型、HTTP SSE、函数调用续调 |
| OpenAI 图片 | [Images API reference](https://developers.openai.com/api/reference/resources/images/) / [image generation guide](https://developers.openai.com/api/docs/guides/image-generation) | generations 与 edits 的不同请求编码 |
| DeepSeek | [Chat Completions API](https://api-docs.deepseek.com/api/create-chat-completion) / [Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode/) | 兼容聊天、`reasoning_content`、缓存用量 |
| Anthropic | [Create a Message](https://platform.claude.com/docs/en/api/messages/create) / [streaming](https://platform.claude.com/docs/en/build-with-claude/streaming) / [prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) | Messages 块、SSE、思考与缓存 |
| Gemini generateContent | [REST reference](https://ai.google.dev/api/generate-content) / [API overview](https://ai.google.dev/gemini-api/docs) | 仍受支持的 legacy `generateContent`、`streamGenerateContent`、结构化输出与思考配置 |
| Gemini Interactions | [Interactions overview](https://ai.google.dev/gemini-api/docs/interactions-overview) / [v1 REST reference](https://ai.google.dev/api/interactions-api-v1) / [streaming guide](https://ai.google.dev/gemini-api/docs/streaming) / [schema migration notice](https://ai.google.dev/gemini-api/docs/interactions-breaking-changes-may-2026) | GA 推荐入口、Interaction 输入、续调、steps 事件模型 |
| Gemini 图片 | [image generation guide](https://ai.google.dev/gemini-api/docs/image-generation) / [Imagen migration notice](https://ai.google.dev/gemini-api/docs/imagen) | Interactions 图片输出与 Imagen 退出状态 |
| Ollama | [OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility) | 本地 OpenAI 兼容面与未支持字段 |
| Z.AI / GLM | [GLM chat guide](https://docs.z.ai/guides/llm/glm-4.6) | OpenAI 兼容聊天、流式与思考字段 |
| Kimi | [API overview](https://platform.kimi.ai/docs/api/overview) / [SSE guide](https://platform.kimi.ai/docs/guide/utilize-the-streaming-output-feature-of-kimi-api) / [structured output](https://platform.kimi.ai/docs/guide/response_format) | 兼容聊天、SSE、`reasoning_content` 与 JSON Schema |

引用页面仅证明其页面在上述日期的公开合同。模型是否可调用、账户是否有权限、区域是否可用以及渠道限流，仍须以管理员配置的模型目录和一次脱敏的部署验收为准。

## 2. 协议选择矩阵

| InkLore 路径 | 上游方法 | 鉴权 | 流式载体 | 不能混用的要点 |
| --- | --- | --- | --- | --- |
| `openai` | `POST /v1/chat/completions` | `Authorization: Bearer` | SSE，`chat.completion.chunk` | `messages`/`tool_calls` 模型，不是 Responses 的 `input`/`output` |
| `deepseek` | `POST https://api.deepseek.com/chat/completions` | `Authorization: Bearer` | OpenAI 兼容 SSE | `reasoning_content` 是独立字段，不能伪造成 XML 标签 |
| `anthropic` | `POST /v1/messages` | `x-api-key` 加 `anthropic-version` | 命名 SSE 事件 | 工具结果是 `user` content block，不是 OpenAI 的 `role: tool` |
| `gemini` | `POST /v1beta/models/{model}:generateContent` | `x-goog-api-key` 或 API key 查询参数 | `streamGenerateContent?alt=sse` | 仍受支持但已是 legacy；`contents.parts` / `functionCall`，字段采用 camelCase |
| `ollama` | `{base}/v1/chat/completions` | 本地服务通常可忽略 Bearer 值 | OpenAI 兼容流 | 兼容并不等于支持 OpenAI 全部字段 |
| OpenAI 内部模式 | `POST /v1/responses` | `Authorization: Bearer` | Responses HTTP SSE | 非 R1 Provider 值；不要写成 `/v1/responses` WebSocket |
| Gemini 内部模式 | `POST /{api_version}/interactions` | `x-goog-api-key` | Interaction SSE | GA 推荐；渠道固定 API version 后使用其当前 schema，`input` / `previous_interaction_id` 与 generateContent 不兼容 |

所有上游 URL 均须从受控 Channel 的 endpoint/base URL 解析，并经 SecretRef、TLS 与 SSRF allowlist 校验。本文的官方 URL 不能成为允许任意转发的理由。


## 2.1 请求、响应与字段控制矩阵

本表是适配器字段白名单的核验入口；它不授权调用方透传任意上游字段。每个字段仍必须同时满足：已发布的 Model Catalog 能力、Channel 固定的协议/API version、租户策略和供应商当前模型限制。未列出的实验字段一律不发送，不能因“兼容 OpenAI”而假设所有模型都接受。

| 协议 | 请求的必需骨架 | 受控可选字段（仅按模型能力发送） | 成功响应必须读取 | 流式/工具关键点 |
| --- | --- | --- | --- | --- |
| OpenAI Chat Completions | `model`、`messages` | `stream`、`stream_options.include_usage`、`tools`、`tool_choice`、`parallel_tool_calls`、`response_format`、`max_completion_tokens`、`temperature`、`top_p`、`reasoning_effort` | `id`、`model`、`choices[].message`、`finish_reason`、`usage` | 增量在 `choices[].delta`；按 tool-call index 累积 arguments；usage 可在空 choices 终帧出现。 |
| OpenAI Responses | `model`、`input` | `instructions`、`previous_response_id` 或 `conversation`、`store`、`stream`、`tools`、`tool_choice`、`text.format`、`max_output_tokens`、`reasoning` | `id`、`status`、`output[]`、`output_text`、`usage`、`error` | 不是 Chat delta：按 typed SSE event（如 `response.output_text.delta`、`response.completed`）分支；函数调用结果用原 `call_id` 续调。 |
| DeepSeek Chat Completions | `model`、`messages` | `stream`、`stream_options`、`tools`、`tool_choice`、`response_format`、`max_tokens`、`temperature`、`top_p`、`frequency_penalty`、`presence_penalty` | `choices[].message.content`、`reasoning_content`、`tool_calls`、`finish_reason`、`usage` | `reasoning_content` 不是可见正文；工具续调必须原样保留 assistant 的 reasoning、content 与 tool_calls；strict tool schema 仍是能力/版本受限项。 |
| Anthropic Messages | `model`、`max_tokens`、`messages` | `system`、`stream`、`tools`、`tool_choice`、`thinking`、`temperature`、`top_p`、`top_k`、`stop_sequences`、`metadata`、content-block `cache_control` | `id`、`role`、`content[]`、`stop_reason`、`usage` | 读取命名 SSE 与 block index；`tool_use.input` 由 `input_json_delta` 累积；thinking/signature 只用于协议续调，不向 InkLore 用户 SSE 透出。 |
| Gemini generateContent | `contents` | `systemInstruction`、`tools`、`toolConfig`、`safetySettings`、`generationConfig`（含 `temperature`、`topP`、`topK`、`maxOutputTokens`、`responseMimeType`、`responseJsonSchema`、`thinkingConfig`） | `candidates[].content.parts[]`、`finishReason`、`promptFeedback`、`usageMetadata` | 仅既有 legacy adapter 使用；按 candidate/part 读取 `functionCall`、`inlineData` 与 text，结果用 `functionResponse` 回传。 |
| Gemini Interactions | `model`、`input` | `stream`、`previous_interaction_id`、`store`、`tools`、`generation_config`、`response_format`、`background` | `id`、`status`、`steps[]`、`usage`、模型输出/图片输出 | 新项目推荐；按 typed step 读取，函数调用的 `arguments_delta` 完整累积后执行，`requires_action` 必须由 `function_result` 续调。 |
| Ollama OpenAI Compatibility | `model`、`messages`（或已验证的 `input`） | `stream`、`tools`、`response_format`/JSON mode、`think`；原生调优参数仅用 Ollama 原生端点验证后发送 | 兼容对象的 `choices[]` 或 `output_text`，及模型实际提供的 usage/终态 | 只支持 OpenAI 的子集；本地 Chat 与云端 Responses 的可用能力不同，不能使用 OpenAI 的字段清单做透传白名单。 |
| Z.AI / GLM（候选 Channel） | `model`、`messages` | `stream`、`max_tokens`、`tools`、`tool_choice`、`thinking`、`response_format` | Chat completion 的 `choices[]`、`finish_reason`、`usage`，以及供应商定义的 reasoning/tool 字段 | 仅视为 OpenAI 兼容渠道；需要按目标模型核验流式 reasoning/tool 增量，不能把 GLM 特有扩展写进通用 OpenAI 请求。 |
| Kimi / Moonshot（候选 Channel） | `model`、`messages` | `stream`、`stream_options.include_usage`、`tools`、`tool_choice`、`response_format`、`thinking` 或 `reasoning_effort`、`partial`、`logprobs` | `choices[].message`、`reasoning_content`、`tool_calls`、`finish_reason`、最终 `usage` | 多轮与工具调用要回传完整 assistant message（含 reasoning）；`partial` 不与 JSON object 模式混用；工具结果是 `role:"tool"` 加匹配的 `tool_call_id`。 |

### 2.2 字段控制的统一规则

1. **字段白名单而非透传**：Application/Domain 只表达统一意图；基础设施适配器根据 `protocol + api_version + model capability` 组装上游对象。任意调用方提供的 model、base URL、header、provider-only 参数或工具 schema 都不得直接穿透。
2. **互斥与依赖关系必须前置校验**：例如 OpenAI Responses 的结构化输出位于 `text.format` 而非 Chat 的 `response_format`；Gemini 两种 API 分别使用 camelCase 与 snake_case；Kimi `partial` 不与 `json_object` 混用。供应商拒绝的组合应映射为本地可审计请求/能力失败，不能静默删字段后继续执行。
3. **结构化输出不等于本地校验通过**：所有 JSON mode、JSON Schema、strict tool schema 只约束上游生成；响应仍必须完成 UTF-8、单值、重复键、大小/深度及 R1 Draft 2020-12 校验。
4. **思考字段不得可见**：`reasoning_content`、Anthropic thinking/signature、Gemini thought 与 OpenAI reasoning item 仅作为协议续调和受限审计数据；绝不写入创作者 SSE、日志正文或管理端可读详情。
5. **控制字段必须可审计**：记录脱敏后的 provider、channel、model、protocol/api version、允许的字段集合、请求关联 ID、供应商 usage 与受限失败类别；不记录 Authorization、SecretRef 解密值、完整 prompt 或原始响应。

## 3. OpenAI Chat Completions

### 3.1 端点与最小请求

```http
POST https://api.openai.com/v1/chat/completions
Authorization: Bearer $OPENAI_API_KEY
Content-Type: application/json
```

`model` 与 `messages` 是调用所需的核心字段。消息使用 `system`、`developer`、`user`、`assistant` 等 Chat 语义；函数工具放在 `tools`，模型返回的调用放在 assistant 消息的 `tool_calls`，工具执行结果以 `role: "tool"` 和对应的 `tool_call_id` 回传。已经废弃的 `function_call` 不能作为新的网关输出格式。

```json
{
  "model": "MODEL_ID_FROM_CATALOG",
  "messages": [
    {"role": "developer", "content": "Follow the supplied policy."},
    {"role": "user", "content": "Perform the requested task."}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "tool_name",
        "description": "A scoped operation.",
        "parameters": {
          "type": "object",
          "properties": {"id": {"type": "string"}},
          "required": ["id"],
          "additionalProperties": false
        }
      }
    }
  ],
  "stream": true,
  "stream_options": {"include_usage": true}
}
```

`temperature`、`top_p`、`max_completion_tokens`、`response_format`、`parallel_tool_calls`、`service_tier` 等均是模型和账户能力相关的可选项。网关不得按模型名称包含某个子串，就全局删除或强行写入这些字段；应以 Model Catalog 的能力、供应商实际 4xx 响应和已验证的渠道配置决定。

### 3.2 SSE 与工具参数拼接

流式响应为 `data:` 帧，通常包含 `object: "chat.completion.chunk"`。`choices[].delta.content`、`delta.tool_calls[]` 和终止 `finish_reason` 都可能分散在多帧中；同一工具调用按其 index 累积 `id`、函数名和 `arguments`，只在收齐后解析 JSON。若请求了 `stream_options.include_usage`，最终可能存在 `choices: []` 的 usage 帧，之后才是 `data: [DONE]`。

不得把最终 usage 帧当正文，也不得将局部工具参数当作完整 JSON 执行。网络中断、非 2xx 或提前 EOF 时，不能补造 `[DONE]`、成功终态或用量。

## 4. DeepSeek Chat Completions

### 4.1 端点与兼容边界

```http
POST https://api.deepseek.com/chat/completions
Authorization: Bearer $DEEPSEEK_API_KEY
Content-Type: application/json
```

DeepSeek 使用 OpenAI 风格的 `model`、`messages`、`stream`、`tools` 与 `tool_choice`，但它不是 OpenAI 的逐字段替身。参数是否可用、模型标识及思考开关必须以 [官方 Chat 参考](https://api-docs.deepseek.com/api/create-chat-completion) 的模型说明为准。

### 4.2 `reasoning_content` 与多轮工具调用

启用思考的模型会在 assistant 响应或流式 `delta` 中给出独立的 `reasoning_content`。官方的工具调用指导要求：在工具链的后续请求中，保留原 assistant 的 `content`、`tool_calls` 和相应 `reasoning_content`，再附加 tool result。它不是应由网关从正文中猜测、抽取或包裹为 `<think>` 的字段。

适配器应分别累积 `reasoning_content`、可见 `content` 与按 index 分片的 `tool_calls`；只向最终用户输出项目允许公开的正文。内部思考内容不得进入 InkLore 的外部 SSE、审计正文或日志。若上游明确拒绝某字段，记录脱敏的渠道/模型/状态信息；不要改写历史消息来掩盖协议错误。

### 4.3 用量与缓存

DeepSeek 的 usage 可含 `prompt_cache_hit_tokens`、`prompt_cache_miss_tokens`、`prompt_tokens_details.cached_tokens` 和 `completion_tokens_details.reasoning_tokens`。这些字段存在与否取决于模型和响应；审计层必须把缺失值视为未知，而不是推断为零。缓存 token 与 reasoning token 的计费语义应由本地 Pricing revision 决定，不从一次响应猜测价格。

## 5. OpenAI Responses API

### 5.1 HTTP 请求模型

```http
POST https://api.openai.com/v1/responses
Authorization: Bearer $OPENAI_API_KEY
Content-Type: application/json
```

Responses 使用 `model`、`instructions`、`input`、`tools`、`reasoning`、`stream` 和 `max_output_tokens` 等字段。它以 output item 表示模型产物，不能将 Chat Completions 的 `messages`、`max_completion_tokens` 或 `tool_calls` 原样送入。

函数续调的正确对象是 Responses item：先从 `response.output` 读取 `function_call` item，再把 `function_call_output`（含相同 `call_id`）加入下一次的 `input`。需要无状态连续推理时，按官方说明保留可复用的 reasoning item；不能复制或暴露隐藏推理原文。

### 5.2 HTTP SSE，非 Responses WebSocket

设置 `stream: true` 后，Responses 通过 HTTP SSE 输出语义事件。文本常用事件包括 `response.created`、`response.output_text.delta`、`response.completed` 和 `error`；工具参数使用 `response.function_call_arguments.delta`，其分片必须依据 `item_id`/`output_index` 累积。完整事件集合以 [官方 streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events/) 为准，并可能随工具类型扩展。

原文中的 `wss://api.openai.com/v1/responses`、`OpenAI-Beta: responses-2025`、`response.create` 与 `response.cancel` 不属于本 Responses HTTP 合同，已移除。双向 WebSocket 是独立的 [Realtime API](https://developers.openai.com/api/docs/guides/realtime-conversations) 产品选择；如未来需要，必须新增明确的内部协议和安全评审，不能隐式复用 Responses 适配器。

## 6. Anthropic Messages API

### 6.1 端点、请求块与头部

```http
POST https://api.anthropic.com/v1/messages
x-api-key: $ANTHROPIC_API_KEY
anthropic-version: 2023-06-01
Content-Type: application/json
```

请求必须携带 `model`、`max_tokens` 与 `messages`。系统指令使用顶层 `system`；消息 content 可以是字符串或 typed content blocks。工具定义使用 `tools[].name`、`description`、`input_schema`，模型返回 `tool_use` block，调用方将结果作为 `role: "user"` 的 `tool_result` block 送回。不要将 OpenAI 的 `role: "tool"` 直接转发给 Anthropic。

```json
{
  "model": "MODEL_ID_FROM_CATALOG",
  "max_tokens": 1024,
  "system": [{"type": "text", "text": "Follow the supplied policy."}],
  "messages": [{"role": "user", "content": "Perform the requested task."}],
  "stream": true
}
```

`thinking`、`temperature`、`top_p`、`top_k` 和 prompt caching 的可用组合取决于具体模型与 API 版本。当前文档不能以“启用 thinking 就全局禁止 temperature/top_p”替代官方模型约束。适配器必须依据所选模型的官方能力进行验证，并在模型拒绝时返回本地的能力/请求错误，而不是静默清除调用者意图。

### 6.2 流式块解析与缓存用量

SSE 从 `message_start` 开始，经 `content_block_start`、零个或多个 `content_block_delta`、`content_block_stop`，在 `message_delta` 后以 `message_stop` 结束。`content_block_delta.delta.type` 决定分片含义，例如 `text_delta`、`thinking_delta`、工具输入的 `input_json_delta` 以及思考签名的 `signature_delta`。必须按 block index 累积；工具 JSON 只能在 block 完成后解析。

可审计 usage 字段包括 `input_tokens`、`output_tokens`、`cache_read_input_tokens` 与 `cache_creation_input_tokens`。它们反映供应商报告的 token 数，不等于 InkLore 价格；本地预算仍以已冻结的 Pricing revision 结算。多轮思考/工具上下文必须保留供应商要求的签名和 block 结构，不能用自造的 reasoning 文本替换。

## 7. Gemini 原生 generateContent（legacy，仍受支持）

### 7.1 端点、鉴权与请求形状

```http
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse
x-goog-api-key: $GEMINI_API_KEY
Content-Type: application/json
```

`contents` 是 `Content` 数组，每项含 `role` 与 `parts`。系统说明是 `systemInstruction`，不是 Chat 的 `system`；函数调用是 model part 内的 `functionCall`，工具结果是 user part 内的 `functionResponse`。字段采用 camelCase。

```json
{
  "systemInstruction": {
    "parts": [{"text": "Follow the supplied policy."}]
  },
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "Perform the requested task."}]
    }
  ],
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "tool_name",
          "description": "A scoped operation.",
          "parameters": {
            "type": "OBJECT",
            "properties": {"id": {"type": "STRING"}},
            "required": ["id"]
          }
        }
      ]
    }
  ],
  "generationConfig": {"maxOutputTokens": 1024}
}
```

`generationConfig` 支持与模型相关的生成控制。严格 JSON 应使用 `responseMimeType: "application/json"` 与 `responseJsonSchema`，再由 InkLore 按 R1 的 Draft 2020-12 规则二次校验。原文的通用 `responseSchema` 映射并非当前 REST 参考推荐字段。

### 7.2 思考、安全与响应

思考配置位于 `generationConfig.thinkingConfig`，可包含 `includeThoughts`、`thinkingBudget` 与 `thinkingLevel`。`thinkingLevel` 面向支持它的较新模型；`thinkingBudget` 也不是所有模型的通用开关。请先按模型能力选择一个合法字段，而非用系列名称猜测。

响应从 `candidates[].content.parts[]` 读取 text、`functionCall`、`inlineData` 等 part；用量在 `usageMetadata`，字段可包括 `promptTokenCount`、`candidatesTokenCount`、`totalTokenCount` 和模型支持时的 thought token 统计。流式响应同样必须遍历所有 candidate 与 part，不能假定每个帧只有一个 text。

`safetySettings` 是供应商可用能力与部署内容政策的一部分。项目不能统一注入 `BLOCK_NONE` 来绕过安全过滤；未通过的响应要记录为可追踪的上游结果，并由本地合规规则决定后续动作。

## 8. Gemini Interactions API（GA 推荐）

截至本次核验，Interactions API 自 2026 年 6 月起是 Gemini 新项目与 Agent 的推荐入口；`generateContent` 仍可用于既有适配器，但不再是新接入的默认选择。当前 REST 文档同时覆盖 `v1` 与 `v1beta`；Channel 必须在配置中固定已验收的 API version，不能将 `v1beta` 写死为永久规则。

2026-06-08 后旧 Interactions schema 已永久退出。解析非流式响应必须以 `steps` 为权威，而不是已移除的 `outputs`；至少识别 `user_input`、`model_output`、`function_call` 以及已启用服务端工具产生的专用 step。

### 8.1 独立的 Interaction 合同

```http
POST https://generativelanguage.googleapis.com/{api_version}/interactions
x-goog-api-key: $GEMINI_API_KEY
Content-Type: application/json
```

Interactions API 使用 `model` 和 `input`，可使用 `previous_interaction_id` 延续已存储的服务端状态，或使用 `store: false` 加显式步骤维持无状态历史。它不是 `generateContent` 的别名：`contents`、`systemInstruction`、`generationConfig` 与 `functionResponse` 不能原样搬到此端点。其生成配置、工具和 response format 使用该 API 的 snake_case 请求字段，例如 `generation_config`、`response_format`。

```json
{
  "model": "MODEL_ID_FROM_CATALOG",
  "input": "Perform the requested task.",
  "stream": true
}
```

### 8.2 流式事件与函数续调

流式调用提交到已固定版本的 `/{api_version}/interactions`，带 `stream: true`；官方 SSE/SDK 流用事件类型区分 `interaction.created`、`interaction.in_progress`、`step.start`、`step.delta`、`step.stop` 与 `interaction.completed`。 `step.delta` 可以是 text、工具参数或其他 step 类型；函数参数位于 `function_call` step 的增量中，必须按 step/index 累积 `arguments_delta` 后才解析，不能按原文捏造为固定的 `delta.type: "thought"`/`"text"` 二选一。

函数调用完成后，后续请求使用前一 Interaction 的 ID 和 `function_result` input item：

```json
{
  "model": "MODEL_ID_FROM_CATALOG",
  "previous_interaction_id": "INTERACTION_ID",
  "input": [
    {
      "type": "function_result",
      "name": "tool_name",
      "call_id": "CALL_ID",
      "result": {
        "content": [{"type": "text", "text": "{\"result\":true}"}]
      }
    }
  ],
  "stream": true
}
```

只有 `interaction.completed` 的 Interaction status 表示一次 Interaction 已达到当前终态。若 status 为 `requires_action`，网关需要工具结果续调，不能把它映射为最终文本成功。

## 9. Ollama OpenAI Compatibility

Ollama 可在其服务器 base URL 下提供 OpenAI 兼容的 `POST /v1/chat/completions`，请求核心仍是 `model` 和 `messages`，并支持流式与 tools。典型本地 base URL 为 `http://localhost:11434/v1`；开发 SDK 可能需要一个非空 API key 占位值，但本地服务通常不会校验该值。生产网络上的 Ollama 端点仍必须接受本地 SecretRef、TLS、网络边界和 SSRF allowlist 约束。

兼容页明确说明存在未支持字段，例如 logprobs，以及具体模型决定的 reasoning/structured output 能力。故适配器应只发送该渠道和模型已验证的字段；不要因为 OpenAI 上游接受某字段就将其发送给 Ollama。

## 10. 候选 OpenAI 兼容供应商

### 10.1 Z.AI / GLM

官方 GLM 指南给出的聊天路径为：

```http
POST https://api.z.ai/api/paas/v4/chat/completions
Authorization: Bearer $ZAI_API_KEY
Content-Type: application/json
```

它采用 `model`、`messages`、`max_tokens`、`stream` 与可选 `thinking: {"type":"enabled"}` 的 OpenAI 风格请求，流式 delta 可含 `reasoning_content` 和 `content`。该页面中的 `glm-4.6` 仅是示例，不可写成“最新或唯一模型”。工具调用、JSON 模式、速率和模型上下文窗口需逐模型重新核验。

### 10.2 Kimi / Moonshot

Kimi 官方兼容路径为：

```http
POST https://api.moonshot.ai/v1/chat/completions
Authorization: Bearer $MOONSHOT_API_KEY
Content-Type: application/json
```

Kimi 的 SSE 使用 `chat.completion.chunk`、`data: [DONE]`、`stream_options.include_usage` 和分片 `tool_calls`。在思考/工具调用场景，官方指导将累积的 `reasoning_content`、完整 assistant tool call 与 tool result 一同续传；适配器同样必须延迟解析分片 arguments。Kimi 还公开 `response_format.type: "json_schema"`，但本地仍必须执行 R1 的严格 JSON Schema 验证。

两家供应商都只能在其端点、认证方式、工具分片和能力经过独立验收后，作为受控 `openai` 兼容 Channel 引入。它们不是 `LLM_PROTOCOL=glm` 或 `LLM_PROTOCOL=kimi` 的现有 R1 配置值。

## 11. 图像生成与编辑

### 11.1 OpenAI Images API

OpenAI 的图片生成与编辑是两个不同端点：

| 用例 | 官方端点 | 请求编码 | 适配注意事项 |
| --- | --- | --- | --- |
| 文生图 | `POST /v1/images/generations` | JSON | `prompt` 是核心输入；模型、尺寸、质量、背景与输出格式必须按所选图片模型能力发送 |
| 图像编辑/图生图 | `POST /v1/images/edits` | `multipart/form-data` | 图片文件、可选 mask 与 prompt 需要真实 multipart 二进制上传，不能把参考图数量拼进 prompt |

`style`、`response_format`、`output_format`、`quality`、`n` 与可用 size 在不同图片模型之间并不相同。必须查询图片模型能力；不得以旧 DALL-E 参数套用到 GPT Image。响应中的 URL、base64 数据、修订提示与 usage 也要按实际模型响应读取，不能假定每个字段必然存在。

### 11.2 Gemini 图片

截至核验日，Gemini 的新图片生成/编辑接入应优先使用支持图片输出的模型与 Interactions API；`response_format` 可声明 `type: "image"`、`aspect_ratio` 与模型支持时的 `image_size`，并可与 text 输出组合。现有 `generateContent` 仍受支持，但不能把它或已退出的 Imagen 当作 Interactions 的替代对象模型。输出图片位于 Interaction 的 image output，不是 Imagen `predict` 的 `predictions[].bytesBase64Encoded` 合同。

官方 [Imagen 页面](https://ai.google.dev/gemini-api/docs/imagen) 明确说明 Imagen 已关闭且不再通过 Gemini API 提供。因此，`POST /v1beta/models/{model}:predict`、Imagen 3 模型名、`personGeneration: "ALLOW_ADULT"` 和 `safetySetting: "block_none"` 不得继续作为 InkLore 可用的 Gemini API 规范。


### 11.3 图像字段控制

| 协议 | 文生图受控字段 | 编辑/图生图受控字段 | 响应读取与禁止事项 |
| --- | --- | --- | --- |
| OpenAI Images | `model`、`prompt`、模型支持时的 `size`、`quality`、`background`、`output_format`、`output_compression`、`n`/stream 控制 | `/images/edits` 的 multipart 文件、可选 mask 与 `prompt`；参数仅按目标图片模型能力附加 | 按实际响应读取 base64/URL、revised prompt 与 usage；不可将图片摘要塞进 prompt 后假称编辑。 |
| Gemini Interactions Images | `model`、`input`、`response_format` 的 `type:image`、`aspect_ratio`、`image_size` | 将真实 image input 与 text input 组成 `input`，可用 `previous_interaction_id` 延续编辑会话 | 从 image output/steps 读取图片；不可发送已退出 Imagen `:predict` 字段、`personGeneration` 或默认 `block_none` 安全设置。 |

### 11.4 实现状态与待验收边界

INK-218 已按 [ADR-0025](../.agents/notes/reference/decisions/0025-image-adapter-current-provider-contract.md) 修复图片适配器：已删除已退出的 Imagen `:predict` 调用并对 `imagen` 模型明确失败；OpenAI 图生图改用真实 multipart `/images/edits`；Gemini 图片请求不再默认发送全量放开的安全阈值。

该变更尚未使用真实凭据验收。部署环境必须分别验证 OpenAI 文生图、OpenAI 编辑、Gemini 图片、已退出 Imagen 的 422 拒绝、供应商安全拦截及无密钥失败；不得把编译或文档核对当作上述上游行为的证据。Gemini Interactions 的 steps schema/工具续调迁移另列 INK-219，不在本图片修复范围内。

## 12. 统一实现规则

### 12.1 请求前

1. 只从 SecretRef 在调用时解析密钥；任何日志、usage、错误详情、SSE 或管理 API 都不得含明文密钥、Authorization 值、Cookie 或完整 prompt/response。
2. 从已发布策略冻结的模型、渠道、模型能力和 Pricing revision 构造请求；不允许调用方任意覆写 upstream model 或 base URL。
3. 对每一个原生协议使用其真实请求对象。协议转换必须在基础设施层完成，领域层只处理统一的 typed request/result。
4. 原生 JSON Mode 或 JSON Schema 仅是上游约束。返回文本仍需通过 InkLore 的 UTF-8、单值、无重复键和 Draft 2020-12 验证。

### 12.2 流式期间

1. 上游帧要先完整读取、按协议解码，再转换为内部事件；不要把原始供应商 SSE 转发给浏览器。
2. 按 block index、choice index、output index 或 tool-call index 累积内容；在完整终态前不解析工具参数 JSON。
3. 隐藏推理、签名和供应商调试字段不得写入项目的用户可见 SSE。项目 SSE 只公开允许的文本、工具状态和最终结果。
4. 一旦项目响应已经开始，外部 SSE 只能完成或关闭，不能在同一响应尾部附加 JSON 错误包。

### 12.3 用量、失败与重试

供应商 HTTP status、错误 JSON 形状、Retry-After 和 usage 字段不统一。基础设施只应提取状态、受限的错误类别、Retry-After、模型/渠道标识和供应商报告 token；再由 R1 调度器决定 Key health、熔断、预算结算、第一次 delta 前的技术重试与 fallback。不得把本文的示例映射为新的公开错误码。

上游已 dispatch 但没有可靠终态/usage 时，按 R1 的保守结算与 `llm.upstream_outcome_unknown` 语义处理；绝不能以“估计成功”补造用户输出。首个可见 delta 后的失败不得无提示地切换模型拼接文本。

## 13. 每次供应商升级的复核清单

1. 通过本文件官方来源重新确认 exact URL、认证头、API 版本、模型状态和区域限制。
2. 为目标模型逐项确认 text、tools、structured output、thinking、streaming、image 与 token usage 能力，更新本地 Model Catalog，而非写死名称判断。
3. 在隔离环境使用脱敏渠道做一次非流、流式、工具续调、结构化输出、限流/无效参数失败验证；真实密钥和正文不进入证据。
4. 确认首个 delta 前后、取消、EOF、429、401、5xx 和 usage 缺失时的本地状态机及审计结果。
5. 若变更涉及 Provider 枚举、数据库、外部 HTTP 合同、通信协议或能力模型，先写 ADR；仅外部来源的文档漂移按 INK 任务记录来源、结论和未验证边界。
