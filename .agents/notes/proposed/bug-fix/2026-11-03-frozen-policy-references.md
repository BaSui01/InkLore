# Agent Note: 阻止已引用冻结策略误删并保留模型 ID

Status: proposed

Task: INK-264
Scope: internal/llm/infrastructure/catalog.go、internal/app/adapters/agent_task_model_gateway.go、cmd/ink228verify

## Problem

已有 Agent Run 使用 llm_task_policy_versions 的不可变 PolicyVersionID。当前 Catalog.Delete 的 InUse 仅查看 reserved/dispatched llm_reservations，而 Agent Run、Agent 问答和小说引导会话持有的冻结版本未计入引用。对任务策略进行物理删除，可能先删除版本，然后 Worker 再尝试读取时得到不存在的错误。没有与 CreateRun 持有的 FOR SHARE 锁同步，删除还可能与创建 Run 并发竞争。

小说引导保存已经从冻结策略得到的 ModelName，但 Gateway 又把同一个名称重新查询成 ModelID。两个 Provider 合法共享同一个 upstream_name 时，该查询返回歧义失败，造成已有明确 ModelID 的引导失败。

## Proposal

任务策略引用检查覆盖当前活跃 LLM 预留、全部 agent_runs、agent_run_questions 和 agent_novel_creation_sessions 的 PolicyVersionID。进行政策 InUse 检查时在当前 UnitOfWork 内对策略头取得 FOR UPDATE，与 CreateRun 的 FOR SHARE 串行化；持有引用时拒绝物理删除并返回既有 llm.resource_in_use，而不是临时迁移或静默软删除。Store.Delete 重复进行引用检查，防止绕开应用层检查直接误删。

模型生成使用冻结策略读取到的 ModelID；当 input.Model 是相同的冻结模型名或 ID 时，不做模糊二次解析。不同的显式模型选择仍通过现有租户 ResolveEnabledModel 路径，跨租户或同名歧义仍失败关闭。

## Alternatives considered

- 只查询活跃 LLM 预留：无法覆盖排队 Agent / 引导会话，拒绝。
- 删除策略同时级联删除 Agent Runs：破坏审计与历史冻结语义，拒绝。
- 将同名模型选择改为随机选第一个 Provider：可能使用错误密钥，拒绝。
- 改写已经在生产应用的 R1 迁移和冻结 API：违反数据库契约治理，不采用。

## Risks

所有历史 Agent 引用都会阻止物理删除策略，这是有意的保守安全限制。后续真实清理需要独立保留策略、归档或软删除的正式合同，不能通过取消本守卫绕过。该修复不覆盖模型/供应商删除时的所有已冻结引用，相关路径应单独进行同等级验收。冻结 ModelID 方案也不代替真实供应商调用、费用与 Creator 浏览器交互验收。

## Acceptance criteria

- Go build/vet/gofmt/archguard、Skills/notes、GitHub 必需 CI 通过。
- 临时 PG17/Redis7 执行全部迁移与两租户运行；排队 Agent 的策略在 InUse 中必须返回 true，物理 Delete 必须返回 ErrInUse，之后仍能读取原冻结版本。
- 同租户构造两个 Provider 的相同 upstream_name，名称歧义拒绝，已冻结 ModelID 仍选原租户渠道及 Key。此实验只调用环回网络，不使用真实模型密钥。
- 不修改已应用数据库迁移、正式 API、生产配置或真实 Secret；仅在最新 head 通过 CI、独立 Review 后进入 dev。稳定 private 的 #41 仍按发布验收单独保护。
