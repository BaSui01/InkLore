# Agent Note: 前端 BaseURL 与 Email 线性时间校验

Status: proposed

Task: INK-263
Scope: frontend/packages/transport/src/base-url.ts、http/session.ts、http/client.ts、notification/client.ts、stream/client.ts、frontend/packages/ui/src/components/AuthView.tsx

## Problem

CodeQL 在四个 Transport 客户端初始化时的 `/\/+$/` 及 AuthView 的邮箱校验 `/^[^\s@]+@[^\s@]+\.[^\s@]+$/` 报告 High `js/polynomial-redos`。在较长攻击者可控输入下，正则引擎可能反复回溯，影响浏览器响应。当前代码使用相同的末尾斜杠删除逻辑四次，但通过不同客户端加载 API、SSE、WS 和登录会话。

## Proposal

- 在 Transport 内部引入小型 `stripTrailingSlashes` 工具，从字符串末尾线性扫描 ASCII 正斜杠并一次切片；原本空字符串、全斜杠和保留中间斜杠的语义不变。
- 四个构造器改为调用同一工具，不改入参、HTTP URL、会话、SSE/WS 重连或身份存储逻辑。
- AuthView 使用一次 `@` 查找、额外 `@` 检查、域段中 `.` 查找及单字符类空白匹配，实现无歧义的线性复杂度，保持正常邮箱语义；最终认证合法性仍由后端权威校验。
- 仅提交 `dev` 的受审 PR，不直接改稳定分支。

## Alternatives considered

- 给原正则附加输入长度上限：虽限制最坏情形，但仍保留潜在回溯与重复实现，且可能意外截断业务地址。
- 改用更复杂的 RFC 5322 邮箱正则：增加维护成本和新的性能风险，前端只需要非权威的基本格式提示。
- 在四处手写相同 while 循环：可行但重复，不如一个共享工具易核对和维护。

## Acceptance criteria

- 代码审阅确认四处原 `/\/+$/` 调用已被替换，共享方法空串、斜杠串、常规路径与极长输入符合原有语义。
- 邮箱输入覆盖普通合法/无效、缺失 `@`、额外 `@`、缺失域名点、首尾空白、内含空白以及极长格式异常值，不发生长时间阻塞。
- `pnpm install --frozen-lockfile`、前端双端构建、GitHub 三项必需 CI 与 CodeQL 最新 PR HEAD 真实结果通过；注意默认分支告警要在合并后再次核验。
- 如果可以连接授权隔离环境，实际完成登录/注册错误提示、API、SSE 与 WS URL 行为验收；无法执行则保留 review / 未测。
- `docs/backlog.md`、Issue #75、PR 审查与上述证据一致；不得假称编译等于完整运行验收。

## Risks

- Email 检查接受的少见边界字符串可能与旧正则不同，需比对原语义和现有后端校验。
- 新帮助函数是 Transport 包内实现，不改变对外 exports；构建之前不得假设各端实际导入正确。
- 当前属于实施中提案，尚未完成真实浏览器/服务端鉴权与长串性能独立验收。
