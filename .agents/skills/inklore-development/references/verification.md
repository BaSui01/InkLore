# 按影响选择验证

命令从仓库根运行，先核对当前脚本。选择能验证本次改动的检查；既有受保护 CI 的必需门禁仍由正常 PR 流程执行。

| 改动影响 | 适用检查 | 能证明的范围 |
| --- | --- | --- |
| 文档、链接或文案 | 相关链接/渲染检查、`git diff --check` | 内容可读取、路径有效；无需为纯文档启动数据库或模型 |
| 项目技能与指南 | `node scripts/check-project-skills.mjs` | 元数据、大小和本地文件链接；不验证语义和外站 |
| 工程笔记 | `node scripts/agent-notes.mjs check`；涉及历史归档时加 `--base <变更前真实提交>` | 格式、链接与封印；Task 格式合规不代表已在线验证任务存在 |
| Go 逻辑、边界或装配 | 受影响包的构建/测试与 gofmt；按影响运行 `go vet ./...`、`go run ./cmd/archguard -reachability` | 编译、静态边界与回归行为；可达不等于业务可用 |
| 前端逻辑或组件 | 对应应用的类型检查/构建、相关测试及实际交互；共享包改动考虑 `pnpm --dir frontend run build` | 构建与用户行为分别记录 |
| i18n 词条 | `go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts` | Key 集合一致；仍需检查页面使用和显示 |
| API/机器契约 | `node docs/system-design/tools/check-design.cjs`、`node scripts/check-platform-contracts.mjs`，再核对路由与真实请求 | 静态语义/错误目录一致性；不证明运行时校验和授权已经接通 |
| 数据、事务或授权 | 授权的隔离环境中执行相关 API/SQL 正反例 | 持久化、回滚、并发或跨租户边界，按影响选择 |
| Agent/LLM/SSE | 相关正常生成、审批、断线、取消、Worker 恢复与上游失败场景 | 核对 Run/Job/产物和费用结果；桩上游不能证明真实模型效果 |
| 发布或恢复 | 当前发布配置、版本、healthz/readyz、迁移兼容与代表性路径 | 实际服务与恢复行为；发布授权单独核对 |

针对根因和易回归的边界增加有价值的测试，不写机械复刻实现的测试。验证已有充分证据后，不因流程习惯反复运行同一套检查。

失败时记录命令、退出码、原始错误与影响，区分已存在的差异和本次引入的问题；不关闭检查、修改历史迁移或伪造通过来消除红灯。缺依赖的检查明确记为 `NOT_VERIFIED`。

需要全仓交付验证时复用 `make ci NOTES_BASE=<交付前真实提交>`；Windows 入口为 [check.ps1](../../../../scripts/check.ps1)。综合构建通过仍不替代数据库、浏览器和 Agent 行为验收。缺少真实 Git 历史时不能宣称 `--base` 历史封印校验通过。
