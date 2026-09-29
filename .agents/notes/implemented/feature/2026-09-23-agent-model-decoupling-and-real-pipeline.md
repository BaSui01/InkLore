# Agent Note: 智能体与模型完全解耦、消除假数据Fallback与创作者自选模型闭环

Status: implemented

Task: INK-224
Scope: internal/agent/, internal/app/adapters/, internal/llm/, frontend/apps/creator/, frontend/apps/admin/, frontend/packages/transport/, frontend/packages/contracts/

## Problem

此前系统存在三大严重偏离用户指令与架构铁律的缺陷：
1. 智能体与模型强耦合死锁：调度链路通过 `task_key` 从 `llm_task_policies` 查询策略绑定的默认 `primary_model_id`，导致智能体死死绑定固定模型，前端用户无法根据创作需求自由动态选择推理模型（如 deepseek-chat、gpt-4o-mini 等）；
2. 前端伪造文本假数据恶劣遗留：`EditorRoute.tsx` 在润色/扩写/精简结果未返回时，使用三元表达式硬编码了“（周遭灵力如丝如缕汇聚...）”等伪造小说段落，严重违背真实数据流与作者事实权威原则；
3. 管理端渠道连通性测试为纯前端格式校验：`ChannelModal.tsx` 未发出真实网络探测与延迟测量，产生虚假成功假象。

## Decision

1. 智能体与模型彻底解耦（Decouple Agent & LLM）：
   - `CreateRunInput` 扩展支持携带可选 `model_id` 与 `model_key`，并在 `agent-core-schemas.json` 契约各分支中显式放行；
   - 在 `run_service.CreateRun` 中将用户指定的模型配置安全挂载至根节点运行时 `checkpoint`；
   - `scheduler.ClaimAndExecute` 执行节点时将模型上下文注入 `ModelGenerateInput`；
   - 组合根适配器 `AgentTaskModelGateway.Generate` 优先解析调用方显式指定的模型（验证其为租户内已启用在册模型并映射其 `upstream_name`），仅在未指定时回退至策略默认模型，并向下透传给 `ProviderRouter`。
2. 彻底清除前端假数据与伪造文本（Zero Hardcoded Fake Data）：
   - 物理删除 `EditorRoute.tsx` 中第 508-517 行的三元表达式与假正文 fallback；
   - `startInlineRun` 改为异步轮询或等待真实 `AgentRun` 终态（`succeeded` / `failed` / `cancelled`）与 `listArtifacts` 产物，如果模型生成失败或未产出有效文本，抛出真实错误并由 `ErrorPresenter` 明确呈现，严禁伪造正文。
3. 创作者端动态接入在册模型矩阵：
   - 允许创作者安全只读访问 `/v1/llm/models`，SDK 补齐 `api.llm.listModels`；
   - `EditorRoute.tsx` 动态加载在册启用模型，提供顶部模型选择下拉器，用户选中的 `modelId` 实时透传至调度链路。
4. 管理端真实渠道探测（Ping 探针）：
   - `ChannelModal.tsx` 真实发起带超时的网络探测请求，捕获 RTT 往返延迟或错误拒绝，杜绝假测试。

## Alternatives considered

- 在前端将模型写死为固定字符串：严重破坏租户隔离与模型自由选择，违背用户明确指令，坚决弃用。
- 如果模型失败自动返回原句并静默关闭弹窗：掩盖底层上游或网络异常，作者无法得知执行失败原因，坚决弃用真实报错方案。

## Consequences

智能体与大模型实现完全解耦，用户拥有模型的绝对主权；所有生成内容 100% 来源于真实后端 LLM 与产物账本，零硬编码、零伪造段落。

## Verification

全量自动化编译、双端生产构建、纯净双语词条检验与架构门禁通过。
