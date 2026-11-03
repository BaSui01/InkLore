# ADR-0025：图片适配器按当前供应商合同 fail-closed

- **状态**：Accepted
- **日期**：2026-09-20
- **决策者**：用户授权的 InkLore 维护者
- **关联任务**：INK-218
- **替代关系**：无

## 背景（Context）

`internal/llm/infrastructure/image_adapters.go` 仍将模型名包含 `imagen` 的请求发送到已退出的 Gemini Imagen `:predict` 合同，并在 OpenAI 图生图场景把参考图数量拼入 prompt 后继续请求 `/images/generations`。Gemini 图片请求还默认发送 `BLOCK_NONE` 安全阈值。这三条路径会把一个已不存在、未上传图片或绕过安全策略的调用伪装成受支持的图片能力。

R1 的公开 Provider 枚举、数据库和 HTTP API 不因此改变；本决策只收紧 LLM infrastructure 的上游请求序列化。它遵守 [LLM Gateway R1 设计](../system-design/09-DOMAIN-LLM-GATEWAY-SPEC.md) 中“能力不匹配返回 `llm.capability_mismatch`、不得静默删除约束”的规则，以及 [供应商协议规范](../../../../docs/llm-provider-specifications.md) 的当前图片合同。

## 决策（Decision）

图片适配器按当前官方合同 fail-closed：不再调用退出的 Imagen `:predict`，OpenAI 图生图使用真实 `multipart/form-data` 的 `/images/edits`，Gemini 不再默认下发全量放开安全阈值。

我们希望：

1. 请求 `imagen` 模型时明确返回既有 `llm.capability_mismatch`，不尝试任何退化请求。
2. 仅在请求携带可解码的真实参考图片时调用 OpenAI Images edits；文件作为 multipart image 字段上传，不能把参考图元数据拼入 prompt。
3. Gemini 图片请求保持供应商默认安全策略；未来若需要可配置安全阈值，必须由已发布的模型/渠道能力与本地合规策略共同控制，不能在 adapter 内写死绕过值。
4. 不扩张 Provider 枚举、R1 公共 API、模型目录或 SecretRef 边界。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 保留 Imagen `:predict` 作为兼容回退 | 改动最小 | 上游已退出，失败结果不确定且继续暴露不安全参数 | 不接受已失效合同作为回退路径 |
| OpenAI 图生图明确拒绝 | 避免 multipart 实现复杂度 | 已有图生图输入会失去真实可用能力 | 官方 edits 合同清晰，适配器应上传真实图片 |
| 默认 `BLOCK_NONE` 再依赖本地合规 | 看似减少供应商拦截 | 绕过上游安全能力，和本地策略分层冲突 | 采用供应商默认值，后续配置必须显式建模 |

## 后果（Consequences）

**正面**

- 图片调用与当前官方端点、编码和安全边界一致；无效模型和无效图片输入会得到明确失败。
- 参考图不会以文字摘要替代二进制内容，模型才会真正接收到编辑输入。

**负面 / 代价**

- 已配置为 Imagen 的旧渠道会立即失败，需要管理员迁移到受支持的 Gemini 图片模型。
- OpenAI edits 需要处理 base64/data URL 解码和 multipart 上传；实际模型对图片数量、尺寸和字段的限制仍需部署环境验收。

**需要长期承担的义务**

- 每次图片模型或 API 版本变动时复核 `docs/llm-provider-specifications.md` 与 Model Catalog 能力。
- 真实凭据验收要覆盖文生图、编辑、供应商拒绝、安全拦截与无密钥失败路径；不得以本地编译代替上游验证。

## 验证方式（Verification）

- 静态检查：仓库不再存在 `generateImageGoogleImagen`、`:predict`、`ALLOW_ADULT` 或默认 `BLOCK_NONE` 图片请求路径；`image_to_image` 只走 `/images/edits` multipart。
- 工程检查：`go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`、`node scripts/agent-notes.mjs check`、`node scripts/check-project-skills.mjs` 与定向 `git diff --check`。
- 部署验收：使用脱敏真实渠道分别验证 OpenAI 文生图、图像编辑、Gemini 图片、已退出 Imagen 的 422 和上游安全阻断；不记录密钥或原始图片。
