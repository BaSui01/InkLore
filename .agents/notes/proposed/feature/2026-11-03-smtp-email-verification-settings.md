# Agent Note: Admin SMTP 持久化配置与真实邮箱验证码方案

Status: proposed

Task: INK-273
Scope: Platform 邮件配置与验证码存储、Admin 设置页、Creator/Admin AuthSession

## Problem

InkLore 的 80 张 R1 私有业务表没有可持久化 SMTP 配置的通用 settings 表。管理端界面未提供邮箱发送配置，AuthView 验证码发送路径在回调缺席时会伪造成功，生产 forgot-password 明确返回 503，验证码 Token 尚使用进程内存。用户希望通过 Admin 的 SMTP 模式配置邮件并写入 PostgreSQL，支持可实际收取/消费的验证码。

## Proposal

在经审查的后续版本化迁移中新增 platform_mail_settings（全局一行、CAS revision，保存 host/port/TLS 模式/发件人/凭据密文或 Secret Store 引用、启用状态、操作审计）与 platform_email_challenges（邮箱/用途/租户作用域、带服务端 pepper 的 OTP 哈希、失效时间/尝试次数/消费状态，不存明文 OTP）。初版只支持 SMTP；可选 STARTTLS 587 与 implicit TLS 465，始终验证服务端证书，不提供跳过校验。管理员只允许经授权的 super_admin 更新并发送测试邮件。配置读取只显示安全摘要与 has_password，密码更换独立；运行时不从日志/响应输出明文。

优先复用 Platform 组合根、UoW/GORM、现有 audit/identity/captcha/rate limiter、strictjson/requestschema 与 AuthView/transport/i18n；AES-GCM 能力如需共享应提取安全基础组件，不允许 Platform 直接导入 llm/infrastructure，也不能复用其开发密钥后备。验证码 6 位、5 分钟有效、单次消费、最多 5 次失败、60 秒冷却，发送与消费关联明确目的和身份，避免帐号枚举/并发重放；若使用 Redis 限速，其 Free 实例重启不能作为唯一挑战存储。邮件发送避免在持锁业务事务内阻塞，必要时使用现有 outbox/jobs 且按 5 分钟 TTL 停止重试。

Admin 采用独立“邮件与通知”路由，仅显示服务器端保存的配置、测试发送与发送状态；Creator/Admin 双端继续复用 AuthView，真实 API 后端准备前 OTP 入口保持禁用。冻结 R1 的 17 个旧迁移不改、不对共享 Supabase 直接迁移。

## Alternatives considered

- 使用 config.yaml/Render 环境变量作为管理端配置权威：不能支持 Admin 动态编辑/审计，不符合持久化要求；仅保留密钥主密钥在安全环境变量。
- 复用现有 llm secret_crypto.go：它属于 LLM Infrastructure，跨域导入违反 AGENTS 依赖图，不能直接使用。
- 只用 Render Free Redis 保存邮箱 OTP：进程重启后挑战丢失，消费和审计不可复核，接受 Redis 作限流不作为唯一权威。
- 引入新的 SMTP/Resend/余额提醒一次性全功能：增加供应商、支付、权限边界，首版只做 SMTP。

## Acceptance criteria

迁移新增两张表且在独立 PG17/schema 下通过重放及权限核验；无明文凭据落库/日志/响应、仅 super_admin 可编辑；TLS 验证和 host 安全约束；SMTP 测试桩实际发出 6 位验证码，首次消费成功后重放、过期、跨租户、次数超过上限拒绝；不存在用户也不泄露账号；Admin/Creator/重置流程真浏览器集成验收，失败不显示伪成功。不得使用付费模型或用户生产邮箱做高频测试。

## Risks

SMTP 供应商可能限制发件率/需要单独授权，邮件投递可延时，安全挑战必须与真实发信原子状态一致并抗重复消费。发件口令轮换需要迁移与撤权。新增表及 API 属于正式合同变更，需 ADR/Codegen/安全审查，并在隔离环境验证后迁移共享 Staging。
