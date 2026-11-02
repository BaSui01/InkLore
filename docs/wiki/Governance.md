# GitHub 协作与安全治理

## 工作入口

- [Issues](https://github.com/BaSui01/InkLore/issues)：缺陷、需求、独立验收
- [Pull Requests](https://github.com/BaSui01/InkLore/pulls)：实现、CI、审查
- [Projects 看板](https://github.com/users/BaSui01/projects/1)：工作项视图
- [Actions](https://github.com/BaSui01/InkLore/actions)：CI、发布、每周只读健康报告
- [Insights](https://github.com/BaSui01/InkLore/pulse)：提交、合并与贡献概况
- [Dependency graph](https://github.com/BaSui01/InkLore/network/dependencies)：软件依赖
- [Security](https://github.com/BaSui01/InkLore/security)：依赖漏洞与安全政策

## 分支政策

`private` 为稳定主分支，`dev` 为开发集成分支。功能修改先提交到 `dev`；要发布到 `private`，必须有单独 PR。敏感路径、Draft、缺独立验收或未通过检查的改动不得自动合并。

GitHub Projects 是可视化协作视图，唯一任务状态和验收依据仍是 [docs/backlog.md](https://github.com/BaSui01/InkLore/blob/dev/docs/backlog.md)。

## 依赖升级

Dependabot 每周提出 Go、pnpm、Actions 和 Docker 的版本更新 PR。普通版本更新以 `dev` 为目标；安全更新由 GitHub 决定目标默认分支。高风险升级需人审、保留锁文件、记录构建和独立验收，绝不靠跳过检查“变绿”。

依赖告警须按严重度与可达性逐项处理；代码扫描、秘密扫描和私密漏洞报告应在仓库 Settings 中核对功能授权与开关。查看 [仓库治理说明](https://github.com/BaSui01/InkLore/blob/dev/docs/github-platform.md)。
