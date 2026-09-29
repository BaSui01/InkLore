# Agent Note: 渠道协议自适应、API Key直填与上游模型列表拉取

Status: implemented

Task: INK-227
Scope: internal/llm 传输层与基础设施、docs API Schema 与 Admin Console 渠道弹窗 (ChannelModal)

## Problem

在原有模型渠道创建与编辑弹窗中，用户界面仅暴露“选择供应商”下拉列表，当数据库中尚无配置 Provider 或 Provider 不全时下拉选项为空白，缺少直观的端点协议选择（如 OpenAI Chat, OpenAI Responses, Anthropic, Gemini, DeepSeek, Ollama）；弹窗仅提示从列表的密钥按钮管理凭据，无输入 API Key 的入口；且无法实时从上游端点探测可用模型列表，容易导致用户未配置模型时直接触发“请输入上游模型标识”报错。

## Decision

1. 前端弹窗显式提供“接口协议 (Protocol)”选择器，内建覆盖 OpenAI Chat、OpenAI Responses、Anthropic Claude、Google Gemini、DeepSeek 及 Ollama 等主流协议。
2. 协议与网关 Base URL 深度联动，自适应推荐默认官方端点，并自动根据所选协议关联或在保存时自动注册 Provider，彻底避免下拉框空白与割裂流程。
3. 渠道弹窗直接集成“API Key / 访问凭据”输入框（支持明文/密文切换），用户既可输入上游 API Key 明文，也可输入系统环境变量名引用。
4. 后端提供正式声明的 `/v1/llm/upstream-models` 接口（注册至 API Schema 契约并通过 requestschema 校验），根据指定的 protocol、base_url 和 api_key 向上游发送带凭据的标准化 GET 请求获取模型标识列表，并在上游未开放该接口或网络异常时提供协议级热门模型智能回退。
5. 前端提供“从上游获取模型列表”操作面板（UpstreamModelPicker），支持搜索、全选/单选与一键批量导入到渠道模型中；同时提供常用推荐模型徽章快速点击添加。
6. 新建渠道时若输入了环境变量名凭据，自动创建对应的 Channel Key，实现一站式配置体验。

## Alternatives considered

- 前端浏览器直接 fetch 上游 `/models` 接口：受到大多数上游端点的 CORS 跨域限制，且在前端直接暴露敏感凭据发起请求不符合网络安全实践；采用后端轻量安全代理探测。
- 将全部上游模型选择逻辑塞在 ChannelModal.tsx 中：导致单个手写文件体积逼近 50KB 架构红线；将面板独立拆分为 UpstreamModelPicker.tsx 组件，控制单文件体积在 43KB 以下。

## Consequences

- 用户在接入新渠道时能直观选择协议（如 OpenAI、Anthropic、Gemini 等），直接录入 API Key，并可一键从上游拉取导入模型，不再卡在“请输入上游模型标识”与空下拉框。
- 新增接口完全受 `httpguard` 与静态编译请求契约保护，严格遵从架构红线与 No Emoji 规范。

## Verification

- `go build ./...` 退出码 0，后端编译通过。
- `pnpm --dir frontend run build` 退出码 0，Creator 与 Admin 前端全量打包通过。
- `go run ./cmd/archguard -reachability` 退出码 0，架构红线（R1-R5）全绿。
- `node scripts/build-request-schemas.cjs --check` 退出码 0，247 个操作与 112 个 JSON 请求契约一致。
- `lsp-diagnostics` 静态分析 0 errors, 0 warnings。
