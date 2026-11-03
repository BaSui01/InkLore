> 源码基线：[dev@60279c40](https://github.com/BaSui01/InkLore/tree/60279c40f5c81982c69d035c0c886931d22334cd)。这是源码审计，业务行为均未在本次运行验收；治理变更与构建证据见[总报告](2026-11-03-source-baseline.md)。

# InkLore 治理与 R1 影响只读审计

基线：`60279c40f5c81982c69d035c0c886931d22334cd`；本报告为修改前审计，路径及行号均指该 SHA。未修改仓库文件，未运行会写入仓库的生成/冻结脚本，未访问数据库或外部模型。

## 结论

R1 的主要实际影响是规范与 Agent 提示词把历史设计提升为永久目标、冻结校验把任意未登记编辑视作错误、CI 禁止测试文件、工程笔记强制依赖 INK/backlog。有效业务能力与安全约束并非应删除对象。`migrations/r1` 由生产应用、迁移 CLI、管理 CLI 直接消费，应原样保留。`archguard` 的 `R1` 是规则编号，DeepSeek R1 是模型名称，两者均不能判作产品阶段污染。

必需 CI 当前执行 Skills/notes、构建、vet、格式、i18n、archguard、前端构建，**并未执行** `check-design.cjs` 或 `check-r1-platform.mjs`。因此不能把去冻结表述成“修好了必需 CI 的冻结失败”；实际必需 CI 的历史治理阻断是测试文件禁令与 notes 对 INK/backlog 的硬依赖。

## 证据和处理清单

| ID | 路径与行号/符号 | 实际影响 | 处理建议 | 风险 |
|---|---|---|---|---|
| GOV-01 | `AGENTS.md:3,168,174,198,224` | 将 R1 设为唯一目标，架构事实/backlog 不得覆盖旧目标，必须按历史计划重验 | REPLACE：当前源码/可复现行为确定事实；现行批准需求确定目标；历史设计仅背景 | 低：仅规则 |
| GOV-02 | `AGENTS.md:50–56` | 必须恰好七域、每域必须四层、每个跨域调用必须 Port+Adapter | SIMPLIFY：列为当前结构，按职责/实际调用演进；保持无循环、领域数据 Owner、禁止跨私表/跨 infra | 中：不可借此更改运行边界 |
| GOV-03 | `AGENTS.md:129–130,194,196`; `cmd/archguard/main.go:8,74,93,151–173` | 禁止所有测试文件，archguard 实际退出非零 | REPLACE：风险驱动验证；允许必要回归测试，低影响机械修改不强制加测试；移除 `checkTestFiles` 的禁令实现，保留其他检查 | 低至中：降低的是有害禁令，非质量检查 |
| GOV-04 | `AGENTS.md:179–182,191–193,201–204`; `docs/project-management.md:5–10` | 每项工作须 INK 编号、backlog 状态、旧计划依赖 done，并机械更新多个文档 | REPLACE：GitHub Issue/PR/Milestones 承载当前任务；既有 INK 为历史引用；只更新受影响文档 | 低：历史记录保留 |
| GOV-05 | `scripts/lib/agent-note-validation.mjs:59–60,96–97` | 笔记 Task 必须命中 docs/backlog 的 INK 表行，强制双系统维护 | SIMPLIFY：兼容历史 INK 引用，同时允许仓库 Issue/PR URL；去除新任务必须登记旧表；保留格式、链接、封印真实性检查 | 中：需验证已有笔记继续通过 |
| GOV-06 | `.agents/skills/inklore-development/SKILL.md:15–25,29–37,43–45`; maintenance `:10,16,34,46`; doc-routing `:3,10–13,22` | 即使 AGENTS 改了，技能仍要求 R1、INK、冻结和禁测试 | 同批同步技能及模板；只保留阅读源码、真实验证、按需记忆 | 低 |
| GOV-07 | `.dsh/SUBAGENTS.md:42–54,64`; preset `agent.cordis.yml:37–38,304–526` | 主代理和 11 个角色提示词仍写 Follow frozen R1；任务从 INK-160 按冻结顺序开始 | REPLACE：按当前用户目标、源码依赖、文件 Owner 执行；保留并发协调和已有授权边界；无需改变供应商/模型配置 | 低 |
| GOV-08 | `docs/system-design/00-INDEX.md:3–5,17,38,43`; `00-CONTRACT-BASELINE.md:1–5,13,72–74`; `12-ENGINEERING-GOVERNANCE.md:3–27` | 唯一施工基线、冻结变更/旧任务顺序仍能限制正常迭代 | 重写治理入口，保留机器契约与必要业务语义；历史只在明确历史区域可回溯 | 低至中：仅清阶段要求，不改 API/DDL |
| GOV-09 | `docs/system-design/18-DELIVERY-AND-ACCEPTANCE-PLAN.md:1–30,69–83` | 历史固定 INK-160…177 顺序及 R1.x 门禁；仍称功能未实施 | 将固定实施顺序降级为历史；保留其中有用的实际集成场景并去除“全部按此开工”的效力 | 低：历史验收编号保留 |
| GOV-10 | `docs/system-design/tools/check-design.cjs:20–24`; freeze manifest | 当前 116 个冻结项中 41 项 SHA 漂移；合法文案或新增文件也失败 | REMOVE：当前设计文件 hash 冻结与 `--freeze` 更新机制；保留语义校验；旧 manifest 可只作历史证据 | 低至中：不要删除整套 checker |
| GOV-11 | `check-design.cjs:6–19` | refs/operationId/权限/幂等/CAS/路径参数/错误映射/DDL Owner/FK 的真实性检查 | KEEP。固定 `FR-01..24` 覆盖约束与“不得出现待评审”宜分离为历史目标检查；不能因去 R1 删有效权限或契约校验 | 中 |
| GOV-12 | `scripts/check-r1-platform.mjs:9–15` | 把当前聚合 DDL 与初始迁移逐字比较，platform/llm 已不相等；末段 error-catalog 同步仍有用 | REPLACE 为通用错误目录同步检查；DDL 改用真实迁移重放/结构验收，绝不重写初始迁移 | 低：无当前 CI/脚本调用，只有历史 notes 提及 |
| GOV-13 | `.github/workflows/ci.yml:19–35,39–71,74–101` | 有效质量门禁与历史 notes/禁测试共存 | KEEP required job 名称及 build/vet/fmt/i18n/frontend；适配 notes、archguard 实现；可加入通用契约校验但不能假称它是此前必需门禁 | 低至中 |
| GOV-14 | `.github/workflows/isolated-integration.yml:19,72`; ink228 `:80` | 作业展示名带 R1；后者明确扫描 `migrations/r1` | 只去展示名阶段标签；迁移目录扫描必须保留 | 低 |
| GOV-15 | `internal/app/platform.go:31,79–85`; `cmd/migrate/main.go:24,63,77,87`; `cmd/admin/main.go:21`; `cmd/admin/root.go:14` | 当前启动/验迁移都依赖 `migrations/r1` | KEEP 历史目录/包/文件身份与数据连续性；注释可去阶段话术 | 高：不可仅为关键词改迁移身份 |
| GOV-16 | `internal/app/app.go:130–133` | `R1Runtime()` 是 `Runtime()` 兼容 alias | 在全仓与可验证外部消费者范围核查后删除或标记兼容；外部验收脚本消费者未知时先保留，不以无仓内调用证明全世界无人用 | 中 |
| GOV-17 | `cmd/migrate/r1migration/{main.go,types.go,deadcode.go,r1_apply.go}` | 历史导入模型 JSON 字段、报告、工具涉及 R1 命名 | 独立 CLI 入口仍存在；保留数据格式、映射身份和默认报告目录兼容，不能由生产应用未调用推定历史导入不再需要；列待审查 | 高 |
| GOV-18 | `frontend/packages/i18n/src/locales/editor.ts:36,56,116,136` | Creator 文案显示“R1安全目录/当前R1界面” | 可去阶段词，保留真实未接通能力说明，不把文案变成假完成 | 低 |
| GOV-19 | `frontend/packages/i18n/src/locales/admin-models.ts:203,427` | DeepSeek R1 模型示例 | KEEP：不是项目阶段 |
| GOV-20 | `.agents/notes/README.md:80–86`; `archive-manifest.json` | 不可变历史笔记封印，不冻结当前设计 | KEEP 历史追溯；不要把此机制与设计冻结一并移除 | 低 |
| GOV-21 | `.github/workflows/safe-auto-merge.yml:87` | system-design/decisions 被敏感路径排除自动合并 | KEEP；本次 PR 需 Draft/manual-review，不能通过迁移路径绕过人工审查 | 低 |

## 工程规则分组判断

| 原规则 | 分类 | 原因与长期规则 |
|---|---|---|
| 模块化单体 | KEEP | 当前部署与代码事实，未见需要微服务的证据 |
| 七域是永久唯一数量 | REMOVE | 当前结构不应变成禁止演进的数量约束 |
| 单向依赖与领域 Owner | KEEP | 防循环、保护事务与数据边界，非 R1 独有 |
| 单实现一律 Port/Factory/Adapter | SIMPLIFY | 按边界和替换需求建抽象，复用当前有效接口，不批量重写 |
| Gin 只留 HTTP 接入/组合根 | KEEP | 有明确层次职责；无需为去阶段化改变 |
| middleware/JSON/SSE/WS 的协议与权限 | KEEP | 有效安全与外部行为，不能变成可随意删除的“旧规定” |
| 238 操作/107 Schema 硬编码规范数字 | SIMPLIFY | 用生成资产和启动检查判断当前数量，文档无需固定数字 |
| 一律禁止测试文件 | REPLACE | 禁令无法区分有效回归测试与低价值测试；改为针对风险验收 |
| Secret Store/env 与不回显密钥 | KEEP | 与阶段无关的安全机制 |
| UTC 存储、北京时间业务语义 | KEEP | 现有业务合同，不涉及去阶段范围 |
| 500KiB 源码大小 | KEEP | 既有用户明确选择，可单独再审，当前无必要改变 |
| 不伪造业务数据/进度 | KEEP | 保护真实功能闭环 |
| 一律 60000 Token/长提示词禁令 | SIMPLIFY | 实际限制以模型/策略和上下文实现为准，当前不动业务算法 |
| Emoji/i18n 与双端独立应用 | KEEP | 产品风格/权限边界非 R1 因果；无需扩大本次改动 |
| 文档永远高于源码事实 | REPLACE | 文档描述目标；当前实现由源码/可复现实测证明；冲突修文档或代码需判断 |
| 每项任务 INK/backlog | REPLACE | GitHub 原生任务为当前主入口，旧编号仅历史关联 |
| 所有架构选择 ADR-First | SIMPLIFY | 重要长期取舍需理由；局部常规实现不制造审批仪式 |
| 接受 ADR 永远不可推翻 | REPLACE | 保留旧记录，允许当前需求/证据替代，不以旧 ADR 永久冻结架构 |
| 同时更新 architecture/backlog/CHANGELOG | SIMPLIFY | 只更新确实受影响内容，避免多处同步同一任务状态 |
| 历史 archived notes 内容不可覆写 | KEEP | 真正历史审计追溯，与当前开发设计冻结分开 |
| 用户未授权的高风险 API/迁移 | NEEDS_DECISION | 本轮只报告兼容成本/影响，保持用户明确审批门 |

## 已存在 GitHub 工作与冲突控制

- **Issue #15** 已覆盖架构审查与去 R1 标签，可复用关联。其历史正文要求重建 hash、维持七域，本轮用户的新任务书已明确授权去除这些约束；不要机械把旧 Issue 文本视为当前更高优先规则。
- **PR #110** 基于同一 dev SHA，负责 ADR 和阶段过程文档迁移，但明确保留冻结体系。新 PR 不应再次批量搬同一批文件；如需合并其路径变更，应在整合分支显式保存来源并审查，不暗中改写 #110。用户未授权合并任何 PR。
- **PR #108** 已在平台创建 5 个 M0–M4 Milestones 与 Issues #103–#107，仓库文档尚未合并。其“GitHub 关闭比例不覆盖 backlog 状态”与本轮 GitHub 作为任务主承载目标有冲突。现阶段只记录，不关闭/删改 Milestones 或 Issues；待用户确认差距/草案后去重调整。
- **Issue #2** 专门跟踪历史冻结 drift，不应在 hash gate 退役后声称其所有验收目标完成。API/DDL 语义差异及真实运行验收仍需独立审查。
- **PR #102** 邮件配置正处于独立实现，标题 post-R1 属于阶段命名线索；不因去阶段任务重写其业务或抢改其分支。

## 最小实施顺序建议

1. 在独立分支修订 AGENTS、开发维护入口、当前治理/设计索引，明确 GitHub 任务入口和历史材料地位。
2. 同步项目 Skills、notes Task 校验、DSH 提示词，让工具实际按新规则运行；保留 archive seal。
3. 从 archguard 删除测试禁令；其余 R1/R2/R3/R5 **编号是规则身份，可保留**。无需为了去阶段把它们重新编号。
4. check-design 去除 freeze hash/--freeze，但保留语义检查；历史 manifest 不再进入运行治理。单独保留 API 与迁移差异证据。
5. 将 check-r1-platform 的错误目录检查改用稳定能力名称；退役初始迁移逐字比较。确认没有遗漏调用引用。
6. 只去当前可见文案与明确无行为变化注释中的阶段词。迁移、JSON 外部格式、模型名、历史验收编号一律不做盲替换。
7. 运行现有构建/静态门禁；对 checker 用有效/无效小夹具验证保留的拒绝能力；不为文案改动增加无价值测试。创建 Draft/manual-review PR，真实说明还未业务验收的内容。

## 必须保留的验证边界

- `check-design` 通过只能证明静态文档/OpenAPI/DDL 部分结构一致，不证明 API 路由真实挂载、数据库升级成功、Agent 可工作。
- `archguard -reachability` 报告不可达包是信息项；其 go list 失败会打印错误但不作为 violation。报告时不能把“退出 0”自动等同可达性完整证据。
- 当前 design manifest 的 41 项漂移已真实计算；未将它们自动重算成新冻结值。
- 未修改已执行迁移，未执行生产/共享数据库迁移，未改真实密钥，未进行竞品正式研究。
