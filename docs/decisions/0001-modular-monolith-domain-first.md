# ADR-0001：采用模块化单体与 Domain-First 分层

- **状态**：Accepted
- **日期**：2026-09-16
- **决策者**：项目负责人
- **关联任务**：—
- **替代关系**：**第 2 条（业务域清单）已被 [ADR-0014](./0014-domain-boundary-expansion-to-seven-contexts.md) 替代**；其余条款继续有效。

> **替代说明（2026-09-18 追加）**：本 ADR 第 2 条原定「业务代码严格收敛在三个限界上下文 `internal/novel` / `internal/agent` / `internal/llm`」。
> 该清单已由 [ADR-0014](./0014-domain-boundary-expansion-to-seven-contexts.md) 扩张为 7 个业务域（content / outline / worldview / inspiration / compliance / agent / llm）。
> 本文正文按「已接受的 ADR 不修改」的约定保持原样，阅读第 2 条时请以 ADR-0014 为准。

## 背景（Context）

InkLore 是一个 AI 长篇小说创作平台，需要同时承载：多 Provider 模型网关、具备 Think-Act-Observe 循环的创作 Agent、以及作品事实账本。三类能力耦合度高（章节生成必须穿过 Agent 才能到达模型），团队规模小，部署环境以单机或单容器为主。

同时必须解决一个具体风险：**AI 生成的章节若绕过作品域直接写库，事实账本必然腐化**，长篇创作会因此崩设定。

## 决策（Decision）

采用**模块化单体（Modular Monolith）**而不是微服务，并用 **Domain-First 分层**约束模块内部结构。

1. HTTP API 与后台能力运行在同一进程、同一仓库（`cmd/server`）。
2. 业务代码严格收敛在三个限界上下文：`internal/novel`、`internal/agent`、`internal/llm`。
3. 横切技术能力收敛进微内核基座 `internal/platform`，不按业务域拆分。
4. 域内依赖方向固定为 `transport → application → domain ← infrastructure`。
5. 跨域调用只能经消费方自定义的 **Port**，并在 `internal/app/adapters` 完成装配。
6. 组合根唯一：`internal/app`，其他任何位置不得做全局装配。
7. 作品事实的唯一权威 Owner 是 `internal/novel`；Agent 只能读上下文、产出草稿与建议。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 微服务拆分（网关/Agent/作品三个服务） | 独立伸缩、故障隔离 | 分布式事务、跨服务事实一致性、运维成本 | 当前规模下收益远小于成本；事实一致性会被网络边界放大 |
| 传统三层扁平结构（controller/service/repository） | 上手快、样板少 | 域边界模糊，事实权威容易失守 | 无法承载"事实单一来源"这一核心约束 |
| 事件驱动 + 完全解耦模块（同进程消息总线） | 模块间零直接依赖 | 调试困难、时序隐式，早期迭代代价高 | 保留为未来 M4 之后的演进选项，不作为起点 |

## 后果（Consequences）

**正面**

- 跨域一致性可在单进程内用一次数据库事务或显式补偿解决。
- 域边界清晰，AI 编码代理可以按"域"切分任务，减少越界改动。
- 部署形态简单：一个二进制 + 外部依赖。

**负面 / 代价**

- 单体意味着构建体积与启动时间随功能增长。
- 缺少编译期强制的模块边界（Go 无 module 系统级隔离），依赖方向只能靠规约 + 护栏脚本维持。
- 跨域调用需要额外定义 Port 与 adapter，存在样板成本。

**需要长期承担的义务**

- 任何跨域调用必须显式定义 Port，禁止直接导入对方 `infrastructure`。
- 单文件 ≤ 50 KB，模块过大时必须垂直/水平拆分。

## 验证方式（Verification）

1. `go list -deps ./cmd/server` 与 `go list ./...` 对比，确认关键包在运行路径上。
2. 全库检索确认不存在跨域 `infrastructure` 导入（由 INK-033 护栏脚本自动校验）。
3. `/v1/architecture` 端点返回的 `style` 与 `rules` 与实际结构一致。
