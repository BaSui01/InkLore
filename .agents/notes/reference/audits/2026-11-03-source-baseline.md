# InkLore 去阶段化交付与真实源码基线

日期：2026-11-03。仓库：[BaSui01/InkLore](https://github.com/BaSui01/InkLore)。关联任务：[Issue #15](https://github.com/BaSui01/InkLore/issues/15)。交付：[Draft PR #110](https://github.com/BaSui01/InkLore/pull/110)，目标 dev，保留人工审查，不合并。

业务源码基线为 [dev@60279c40f5c81982c69d035c0c886931d22334cd](https://github.com/BaSui01/InkLore/tree/60279c40f5c81982c69d035c0c886931d22334cd)。本轮在同基线的既有文档整理提交 b0732b31 上追加治理改造，保留其 ADR 和过程记录迁移，避免再开重复搬迁 PR。下文文件与行号指业务基线；治理改动以 PR diff 为准。本文是带日期的审计证据，不维护第二份任务状态看板。

## 1. 结论与证据口径

本次移除的是历史阶段对当前工作的约束：唯一 R1 设计入口、设计文件哈希冻结、固定实施顺序、强制 INK/backlog 登记、全面测试禁令、固定领域数量和一律 ADR-First。当前规则改为真实源码与实际装配确定现状、需求与风险确定修改、按影响验证、通过受保护 PR 交付。

本次没有重写创作业务、公开 API 或数据库历史。正文与事实所有权、tenant/owner 授权、UoW、CAS、幂等、策略版本快照、审批、租约 fencing、未知外部结果失败关闭、密钥保护和历史记录封印继续保留。仓库仍有合理的 r1 路径、数据值、兼容别名和模型名称；“字符串清零”不是验收目标。

源码显示 InkLore 已有真实领域存储、章节版本、事实事件、持久 Run/Node/Job、模型调用及 SSE。作者从开书到长篇创作、确认事实、质检和导出的链路仍有确定断点，不能称为完整可工作的长篇小说 Agent。主要原因是未接通既有能力、手写 SDK 与契约分叉、重复前端修订状态和缺依赖时仍返回成功；没有证据要求新建微服务、第二套任务引擎或第二套事实系统。

状态定义沿用用户要求：

- CONFIRMED：所描述边界内有完整真实源码实现；不是运行通过的同义词。
- PARTIAL：有部分实现，但产品链或生产装配不完整。
- ABSENT：在注明范围内检索后未找到可用实现或入口。
- UNCERTAIN：证据不足，不能确认存在或不存在。
- RUNTIME_VERIFIED 与 NOT_RUNTIME_VERIFIED 单独记录。本文业务能力表均为 NOT_RUNTIME_VERIFIED；构建、静态检查及限定 CI 场景不能代替作者完整业务验收。

详细证据：[治理影响审计](2026-11-03-governance.md)、[Agent / LLM 审计](2026-11-03-agent-llm.md)、[创作与数据审计](2026-11-03-creation.md)。这些是本轮修改前的证据快照；其中“建议修改”与“未修改”的叙述不覆盖随后实施的治理改造。

## 2. 访问与仓库基线

| 项目 | 实际状态 | 证据与边界 |
| --- | --- | --- |
| 仓库读取 | 可读；当前为 public | GitHub 连接器 get_repo、目录/文件读取和本地 clone 成功；没有借历史截图推断源码 |
| 默认/稳定分支 | private，读取时 f44f02809d6b64b80b23f9044dcc10ad9ce8940a | 仓库 metadata 与分支读取 |
| 开发分支 | dev，60279c40f5c81982c69d035c0c886931d22334cd | 审计前读取，交付准备时再次读取未变化 |
| 提交/PR 写入能力 | metadata 的 push/maintain 为 true；Git 对象、分支及 PR 写工具可用 | 本轮复用独立分支 chore/agent-decision-docs-cleanup-20261103 与 Draft #110；实际追加提交及 PR 状态见链接，不直接写保护分支 |
| Issue 写入能力 | 连接器暴露 create/update Issue，metadata 允许 push；未执行写探测 | 用户尚未批准任务写入；未为探测权限创建或修改 Issue |
| Milestone 写入能力 | 可读取；当前连接器未提供创建/修改工具；本地 gh 未登录 | 公开 REST 读取到 5 个里程碑；不能从读取或 metadata 推断可创建。待获写入批准后若仍受限，再生成可执行 gh 脚本 |
| 分支保护 | dev/private 标记 protected；可见 3 个 required checks | 项目 Skills 与工程记忆校验；后端（构建 / vet / 格式 / 架构红线）；前端（类型检查 / 构建） |
| 完整保护配置 | UNCERTAIN | 连接器读取 protection 详情返回 403 Resource not accessible by integration；不能声称已核完 reviewer、bypass 等全部设置 |
| 自动合并 | 仓库启用能力；工作流限制敏感路径 | safe-auto-merge.yml 仍拒绝 Draft、manual-review、非 dev、广泛变更、.agents/.github/scripts/持久化等敏感路径；本 PR 不申请自动合并 |

初始快照有 12 个 open PR。与本任务直接相关的是 #110（已有文档搬迁）、#108（既有里程碑/验收规划）、#102（邮件 schema 基础）、#109（dev 到 private 发布）、#63（Staging 预览）。其余包括依赖升级与分支协调。本轮只复用 #110，不改他项分支或合并状态。

## 3. R1 影响分类与处理

修改前逐文件行号、21 项影响清单及完整规则分类见[治理审计](2026-11-03-governance.md)。本轮处理结果如下。

| 类别 / 规则 | 判断 | 本轮处理 | 保留理由或剩余风险 |
| --- | --- | --- | --- |
| R1 设计是唯一实现入口 | REPLACE | 重写 AGENTS、README、当前架构、开发指南、项目治理与 Wiki 导航 | 历史设计不再高于真实实现和当前用户需求 |
| 恰好七域、所有跨域一律多层抽象 | SIMPLIFY | 记录现有布局；按职责与消费者决定抽象 | 保留领域 Owner、禁止跨私表/跨 infrastructure、platform 不反向依赖 |
| 禁止所有测试文件 | REPLACE | 删除 archguard 的测试文件禁令，允许针对具体风险的验证 | 保留其余四类拒绝检查；机械文案不强制堆测试 |
| 设计 Hash / Freeze Manifest | REMOVE | checker 不读取哈希冻结，不支持 --freeze；manifest 作为历史资料保留 | 不重算漂移以伪装设计验证通过 |
| 固定 FR 覆盖、阶段文档大小和待评审禁令 | REMOVE | 从检查器去除；生成器停止重建固定需求覆盖报告 | 保留真实 refs、operationId、权限、幂等、CAS、路径参数、错误及表 Owner/FK 检查 |
| 当前 DDL 必须等于初始迁移 | REPLACE | 退役逐字比对；check-r1-platform 改名 check-platform-contracts，保留错误码同步 | 真实结构正确性通过迁移历史与隔离重放判断，不重写初始迁移 |
| 新笔记必须在旧 backlog 有 INK 行 | REPLACE | Task 接受真实 Issue/PR URL、#编号；兼容历史 INK | 格式检查不替代任务真实性，不再复制状态看板 |
| Accepted ADR 永远有效、局部修改一律 ADR-First | SIMPLIFY | 明确历史假设可被证据替代；重要长期选择记录原因，常规修改不过度仪式化 | 原 ADR 原文与独特理由保留，索引说明替代范围 |
| Skills 与 DSH 固定阶段/任务顺序 | REPLACE | 同步三个项目技能、模板与角色提示词 | DSH 模型、供应商、工具及权限配置未改 |
| 重复设计与阶段完成记录 | REPLACE | 历史设计、UI 规划、旧 backlog、旧架构、交接和截图转 reference；当前 docs 提炼真实说明 | 生成器消费的 API JSON、聚合 SQL 及工具保持原路径 |
| 内部 llmr1 导入别名和当前 UI 文案 | SIMPLIFY | 改稳定能力名称及注释，保留未接线的界面限制说明 | Go token 等价核对通过；不改公开错误、路由或持久化值 |
| 普通 CI 固定 17 次迁移 / 80 张表 | REPLACE | 按仓库实际版本、名称、校验和核对；隔离 schema 与同轮主库比较实际表集合 | 非固定版本阻断；同名但历史内容变化仍失败 |
| staging 权限覆盖仅适用前 17 次迁移 | NEEDS_DECISION | 明确保留权限复核 guard，不更改授权 SQL | 新迁移涉及新表授权，不能自动扩大 GRANT；需另行安全审查 |
| migrations/r1、历史 SQL 与 checksum | KEEP | 字节不变；不重命名历史流 | 当前启动、迁移 CLI 和管理命令有消费者 |
| R1Runtime、历史数据导入工具 | KEEP / NEEDS_DECISION | 当前新入口为 Runtime；兼容别名和导入格式保留 | 仓内无调用不能排除外部 harness；删除需补消费者证据 |
| Redis r1 前缀、上下文键、formula_version、错误目录版本值 | KEEP | 不作无兼容计划的替换 | 有运行或数据语义；改名字可能重置限流/缓存或破坏记录 |
| DeepSeek R1；archguard R1/R2/R3/R5 | KEEP | 保留 | 分别是模型名称和规则标识，不是产品阶段 |
| archived note 封印 | KEEP | 保留内容哈希与 --base 历史防篡改 | 保护不可变历史，与冻结当前设计用途不同 |

测试禁令的替代不是强制全量测试：业务/安全问题按可观察风险补正反例；低影响文案和机械重命名用构建、差异与针对性检查即可。当前检查器改造用隔离夹具验证了“合法演进可通过、真实违规仍失败”，避免靠删掉检查获得绿灯。

## 4. 当前能力矩阵

下表“运行”均指该完整能力，N = NOT_RUNTIME_VERIFIED；不存在把编译、生成类型、接口 200 或旧 note 的 done 当业务证据。更细的 20 项 Agent 和 26 项创作能力见分报告。

| 能力 | 状态 | 源码证据 / 当前边界 | 运行 | 已有 Issue/PR |
| --- | --- | --- | --- | --- |
| 注册、密码会话、真实书架读取 | CONFIRMED（基础链）；注册验证码 PARTIAL | CreatorSessionContext → identity_http/service；注册挑战字段未贯穿全链 | N | #97、#107 |
| 邮件 OTP / 生产找回密码 | PARTIAL | forgot-password 生产分支503、reset token进程内；邮件 PR 仅基础工作 | N | #98 / PR #102 |
| Content API 手工建书并初始化世界观 | CONFIRMED | content/application/novels.go:26–45，统一写事务 | N | #107 |
| 作者可达的手工建书 UI | ABSENT | CreateNovelDialog 无引用；Home 创建入口均转 guide | N | #107 |
| Agent 引导开书 / 只创建一次 | PARTIAL | novel_guide.go、novel_creation.go、AgentNovelGuidePage；完成命令缺事务，确认未绑定 session | N | #4、#107 |
| 创建卷章、读取正文、服务器保存 | CONFIRMED（服务边界） | content/application/chapters.go、versions、scope/outbox 同事务 | N | #107 |
| 20章以上目录、分卷、指定章节导航 | PARTIAL | sdk.ts丢cursor；Editor按返回数组置于第一卷；navigation仅editor保留novelId | N | #106、#107 |
| 保存冲突比较与作者选择 | CONFIRMED（当前边界） | Editor 409 + DiffView、基于服务端revision显式重存 | N | #107 |
| 历史版本浏览与回滚产品流程 | PARTIAL | 后端version/rollback存在；SDK/UI入口缺失 | N | #107 |
| 本地持久草稿恢复 | PARTIAL | recovery包存在，Editor未消费；内存意图和手工下载不等于崩溃恢复 | N | #106、#107 |
| 大纲、节拍、伏笔、场景 | PARTIAL | outline服务/表真实；页面未消费完整nodes，scene chapter_id/node_id错配 | N | #15、#107 |
| 人物、设定、关系图 | PARTIAL | worldview实体/关系真实；手写请求必填及显示字段不一致 | N | #15、#107 |
| 节奏分析产品体验 | PARTIAL | 后端tension 0–100；UI使用0–10与不存在的整体分数 | N | #107 |
| 章节定稿与版本连续性 | PARTIAL | world-revision查询路径错；保存/确认两份revision失步 | N | #107 |
| 事实提案审阅、冲突处理、入账 | PARTIAL | CommitFacts的事务/hash/CAS真实；UI永远空proposals且无审阅闭环 | N | #107 |
| 长篇事实与模型上下文协同 | PARTIAL | 账本/Context实现存在；生产Run未把章节、大纲、设定送模型 | N | #107 |
| Agent 定义、工具、Workflow配置 | CONFIRMED（管理边界） | definitions.go/tools.go、Admin AgentGovernanceRoute、版本仓储 | N | #15、#106 |
| Run 绑定实际策略与不可变版本 | CONFIRMED（基础机制） | run_service.go:153–333、AgentTaskModelGateway读取策略 | N | #3、#83 |
| PostgreSQL Run/Node/Job、租约/fencing | CONFIRMED（基础机制） | jobs_adapter.go、run_repository.go；未知外部结果保守失败 | N | #3、#105 |
| TAO 多轮及 DAG 工作流执行 | PARTIAL | Scheduler真实循环；tool节点与Definition规则冲突，InputBindings不消费 | N | #15、#107 |
| 动态小说工具、MCP、审批续点 | PARTIAL | 生产executor仅platform_utc_now；MCP/副作用/需审批未绑定，审批Act续点不完整 | N | #106、#107 |
| 通用 Run 澄清与原 Run 恢复 | PARTIAL；运行时 ABSENT | migration000011与生成类型存在；应用状态、实际路由和UI事件缺失 | N | #104、#106 |
| SSE 持久重放 | CONFIRMED（服务基础） | event_service.go按Last-Event-ID重放/鉴权 | N | #105、#106 |
| 模型逐字输出、断线/刷新恢复 | PARTIAL | 同步GenerateChat后发整段delta；heartbeat abort不重连，run引用仅内存 | N | #103、#106、#107 |
| 作者显式采纳 AI 建议 | CONFIRMED（编辑器边界） | target/revision/原文快照/partial守卫，采纳后仍普通保存 | N | #107 |
| 租户模型选择与真实 Provider 调用 | CONFIRMED（源码链） | agent_task_model_gateway、provider_router、tenant_channel，无全局key回退 | N | #3、#83、#103 |
| 预算、费用、渠道池、限流熔断 | PARTIAL | 管理服务与实现存在，GenerateChat未完整经过Reserve/Settle/Admission/Pool | N | #103 |
| 规则扫描 / 深度审查 / 改写 | PARTIAL（规则后端完整） | matcher持久报告真实；UI失败可显示通过；ExecuteAudit/Rewrite无调用 | N | #107 |
| 灵感分析、拆书、风格与卡片 | PARTIAL | 领域API/表存在；无Creator入口，ExecuteAnalysis无生产调用 | N | #107 |
| 导入、可下载导出、带规划复制 | PARTIAL | Files/Reviews未注入可伪成功；复制flags未执行且缺world初始化 | N | #107、#15 |
| tenant/owner、事实Owner、历史不可变性 | CONFIRMED（源码机制） | scope/SQL过滤/UoW/触发器存在，不能据此宣布所有越权反例已通过 | N | #105、#107 |
| 通用请求 Schema / 路由覆盖运行门禁 | ABSENT（这一通用机制） | requestschema.Initialize/Validate/CheckRoutes均return nil；局部DTO校验仍有效 | N | #15 |
| Admin 管理与运行审计完整性 | PARTIAL / UNCERTAIN | 配置、日志与审计页面存在；预算实际消费和失败可观测性不完整，未逐项实跑所有管理操作 | N | #103、#106 |

## 5. 核心创作链路

| 用户操作 | UI | API | 业务服务 | 持久化 | Agent/模型 | 整体结果 |
| --- | --- | --- | --- | --- | --- | --- |
| 注册并打开书架 | AuthSession、Home | identity + content list | identity/content | users/memberships/novels | 无需模型 | 基础实现真实；验证码与邮件能力分开验收 |
| 引导建立作品 | AgentNovelGuidePage | /v1/agents/novel-guide/*，确认另调content create | NovelCreation + Content.CreateNovel | guide session/command + novel/world revision | 同步真实模型、独立于持久Run | PARTIAL：事务完成、断线恢复、session绑定欠缺 |
| 建立大纲人物世界观 | OutlineRoute、WorldviewRoute | outline/worldview | 各领域应用服务 | nodes/scenes/entities/relations | AI蓝图无完整调用 | PARTIAL：SDK字段和作品路由断点 |
| 编辑并保存章节 | Editor、ApiClient | content chapters | SaveBody → writeBody + UoW/CAS | body/version/writing events/scope | 无需模型 | 服务端链完整；UI revision与目录另有缺口 |
| AI续写/润色/多步创作 | Workbench | agents/runs + SSE | RunService → Scheduler → Gateway | Run/Node/Job/events/artifacts | 真调用，但正文/设定上下文未接入 | PARTIAL：无法证明针对当前作品有效创作 |
| 作者审阅采纳并定稿 | Workbench/Editor/ConfirmModal | content save/confirm → worldview | Content + WorldFactAdapter | chapter/version + fact events/heads同UoW | 模型产物只为建议 | PARTIAL：路径、revision与空事实提案阻断 |
| 规则质检与深度改写 | ComplianceReviewModal | compliance scan/audit/rewrite | matcher/ExecuteAudit/ExecuteRewrite | reports/issues/decisions/rewrites | 深度Execute未接worker | PARTIAL：规则后端真实，UI/执行交接不完整 |
| 导出可下载作品 | Creator入口缺失 | content jobs export | CreateExport + 应有Files/Reviews | jobs/snapshots；可能无output_file_id | 无需额外模型 | PARTIAL：渲染字节可被丢弃却写succeeded |

## 6. 实际架构问题与建议

以下是源码审计优先级，不是经竞品确认后的正式 GAP 或获批实施计划。建议复用现有 Issue，不在本轮业务改造。

| 问题 | 证据 | 根因 | 作者/运营影响 | 最小建议 |
| --- | --- | --- | --- | --- |
| 开书回答后完成命令缺事务（P0） | novel_guide.go:78–108 → repository:207–225 → uow:80–84；Agent A-01 | BeginCommand事务结束后，用原ctx完成命令 | 回答已保存仍报错，command可能停processing | 短事务完成/失败、session与命令终态协调，隔离PG验证 |
| 模型缺作品上下文（P0） | domain_services.go:83–89、178–184；scheduler:417–427；A-02 | 资源读取只保留hash/revision，ContextText只有工具observation | “续写当前章”没有当前章文本、选区、大纲或世界设定 | 接已有只读上下文端口，保留快照/权限，不造第二套记忆 |
| 合法工具节点不可执行（P0） | dag.go:50–55 vs scheduler:374–387；A-03 | 校验与执行分发要求相反；绑定仅校验不消费 | 多步流程能保存但工具节点失败、下游无上游结果 | 按节点类别执行并消费既有InputBindings |
| 预算与真实模型调用断开（P0/P1，依付费开放范围） | domains.go:118–143；gateway_service:87–96；A-05 | 管理服务与运行路径分离，成本未赋值 | 预算页面不能证明受控消费，0费用不能代表免费 | 复用现有预留/结算/限流单一路径并对账 |
| SDK与接口真相分叉（P1） | requestschema/registry.go:17–29、sdk.ts、worldview contracts；CR-01 | 手工请求不受生成契约约束，通用校验是no-op | 人物/场景/图谱/修订路径失效 | 收敛现有生成类型与视图适配，补真实路由/输入验证 |
| 保存→定稿→再编辑版本不连续（P1） | Editor:343–359、1053–1062；ConfirmModal:42–70；CR-02 | revisionRef与activeChapter双份状态 | 404后还有409风险，状态显示可能陈旧 | 统一采纳服务端chapter快照，保留CAS拒绝 |
| 空提案显示“事实入账”（P1） | ConfirmModal:67–70、108–111；facts.go:97–137；CR-03 | UI承诺超出实际提交 | 作者误以为章节事实被提取并用于长篇一致性 | 明确保存/正文确认/事实确认，接现有提案审阅 |
| 缺必要依赖仍报告成功（P1） | app/domains.go:71；content/jobs.go:42–64、353–398；CR-04 | Files/Reviews可nil且被静默跳过 | 空导入预览、导出100%却无下载文件 | 组合根接线，缺依赖明确失败，验真实文件与快照 |
| 长篇目录及作品上下文丢失（P1） | sdk:44–46、123–130；navigation:28–35；Editor:817–839；CR-05 | 丢分页元信息、默认第一页、路由不携作品 | 超20章被截断，页面可能转第一本或错误章节 | 复用书架无限查询，按volume/position及稳定路由展示 |
| 错误状态变成“全部通过”（P1） | ComplianceReviewModal:76–79、180–201；CR-06 | 成功条件只看空数组，未核report与error | 失败可能误导作者采纳旧报告 | 显式成功/失败/stale状态，建议与源快照绑定 |
| 多套Agent实现与无人消费的执行入口（P1） | 旧runtime/loop、动态工具与ExecuteAnalysis/Audit/Rewrite；A/CR-07 | 代码存在但生产组合根未接入 | 框架容量与真实可用能力不一致 | 优先现有Scheduler交付最小作品链，逐消费者迁移后再删 |
| 全聚合加载与长事务（P2，待测量） | export逐章查询；outline Store.Load；chapter organization锁；CR-12 | 小操作复用完整聚合/渲染置于事务 | 随长篇规模增长的查询与锁成本风险 | 先测查询数和锁持有时间，再定向读/缩小事务；不声称已发生性能事故 |

## 7. 与既有任务去重

| 已有任务 | 本轮对应证据 | 处理边界 |
| --- | --- | --- |
| [#15](https://github.com/BaSui01/InkLore/issues/15) 架构治理与去R1 | 规则、checker、架构事实、契约分叉 | 关联本 PR；旧正文中的hash/固定七域要求已被当前用户任务替代；未改Issue |
| [#4](https://github.com/BaSui01/InkLore/issues/4) 开书只创建一次 | A-01、A-07、A-08 | 应复用，不另开重复“导览完成”总任务 |
| [#103](https://github.com/BaSui01/InkLore/issues/103) Gateway协议/预算 | A-05、流式/供应商运行验收 | 有服务和协议文档不等于真实预算闭环 |
| [#104](https://github.com/BaSui01/InkLore/issues/104) 通用澄清 | A-06 | 已有DDL不能重复报成“完全没有任何基础” |
| [#105](https://github.com/BaSui01/InkLore/issues/105) 隔离运行验收 | PG/Redis/权限/恢复 | CI迁移冒烟与真实模型/浏览器范围分开 |
| [#106](https://github.com/BaSui01/InkLore/issues/106) 双端Agent体验 | 审批、澄清、SSE、恢复、产物状态 | 需要与#107具体用户任务拆分去重 |
| [#107](https://github.com/BaSui01/InkLore/issues/107) 完整作品交付 | A-02/A-03与CR-01至CR-12主要闭环 | 总目标已存在，后续只在批准范围补具体缺口与验收 |
| [#3](https://github.com/BaSui01/InkLore/issues/3)、[#83](https://github.com/BaSui01/InkLore/issues/83) 策略/引用 | Run快照和Worker消费的已有实现 | 相应修复PR已有合入；不把现有正确机制再次列为缺失 |
| [#97](https://github.com/BaSui01/InkLore/issues/97)、[#98](https://github.com/BaSui01/InkLore/issues/98) 身份/邮件 | 注册challenge传递、SMTP/OTP | PR #102仅部分schema基础，不抢改其业务分支 |
| [#2](https://github.com/BaSui01/InkLore/issues/2) 旧冻结验收 | 设计hash退役后的语义差异 | 移除hash不等于问题全部解决，不自动关闭 |
| [PR #108](https://github.com/BaSui01/InkLore/pull/108) 已有里程碑规划 | 当前已存在M0–M4与#103–107 | 与新治理的状态来源冲突需要后续协调，当前不改、不合并 |

已存在的五个 Milestones 是 [M0 安全与 CI 基线](https://github.com/BaSui01/InkLore/milestone/1)、[M1 身份与模型服务闭环](https://github.com/BaSui01/InkLore/milestone/2)、[M2 Agent 持久执行与策略可信](https://github.com/BaSui01/InkLore/milestone/3)、[M3 Creator 小说创作闭环](https://github.com/BaSui01/InkLore/milestone/4)、[M4 Staging 验收与受保护发布](https://github.com/BaSui01/InkLore/milestone/5)。这是读取到的现状，不是本轮新建或已批准的新方案。

## 8. 验证证据与未通过项

工具环境：Go 1.27.1、Node 24.22.1、pnpm 11.25.0。验证作用范围为当前治理改动，未运行生产迁移、共享数据库写入、真实模型调用或作者浏览器业务验收。

| 检查 | 结果 | 证明范围 |
| --- | --- | --- |
| go build ./... | PASS | 全部Go包可编译 |
| go vet ./... | PASS | 当前静态分析通过 |
| gofmt -l cmd internal | PASS，输出为空 | Go改动格式 |
| go run ./cmd/archguard -reachability | PASS | 跨infra/平台逆依赖/500KiB/语法门禁；109个非cmd包中85可达、24不可达为信息项 |
| go run ./cmd/i18ncheck ... | PASS，920/920 keys | zh-CN/en-US基准key集合；不是文案质量证明 |
| pnpm install --frozen-lockfile | PASS | 未修改依赖声明或lockfile |
| pnpm run build | PASS | Creator/Admin tsc与Vite；当前UI文案改后重跑 |
| Skills / notes --base 60279c40 | PASS；12技能Markdown、51相对目标、101生命周期笔记，历史封印与基线一致 | 元数据/链接/历史封印；不判断业务完成 |
| 全树Markdown目标 | PASS；最终240个Markdown、1118个目标，其中954个相对路径全部存在 | 包含隐藏目录与新归档；不验证外网或fragment |
| 治理工具正反例 | PASS，28/28 | 合法Task/长历史资料/测试文件可通过；错误权限/CAS/refs/Owner/FK/跨infra/语法/历史篡改仍拒绝 |
| 迁移CI正反例与脚本语法 | PASS；8/8行为检查、YAML/Bash/Python语法 | 真实Go loader的17项清单通过；缺项、等数量错版本/名称/hash、多项和重复版本拒绝；隔离新增第18项含CRLF通过 |
| 生成器隔离对比 | PASS | 34输入相同，70产物逐字节相同；仅OpenAPI标题描述/README及不再生成旧FR报告有预期差异；重复生成确定 |
| Go业务token等价 | PASS | 18文件中17个仅注释，domains.go仅4个导入别名等价；排除主动改逻辑的archguard |
| migrations、配置、依赖清单、权限SQL及现有API JSON/DDL | PASS；相对dev基线无差异 | 历史SQL、机器协议源及生成资产不因清理改变；不代表现有契约无缺陷 |
| 最新PR CI/隔离PG/Redis | 以PR对应最新head记录为准 | 不沿用旧提交绿灯；不将限定迁移/身份冒烟扩写为完整创作运行通过 |
| PowerShell脚本、DSH实际调度 | NOT_VERIFIED | 环境没有pwsh与已配置DSH；DSH非提示词配置等价已单独验证 |
| 真实模型、完整浏览器、共享Staging | NOT_RUNTIME_VERIFIED | 未获得这些场景的当前运行证据 |

治理工具与生成器的具体结果见 [治理检查夹具结果](2026-11-03-governance-checks.json) 、[迁移历史检查结果](2026-11-03-migration-checks.json)和 [生成器/Go等价验证](2026-11-03-equivalence-checks.md)。

**静态契约检查仍失败，不能宣称全绿：**

1. check-design.cjs 保留语义检查后报告 `idempotency missing llm_fetch_upstream_models`；本轮扫描246个operation、7844个ref、77个聚合DDL表。
2. check-platform-contracts.mjs 报错误码目录漂移：源目录102个，Go97个，缺 agent.question_already_answered、agent.question_binding_mismatch、agent.question_expired、agent.task_policy_version_changed、auth.invalid_token。
3. 本轮未修改 API 幂等声明或错误码业务来掩盖问题。二者原本不在三个required CI jobs中；新增的 make contracts-check 保持显式可调用，不把既有失败偷偷设为必须通过，也未禁用原有必需检查。
4. 普通迁移检查与权限overlay范围保护分开。overlay固定17依然可能在未来追加迁移时阻断，原因是需要审核授权集合，应有具体安全评审后再改。

基线 dev 曾有 [CI 成功记录](https://github.com/BaSui01/InkLore/actions/runs/41589879732) 和 [隔离集成成功记录](https://github.com/BaSui01/InkLore/actions/runs/41589879727)。它们只证明当时工作流覆盖的范围，不是本轮新head或完整业务通过的证据。

## 9. 不确定项与高风险后续

| 问题 | 已尝试检索/检查 | 尚缺证据 |
| --- | --- | --- |
| R1Runtime与离线导入工具能否删 | 仓内Go/脚本/YAML、BootstrapR1/R1Runtime调用及migration格式 | 外部harness/历史迁移使用者与兼容计划 |
| 旧Agent runtime能否整体删或合并 | 组合根、构造调用、实际worker、go list可达性 | 动态/外部消费者、仍被引用的ports迁移；不能只按不可达列表删包 |
| 全Provider可用性/费用 | 协议代码、模型路由、预算/限流构造和调用点 | 授权真实渠道、usage对账、失败/取消/重试证据 |
| 数据权限完全正确 | Owner/tenant过滤、UoW、触发器和现有CI脚本 | 针对具体创作动作的双租户/并发反例；完整保护配置仍403 |
| 长篇性能 | N+1、全聚合加载、渲染事务边界源码 | 长作品数据集、查询数、p95、锁等待；没有实测不报事故 |
| Run总时限/容量/恢复 | deadline只见存取；count-active→create；checkpoint/租约路径 | 并发容量、崩溃、断线、未知上游结果的隔离运行矩阵 |
| 上下文token预算准确性 | worldview ReadContext使用JSON字节len；当前生产Run尚未接入 | 具体模型token计量及上下文裁剪质量；先修接线再量化 |
| 共享Staging当前状态 | 读取工作流和历史记录，未连接共享DB | 当前配置/可用性/真实运行；历史记录已从当前操作指南分离 |
| 新迁移后的私有schema授权 | 保留权限overlay guard与拒绝测试 | 新表/序列/历史不可变对象的授权差异审查 |

## 10. 下一批准点

本轮交付只覆盖已授权的低风险治理与源码审计，不创建或修改 Milestones/Issues，不合并 PR，不修复上述业务断点。

按用户任务第七节，先提交 Sudowrite、Novelcrafter、NovelAI 的候选官网与研究价值；确认候选后才开始正式竞品差距研究。差距清单/里程碑主题另有批准点，完整任务草案及 GitHub 写入再分别确认。现有 Issue 和 Milestone 将作为去重输入，不因标题带旧阶段词就删除或重建。
