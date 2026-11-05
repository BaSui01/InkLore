# Agent Note: 当前语义契约门禁的真实剩余差异

Status: implemented

Task: #2
Scope: OpenAPI 幂等声明、权限投影、错误目录与 CI

## Problem

基准 dev 219d981 的 check-design 仍报告 llm_fetch_upstream_models 缺幂等头；随后 check-platform-contracts 报告五个已声明错误缺少运行目录映射。#2 引用的旧冻结哈希和先设计再实施顺序已由合并 PR #110 的源码驱动治理退役；不能据此重新计算旧 hash 或阻止 #103/#104 的真实实现。

## Decision

重读历史 ADR-0027 与现 AGENTS、源码驱动治理记录、当前 00-INDEX 后，保留其不可变策略、澄清 CAS、租户隔离等实际约束，依据当前机器资产检查引用、权限、幂等和错误状态。供应商模型目录的 POST 只执行授权与上游 GET、不落凭据或目录，POST 用于避免把 key 放 URL；增加明确只读声明，检查器同时绑定唯一 operationId、HTTP 方法、路径及声明结构。普通写入不能套用例外。修正 auditor 声明与实际 operator 授权一致。

登记现有三个 question 409、task_policy_version_changed 409 和 auth.invalid_token 400；保留未知错误默认拒绝。现有 CI 加入 contracts-check 和只读例外的正反例。

## Alternatives considered

给无副作用目录查询引入幂等数据库写入会增加不必要的状态。跳过全部 POST 或只凭扩展字段放行会掩盖真实写入缺陷。重算历史冻结 hash 无法修复当前错误目录，因此均未采用。

## Consequences

不改业务接口行为、不执行迁移、不放宽运行授权。此变更只解决 #2 当前静态门禁差异；#103/#104/#106 的真实策略消费和问答恢复仍单独验收。历史 Accepted ADR 和设计文件不构成运行完成证据。同步生成器确认的 README、Agent 内建只读工具导航与 OpenAPI 格式；导航移除的旧工具写入口与现源契约及 ADR-0035 一致，没有新增或删除运行路由。

## Verification

基准的 check-design 退出 1（缺 llm_fetch_upstream_models 幂等声明），修复后 make contracts-check 退出 0，102 个错误映射一致；严格例外三个正反例通过。node docs/system-design/tools/build-contracts.cjs 退出 0，246 个操作、17 个路径分片、102 个错误，无生成问题；保留确定性生成视图。Go build/vet、gofmt、架构检查及工程笔记通过。冻结清单和历史 ADR 未修改；CI 运行结果以本 PR 最新提交为准。
