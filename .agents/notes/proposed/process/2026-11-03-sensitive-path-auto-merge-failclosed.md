# Agent Note: 扩展安全自动合并的身份与持久化路径保护

Status: proposed

Task: INK-254
Scope: .github/workflows/safe-auto-merge.yml、默认分支安全治理

## Problem

自动合并工作流默认只将若干安全路径加入敏感目录正则。实际 InkLore 身份模块位于 internal/platform/identity/，许多持久化仓储位于 internal/content/infrastructure/ 和其他领域的 infrastructure 子目录；原正则未覆盖这些路径。若维护者为同仓库 dev PR 标记 auto-merge-approved，可能导致身份和持久化改动绕过本项目“此类变更始终人工合并”的治理意图。

## Proposal

保持现有低风险文档白名单和源码授权机制不变，扩大不可自动合并的路径匹配：
- internal/platform/identity/ 和此前的 auth、crypto、database、uow、middleware、secret 目录仍应人工合并。
- internal 下任意业务域及深层嵌套的 infrastructure 目录都按敏感持久化/外部基础设施处理。
- 路径检查仍同时检查 filename 和 previous_filename，重命名和 Draft、旧 Head 竞态撤销逻辑保持不变。

自动合并工作流运行在默认分支，更新先走受保护 PR 合入 private；无需新的 token、权限或执行 PR 源码。此变更不会让业务代码自动通过 CI，不删除任何分支。

## Alternatives considered

- 只保护 internal/platform/identity 而忽略其他领域基础设施：仍可能自动合并数据库和第三方 Provider 更改，拒绝。
- 禁止所有 internal 源码自动合并：保护面最广，但会取消现行非敏感源文件显式批准能力，不采用。
- 通过额外标签绕过安全路径规则：与已明确的“敏感路径始终人工合并”冲突，拒绝。

## Risks

保护范围更广会提高部分 PR 的人工 Review 数量，但保证身份/数据库/持久化改动不会误入自动合并。已有由人批准的普通代码 PR 不改变审阅规则；还须继续依靠 GitHub 分支保护检查与 Review 对话。路径正则只作为 fail-closed 分类，不是认证或代码安全测试。

## Acceptance criteria

- 认证目录及任意业务域 infrastructure 目录在脚本的路径匹配中均被判为敏感，不能因 auto-merge-approved 例外放行。
- 低风险文档的原有自动合并策略不变，真正的 GitHub PR 必需 CI 与 Review 对话仍受保护。
- actionlint、脚本正反样例、Skills/notes、Git diff 和最新 GitHub 检查成功后仅由人工合入默认分支。
## Verification

须在最终 PR head 运行 actionlint、安全路径正负样例检查、现有 Skills 与 Notes 校验，以及 GitHub 三项必需 CI。测试样例至少包含 internal/platform/identity/login.go、internal/content/infrastructure/repository.go、internal/agent/runtime/infrastructure/worker.go 均禁止自动合并；非敏感 internal/content/application/handler.go 仍只在显式授权下可合并。真正的 GitHub pull_request_target 事件也应在默认分支合并后监控。
