# Agent Note: 作者问答、策略版本与原 Run 恢复

Status: implemented

Task: #106
Scope: Creator AgentWorkbench、run-ui、transport、Admin 策略与脱敏审计、Agent HTTP 分页

## Problem

工作台使用自由输入的 task_key，waiting_clarification 被当成 running，刷新丢失原 Run；空或截断的成功响应可能让客户端错误清除幂等意图。Admin 没有任务策略发布入口，也缺少契约中定义的只读 Run 摘要。运行与审批分页没有解码返回的位置，目录分页同样缺少对查询位置的恢复。

## Decision

Creator 只选择安全目录中的策略版本与允许的任务类型，提交 task_policy_version_id。文本、单选、多选澄清使用真实 Question GET/Answer，绑定 revision、question_version 和稳定 Idempotency-Key；未知响应保留原意图，冲突重新读取原 Question/Run。审批也读取本人授权的冻结参数，经 CAS 批准或拒绝。SDK 检验写入响应，JSON 截断、null 或不完整对象都不能作为成功确认。

URL 只保存 Run、章节与未确认创建请求的标识，作者指令和回答留在内存。刷新选择原章节后 GET 原 Run；网络重连、EOF、游标过期与定期核对均恢复该 Run。410 先 GET 权威状态，再从保留事件起点重订阅；旧订阅清理不能覆盖新游标。403/404、身份变化清除敏感状态。事件缓冲及其聚合展示共同受 2000 条/2MiB 上限约束。

Admin 在现有 Agent 治理页发布真实 LLM 不可变策略版本，显示当前版本、修订、输出预算和超时，使用 CAS 和稳定写入键。审计仓储直接查询允许的 Run 元数据，不加载正文、指令、事件或答案后再做隐藏。账单汇总通过 Agent RunUsagePort 委托 LLM 域；空回执为零，有回执却缺失费用读取能力时明确失败。

运行、审批和 Creator 目录分页使用绑定租户、主体、权限修订、操作及排序的签名游标。SDK 检查游标前进，避免错误服务响应形成无限循环。

## Alternatives considered

后端安全目录只投影有效模型、当前启用版本、已显式发布的 task_kinds 和预算。选择策略版本与创建 Run 共用事务，锁定当前策略、版本、模型和供应商；旧版本返回明确冲突，原幂等请求仍先重放已有 Run。HTTP 严格拒绝未知字段及 task_key/model_id/model_key 覆盖；内部 Go 调用兼容字段不暴露为 JSON。历史空 task_kinds 不推测回填，管理员须发布明确的新版本。Creator 的 LLM 通用目录只保留编辑器实际依赖的安全 Models DTO，其余管理资源拒绝访问。

继续让 Creator 输入 task_key 或模型覆盖会绕过安全目录意图；改成只选真实策略版本。把完整 Run 写 localStorage 会持久化作者数据；只保存资源标识并向服务端恢复。把 EOF 视作成功、410 后创建新 Run、空响应视作成功都可能重复执行或丢失恢复能力，予以拒绝。Admin 复用完整作者 Run DTO 会扩大泄露面，因此使用明确允许字段的独立查询和组件模式。

## Consequences

现有内部 Go 调用的兼容字段由后端保留；Creator HTTP 与 SDK 使用显式策略版本。原创建响应丢失且页面重载后，通过本人章节运行历史选择恢复，不自动创建替代任务。Admin 发布改变后续 Run 的策略，既有 Run 的冻结版本保持原值。

## Verification

- `go build ./...`、`go vet ./...`、`go run ./cmd/archguard` 通过。
- `pnpm --dir frontend run build` 的 Creator/Admin 类型检查和生产构建通过。
- `go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts` 通过；新增 agent 字典中英文键一致。
- `scripts/ci/agent_ui_acceptance.py` 在隔离 PostgreSQL 17 / Redis 7 / 真实 API 上通过安全目录、策略版本 CAS、旧 key 重放、参数覆盖拒绝、取消、Question 读写与冲突、正文不变、Admin 白名单及跨租户分页验证。
- Question UI 使用明确标记的 PostgreSQL 暂停态 fixture，随后由真实 API 执行。它不证明线上模型生成，也没有伪造模型成功。
- `scripts/ci/agent_question_ui_browser.cjs` 在 GitHub Actions run 41965922098 的真实 Chromium 上通过双端真实认证、390px、IME、刷新、断网、EOF、真实 410、已提交响应截断/null、稳定重试键、Admin 发布/脱敏与真实 403。凭据只走匿名 stdin 管道。本地 Chromium 启动受环境限制，未将其计为通过。移动截图等待实际侧栏收起后再留证。
- API 验收通过真实作者取消接口终结已回答的临时 Run，仅在明确的 revision CAS 冲突时重新读取。随后同库 `TestQuestionHTTPConcurrentAnswerAndRestartPG -race` 通过（21.753s），避免后续全局队列 Worker 误领验收遗留工作。
- 真实模型凭据未配置，live provider 验收为 NOT_VERIFIED。
