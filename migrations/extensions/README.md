# Platform extension migrations

这个目录是**独立扩展流**：`cmd/migrate` 先用 `InkLore/migrations`（主流）应用与校验 17 条既有迁移，
再用 `InkLore/migrations/extensions` 与独立历史表 `schema_migrations_extensions` 应用本目录的扩展迁移。
主流迁移的版本、文件名与校验和必须保持不变。

既有环境若仍在使用旧表名 `schema_migrations_post_r1`，必须先按
[切换指引](../../docs/maintenance/stage-naming-cutover.md) 完成一次幂等改名再运行迁移：目标版本表的创建是
`CREATE TABLE IF NOT EXISTS`，顺序错误会让本目录的扩展迁移被当作未应用。

## 邮件与 OTP（INK-273 第一阶段）

`000001_platform_mail_settings.sql` 是扩展流的迁移，负责两个由数据库持有的对象：

- `platform_mail_settings`：全局 Admin 邮件服务配置，含 CAS `revision`；凭据使用外部密钥加密或经环境变量引用，
  不落明文、不经 API 暴露。
- `platform_email_challenges`：带 audience/purpose/租户作用域的哈希 OTP，含过期、尝试计数与一次性消费；
  即使 Render Free Redis 重启也必须持久。

该迁移使用当前 SQL search path：在专用 Supabase staging schema 上执行时，执行身份必须先把路径设为
`inklore_ci,extensions,public` 并核对 schema 与数据库身份；在临时 CI 数据库中则运行于 `public`。
本迁移**不属于主流**；其元数据、执行器与发布门禁只随已评审的邮件运行时启用，不会在服务启动时被静默执行。

初始范围仅包含 **schema + 约束 + 权限**。SMTP 投递、Resend HTTP API、Admin 页面、邮箱 OTP 登录与密码找回
都不由这份 SQL 启用；它们必须复用现有 Identity、AuthSession、httpguard/requestschema、audit 与 GORM/UoW，
并在生产发布前完成独立的浏览器与供应商 mock 验收。

独立验证使用 `.github/workflows/mail-persistence.yml`：它启动一次性 PostgreSQL 18、执行 17 条主流迁移、
应用本扩展一次，然后检查 schema 约束/权限与原子一次性 OTP 消费。

在完整邮件契约与数据库迁移门禁通过评审前，不要直接对共享 Supabase 项目执行这份 SQL。
