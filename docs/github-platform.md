# GitHub 功能整合与运维入口（INK-253）

本文件是 GitHub 页面功能的操作指南，不替代 AGENTS、R1 设计、backlog 或运行验收。功能开关、收费与可用性应以 GitHub 当前页面为准。

## 1. 任务与看板

- [Issues](https://github.com/BaSui01/InkLore/issues)：使用缺陷、功能、实施 / 验收表单。若已有 INK 编号，则引用原任务，不重复创建。
- [Pull Requests](https://github.com/BaSui01/InkLore/pulls)：所有代码与治理文件先向 `dev` 提 PR。敏感变更加 `manual-review` 标签，禁止在未完成验收时切换为可自动合并。
- [InkLore Projects](https://github.com/users/BaSui01/projects/1)：协作可视化看板，已关联本仓库。仓库 `docs/backlog.md` 是唯一任务状态与验收事实来源，GitHub Project 的 Status 只作为视图。
- [交付里程碑](https://github.com/BaSui01/InkLore/milestones)：以 GitHub 原生 Milestone 聚合 Issue/PR 的验收关卡；阶段说明与退出条件以各 Milestone 描述为准。
- [贡献指南](../CONTRIBUTING.md) 与 [治理规则](repository-governance.md) 给出分支政策。

### 交付里程碑与验收口径（2026-11-03）

| Milestone | 交付关卡 | 已有执行入口示例 |
| --- | --- | --- |
| [M0 · 安全与 CI 基线](https://github.com/BaSui01/InkLore/milestone/1) | 已知安全缺陷与登录风险收敛、最新开发 CI 可信 | [INK-262](https://github.com/BaSui01/InkLore/issues/74)、[INK-263](https://github.com/BaSui01/InkLore/issues/75)、[INK-272](https://github.com/BaSui01/InkLore/issues/97) |
| [M1 · 身份与模型服务闭环](https://github.com/BaSui01/InkLore/milestone/2) | SMTP/OTP 真实投递、多供应商模型协议与预算验收 | [INK-273](https://github.com/BaSui01/InkLore/issues/98)、[INK-205](https://github.com/BaSui01/InkLore/issues/103) |
| [M2 · Agent 持久执行与策略可信](https://github.com/BaSui01/InkLore/milestone/3) | 策略冻结、工具授权、Question、审批和 Worker 恢复 | [INK-228](https://github.com/BaSui01/InkLore/issues/3)、[INK-229](https://github.com/BaSui01/InkLore/issues/104)、[INK-264](https://github.com/BaSui01/InkLore/issues/83) |
| [M3 · Creator 小说创作闭环](https://github.com/BaSui01/InkLore/milestone/4) | 开书到章节交付、双端问答与跨域真实验收 | [INK-230](https://github.com/BaSui01/InkLore/issues/106)、[INK-236](https://github.com/BaSui01/InkLore/issues/4)、[INK-274](https://github.com/BaSui01/InkLore/issues/107) |
| [M4 · Staging 验收与受保护发布](https://github.com/BaSui01/InkLore/milestone/5) | 隔离云端验收、dev/private 分叉收敛及人工发布审查 | [INK-255](https://github.com/BaSui01/InkLore/issues/105)、[INK-256](https://github.com/BaSui01/InkLore/issues/39)、[发布 PR #101](https://github.com/BaSui01/InkLore/pull/101) |

约定：里程碑按交付验收而非代码目录分组，阶段间允许并行实施，**完成判定依赖真实验收证据**。GitHub 的进度百分比只统计关联 Issue/PR 的开闭状态，不能替代 `docs/backlog.md` 的 todo/doing/review/done/blocked 事实；已写入旧任务编号继续复用，不批量把历史 done 重新计入新阶段。没有经确认的交付排期，不设置虚构 due date，也不为凑齐进度强行关闭 Issue。

跨阶段依赖写在 Issue 正文而非重复创建同义任务。发现 [INK-265 #89](https://github.com/BaSui01/InkLore/issues/89) 与 [INK-271 #91](https://github.com/BaSui01/InkLore/issues/91) 的 403 会话处理策略冲突，先按现行 backlog 的人工重新开始语义核对并留记录，**不自动合并相反实现**。已被新 500KiB 规则覆盖的旧 50KiB 任务也需先核对实际已合并代码，再决定是否关闭；待分流项可暂不绑定 Milestone。

## 2. 依赖关系与 Dependabot

- [Dependency graph](https://github.com/BaSui01/InkLore/network/dependencies) 由 GitHub 解析 Go `go.mod` / `go.sum`、前端 `package.json` / `pnpm-lock.yaml`、Actions 与容器定义。
- [.github/dependabot.yml](../.github/dependabot.yml) 每周检测 `gomod`、`npm`（兼容 pnpm）、`github-actions`、`docker`。普通版本更新目标 `dev`，PR 数量有限制；不自动修改业务代码或跳过 CI。
- **GitHub 只从默认分支读取 Dependabot 配置**。仓库默认分支是 `private`，本次配置先经 PR 进入 `dev`，待合规发布到 `private` 才正式生效。Dependabot 安全更新并不遵循 `target-branch: dev`，仍面向默认分支。
- [Dependabot alerts](https://github.com/BaSui01/InkLore/security/dependabot) 提供漏洞警报；优先处置高危可达依赖，区分升级可行性和真实回归验收。更新 `pnpm-lock.yaml` 时保持 pnpm 锁文件作为唯一版本锁。

## 3. 安全与质量

- [Security and quality](https://github.com/BaSui01/InkLore/security) 可查看 Dependabot、代码扫描与安全策略；[SECURITY.md](../SECURITY.md) 指引私密报告与密钥轮换。
- 安全告警不会因为开启 Dependabot 就自动解决；代码扫描与私有仓库 Secret Scanning / Push Protection 受当前套餐、授权与仓库设置限制，需要在 **Settings → Advanced Security** 检查。
- 已有 [INK-252](backlog.md) 记录公开前凭据治理阻塞。未轮换真实凭据、未检查可达 Git 历史与 PR diff 前，不切换为 Public。
- 分支保护已要求三项 CI：Skills / notes、Go 构建与护栏、前端类型检查与构建；规则变更在 [Settings → Branches](https://github.com/BaSui01/InkLore/settings/branches) 审核，不能只看是否存在 Rulesets。

## 4. Actions 与 Insights

- [Actions](https://github.com/BaSui01/InkLore/actions) 中 CI、发布、Qodana 与受限自动合并各自有独立职责。Qodana 失败必须查看真实日志，不把未配置 Token 等同代码存在缺陷。
- [Repository Health Snapshot](https://github.com/BaSui01/InkLore/actions/workflows/repository-health.yml)：每周一北京时间 10:21 汇总过去七天 Issue / PR 的计数至 **Job Summary**，只读、无部署密钥、无自动修改。支持手动触发。
- **GitHub Schedule 仅在默认分支生效**；该工作流需随 `dev` 发布到 `private` 后才会定时执行。
- [Insights / Pulse](https://github.com/BaSui01/InkLore/pulse) 与 [Traffic](https://github.com/BaSui01/InkLore/graphs/traffic) 是 GitHub 原生分析；仓库可见性与套餐会影响可用图表，生成的周报不替代这些官方指标。

## 5. Wiki

- 本仓库启用了 Wiki 功能；源文档为 [docs/wiki](wiki/Home.md)，包含 `Home.md`、`Architecture.md`、`Development.md`、`Governance.md` 与 `_Sidebar.md`。
- Wiki 是**单独的 Git 仓库**，不会因为 `docs/wiki` 被合并就自动上线。若尚未初始化，请先打开 [Wiki](https://github.com/BaSui01/InkLore/wiki)，手工创建并保存第一个 `Home` 页面；此后可克隆 `https://github.com/BaSui01/InkLore.wiki.git` 并复制上述文件推送。
- 仅同步导航和公开适用的说明，绝不复制本地环境凭据、部署密钥、内部敏感信息或未审查的运维记录。同步应由受信任的维护者执行，不把 Wiki 写权限交给任意 PR。

## 6. Projects 自动收录与 CI 状态追踪（INK-253）

- GitHub Projects 控制台：https://github.com/users/BaSui01/projects/1。字段 `Track` / `Priority` / `Gate` / `Target` 和各个视图属于协作展示层，不替代 `docs/backlog.md` 的验收事实。
- **自动收录首选 GitHub 原生 Auto-add**：打开 Projects → 顶部菜单 → Workflows → Auto-add to project → Edit，仓库选 `BaSui01/InkLore`，过滤条件 `is:open`，点 `Save and turn on workflow`。启用后新建或更新的符合条件 Issue/PR 才会自动进入项目；既有项不自动补录，需要单独导入。此设置无法通过当前 GitHub 官方 Projects GraphQL/CLI 写入，不得用广权限 PAT 充当临时替代。
- **CI 实时事实**：PR 的 [Checks](https://github.com/BaSui01/InkLore/pulls)、分支的 [Actions](https://github.com/BaSui01/InkLore/actions) 及 GitHub branch protection 是权威源。Project 的 `Gate` 由维护者记录审核步骤，`Verified` 不表示自动读取了 GitHub Check Run，也不代表真实业务验收通过。
- **只读 CI 摘要工作流**：[InkLore CI Status Overview](https://github.com/BaSui01/InkLore/actions/workflows/ci-status-overview.yml)（源文件 [ci-status-overview.yml](../.github/workflows/ci-status-overview.yml)）：默认分支发布后支持手动运行、每六小时轮询，以及在 CI/Isolated Integration 完成后生成 Job Summary。展示 `private` / `dev` 分支头的 CI、隔离验收、Qodana 与 CodeQL 最近可观察状态，最多 25 个开放 PR 的独立检查入口。超过最近 100 个 Runs、PR 合成合并 SHA 或尚无记录时，明确标记 `Not observed` / `STALE`，**不得当作绿色检查**。
- 工作流所有权限均为只读；不检出 PR 代码、不执行外部提交、不读 Secret、不写 Projects、不合并 PR、不部署服务。GitHub 原生 Project Auto-add 不需要创建新的 PAT 或在 Actions 中储存项目级 Token。
- 提交此配置到 `dev` **不会立刻激活 schedule/workflow_run**，需要独立验收后通过受保护的发布 PR 进入默认 `private` 才能开始定时运行。不得为激活周报绕过发布安全门禁。
- [代码扫描](https://github.com/BaSui01/InkLore/security/code-scanning)、[Dependabot 告警](https://github.com/BaSui01/InkLore/security/dependabot) 和 [Security Policy](../SECURITY.md) 由 GitHub 管理；静态扫描、无告警均不等价于历史凭据已吊销或 Agent/LLM/多租户已完成独立运行验收。

只读周报的权限取舍、误报防护及默认分支生效约束见 [INK-253 工程笔记](../.agents/notes/proposed/process/2026-11-03-ci-status-observability.md)。
