# ADR-0032：Agent 引导式创建小说

- **状态**：Accepted；用户明确要求由真实 Agent 提问、提供建议并引导创建小说
- **日期**：2026-09-28
- **决策者**：BaSui
- **关联任务**：INK-236
- **替代关系**：替代 INK-206/INK-231 记录中的“用户在一次性表单手填完整元数据后直接创建”交互；保留 Content 对作品的唯一写入权威及 ADR-0027 的 Question 安全边界。

## 背景（Context）

当前 Home 的 `CreateNovelDialog` 是一次性静态表单，直接 POST Content；它没有询问 Agent、逐轮收集用户意图或生成候选建议。R1 通用 AgentRun 要求绑定现有资源，而开书前尚无 Novel 资源；复用不可达的 legacy runtime `ask_user` 也无法证明请求进入当前 `cmd/server` 运行路径。用户要求在创建小说前由真实 Agent 主动提问、给建议并允许作者输入。

## 决策（Decision）

在 `agent` bounded context 增加持久化 Novel Creation Guide session。它加载当前租户已启用的 `novel_architect` definition version，并通过 Agent 的 LLM Gateway 解析当前已发布 `novel_architect` task policy；工具列表由服务端固定提供 `ask_user` 与 `propose_novel`。模型必须返回工具调用，问题/建议与提案经过服务端字段白名单及长度校验后保存，普通文本或未知工具调用不视为成功。

Guide 初始输入是作者灵感。`ask_user` 每轮只能创建一个问题，建议来自真实模型输出；作者可选建议或自由填写答案，答案在下一轮作为 untrusted user input。最多8轮，session使用revision CAS并将对话安全数据持久化至 Agent 专属表，状态可在刷新后读取。服务端不得把模型工具参数当作作品写操作。

`propose_novel` 只返回可编辑的作品标题、题材、简介和目标字数。Creator UI呈现方案并等待作者显式确认；最终写入仍通过现有 Content `/v1/content/novels` 创建操作、Idempotency-Key 和 `content` application，owner固定为已认证 Creator。关闭/拒绝提案不创建作品。Agent或当前tenant模型渠道不可用时返回真实错误，不显示伪造问题/建议。

每个工具调用使用既有 root `novel_architect` task policy 与 SecretRef 机制；没有已启用渠道时，必须由 Admin 以 environment SecretRef配置 tenant channel。密钥值不入库、不经浏览器传输、不写日志。用户答案、问题与提案只用于本人、当前tenant的引导session，不覆盖正文或事实账本。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 保留一次性静态表单 | 代码和网络操作少 | 不满足 Agent 逐步提问、建议与作者输入的明确要求 | 用户明确要求 Agent 主动发问引导 |
| 把未创建的Novel伪装成通用AgentRun资源 | 可复用运行面板与SSE | 违反 resource/owner snapshot 契约，Run绑定不存在的事实资源 | 开书session在创建前不是Novel资源；使用独立Agent引导聚合 |
| 复用 `internal/agent/runtime` 中不可达的 `ask_user` | 表面上已有同名工具 | 不从当前server运行路径可达，且不能证明持久恢复/真实回写 | 使用可达的 `internal/agent/application`、Gateway与HTTP装配 |
| 让模型直接创建作品 | 操作一次完成 | 模型会成为Content写入口，用户失去最终确认且边界难审计 | `propose_novel`只能建议，作者确认后由Content创建 |

## 后果（Consequences）

**正面**

- 创作者得到真实逐轮访谈、可选建议和自由输入；刷新后可恢复当前问题。
- 所有作品落库仍经过既有Content owner、tenant、revision和幂等事务边界。
- 模型、定义和策略由tenant治理，凭据仍经环境 SecretRef提供。

**负面 / 代价**

- 新增Agent guide session表、CAS/幂等命令、Question/Proposal Schema及Creator页面状态。
- Agent体验要求tenant同时具有启用的 `novel_architect` 定义、task policy、model和channel；未配置时功能明确失败。
- 每轮真实LLM请求产生上游费用，最多8个问题；API超时后基于session revision恢复，不能盲目创建第二个session。

**需要长期承担的义务**

- `ask_user`/`propose_novel` 工具只接受固定安全Schema，不能由模型生成权限、SQL、任意task config或资源ID。
- 每次session突变必须有当前Creator tenant/owner校验、CAS和Idempotency-Key；日志不得包含问题答案、正文、API key。
- Agent不能在最终确认前写入Content，不得直接写章节、世界观事实或作品正文。

## 验证方式（Verification）

验证未配置渠道时不产生假问题/作品；配置 environment SecretRef 后真实调用 `novel_architect` policy，由模型返回 `ask_user` 工具调用并显示动态问题/建议；作者答案引发下一轮模型调用；`propose_novel` 提案经作者可编辑/拒绝，拒绝无Content记录，确认经Content API创建并回显owner作品。测试跨tenant/其他用户session 404、错误revision/重复Idempotency-Key、非法工具参数、8轮上限、模型超时及刷新恢复。执行双端/Go构建、Schema/DDL生成校验、archguard/i18n/notes检查。不新增测试文件或明文密钥。
