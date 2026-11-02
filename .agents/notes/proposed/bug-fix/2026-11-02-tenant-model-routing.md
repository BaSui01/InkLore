# Agent Note: 租户模型渠道隔离与策略冻结并发修复

Status: proposed

Task: INK-228
Scope: internal/llm/infrastructure/tenant_channel.go、provider_router.go、internal/agent/application/run_service.go

## Problem

原有组合根的 DBResolver 查询模型渠道和密钥时未限定 tenant_id，且在非事务模型调用中使用 UoW.Require。查询失败后 Router 又回退到全局 Pool/环境凭据，导致租户隔离、选定模型和实际调用渠道无法形成可审计闭环。Agent 已经保存 policy_version_id，但策略头在创建 Run 的事务外查询，可能与发布切换竞争。仅传模型名称还可能选中同名的另一提供商模型。

## Proposal

- 只对带租户和精确 Model ID、冻结 PolicyVersionID 的 Agent Chat 请求启用数据库渠道选择。LLM Infrastructure 查询 llm_channel_models 并逐表匹配 tenant_id/启用/删除状态，使用真实映射的 upstream_model 和渠道协议。
- 非事务读取使用 UoW.DB；选路、密钥读取失败一律返回错误，不回退其他租户或全局环境凭据。密钥限环境变量或已有加密引用，未接通的 Secret Store 明确不可用。
- Agent 先经 LLM 公共应用服务获得所属租户的 Model ID；非唯一上游名称拒绝；内部传输 metadata 不加入发送给 Provider 的 JSON。
- 在 Run/Job/Event 的现有 UoW 中查询当前启用策略，并通过 PostgreSQL FOR SHARE 与发布更新序列化。已运行 Run 仍按保存的不可变 PolicyVersionID 读取配置。
- 不改动已有历史迁移或冻结 API/DDL。兼容滚动窗口、Creator 安全目录及真实费用链继续由 INK-228 独立验收。

## Alternatives considered

- 只在原 SQL 增加 tenant_id：无法解决 Require 非事务失败、fallback、模型重名和协议错配，不采用。
- 将全局环境变量当租户渠道失败的后备：会将作者请求发往未获租户绑定授权的 Provider，不采用。
- 新建并行 Router 或拆微服务：现有 ProviderRouter、LLM Catalog 和 PostgreSQL 能表达本需求，增加系统复杂度，不采用。

## Acceptance criteria

- Go build、vet、gofmt、archguard、前端和 Skills/notes CI 通过。
- 两租户相同上游名称配不同 Key，只会调用所持租户和 Model ID 对应渠道；无配置、禁用、凭据缺失或未知协议不得产生网络请求，也不得使用全局密钥。
- 同租户同名模型不得随机选第一个。上游请求的模型名来自选中的渠道映射而不是进程默认。
- 策略发布和 CreateRun 竞争要么先取得旧版本并原子提交 Run/Job，要么使用新版本；失败不留半写入记录。
- 在隔离 PG/Redis 和受控模型渠道运行真实成功/失败、跨租户和发布竞态后方可标记完成；静态 CI 不是网络/数据库验收。

## Risks

- 灵感/质检此前可隐式使用无租户数据库渠道，现在必须拥有租户级 inspiration/review 策略；尚未建立策略的租户将收到显式配置错误。集成环境必须验证这两个领域的权限、预算与实际生成结果。非租户调用仍沿用显式进程级 Provider。
- 灵感分析与合规质检也通过租户当前启用的 inspiration/review 任务策略获取版本、Model ID 与渠道。未配置该租户策略时显式失败，不以全局环境渠道代替。该链尚待真实模型与权限回归。
- 当前 Secret Store 尚无真实读取能力时将 fail closed。已执行数据库迁移必须保持原校验和；渠道模型多对多映射依赖现有 000017。
- 现有 LLM 使用量/费用明细与供应商请求的精确渠道 UUID 尚未完成端到端对账，本补丁不能宣称计费准确。
