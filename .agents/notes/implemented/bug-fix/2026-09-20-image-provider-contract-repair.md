# Agent Note: 图片适配器不再伪装已退出或未上传的能力

Status: implemented

Task: INK-218
Scope: internal/llm/infrastructure/image_adapters.go 与 ADR-0025

## Problem

图片适配器将 Imagen 模型请求转发到已退出的 :predict 端点；OpenAI 图生图没有上传参考图片，而是将其数量写入 prompt 后调用 images/generations；Gemini 图片调用默认放开全部供应商安全阈值。三种行为都与当前上游合同和本地合规边界不一致。

改前检索了 INK-217 的供应商协议笔记、ProviderRouter 实现与已接受的 ADR。没有可复用的既有图片请求形状裁决；因此按 ADR-First 新增 ADR-0025。

## Decision

依据 [ADR-0025](../../../../docs/decisions/0025-image-adapter-current-provider-contract.md)，图片适配器现在对 Imagen fail-closed，使用既有 llm.capability_mismatch 语义拒绝，而不再请求 :predict。

OpenAI 的 image_to_image 请求现在仅接受非空 base64 或 data URL 参考图，将解码后的真实字节以 multipart/form-data 的 image[] 文件字段上传到 /images/edits；没有参考图或无法解码时在外呼前明确拒绝。普通文生图继续使用 /images/generations JSON 请求。

Gemini generateContent 图片路径移除了 defaultGeminiSafetySettings 注入，改为使用供应商默认安全策略；未新增管理端安全绕过配置。

## Alternatives considered

- 保留 Imagen 路径并只移除不安全参数：未采用。端点本身已退出，继续请求只会使管理员难以判断配置真实可用性。
- 对所有 OpenAI 图生图返回不支持：未采用。当前输入模型能承载真实图片，官方 edits 使用 multipart，保留能力更符合用户预期。
- 在此任务顺便迁移 Gemini Interactions 的旧 outputs/response_mime_type 处理：未采用。该路径影响 Agent 工具续调的步骤、签名和终态，需要独立任务与真实流式验收，已登记为 INK-219。

## Consequences

- 已配置为 Imagen 的渠道会以 422 能力不匹配失败，管理员必须迁移模型配置。
- OpenAI 编辑路径不再接受仅文字化的参考图描述；调用方需要提供真实可解码图片数据。
- 图片安全阈值不再由 adapter 全局绕过。后续若要配置阈值，必须把模型能力、渠道策略和本地合规共同建模。
- 本改动未改 Provider 枚举、数据库、公共 HTTP API 或 SecretRef。Gemini Interactions 旧 schema 风险仍待 INK-219 处理。

## Verification

- 已回读 GitHub 提交 26775efd1695cf2998f0696824aebb2986e592ec：已不存在 generateImageGoogleImagen、:predict、ALLOW_ADULT 或图片请求中的 defaultGeminiSafetySettings；OpenAI 编辑分支使用 /images/edits 与 multipart writer。
- 未验证：当前执行环境没有可用仓库工作树或 GitHub Actions 运行权限，尚未运行 go build ./...、go vet ./...、archguard、notes/skills 检查或定向 diff；未使用真实密钥调用任何图片上游。
- 部署验收仍需覆盖文生图、图生图、422 Imagen 拒绝、Gemini 安全阻断和无密钥失败。
