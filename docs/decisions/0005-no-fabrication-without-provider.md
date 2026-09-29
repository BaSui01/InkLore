# ADR-0005：未配置模型 Provider 时默认拒绝生成，离线演示需显式开启

- **状态**：Accepted
- **日期**：2026-09-17
- **决策者**：项目负责人
- **关联任务**：INK-039
- **替代关系**：无

## 背景（Context）

~~ProviderRouter~~ 在未配置 API Key 时会走「内置创意引擎」：生成模板正文、模拟思考链，并推送一个 ~~call_fact_check_001~~ 的工具调用与 ~~success~~ 回执。

这与 README 的承诺直接冲突：

> 未配置 Provider 时，系统会明确返回模型网关未配置，不会伪造小说内容。

更严重的是伪造的**工具调用事件**：它会在前端与审计记录里留下一次**从未发生**的工具执行，比伪造正文更具误导性——读者看到的是「系统真的调用并校验过世界观」，而实际上什么都没有发生。

## 决策（Decision）

1. **默认拒绝**：未设置 ~~LLM_BUILTIN_FALLBACK~~（或为假）时，未配置 Provider 的所有生成入口一律返回 ~~ErrProviderNotConfigured~~，对外表现为 HTTP 503 + ~~llm_provider_not_configured~~。
2. **显式开启离线演示**：仅当 ~~LLM_BUILTIN_FALLBACK=true~~ 时启用内置模板引擎，且必须同时满足：
   - 启动时与每次生成都打印 WARN，说明输出是模板推演；
   - ~~ProviderInfo~~ 的 name / type / capabilities 标明「离线演示」与 ~~template_based~~；
   - 结果 ~~Model~~ 固定为 ~~builtin-creative-engine~~；
   - **绝不产生任何 tool_call / tool_result 事件**。
3. **工具调用对话不接受内置引擎**：~~GenerateChat~~ 在任何模式下都只走真实上游，TAO 循环必须真实执行。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 直接删除内置引擎 | 零歧义，代码最少 | 无 Key 时无法做任何界面演示 | 损失离线演示能力，而该能力已有可标记的实现基础 |
| 保持现状（静默回退） | 用户无感 | 与 README 冲突；伪造执行轨迹 | 不可接受 |
| 默认开启内置引擎 | 开箱即用 | 默认行为就是伪造内容 | 与「不伪造」契约冲突 |
| 仅删除伪造工具事件，保留正文回退 | 改动最小 | 默认仍会输出模板正文冒充作品 | 未解决主要矛盾 |

## 后果（Consequences）

**正面**

- 默认行为与 README、ADR 契约一致：未配置即明确失败，不产出任何假内容。
- 伪造的执行轨迹被彻底移除，审计记录不会出现未发生的工具执行。
- 离线演示能力保留，且可观测（日志、ProviderInfo、Model 名、能力标记）。

**负面 / 代价**

- 无 Key 的首次运行会直接得到 503，需显式设置开关才能看到演示内容。
- 前端需要能正确展示该错误态（当前会显示后端返回的 code 与中文文案）。

**需要长期承担的义务**

- 新增任何生成入口都必须继承同一门控，不得绕过 ~~ProviderRouter~~ 自行回退。
- 任何模式下都不得再引入 tool_call / tool_result 的伪造事件。

## 验证方式（Verification）

1. 探针（tmp，跑完即删）14 项断言：默认模式下 ~~Generate~~ / ~~GenerateStream~~ / ~~GenerateEventsStream~~ / ~~GenerateChat~~ 全部返回 ~~ErrProviderNotConfigured~~，且流式与事件流**产出为零**；开启开关后 ~~Generate~~ 可产出模板内容，但事件流中 tool 事件数**恒为 0**。
2. ~~POST /v1/llm/generate~~ 默认返回 ~~503 {"code":"llm_provider_not_configured"}~~。
3. ~~GET /v1/llm/provider/status~~ 默认返回 ~~type=none~~，开启后返回含 ~~template_based~~ 能力标记的 ~~smart_builtin~~。
