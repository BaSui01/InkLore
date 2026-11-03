# InkLore 双向基线核验 · 2026-11-03

> 候选基线（发布前需以 GitHub Actions 与 Review 实际结果为准），非生产部署。

## 基线来源
- 默认稳定分支 `private`：`0fecb67134405b408aa90d53c53266d256d4b73a`
- 开发集成分支 `dev`：`8f6131af65ee9da194c05b9a84a3f6336b71f43c`
- 共同祖先：`eb01e2d975b2f03c6ba0a359970db34459685c2f`
- 两个父提交都应保留在 Merge Commit 中；不允许 squash、rebase 或 force push。

## 差异与冲突
- 合并前 dev 独有提交约 64 个，private 独有约 23 个（独立 PR 可能继续推进，最终以 GitHub Compare 为准）。
- Git 三方树级对比：private 独有 12 个文件，开发侧独有业务修改保留；9 个共同变更但内容不同的文件单独裁决。
- `.github/workflows/ci.yml`、`ci-status-overview.yml`、`safe-auto-merge.yml`、`supabase-staging-preflight.yml`：选择稳定分支经实测的安全版本。
- README/治理文档保留稳定分支的保护规则与开发分支的 GitHub 导航；CHANGELOG/architecture 采用开发侧当前说明并纳入稳定分支独有交付章节。
- backlog 的任务行保留开发侧已推进状态和稳定分支独有任务。原 INK-262 在开发侧用于 SSRF 修复、稳定侧历史用于 Supabase IPv4；合并报表将后者登记为 INK-272（保留原 PR #80/历史笔记），避免两个不同任务争用同一个 ID。
- Draft [PR #41](https://github.com/BaSui01/InkLore/pull/41) 仍是旧版候选，不直接合并；开发分支目前开放的 PR、Dependabot 分支不在本次范围。

## 发布门禁与后续
1. 先检查候选 PR 最新 SHA 的 Skills/notes、Go build/vet/gofmt/archguard、Creator/Admin 构建和锁文件，数据库迁移/租户/Agent Worker 隔离 E2E、代码审查/安全告警。
2. 任何高风险 Agent、LLM/Provider、权限和密钥逻辑发布到稳定分支前须独立安全 Review；CI 全绿不能冒充真实付费模型或全部业务覆盖。
3. 通过后只使用受保护 Merge Commit 进入 private，反向经 PR 将该稳定基线同步到 dev，最终校验两边文件树内容一致，不重置 dev 或抹去历史。
4. 清理只删除已合并或确认内容可从保留分支恢复、没有开放 PR 的临时分支，绝不删除受保护分支或未合并草稿。

## 候选生成后的并行更新

- private `9b04b721856a52343c376b86359258ea6ec4f321` 已发布前端线性 ReDoS 修复（候选树先前已含完全相同的源码 blob）；同步补入稳定侧的发布记录。
- dev `fd138a88382558211799301219daf2e5b4d80f9a` 新增 LLM 目录发现/SSRF 的回归补丁；本候选同时纳入此 4 个源码/笔记 blob。
- 后续合并提交包含此前的双父基线及这两个新增保护分支提交为祖先，不修改父提交或重写任一长期分支的历史。
