# GitHub 功能整合与运维入口（INK-253）

本文件是 GitHub 页面功能的操作指南，不替代 AGENTS、R1 设计、backlog 或运行验收。功能开关、收费与可用性应以 GitHub 当前页面为准。

## 1. 任务与看板

- [Issues](https://github.com/BaSui01/InkLore/issues)：使用缺陷、功能、实施 / 验收表单。若已有 INK 编号，则引用原任务，不重复创建。
- [Pull Requests](https://github.com/BaSui01/InkLore/pulls)：所有代码与治理文件先向 `dev` 提 PR。敏感变更加 `manual-review` 标签，禁止在未完成验收时切换为可自动合并。
- [InkLore Projects](https://github.com/users/BaSui01/projects/1)：协作可视化看板，已关联本仓库。仓库 `docs/backlog.md` 是唯一任务状态与验收事实来源，GitHub Project 的 Status 只作为视图。
- [贡献指南](../CONTRIBUTING.md) 与 [治理规则](repository-governance.md) 给出分支政策。

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
