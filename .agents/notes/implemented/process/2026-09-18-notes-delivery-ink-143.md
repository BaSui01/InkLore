# Agent Note: INK-143 工程记忆交付记录

Status: implemented

Task: INK-143
Scope: 原 docs/notes-delivery-INK-143.md 的历史材料迁移与链接维护

## Problem

阶段性交付证明、契约差异、外部 UI 调研等历史记录混放在 docs 读者文档目录，容易让读者误认成最新有效的规范或功能事实。

## Decision

将原始材料迁入工程记忆，保留其来源、历史约束、验证记录和未完成边界。这是记录整理，不代表原记录涉及的能力已在当前云端完成验收。

## Alternatives considered

继续置于 docs 可维持旧入口但扩大文档噪声；删除历史材料会丢失决策依据与复现信息。因此保留其原有正文并收敛为 notes 记录。

## Consequences

原路径不再作为权威入口；引用必须更新。外部研究中的 UI 功能分析仅作为参考，不对第三方功能或当前 InkLore 的实现状态背书。

## Verification

保留源文档文本，按工程笔记结构调整标题和相对链接；禁用的表意图符号按仓库笔记规范去除。本轮没有重跑源记录中的历史命令。

### 历史原始资料

### INK-143 工程记忆交付记录

状态：done。用户明确要求增加 .agents/notes 长期工程记忆，采用此前讨论的 ADR / notes / backlog 分工。日期：2026-09-18（北京时间）。

### 范围与实际交付

- notes 规约、四生命周期与六分类、固定头块/必需节及三个模板。
- 三个项目 skills 接入改前检索、非平凡变更同批留因果、活跃笔记演进与历史归档。
- 独立 Node CLI 和两个库模块：格式/任务关联/链接校验，归档哈希与 Git 基线保护；不安装第三方包。
- AGENTS、管理规约、文档指南、文档入口、architecture、CHANGELOG、backlog 和 CI 同步。
- 首篇真实流程笔记记录此次引入 notes 的原因与真实备选；没有为填满目录虚构业务历史或否决案例。

本轮属于已授权的工程流程工具落地，没有改变业务分层、域边界、存储或对外依赖，不新增业务架构 ADR。既有 Accepted ADR 正文和 INK-142 历史验收保持原样；未改变冻结实施顺序，不修改R1实施计划。并行INK-141清理旧文档后，本轮把技能中的5处失效链接修复到现行R1分册，并按更新后的AGENTS调整阅读顺序。

### 正式工作区验证

环境：Windows amd64、Node v24.9.0、Go go1.26.4。工作区有其他任务未提交修改，下列结果不代表干净提交的远端CI。

| 命令 / 操作 | 实际结果 | 证明范围 |
| --- | --- | --- |
| node scripts/agent-notes.mjs check | 退出0，1篇真实笔记通过 | 笔记结构、任务关联、链接与工作树封印一致性 |
| node scripts/agent-notes.mjs check --base HEAD | 退出0，明确bootstrap | HEAD存在但没有历史manifest，允许首次引入；不是已保护历史归档的实库证据 |
| node scripts/check-project-skills.mjs | 退出0，12份Markdown、74个相对目标 | 三技能与模板/参考文档链接 |
| go build ./... | 退出0 | 当前工作区可构建；不证明业务正确 |
| git diff --check | 退出0 | 已跟踪diff空白检查 |

CI 配置 Node22，完整checkout历史后使用 PR base SHA 或 push before SHA 校验；没有前序提交时显式bootstrap。远端 GitHub Actions 未执行，本地为Node24。当前真实manifest为空，没有制造“已归档历史”。

### 隔离冒烟验证

在独立临时目录复制工具、建立最小notes/任务夹具；涉及Git的场景仅对临时仓库init/commit，不写正式仓库Git历史。原始28个场景及最终4个针对性场景共32项符合预期，非零退出均为预期拒绝；临时目录已清理，没有留下测试文件。

| 场景组 | 实际结果 |
| --- | --- |
| 合法implemented、围栏中的示例 | 通过；代码围栏不充当真实链接/必需节 |
| 状态不匹配、缺Alternatives、TODO空壳、重复必需节、中文计划标题、未登记Task | 拒绝 |
| inline/reference失效链接、README失效链接 | 拒绝 |
| 治理文档inline/reference入站引用、移动后相对链接失效 | 归档拒绝，避免留死链 |
| archive成功及BOM/CRLF保留 | 通过，生成目标与manifest并移除源 |
| LF/CR换行转换 | 哈希规范化后不漂移 |
| 仅改归档正文 | 普通check拒绝SHA-256不一致 |
| 同时改正文与manifest | 普通check可通过，带 --base HEAD 拒绝旧封印变化 |
| 无效Git基线、以短横线开头ref、archive带不支持的--base | 拒绝，错误不静默降级 |
| 有效历史提交没有manifest | 输出bootstrap并允许首次引入 |

最终4项复查：占用排他锁时退出1且源/manifest/已有锁字节不变；Setext形式的Risks计划标题被拒；Windows大小写不同的源文件入站链接仍阻止归档；正常归档退出0并释放锁，结果check通过。三个最终模块均通过 `node --check`。

归档CLI使用 `.agents/notes-archive.lock` 串行保护读/校验/写/回滚，临时锁已加入gitignore，笔记本体与manifest仍纳入版本管理。工具只协调CLI，不阻止手工编辑；强杀后需核对恢复副本和残留锁，未执行断电故障注入。

### 运行路径与限制

没有修改Go包、应用依赖、组合根、HTTP/Worker接线、数据库迁移或前端业务，运行路径不受本任务影响。新代码仅在开发和CI执行，手写文件均低于50KB，未新增或修改项目禁止的测试文件，未写入凭据。

笔记门禁不自动判断每次代码修改是否非平凡、是否遗漏笔记、理由/备选是否真实，也不自动回填历史。归档哈希不是数字签名，CI与受信基线仍需仓库评审保护。归档前需处理链接，已封印历史不能通过重算哈希“修复”。

