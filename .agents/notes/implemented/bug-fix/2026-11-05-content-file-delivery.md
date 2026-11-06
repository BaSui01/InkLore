# Agent Note: 作品导出实际文件与确认入口恢复

Status: implemented

Task: #107
Scope: Content 文件适配、审计动作、Creator 导出与独立数据库验收

## Problem

dev 219d981 的 Content 组合根没有注入 Files/Reviews。导出可登记成功但 output_file_id 为空，导入可产生空预览；平台文件层也不接受已声明的 Markdown/EPUB 输出。真实数据库请求进一步发现 content.job.created 等动作未注册，导致提交时整笔事务被审计校验回滚。Creator 的世界观版本读取路径与后端不一致，阻塞作者确认。

## Decision

组合根复用平台文件服务和 Compliance 收据验证，导入以主体、作品、用途绑定文件并加入原事务；缺依赖直接返回统一 503。导出渲染真实字节、校验类型、持久化摘要和文件归属，加入三个精确的任务审计动作。Creator 提供四种格式及可恢复下载历史；丢失响应或 operation_in_progress 保留原幂等键。下载校验会话 epoch，刷新将过期令牌，并公开跨域文件名响应头。确认入口改用现有 world-revision 路由。

## Alternatives considered

保留无文件的成功状态无法满足真实交付。另建文件存储或导出服务会重复已有领域能力。放宽所有文件类型、全局审计动作或租户校验会扩大权限；仅补齐现有格式和动作。

## Consequences

没有数据库迁移或新业务依赖。现有 review_ticket 仍按原契约可选，提供时执行真实收据校验。此修复只完成 #107 的文件交付和确认入口，不代表完整小说 Agent、真实模型调用或全部 Issue 已验收。涉及组合根、平台和工作流，按仓库规则人工审核。

## Verification

隔离 PostgreSQL 17.11、Redis 7.4.2、真实 API：作者保存并确认章节后导出 TXT/Markdown/DOCX/EPUB，逐个验证正文、ZIP/XML、EPUB mimetype、文件大小与 SHA256；并发同键重试返回同一任务/文件；其他租户读取文件、下载与任务全部 404；错误版本、错误收据和错误文件用途被拒绝且无副作用；SQL 确认恰好四份任务对应文件。

Go build/vet、架构可达性检查、文件适配/缺依赖回归测试、gofmt、双端 TypeScript/Vite 构建及 920 个 i18n key 对齐通过。Chromium 在本地受 socket 权限限制，下载、断线后同键恢复、刷新历史与移动视口验证已接入现有浏览器 CI，实际结果以 PR 门禁为准。迁移重放成功但日志暴露历史 BEGIN/COMMIT 提前结束外层事务，另归 #105 修复，不以迁移退出码证明原子性。
