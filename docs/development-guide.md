# InkLore 开发维护指南与 Skills

本指南是项目流程入口。工程红线见 [AGENTS](../AGENTS.md)，现状见 [architecture](./architecture.md)，设计从 [system-design](./system-design/00-INDEX.md) 进入，任务只在 [backlog](./backlog.md) 管理。

## 三个技能

| 技能 | 使用时机 | 入口 |
| --- | --- | --- |
| inklore-development | 新功能、修复、重构、前后端接线、任务验收 | [SKILL](../.agents/skills/inklore-development/SKILL.md) |
| inklore-decisions | 选型、历史理由、ADR、新旧决定替代、设计治理 | [SKILL](../.agents/skills/inklore-decisions/SKILL.md) |
| inklore-maintenance | 排障、配置对账、文档巡检、发布回滚准备 | [SKILL](../.agents/skills/inklore-maintenance/SKILL.md) |

它们位于仓库级 .agents/skills，随项目版本管理；不覆盖用户全局 skills。支持该目录的工具可发现其 name/description。本次 DSH 会话已发现并成功加载全部三个技能；其他客户端的自动发现能力取决于其配置，不能仅凭目录存在宣称安装成功。

## 如何使用

可直接向代理提出：

- “使用 inklore-development 处理 INK-xxx，先核对依赖和验收条件，再实施。”
- “使用 inklore-decisions 查清章节确认的历史约束，为这个方案准备 ADR。”
- “使用 inklore-maintenance 排查模型配置保存后不生效，记录真实消费路径与证据。”
- “使用 inklore-maintenance 巡检 docs 与源码差异，先登记问题，再修复本次授权范围。”

如果客户端没有列出技能，明确要求它读取相应 SKILL.md 后按链接继续，无需复制一份技能。新会话先读 AGENTS；技能可以组合使用，但不重复建任务，不覆盖已有授权。

## 一次交付的路径

1. 核对规则、当前接线和目标任务，读相关域设计与历史决定。
2. 明确现状、目标、依赖、Owner、验收标准；需要架构决策时先写 ADR。
3. 沿真实调用链实现，保留别人未提交的工作和历史约束。
4. 用构建、护栏、实际请求或浏览器场景验证，不创建项目禁止的测试文件。
5. 同步相应文档、任务与交付日志；验收齐全后从 review 转为 done。

详见 [文档路由](../.agents/skills/inklore-development/references/doc-routing.md)、[验证矩阵](../.agents/skills/inklore-development/references/verification.md)、[验收模板](../.agents/skills/inklore-development/templates/acceptance.md)、[复盘模板](../.agents/skills/inklore-maintenance/templates/incident.md)。

## 与现有文档融合

技能提供执行流程，docs 保存项目事实与设计；不把所有核心文档复制进提示词。AGENTS 与已接受 ADR 仍约束设计；R1 是重构目标，architecture 是运行现状。两者冲突时记录具体条款并裁决，不从“冻结设计”推断“代码完成”。

ADR 保存正式架构裁决，backlog 管理唯一任务状态，[.agents/notes](../.agents/notes/README.md) 集中保存实现因果、隐含约束与失败方案。notes 按四生命周期和六分类组织；非平凡改动先检索、同批更新，纯机械改动免写。已接受 ADR 不照搬上游的原地事实修订；正文保持不可变。来源、采用点与未采用点见 [融合说明](../.agents/skills/inklore-decisions/references/source-adaptation.md)。

## 机械检查与维护

在仓库根运行：

~~~sh
node scripts/check-project-skills.mjs
node scripts/agent-notes.mjs check
node scripts/agent-notes.mjs check --base HEAD
~~~

检查三个技能的 name/description、目录命名、正文行数、Markdown 文件大小、Emoji 与本地文件链接。脚本无第三方依赖；Node 22 可运行。它不校验远程链接、Markdown fragment、ADR 决策正确性，也不声称验证了业务运行。CI 执行技能检查和笔记检查；本地无需安装前端依赖。笔记工具还验证任务关联、节结构与归档哈希；带 --base 才比较既有封印是否被删除或改写。

修改技能或移动被引用文档后重跑该检查。技能正文保持短小，细节放 references/templates；只更新一个权威源，其他地方用相对链接。现有 Go/前端验收按验证矩阵执行。

## 人工场景走查

| 输入场景 | 预期执行路线 |
| --- | --- |
| 修复一个默认值 | 开发技能追到真实消费方；有非显然取舍则写 note 并由任务引用，无需机械新建 ADR |
| 新增跨域写入 | 开发技能核对Owner与Port；决策技能检查已有ADR，违反边界时停止该方案并提出裁决 |
| 按 R1 实施新事件协议 | 同时核对目标、既有ADR与运行协议；完成迁移与验收前不更新为已实现 |
| 只改样式 | 读取相关UI约束、适当构建与浏览器检查，不生成冗余决策笔记 |
| 发布故障 | 维护技能定位版本/探针/依赖，按实际授权修复或回滚，记录恢复证据 |

[Skills 初始交付](../.agents/notes/implemented/process/2026-09-29-skill-delivery-ink-142.md) 和 [工程记忆交付](../.agents/notes/implemented/process/2026-09-18-notes-delivery-ink-143.md) 分别记录对应轮次的真实验收与限制。
