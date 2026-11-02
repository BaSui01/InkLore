# Agent Note: Go 加密依赖安全升级与双分支发布

Status: proposed

Task: INK-258
Scope: go.mod、go.sum、认证和加密运行路径、dev 到 private 安全发布

## Problem

Dependabot PR #28 直接向稳定 private 分支提出 x/crypto 与关联的 x/net/text/sys/sync 升级。原 PR 已有 Review 指出缺少工程任务、运行影响分析及独立验收记录，而且当前 dev 与 private 版本分叉。直接合并 private 会让稳定分支依赖图领先开发分支，增加后续发布合并冲突，也无法证明新的加密和认证运行路径兼容。

## Proposal

在最新 dev 派生独立分支，只复用 Dependabot 官方生成的 go.mod、go.sum 更新。保留依赖锁定结果，不修改业务源码、冻结契约或历史 SQL。提交 PR 指向 dev，运行 Go 1.27.1 模块校验、Go build/vet、架构检查、前端双端构建和 Skills/notes；根据鉴权、密码哈希、租户访问相关的真实 HTTP/隔离 PG Redis 验收给出独立证据。验收完成后再评估如何经受保护 PR 将 dev 的改动发布到 private，原 Dependabot PR #28 保留审查历史直到被正确取代。

## Alternatives considered

- 直接合并原 Dependabot PR #28 到 private：缺少 dev 先验收且违反任务交付要求，拒绝。
- 将整个原 Dependabot 分支 rebase 到 dev：会混入两个长期分支的历史分叉与其他不相关文件，优先采用仅移植已核对的依赖变更。
- 靠 CI 静态全绿便宣称漏洞已修复且运行兼容：无法覆盖身份、授权与实际连接故障，拒绝。

## Acceptance criteria

- 项目记录 INK-258 与相应 PR，所有模块版本和 go.sum 均来自可核实的 Dependabot PR #28；不引入其他依赖。
- 在最终 head 跑 go mod verify、go build ./...、go vet ./...、archguard、notes/skills、pnpm 双端构建；保留 CI 链接及失败信息。
- 在隔离数据库上验证真实登录、Token、授权拒绝/跨租户行为与失败模式，不读取生产密钥，不创建项目禁止的测试文件。
- 通过独立 Review 后先合入 dev，再单独经分支保护发布到 private；不强推、不绕过保护，未验证不标 done。

## Risks

x/crypto 与 x/net 等底层依赖的版本变更可能影响哈希、TLS、SSH 或 Go 内部依赖兼容。Dependabot 的升级建议不保证 InkLore 所有使用场景通过；若回归失败，应保留 review/blocked 并修复，而非关闭门禁。
