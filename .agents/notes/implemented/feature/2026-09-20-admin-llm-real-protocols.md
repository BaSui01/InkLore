# Agent Note: 管理端视觉收敛与真实模型协议运行链

Status: implemented

Task: INK-205
Scope: frontend/apps/admin、internal/llm/infrastructure、internal/app/adapters、internal/agent/infrastructure 与组合根

## Problem

管理端仍有非必要标题小字、主题文字标签和不一致的工具栏层级；更重要的是，ProviderRouter虽已有OpenAI兼容路径，Anthropic、Gemini和OpenAI Responses等真实协议没有进入实际 Agent 运行链，Agent及灵感/质检适配器仍可能返回固定成功结果。资源、文件和 jobs 适配器也存在固定摘要、样本文本和伪造任务 ID。这样会让“配置存在”与“真实上游已调用”混淆，也会让运行记录包含伪造内容或用量。

## Decision

保留冻结的五种公共 Provider 协议枚举。DeepSeek和Ollama使用其实际兼容的 `/v1/chat/completions`；OpenAI Responses通过 `LLM_OPENAI_API=responses` 作为 OpenAI 的内部传输模式，不扩展公共枚举或数据库契约；Anthropic使用 `/v1/messages`、`x-api-key`和`anthropic-version`；Gemini使用 `generateContent`与 `streamGenerateContent?alt=sse`，API key按其协议放在查询参数。所有生成端口经组合根注入同一个 `GatewayService`，缺失配置直接返回错误。

## Alternatives considered

- 继续把所有厂商归一成一套OpenAI请求体：未采用。Anthropic的消息块、Gemini的contents/functionCall和Responses的input/output事件模型不同，伪兼容会在工具调用与流式终态处失败。
- 将 `openai_responses` 加入Provider数据库/API枚举：未采用。R1冻结契约只有五种公共协议，内部传输模式足以支持OpenAI Responses且不会破坏管理端合同。
- 保留固定文本作为无Provider演示回退：未采用。Agent TAO会把伪造输出继续当作事实观察，必须明确失败。

## Consequences

Agent运行创建需要租户下已启用且已发布的LLM策略版本，并把真实 `current_version_id` 固化到运行记录。灵感和质检适配器会解析模型结构化JSON；规则引擎仍是合规权威，模型只提供附加审阅与改写建议。未配置Provider、上游不可用或结构化输出非法时会留下可追踪失败，而不是生成静态报告。真实凭据和生产上游响应仍不在本地验收范围内。
资源快照由对应领域应用服务计算，文件读写由平台文件服务授权，Agent 作业使用真实 `platform_jobs` 行和租约状态；因此审批和重启恢复继续受 revision/hash、租约 epoch 与租户作用域约束。平台服务与资源查询需要真实 PG 才能完成运行验收，不能由编译结果替代。

## Verification

- `gofmt`：本次新增和修改的Go文件已执行。
- `go build ./...`：退出码 0。
- `npm run build:admin` 与 `npm run build:creator`：在 `frontend/` 目录执行，均退出码 0。
- `node scripts/agent-notes.mjs check`：18 篇笔记检查通过；`node scripts/check-project-skills.mjs`：12 份 Markdown、74 个相对链接目标检查通过。
- `go vet ./...`、`go run ./cmd/archguard -reachability`：均退出码 0；Admin Vite 开发入口 `http://127.0.0.1:29001/` 返回 HTTP 200。真实登录态和上游模型网络响应仍待部署环境复核。
- 未新增或修改 `*_test.go` / `*.test.ts`；目标Go文件均小于50KB，协议适配器请求中不打印密钥。
