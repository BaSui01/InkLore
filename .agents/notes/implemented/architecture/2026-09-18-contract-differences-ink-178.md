# Agent Note: INK-178 · R1.1契约勘误记录

Status: implemented

Task: INK-178
Scope: 原 docs/r1-contract-differences-INK-178.md 的历史材料迁移与链接维护

## Problem

阶段性交付证明、契约差异、外部 UI 调研等历史记录混放在 docs 读者文档目录，容易让读者误认成最新有效的规范或功能事实。

## Decision

将原始材料迁入工程记忆，保留其来源、历史约束、验证记录和未完成边界。这是记录整理，不代表原记录涉及的能力已在当前云端完成验收。

## Alternatives considered

继续置于 docs 可维持旧入口但扩大文档噪声；删除历史材料会丢失决策依据与复现信息。因此保留其原有正文并收敛为 notes 记录。

## Consequences

原路径不再作为权威入口；引用必须更新。外部研究中的 UI 功能分析仅作为参考，不对第三方功能或当前 InkLore 的实现状态背书。

## Verification

保留源文档文本，按工程笔记结构调整标题和相对链接；禁用的表意图符号按仓库笔记规范去除。本轮没有重跑源记录中的历史命令。

### 历史原始资料

### INK-178 · R1.1契约勘误记录

用户先选择“保持冻结，推进无冲突部分”；无冲突验收完成后，于2026-09-18明确批准“四项最小勘误，继续开发”。[ADR-0018](../../reference/decisions/0018-minimal-contract-errata.md)已Accepted，R1.1源与派生资产已同步。状态以[backlog](../../reference/planning/backlog.md)为准。

### 原差异与已批准处理

| 操作 | 原机读缺陷 | R1.1修正 |
| --- | --- | --- |
| authChangePassword | parameters为空，与基线副作用幂等要求冲突 | 显式引用公共IdempotencyKey；保留expected_revision及全用户会话吊销 |
| platformDownloadFile | 文字与错误登记要求416，responses缺失 | 补416 application/json Error，复用platform.range_invalid |
| platformExportAudit | 描述要求超限422，responses与错误登记缺失 | 补422及platform.audit_export_limit_exceeded / fix_input；31天或10000条超限整次拒绝 |
| 已知路径未支持方法 | 文字要求405，但无对应JSON错误登记 | platform.method_not_allowed / 405 / fix_input，返回准确Allow（包含中间件支持的OPTIONS）；未知路径仍404 |

本人改密的幂等重放仍先检查当前授权：旧JWT在会话吊销后不能绕过授权读取缓存；新会话同tenant/actor与同请求载荷可重放原结果且不再次吊销。密码请求摘要使用原独立HMAC，缓存不含密码或令牌。

### 变更与追溯

源改动仅在api/platform.json、api/common.json、tools/build-contracts.cjs、00基线和14错误分册。新增错误message_key分别为errors.platform.audit_export_limit_exceeded和errors.platform.method_not_allowed，后续前端任务接入对应词条。

生成器仍输出152路径、233操作；错误目录95→97。与旧manifest逐项比较，除新manifest外仅8个冻结资产哈希变化：上述5个源，加api/errors/platform-1.json及auth/platform两个生成分片。DDL无变化，业务字段、权限、TTL和路径均未变。

修改前清单保存在[0018-freeze-before.json](../../reference/decisions/assets/0018-freeze-before.json)，SHA-256 bcf2bd031d725fec486b484a4da61ac0306b7c0a23863c594c915be54280811d；新清单由原检查器在明确授权后生成。没有改写旧Accepted ADR。

### 验证与边界

运行映射同步97个错误；新实现的真实PG/HTTP与15操作响应验证见[INK-161证据](../testing/2026-09-18-acceptance-ink-161.md)。静态检查通过不能代替运行验收，下载的真实Range行为随INK-162交付。

本次没有调整原依赖与完成门禁。INK-161完整后端验收完成后才启动INK-162/163/164/165；真实Chrome与产品页面验收仍按各自证据记录，不以HTTP请求头模拟冒充浏览器。
