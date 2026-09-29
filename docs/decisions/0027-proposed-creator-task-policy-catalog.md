# ADR-0027：Creator 安全任务策略目录、Run 版本绑定与中途澄清

- **状态**：Accepted；用户明确批准两项 R1.x 范围
- **日期**：2026-09-24
- **决策者**：用户
- **关联任务**：INK-226（既有复核）；INK-227（契约实施计划）
- **替代关系**：补充 ADR-0016 的 Agent CreateRun、安全目录和执行交互契约；不改变其他冻结决策

## 背景（Context）

R1 Creator 目录目前只有 Agent Definition/Workflow，task_key 由客户端自由提交；Admin 的 LLM 策略治理响应不能暴露给 Creator。agent_runs 已有 policy_version_id，但 CreateRun 与真实 Gateway 调用必须以不可变策略版本为权威，不能只存版本号、执行时按 task_key 重查当前版本。另一个缺口是 TAO 执行中无法安全等待作者提供缺失信息：approval.required 是对工具副作用的授权，不能复用于普通澄清。

遵守模块化单体、七域 Owner/DAG、Agent 只产生 Artifact 建议、Run 配置版本冻结、幂等/CAS、PG 持久作业、SSE 持久回放及外部副作用未知不重发。策略仍由 llm 域拥有；Agent 只定义消费方 Port 并经 app/adapters 调用 LLM 公共应用服务，不跨域读表。

## 决策（Decision）

接受且仅接受以下两项 R1.x 变更：Creator 安全任务策略目录与 CreateRun 不可变策略版本绑定；Agent 中途澄清完整持久交互。除此不扩展其他功能。

1. `GET /v1/agents/catalog/task-policies` 面向 creator audience/creator role/tenant scope，使用公共 opaque cursor 分页，排序 `task_key ASC,id ASC`。仅返回本租户当前已发布、enabled、未删除且能力授权的策略安全投影：`policy_id`、不可变 `policy_version_id`、`task_key`、`title`、`version`、由Admin在不可变LLM policy version中发布的`task_kinds`、经严格白名单裁剪的必要输入 JSON Schema 与预算概览。不得返回 system instruction、Provider secrets、内部路由、模型/渠道凭据、工具参数。标准认证与错误封包一致；空目录200空页，跨租户不泄露存在性。目录经 Agent consumer Port 调 LLM 应用服务读取。 `input_schema`由Agent应用按策略版本task_kinds从Creator CreateRun既有分支Schema生成安全白名单投影，只用于渲染现有task_kind/resource/instruction输入，不引入任意policy_inputs字段或LLM内部参数。
2. Creator 七个 CreateRun 创作分支必须提供 `task_policy_version_id`，不得提供 `task_key`。服务端在 Run/Job/幂等同一UoW内锁定/核验tenant、策略启用/current published version、task_kind兼容和定义能力；由版本数据派生持久 task_key、冻结 policy_version_id。若策略更新/停用与创建竞争，串行化后要么绑定提交时仍 current 的版本，要么无 Run/Job 返回409 `agent.task_policy_version_changed`。Admin受控 diagnostic 仍使用既有 task_key 输入并服务端解析冻结版本。既有 `agent_runs.policy_version_id` 是等效字段，不增加重复的task_policy_version_id；task_kinds由llm域写入`llm_task_policy_versions`不可变版本并纳入Admin创建/发布Schema。迁移000010新增该版本列，旧版本回填空数组而不猜映射、且不进入Creator目录；迁移000011新增Agent Question。不得建立跨业务域私表FK。
3. Worker每次模型调用都须携 Run 冻结的 `policy_version_id` 到 Agent→LLM Gateway Port；LLM公共应用服务按该不可变ID读取策略内容，校验tenant归属，不按task_key解析当前头。策略后续发布或停用不改变已建Run的快照或正在执行的版本；停用阻止新Run，不破坏已冻结Run。保存DDL引用为UUID，不越域外键。
4. 旧 Creator task_key 请求只保留一个滚动发布兼容窗口：升级前旧实例可写入原 CreateRun；切换期服务端对旧形态以 tenant 下当前 enabled 策略解析并冻结版本，未知 key fail-closed；部署确认所有 Creator 已升级后关闭旧形态，缺少 policy version 的新请求400。兼容不是长期映射表或代码硬编码，不允许由task_kind、角色、模型名推导策略；sunset由版本化部署门禁完成，任务验收记录旧形态关闭时间/证据。请求同时带task_key与policy_version_id始终400。
5. 新增 `waiting_clarification` Run/Node 状态与独立 `AgentQuestion`。作用是向发起Run的Creator询问完成当前节点所需的明确用户输入，不是授权工具执行；不得用 `approval.required` 代替，Approval仍只批准工具副作用。每个Run最多8个问题、同时最多1个open；question 作为不可变ID/version=1记录，绑定 tenant/run/node、policy_version_id、definition_version_id、源resource revision/hash、发起者及 expires_at。问题内容有 title/help locale keys（UI翻译）、安全渲染用的纯文本context摘要（≤1000码点，不含隐藏推理）、response_schema；event只携用户可见题目结构，不携隐藏推理。
6. Question schema仅允许安全单选、最多20项多选或文本三种JSON值；schema≤8KiB、深度≤8、只允许本地结构/枚举与长度限制、拒绝远程 `$ref`、`default`、条件/组合schema、任意HTML、工具权限/task config/秘密字段。整体题面≤4KiB，选项≤20，每答案≤4KiB。提问必须来自冻结workflow/definition声明的澄清能力，模型不能自定义执行权限或改变task配置；服务端再次校验schema及task policy允许字段。答案永远按untrusted data输入新一轮上下文，明确标注为用户补充，不覆盖用户当前正文/事实/作品源revision，不直接写Owner数据。
7. 发起问题时以同一UoW checkpoint写node/run waiting_clarification、Question(open)、PG job retry_wait、事件 `question.required` 与 `node.updated`；不持有worker lease、不伪造进度/心跳。Run唯一job保留原job_id和绝对deadline，等待不消耗重试次数。用户退出/断流不影响Run；SSE按现有Last-Event-ID重播，另提供本人 GET question 查询以恢复 UI。
8. 回答命令 `POST /v1/agents/runs/{run_id}/questions/{question_id}/answer`，creator audience/role且发起者本人、当前作品scope和membership均有效；body `{expected_revision,question_version,answer}`，必须 Idempotency-Key。后端按job→run→node→question锁序和revision CAS，重新检查当前权限、question尚open、未过期、question/run/node/policy/version/source绑定相同、严格响应Schema、source资源仍授权且源版本未变。第一次合法答案在同一UoW把Question转answered并记录actor/answer/hash，Run/Node回queued/ready、恢复原job并写 `question.answered`、`node.updated`/outbox及幂等响应。重复同key同payload重放原结果；同问题并发不同key仅一次CAS成功，其他409 `agent.question_already_answered`；相同键异载荷按公共409幂等冲突。
9. `question.required` / `question.answered` 为持久化SSE事件，统一 `{run_id,seq,occurred_at,payload}`、按Last-Event-ID回放7天。required payload只含question_id/version/node_id/title_key/help_key/response_schema/options/context_summary/expires_at/source_revision/source_hash；answered payload只含question_id/version/answer_hash/actor_id/answered_at，不在事件回显原始答案。不得包含隐藏推理、工具参数、Secret或正文全文。event append与对应question/run/node状态原子提交，事件seq严格单调；终态唯一约束仍成立。
10. 超时由PG now()及持久job定时恢复收敛；Question过期→expired、节点/run failed (`agent.question_expired`)并在同UoW写 `run.failed`，不能自动跳过或伪造答案。总Run deadline仍为创建时冻结的30分钟（包括等待）；Question TTL为创建时服务端限制的1..15分钟且不超过Run deadline。取消与答复、超时、撤权竞态按锁序+CAS：首个有效事务获胜；已取消/终态的question不可回答。撤权立即拒绝读/答且清客户端敏感缓存；服务端保留等待状态直到原deadline/问题期限收敛，不允许被撤权主体恢复。恢复只从该Run checkpoint继续未完成node；已完成执行/外部副作用绝不重发。未知副作用仍 `agent.execution_outcome_unknown`。
11. 前端 Creator RunInspector展示等待问题、校验答案、提交命令并订阅同一Run后续事件；退出、刷新、重连先GET Run/Question，再以Last-Event-ID恢复，绝不新建Run。Question UI不得渲染HTML。Admin审计只显示脱敏状态摘要；ApprovalCard路径与权限完全分离。

### R1.x 范围边界

本次仅包含上述任务策略安全目录/版本冻结消费和 Agent 中途澄清的协议、持久化、事件、恢复与端到端验收设计。明确不包含自动任务/策略推断、角色到task_key静态映射、Agent直接写正文/事实、开放式工作流生成权限、Admin代答/代批、Approval语义变更、更多问答轮数扩展、跨Run问答、外部副作用重试、微服务拆分或其他新增功能。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| Creator直接读LLM Admin策略API | 少一个consumer port与投影 | 扩大管理信息暴露，违反audience隔离 | 采用Agent consumer Port安全投影 |
| 客户端提交task_key/按task_kind静态映射 | 兼容和界面简单 | 可伪造、策略漂移、违反零硬编码 | 必须选policy version，由服务端派生key |
| Run只存policy version、Worker仍按task_key解析当前头 | DDL改动少 | 实际调用可能漂移，历史Run不确定 | 版本ID必须贯穿每次Gateway调用 |
| 澄清复用approval.required | 复用现有组件/事件 | 将信息收集误当副作用授权，错误地依赖工具票据 | Question有独立聚合、状态和权限 |
| 仅发SSE不落Question/答案 | 不需迁移表 | 断线/退出不可恢复，无法CAS/幂等 | PostgreSQL事务持久Question+事件 |
| 回答后创建新Run | 逻辑看似简单 | 丢失Run checkpoint并可能重发副作用 | 同Run恢复原node/job和冻结版本 |

## 后果（Consequences）

**正面**

- Creator可见目录是按tenant、发布状态、能力与输入约束过滤后的最小安全投影。
- Run创建、LLM实际调用与策略版本具有单一、可审计绑定；Admin后续操作不漂移历史运行。
- Clarification支持离页恢复、持久回放与同Run续跑，并与审批权限彻底隔离。

**负面 / 代价**

- 新增Creator目录API、CreateRun schema迁移、Answer命令、Question持久表/事件/错误、Agent→LLM Port、前后端状态和完整部署兼容窗口。
- 等待期间占用Run生命周期窗口；question Schema必须由服务端严格子集实现与持续拒绝危险结构。
- 旧客户端过渡要求协调服务端/Creator部署，sunset前后必须能明确观察；禁止永久兼容分支。

**需要长期承担的义务**

- 每次CreateRun和每次模型调用都校验冻结policy version，不得从运行时task_key回退。
- 问题/答案必须由PostgreSQL持久化并在同一事务提交状态与事件；所有恢复遵守租约epoch/fence、CAS及不重发边界。
- Creator目录、事件及错误不得泄露prompt、Provider secrets、工具参数或隐藏推理；答案作为不可信数据。

## 验证方式（Verification）

设计资产由 `node docs/system-design/tools/build-contracts.cjs` 重建，并由 `node docs/system-design/tools/check-design.cjs` 检查；源契约与增量迁移更新后必须刷新freeze manifest。实现验收列入INK-228..231：策略目录跨租户/发布能力过滤、策略修改竞争与Worker真实版本消费；question schema反例/权限/回答竞态/过期/取消/撤权；PG故障原子回滚；SSE从Last-Event-ID续传；断线后GET恢复同Run且不增加模型/工具调用；Admin脱敏与Creator端到端回答续跑。当前为契约设计，不代表运行功能通过。
