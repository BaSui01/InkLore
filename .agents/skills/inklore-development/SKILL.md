---
name: inklore-development
description: 用于 InkLore 新功能、缺陷修复、重构、前后端接线与开发任务交付。先核对 docs 和代码现状，再沿七域边界实施，以可复现证据验收并同步文档。
---

# InkLore 开发交付

## 使用边界

在本仓库执行开发任务时使用。纯咨询只读相关资料；仅修文案按影响面简化步骤。决策取舍转到 [inklore-decisions](../inklore-decisions/SKILL.md)，故障、发布与文档巡检转到 [inklore-maintenance](../inklore-maintenance/SKILL.md)。本技能执行宿主规则，不授权改变架构红线，也不把每次操作变成新的确认门。

## 1. 恢复上下文

1. 读 [AGENTS](../../../AGENTS.md) 和目标目录内更具体的规则。
2. 读 [R1设计入口](../../../docs/system-design/00-INDEX.md) 与 [契约基线](../../../docs/system-design/00-CONTRACT-BASELINE.md)，再读 [architecture](../../../docs/architecture.md) 实施现状和 [backlog](../../../docs/backlog.md) 目标任务的状态、依赖、验收条件；历史总数与已完成标题不能替代任务行和本轮验证。
3. 检查工作区改动，保留其他人的未提交工作。按 [文档路由](references/doc-routing.md) 只加载目标域所需设计、契约及 ADR，不把全部 docs 塞入上下文。
4. 分清修复现有行为与实施重构目标。[重构基线](../../../docs/system-design/00-CONTRACT-BASELINE.md) 描述目标，不证明当前处理器已实现。遇到与 AGENTS/已接受 ADR 冲突的依赖边或协议，列出具体条款，暂停该部分并询问；可继续无冲突的资料核对。不得默默挑一份执行。

## 2. 形成可验收任务

- 所有任务登记到唯一 backlog；分配未使用的 INK 编号，不新增第二份 TODO 看板。
- 验收写成“给定状态 → 操作 → 可观察结果”，包括失败路径；依赖已 done，影响域、入口、Owner 清楚才进入 doing。
- 跨多个域的业务实现拆成可独立验收子任务；共享治理资产可作为单独交付。
- 长任务使用一个 goal（写明验证方式）和 todo；当前会话同时只保持一个进行中步骤。没有这些工具时以 backlog 与简短交接记录承接。
- 先按 [工程记忆规约](../../notes/README.md) 检索相关 proposed / implemented / rejected 笔记和既有决策。涉及分层、域边界、存储、协议、全局契约或外部依赖，执行 ADR-First。已有授权覆盖的常规实现直接推进；真正缺少的架构选择集中提问。

## 3. 沿真实调用链实现

按“入口/路由 → Transport → Application → Port/Domain → Infrastructure → app 装配 → 调用方”核对闭环。检索真实包路径；七域迁移未完成时不得假定目标目录已经存在。

- 模块化单体；七域 Owner、DAG 和只读值类型例外逐项对照 AGENTS。跨域 Port 由消费方定义，适配在 app/adapters；不跨域访问私表。
- 正文写入属于 content，事实账本属于 worldview；Agent、灵感和质检只能按契约提交建议。正文确认、版本与事实一致性需要真实事务证据。
- 配置必须追到读取者和运行效果；声明结构、保存成功、包可达都不能证明功能生效。
- 生成事件走 SSE；低频系统终态走 WS。断流、取消、失败与成功终态分开验证，不盲目重发有副作用的生成请求。
- Creator 3000 / Admin 3001 双应用隔离，无互切入口。UI 走真实 API、i18n 与 Lucide；业务数据、进度与提示词按后端契约生成。
- 时间遵守 UTC 存储与 timeutil 北京时间业务转换；密钥只环境/Secret Store 注入且证据脱敏。
- 不写或改测试文件；手写代码小于等于 50KB，不新增 Emoji。复用现有命令与护栏。

## 4. 验证与同步

按 [验证矩阵](references/verification.md) 选择检查。必须运行 go build；行为变更复现正常与失败路径。未配置服务、缺真实模型或没有浏览器时明确“未验证”，不能以编译成功冒充端到端通过。

使用 [验收模板](templates/acceptance.md) 整理命令、预期、实际、退出码和证据位置。证据简短时放 backlog 验收列，长证据另建按 INK 命名的记录并链接；不保存明文 Token、用户正文或生产密钥。

同步受影响的 architecture、设计/接口文档、backlog、CHANGELOG；执行进度只更新 backlog；冻结实施计划仅在正式 R1.x 变更时更新。非平凡变更同批新增或更新 `.agents/notes/`，保留背景、真实备选、代价和实测证据；纯机械改动注明无需 note。方案落地将 proposed 移到 implemented 并更新状态、标题和入站链接；理由翻转另写新 note，不能改写旧结论。运行 `node scripts/agent-notes.mjs check`，涉及归档再加 `--base HEAD`。完成实现先 review，全部 DoD 满足再 done。

## 5. 交付与中断交接

汇报：任务 ID → 状态 → 改了什么 → 验收证据 → 剩余风险。交接保留已读依据、改动文件、已执行命令、未验证项及下一步，避免下个会话重新猜测。不自动提交、发布或重置他人改动。
