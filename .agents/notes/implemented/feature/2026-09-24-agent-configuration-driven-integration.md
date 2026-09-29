# Agent Note: Agent配置驱动链路的集成边界与未验收项

Status: implemented

Task: INK-226
Scope: cmd/admin seed manifest、internal/agent Run/workflow/worker路径与 Creator/Admin 配置治理页面

## Problem

INK-223 的种子和人工审阅功能扩展为多文件实现后，集成时需要区分旧资料描述与当前源代码，并检查空配置、失效执行器、跨租户模型选择是否被错误伪装为成功。R1 冻结了运行/事件和作者确认边界，但没有 task-key 目录或运行中问答事件；不能通过新增 wire 字段或静态默认值掩盖能力缺口。已检索 INK-223 的活跃工程笔记；该笔记记录初始人工审阅链，本篇补充当前集成与未通过运行验收的事实，不替代其历史实现背景。

## Decision

当前创建链从 Creator 安全目录、CreateRun 请求到 Agent DB 配置版本读取；Admin 用现有治理 API 编辑并 CAS 更新，seed manifest 只为缺少的默认资源执行事务播种，不在重复运行时覆盖现存运营配置。定义版本需声明请求 task_kind；工作流版本在读取时解码并校验 DAG；指定模型须经当前租户已启用模型目录授权。worker 缺失 system instruction 或工具执行结果时失败关闭，不能把配置/执行错误转换成成功空内容或 `OK`。Creator 显式取消通过冻结的 POST cancel API 按最新 run revision 提交；离页只 unsubscribe，不会暗中发 cancel。作者只能审阅 artifact 并显式更新本地草稿，正文事实仍归 content。

## Alternatives considered

- 每次 seed 都按 manifest 强制覆写配置能保持初始清单完全一致，但会静默销毁 operator 的可变配置与版本发布，故重复 seed 采用已存在则保留；当前实现因此尚不能把修改清单当作运营配置升级机制。
- 在 Creator 增加固定 task-key 列表或为请求增加澄清事件可以减少输入错误，但会造出与租户 DB 脱节的业务选项，且新事件违反冻结 wire；保留作者输入并由后端验证，缺失的 task catalog 另需正式 R1.x 评估。
- 工具注册动态导入可减少执行器接线，但会让配置或模型获得任意代码/命令能力；只允许稳定受限注册及明确 Port，未知执行器失败关闭。

## Consequences

系统不再以未执行工具的伪结果冒充成功，并拒绝未授权模型及与 definition 不匹配的 task_kind；静态构建无法证明配置数据库、worker、模型或事件持久化行为。seed 对已有数据只保留，不因清单版本更新而迁移既有配置；应通过明确、可审阅的 Admin 发布操作管理运营配置。Run 写入 policy_version_id，但 `ModelGenerateInput` / `AgentTaskModelGateway.Generate` 仅传 task_key，本轮无法证明 worker 实际使用运行创建时冻结的同一 policy 版本。工具外呼失败仍未在本轮证明完整 execution receipt/unknown 状态行为。`internal/agent/tool/infrastructure` 与 `internal/agent/runtime/application` 仍存在旧业务 prompt、task key、动态角色/fallback，本轮 reachability 检查显示不从当前 cmd 入口可达，但源码尚未删除/改为配置。Admin `modelPresets.ts` 不仅静态列举模型与价格，还被模型创建、导入与费用试算路径实际消费，仍是 A 类硬编码产品数据，不应当作真实租户模型目录。另有 `cmd/admin/seed.go` 要求 Definition `task_kinds` 必须同时命中同名 policy key 的硬编码隐式映射；`AgentTaskModelGateway.Generate` 在 task key 为空时以 node key 兜底，需改为 fail-closed。另外 `scheduler.go` 在 `Gateway.Generate` 返回任意错误时均登记 `agent.execution_outcome_unknown`；当前尚无实网证据区分未配置 Provider/模型授权失败等确定性拒绝与已发出但结果未知的上游调用。需要之后基于 LLM gateway 错误类型和数据库/Provider验收收窄映射，不能将静态分支当真实错误结果。

Creator task-key目录、审批决策API以及事件外动态澄清未在R1提供。

## Verification

集成终审复核（2026-09-24）再次执行：`go build ./...` 0、`go vet ./...` 0、`go run ./cmd/archguard -reachability` 0（R1–R5通过，15包不可达，包含旧 Agent runtime/tool）；Creator/Admin `pnpm run build:*` 均 0；正式 i18n 检查 924/924、notes、Skills、`git diff --check` 均 0。补充只读检查 66 个变更源码文件均小于 50KB且未检出 Unicode Emoji；事件合同未新增 `ask_question`/reasoning delta。`node docs/system-design/tools/check-design.cjs` 退出 1，指出 `api/agent-core-schemas.json` 冻结契约增量（七个 CreateRun 创作分支新增可选 `model_id` / `model_key`，generated runtime request schema 对应更新）及 `09-DOMAIN-LLM-GATEWAY-SPEC.md` hash drift；后者对 HEAD 无工作区 diff，无法判断何时引入。前者来自继承的模型自选改动，未见本轮 R1.x 批准；ADR-0027 的 task policy catalog/version proposal 不涵盖 model 选择字段。本终审未更动这些资产，契约差异待用户裁决。没有隔离PG/认证会话，未做 seed/API/worker/SSE/模型/Owner写回实测；未启动真实浏览器登录链，相关DoD仍阻塞。继承的冻结API文件本终审仅读取并保留，未编辑测试文件。
