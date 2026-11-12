# InkLore 云端定期验收与 Creator/Admin/Agent 端到端门禁（INK-270）

## 执行位置与调度

- `Supabase Staging Preflight`：默认保护分支 `private` **每天 03:17 UTC（北京时间 11:17）** 自动运行，保留手动触发。受限 GitHub `supabase-staging` Environment 中的 `SUPABASE_STAGING_DATABASE_URL` 仅供受保护分支的只读查询，验证真正认证过的 IPv4 Session Pooler、17 组迁移、80 张私有表、数据 API 隔离和只读授权。不会连接业务写库或执行迁移。
- `Creator Admin Agent E2E`：默认分支 **每周一和周四 02:43 UTC（北京时间 10:43）** 自动运行；PR 到 `dev` / `private`、这两条分支的 push 和手动启动也运行。Scheduled workflow 只在 GitHub 默认分支启用，按官方 UTC cron 解释；GitHub 不保证准点启动，公共仓库持续 60 天没有活动可能暂停调度。
- 日常 CI 与新 E2E 完全无付费 AI Provider 密钥，也不读取 Supabase 环境 Secret。任何 Fork PR 均不得取得远程数据库登录凭据。

## 实际验收链路

E2E 在 GitHub-hosted Runner 中启动独立的一次性 PostgreSQL 18（pgvector）与 Redis 7，应用既有的 17 组迁移，在同一个一次性库中构建真实 Go Admin CLI 和 API，生成随机管理员与 Creator 身份：

1. **Creator**：真实 `/v1/auth/register` 生成独立租户及会话，校验重复注册 409、作品创建 201、GET 回读、另一租户不能读取或列出作品。
2. **Admin**：通过部署专用 `admin bootstrap` 建立独立 root 后台账号，真实管理员登录及目标租户投影，验证 Agent 定义目录、内置工具目录可读，Creator Token 不可进入 Admin 操作。
3. **Agent**：由 Admin 真实创建 Agent 定义及冻结版本，GET 回读，Creator 在本租户目录中看得到，另一租户目录不可见；缺少资源权限的 Run 请求必须失败且不可留下运行记录。**此门禁不声称已做带真实模型上游的生成，也不伪造 Agent Worker 成功或 SSE 产物。**
4. **真实浏览器**：同时启动 Creator 3000 和 Admin 3001 的 React/Vite 生产构建预览，由 Chromium/Playwright 使用同一后端的 HttpOnly Cookie 会话验证 Creator 受保护页面、Admin Agent 治理页面及新定义的实际展示。

每次作业全部使用临时密码、临时 DB、无价值示例文本；凭据只存于进程环境或权限 0600 的临时文件，脚本不打印 Token、密码、DSN、用户输入正文。浏览器驱动使用固定版本 `playwright@1.64.1` 安装在 `/tmp`，不会改动前端依赖和 `pnpm-lock.yaml`。临时数据随着 Runner 销毁而销毁。

## 完成标准与仍不覆盖的场景

- 最新 PR head 的原有 CI 三项、isolated-integration 和新增浏览器+HTTP E2E 工作流全部成功。
- `private` 发布后的 push E2E 成功，人工 `Supabase Staging Preflight` 仍成功；下一个实际 cron 触发后复查 `event=schedule`。
- Agent 真正模型调用、工具执行、SSE 长连接回放、Worker 故障恢复和浏览器复杂 CRUD 场景需另外登记外部测试环境验收。禁止把静态构建、只读 API 或无 Provider 的拒绝路径冒称生成全链路闭环。
- Supabase Free 不保证永远运行，计划任务可能因项目暂停/网络问题失败，这正是监控应报告的状态，不应在代码中静默跳过。
