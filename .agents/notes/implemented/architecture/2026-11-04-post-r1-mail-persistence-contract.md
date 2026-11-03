# Agent Note: post-R1 邮件配置与邮箱 OTP 持久化契约

Status: implemented

Task: #98
Scope: migrations/post_r1/000001_platform_mail_settings.sql、platform_email_challenges、Admin 邮件配置与 AuthSession OTP 路径

## Problem

R1 冻结的 17 个迁移没有可持久化 SMTP 配置的通用 settings 表；AuthView 验证码在回调缺席时伪造成功，OTP Token 停留在进程内存。本契约是新增的全局配置合同、凭据存储选择与持久化认证挑战，属于跨发布版本的架构边界，按决策记录要求正式留档。背景与提案见 [proposed 笔记](../../proposed/feature/2026-11-03-smtp-email-verification-settings.md)；ADR-0021 明确将邮件供应商集成排除在 R1 范围之外，本记录承接该排除项。

## Decision

新增独立 `migrations/post_r1/` 迁移流（000001 起），与冻结 R1 流并列，不改写、不重放旧迁移：

- `platform_mail_settings`：全局单行（id=1），保存 provider（disabled/smtp/resend）、enabled、SMTP host/port/security、发件人、凭据（kind=env 引用环境变量名，或 kind=encrypted 存 `enc:v1:` 密文 + 外部密钥版本）、CAS revision 与 updated_by 审计。
- 两条复合 CHECK 约束全部用外层 `IS TRUE` 收紧，杜绝 PostgreSQL 对 NULL 结果放行：不完整的 env/encrypted 凭据、enabled 但缺端口/TLS/发件人的配置均被数据库拒绝。
- `platform_email_challenges`：purpose/audience/tenant/actor 作用域，recipient 与 code 只存 SHA-256 哈希，状态机 pending→sent→consumed/failed/expired，5 次上限、单次消费、到期失效；DELETE 权限只授予 retention 身份，runtime 无删除权。
- 凭据明文不入库、不入日志、不入响应；加密密钥只存在于环境变量或 Secret Store。

以上契约由 PR #102 交付。

## Alternatives considered

- 沿用 config.yaml/环境变量作为管理端配置权威：无法支持 Admin 动态编辑与审计，放弃。
- 复用 `llm/infrastructure` 的 secret_crypto：跨域导入违反域依赖红线，放弃；AES-GCM 能力如需共享先提取平台组件。
- Redis 作为 OTP 唯一存储：实例重启丢失挑战且不可审计，只允许作冷却限流。
- 首版同时接入 Resend/多供应商与余额提醒：扩大供应商与权限边界，推迟。

## Consequences

- 收益：Admin 可持久化编辑邮件配置并有真实启用门槛；OTP 从进程内存升级为可审计的数据库状态机；凭据存储双路径（env/encrypted）兼容密钥未就绪的部署。
- 代价：新增两张私有表与一个 post-R1 迁移流，部署必须先执行迁移；DB 是 OTP 权威，Redis 降级只影响冷却体验。
- 失效条件：若未来引入第二租户级 SMTP 配置或邮件模板管理，需扩展本契约而非改写；`enc:v1:` 密文格式变更必须追加迁移。
- 替代关系：替代 ADR-0021 中被排除的邮件集成计划；与 [proposed 笔记](../../proposed/feature/2026-11-03-smtp-email-verification-settings.md) 合并阅读，后者保留方案取舍全文。

## Verification

- `go build ./...` 在本分支退出 0（含合并 dev 去阶段化治理后）。
- 专用 CI「INK-273 Email Persistence Contract」在真实 PostgreSQL 17 服务上执行迁移与 `verify_email_schema.sql`：断言迁移历史 17 条不变、单例持久化、runtime/retention 权限边界、5 组 CHECK 违例（enabled 缺 TLS/凭据、明文密文、env 缺 locator、encrypted 缺 ciphertext、NULL 端口）、OTP 首次消费成功与重放拒绝。
- 未验证边界：真实 SMTP 供应商投递、Admin UI 发送测试邮件与浏览器端到端验收未包含在本基础 PR，由 Issue #98 剩余验收项跟踪。
