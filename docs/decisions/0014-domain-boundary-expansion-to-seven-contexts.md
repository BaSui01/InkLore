# ADR-0014：业务限界上下文由 3 个扩张为 7 个

- **状态**：Accepted
- **日期**：2026-09-18
- **决策者**：项目负责人
- **关联任务**：INK-114（本决策）、INK-115（DDL 迁移落地）
- **替代关系**：**Supersedes ADR-0001 第 2 条**（业务域清单）。ADR-0001 其余条款（模块化单体、Domain-First 分层、单组合根、跨域经 Port）继续有效。

## 背景（Context）

ADR-0001 确立了三层业务域清单：`internal/novel`、`internal/agent`、`internal/llm`。该划分在原型阶段是合适的，但随后暴露出两个具体问题：

**问题一：三类互不相同的聚合根被挤在同一个包内。** `internal/novel` 当前 47 个文件 / 6,720 行，其 `domain` 包同时承载：

- 作品与正文：`Novel`、`Volume`、`Chapter`、`ChapterVersion`、`ChapterDiff`；
- 大纲与剧情：`OutlineNode`、`OutlineContext`、`OutlineLine`、`Scene`；
- 设定与事实：`Character`、`FactEntry`、`FactProposal`、`FactConflict`、`SimilarSegment`。

三者的事实权威边界只存在于文档里，**无法被任何编译期或护栏机制强制**。ADR-0001 第 7 条承诺的「事实唯一权威」因此只能依赖人工纪律。

**问题二：设计规范与代码/红线三方口径不一致。** `docs/system-design/` 体系（v3.1）按 7 个业务域编写领域详细设计与数据库设计，而 `AGENTS.md` §1.2 与 `cmd/archguard` 的 `businessDomains` 都是 3 域。按 7 域路径新建包会被护栏直接判定为违规，设计文档无法落地为可施工的包结构与迁移。

项目约束：禁止编写任何测试文件（`AGENTS.md` §3.1），因此**唯一的自动化门禁是 `go build ./...` 与 `cmd/archguard`**，迁移过程必须每一步都保持可构建。

## 决策（Decision）

**将业务限界上下文由 3 个扩张为 7 个，并以 strangler 方式增量迁移。**

我们希望：

1. 业务域清单固定为：`content`（作品与正文）、`outline`（大纲与剧情）、`worldview`（设定与事实图谱）、`inspiration`（拆书与灵感）、`compliance`（合规与质检）、`agent`（智能体与工作流）、`llm`（模型调度网关）；另有 `platform`（微内核基座）与 `app`（组合根）不属于业务域。
2. 每个域内部结构固定为 `transport/` → `application/` → `domain/` ← `infrastructure/`，与 ADR-0001 第 4 条一致。
3. 域间依赖构成单向 DAG：`outline` 与 `worldview` 不依赖任何业务域；`content` 依赖 `outline` + `worldview`；`agent` 依赖 `content` + `worldview` + `llm`；`llm` 不依赖任何业务域。
4. 跨域访问一律经消费方自定义的 **Port** 并在 `internal/app/adapters` 装配。**唯一例外**：允许跨域导入对方 `domain` 包中的**只读值类型**用于组装读模型；严禁导入对方 `infrastructure`，严禁跨域写对方管辖的表。
5. 事实权威按聚合根重新划界，细化 ADR-0001 第 7 条：
   - 作品、卷、章节正文与版本快照的唯一权威 Owner 是 `content`；
   - 角色档案、世界观词条、关系三元组与事实账本的唯一权威 Owner 是 `worldview`；
   - 「章节确认」（`ConfirmChapter`）是 `content` 的用例，它通过 Port 调用 `worldview` 落账；反向调用不存在。
6. 为打破 `content` 与 `worldview` 之间的循环依赖，明确两处类型归属：
   - `ConfirmResult` 归属 `content`（它是章节确认用例的返回值），其 `WrittenFacts` 引用 `worldview` 的 `*FactEntry`，形成 `content → worldview` 单向；
   - `worldview` 不得引用 `content` 的任何类型，跨域引用一律用 `string` ID 表达（如 `FactConflict.SourceChapterID`）；
   - `ContextPack` 归属 `content`，作为其跨域只读读模型导入 `worldview` 与 `outline` 的 domain 值类型。
7. `inspiration` 与 `compliance` 当前无任何代码。本轮只建立**四层目录骨架 + `doc.go`**（声明域职责与边界），使其在护栏中成为受管域；填充实现另开任务。
8. `AGENTS.md` §1.2 与 `cmd/archguard` 的 `businessDomains` 同步更新为 7 域，使新边界立即获得护栏强制。
9. 迁移采用 strangler 模式：按 `outline` → `worldview` → `content` 的顺序逐域抽取，每一域抽取后必须 `go build ./...` 与 `archguard` 全绿才进入下一域；`internal/novel` 作为过渡容器在高耦合文件（`sql_repository.go`、`service.go`、`context_pack.go`）被拆净后删除。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| A：维持 3 域，7 域仅作逻辑分组 | 零代码改动，立即与 `AGENTS.md` 一致 | 事实权威边界仍无法被护栏强制；`internal/novel` 继续混装三类聚合根；领域详细设计的有 DDL 无包归属；事故面（事实账本腐化）无任何自动化防线 | 无法支撑「按域独立演进」与「域级数据库设计」目标 |
| B：一次性全量迁移 | 一步到位，无中间态 | 6,720 行 + 34 处装配点同时变更；项目禁止测试文件、没有测试网兜底，构建失败风险集中且难定位 | 风险不可控，违反「每步可验证」的 DoD 要求 |
| C：增量 strangler 迁移（**采用**） | 每一步都以 `go build ./...` 为门禁；中间态始终可构建可运行 | 迁移周期内新旧包并存，需防止长期滞留 | — |

## 后果（Consequences）

**正面**

- 域边界从「文档约定」升级为「护栏强制」：`cmd/archguard` 的 R1/R2 立即按 7 域生效。
- 每个域拥有独立的聚合根、状态机与数据库迁移，与 `docs/system-design/domains/` 的详细设计 1:1 对应。
- Agent 无法再通过「同包内直接操作」绕过作品与设定的事实权威。
- 单一职责的包体积下降，为单文件 50KB 红线提供结构性保障。

**负面 / 代价**

- 包数量显著增加，跨域 Port 与 adapter 的样板代码增加。
- 迁移期间 `internal/novel` 与新域并存，需明确禁止新增代码进入 `internal/novel`。
- 前端 `frontend/src/domains/` 需同步重组，短期内前后端域目录不一致。

**需要长期承担的义务**

- 新增业务域必须同时建立 `domain` / `application` / `infrastructure` / `transport` 四层与 `doc.go`，并在 `AGENTS.md` §1.2 与 `archguard` 的 `businessDomains` 登记。
- 跨域导入对方 `domain` 值类型属于受控例外：新增此类导入需在评审中说明，且只能用于只读读模型，不得形成写路径。
- `inspiration` 与 `compliance` 的空骨架不得长期滞留：若本轮之后 2 个里程碑内仍无实现，应重新评估是否保留为域。
- 禁止在 `internal/novel` 中新增任何文件；其唯一去向是被拆空后删除。

## 迁移顺序（Migration Order）

| 步骤 | 动作 | 门禁 |
| :---: | :--- | :--- |
| 1 | 本 ADR + `AGENTS.md` §1.2 + `archguard` 域清单更新 | `archguard` 全绿（此时 `novel` 仍在 7 域清单内） |
| 2 | 抽取 `internal/outline`（最低耦合：`OutlineNode`/`Scene` 及其仓储与 Handler） | `go build ./...` + `archguard` |
| 3 | 抽取 `internal/worldview`（`Character`/`FactEntry`/`FactProposal`/`FactConflict`/`SemanticIndexEntry`） | `go build ./...` + `archguard` |
| 4 | 抽取 `internal/content`（`Novel`/`Volume`/`Chapter`/`ChapterVersion`/`ConfirmResult`/`ContextPack` 及其余全部） | `go build ./...` + `archguard` |
| 5 | 建立 `inspiration` / `compliance` 空骨架，删除已拆空的 `internal/novel` | `archguard -reachability` 不可达 0 |
| 6 | 前端 `frontend/src/domains/` 按 7 域重组 | `tsc -b` + `vite build` |
| 7 | 同步 `architecture.md`、`backlog.md`、`CHANGELOG.md` 与设计文档包路径引用 | 文档链接自检 |

## 验证方式（Verification）

1. `go build ./...` 通过。
2. `go run ./cmd/archguard` 全绿（R1 跨域 infrastructure 导入 0 处、R2 platform 反向依赖 0 处、R3 单文件 ≤ 50KB、R4 无测试文件）。
3. `go run ./cmd/archguard -reachability` 报告不可达包为 0。
4. 全库检索 `InkLore/internal/novel` 命中为 0（迁移完成的判定标准）。
5. `cmd/archguard` 的 `businessDomains` 与实际 `internal/` 下的业务域目录一致；`AGENTS.md` §1.2 与之一致。
6. 全库检索确认不存在「`worldview` 导入 `content`」与任何域的 `infrastructure` 被其他域导入。
