# Agent Note: Creator 无权限旧开书会话的退出路径

Status: proposed

Task: INK-271
Scope: frontend/apps/creator/src/routes/AgentNovelGuidePage.tsx、localStorage 受限作用域与恢复错误视图

## Problem

此前 Creator 的 Agent 开书引导优先恢复本地保存的 session ID。如果当前身份无权访问该会话，API 返回 403，HomeNotice 出于安全策略不提供 retry，hasPendingGuide 仍为 true 并隐藏新的开书表单。用户无法在页面里退出，只能手工清理 localStorage。该问题来自 #41 的未解决 Reviewer 提醒。

## Proposal

保留 403 的原始安全错误展示，另外在无已加载 guide 且待恢复时提供显式“重新开始”按钮。只有用户点击才删除本身份命名空间下的 session、startKey、startIdea，清理本地旧的 start attempt、错误和 guide 展示状态，并阻止 initialIdea 在下一次 render 后自动再次发起引导会话。按钮本身不发送任何 POST，之后由作者重新编辑文本并明确开始。原 404 自动清理行为和登录/token 后端权限保持不变。

## Alternatives considered

- 403 时立即自动删除本地草稿和会话：可能误删临时无权恢复的会话，拒绝。
- 403 时无限 retry：由于访问权未变化，会重复同一失败且无法开新书，拒绝。
- 在 Creator 调用管理员权限接口恢复会话：违反 audience 与租户隔离，拒绝。

## Risks

前端主动丢弃只影响当前浏览器的本地恢复指针，不会删除服务器原会话。若 403 表示用户确实不可访问，重新开始只能创建独立新引导，不能绕过权限。再次使用 initialIdea 自动启动可能导致额外 POST，故重启处理显式关掉自动启动标志。真实浏览器、退出/登录切换和服务器 403 故障注入仍需独立验收。

## Acceptance criteria

- 正常恢复以及 404 自动清理行为不回退，403 有明确按钮但无自动清理。
- 点击重新开始只删除本身份命名空间的三项本地键，重新呈现 start 表单，不触发背景 POST。
- 对已有失败 guide 的 restart 继续可用，后端授权没有变化。
- 最新 PR CI 的 Creator/Admin 类型检查、生产构建、Skills/notes、CodeQL 和隔离 API 运行通过；真实浏览器操作尚未执行时维持 review。
