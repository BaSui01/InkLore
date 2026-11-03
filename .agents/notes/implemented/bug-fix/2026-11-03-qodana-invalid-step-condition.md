# Agent Note: Qodana 工作流的条件上下文与凭据可用性门控

Status: implemented

Task: INK-257
Scope: .github/workflows/qodana_code_quality.yml、Qodana Token 可用性与独立 CI 触发

## Problem

原 Qodana 工作流通过 steps.if 直接访问 secrets.QODANA_TOKEN，在 GitHub Actions 中该上下文不允许用于此位置。工作流在 job 创建前解析失败，因此此前 Qodana 的失败记录只有 0 个 job，不能推断代码静态检查已发现缺陷。触发分支仍为已废弃的 main/main-r1，而当前分支是 dev/private。

## Decision

在 job 的独立步骤 env 中读取 QODANA_TOKEN，只写入 available=true/false 布尔输出。后续 Qodana Scan 与安全跳过步骤仅依赖该布尔输出；原始 Token 仅在环境变量中传递，不写日志、GitHub outputs 或提交。push 触发分支改为 dev/private，降低 contents 至 read。实际扫描仍由既有 Qodana Action 在 Token 授权的上下文执行。

## Alternatives considered

- 直接在 step.if 判断 secrets：GitHub 表达式上下文不允许，不能使用。
- 不存在 Token 时让整个 workflow 解析失败：缺乏可信诊断，导致所有 PR 持续红灯，不采用。
- 使用空白 Token 伪造 Qodana 扫描成功：会误导安全验收，拒绝。
- 临时在源码中保存 Token：违反 AGENTS 安全边界，拒绝。

## Consequences

无 Go、React、数据库或真实模型运行路径修改。工作流不再因无效表达式在初始化前失败；无 Token 会清楚标记为未做实际 Qodana 云端扫描。Qodana 真实高风险检查与供应商 Token 配置仍需独立管理，不能把跳过当成通过。旧 Issue/分支曾使用 INK-254；该编号已分配给仓库自动合并治理，当前唯一任务是 INK-257。

## Verification

变更后的 Qodana workflow 由 actionlint 静态检查通过（退出 0），以前代码的两个 secrets 相关报错不再存在。GitHub PR #34 上已观察到 qodana job 成功创建并走到缺失 Token 时的跳过分支；该成功仅说明工作流可解析，不证明完成外部 Qodana 扫描。重新合并最新 dev 后必须再次运行三项必需 CI；实际带 Token 的云端扫描仍未验收。