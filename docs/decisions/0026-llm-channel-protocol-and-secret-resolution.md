# ADR-0026：LLM 渠道协议与凭据解析收敛

- **状态**：Accepted
- **日期**：2026-09-20
- **决策者**：BaSui
- **关联任务**：INK-218
- **替代关系**：补充 ADR-0004、ADR-0005、ADR-0016 与 ADR-0025；不改变 R1 的公开 Provider 枚举和图片生成功能范围。

## 背景（Context）

INK-217 的官方协议核验发现，现有运行链把模型名和 base URL 当作协议判断依据，环境渠道池没有把所选渠道的协议带到驱动；仅配置数据库渠道时，ProviderRouter 又会在调用前误判为“未配置”。这会让 Gemini OpenAI 兼容路径、Gemini 原生路径和 OpenAI Images 路径发生错误分派。

同时，旧图片适配器仍会把 Imagen 模型请求发送到已经关闭的 Gemini API `:predict` 路径，将参考图摘要写入 prompt 后伪装为 OpenAI 图像编辑，并把 Gemini 可调安全阈值全部显式设为 `BLOCK_NONE`。管理端还允许把 API Key 录入为 SecretRef 定位符，运行期会把 `secret_store` 定位符直接当作密钥内容使用。这违反 ADR-0004 和仓库的密钥红线。

R1 对外 Provider 枚举仍只有 `openai`、`deepseek`、`anthropic`、`gemini`、`ollama`，图片生成不扩展为 R1 公开产品能力。`openai_responses`、`gemini_interactions` 和 `gemini_openai` 只能是基础设施内部传输模式。

## 决策（Decision）

渠道的公开 Provider 与内部 transport protocol 分开保存和解析：`LLM_CHANNELS[].provider` 使用五种 R1 Provider 值，`LLM_CHANNELS[].protocol` 可选指定其受支持的内部传输模式。选路结果必须携带该 protocol；未指定时使用 Provider 的原生协议。数据库渠道也必须在实际解析到有效的环境 SecretRef 后才视为可调用，优先级按数值小者优先。

图片适配器只在已知的原生协议上调用：OpenAI 图像编辑使用 `/v1/images/edits` 和真实 multipart `image[]` 文件字段；Gemini 原生图片请求继续使用已存在的 `generateContent` 适配器。Imagen 模型名、Gemini Interactions 图片路径、Gemini OpenAI 兼容图片路径和其他未验收 transport 必须明确失败，不能猜测端点或退化为文本提示词。

Gemini 请求不再默认传入全量 `BLOCK_NONE` 覆盖，未配置专门安全策略时省略供应商 safety override，由上游默认保护与 InkLore 合规域共同约束。管理端只录入 SecretRef 的 kind 和 locator；当前部署没有 Secret Store 运行时适配器，`secret_store` 引用必须 fail closed，不能把 locator 当成密钥。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 继续用模型名或 URL 推断协议 | 无需调整渠道配置 | 同一模型别名、代理端点和兼容 API 会误分派，无法审计 | 与真实供应商合同冲突 |
| 把 Images 接口扩展为 R1 公开能力 | 可直接在管理端展示完整图片配置 | 需要变更冻结 API、DDL、策略能力、预算和前端产品范围 | 本任务只修复已有内部路径，不扩大 R1 |
| 将 Secret Store locator 作为临时 API Key | 表面上支持两种 SecretRef | locator 与凭据混淆，可能把错误文本发送给上游 | 违反 ADR-0004；待真实 Secret Store adapter 再启用 |

## 后果（Consequences）

**正面**

- 渠道实际选中的 transport、端点与认证方式可确定，环境和数据库配置均不会因为“默认 OpenAI”被错误调用。
- OpenAI 编辑请求会上传真实图像二进制；不支持的图片模式不会再伪造成功路径。
- API Key 不再经浏览器表单或数据库定位符明文传播，缺失 Secret Store 实现时安全地拒绝配置和调用。

**负面 / 代价**

- `LLM_CHANNELS` 中旧的 `openai_compatible` 值需改为 `openai`，并应显式写出非默认 `protocol`。
- 当前没有 Secret Store driver 的部署只能使用已存在的环境变量 SecretRef。
- Gemini Interactions 的图片输出尚无独立内部适配器，使用该 transport 的图片请求会被拒绝，直到其输入/输出对象以官方合同独立实现并验收。

**需要长期承担的义务**

- 每次新增内部 transport 或图片模型能力前，须先核验其端点、请求对象、安全策略与输出，并同步 Model Catalog/渠道能力，不以模型名推断。
- 接入 Secret Store 时必须新增仅在同步调用范围暴露字节凭据的 adapter，不能恢复 locator 直传。

## 验证方式（Verification）

- `LLM_CHANNELS` 解析拒绝未知 Provider、非法 provider/protocol 组合和缺失密钥引用；选路目标携带渠道 protocol。
- 未配置上游时图片调用仍返回 `ErrProviderNotConfigured`；Imagen、未实现 transport 和非 data URI 的 OpenAI 编辑参考图明确失败，不发起错误请求。
- `go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`、工程笔记与项目技能检查、`git diff --check` 必须通过。
- 没有脱敏测试渠道与付费凭据时，不进行供应商实网生成；该边界必须记录在 INK-218 验收中。
