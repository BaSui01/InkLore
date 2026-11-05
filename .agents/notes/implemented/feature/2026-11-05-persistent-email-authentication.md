# Agent Note: 持久化邮件配置与验证码身份闭环

Status: implemented

Task: #98
Scope: platform/mail、identity 邮箱验证/登录/注册/重置、App HTTP、Creator/Admin Auth UI、全局邮件配置

## Problem

基线 `219d981` 已有 `post_r1/000001_platform_mail_settings.sql`，但没有运行时投递器或邮件配置接口。找回密码只在进程内保存开发令牌，生产端点返回不可用；共用 AuthView 在缺失回调时仍可能触发成功，Admin 验证码分支会直接 return。读取契约和既有迁移不能证明投递或身份闭环已实现。

## Decision

复用已有配置/验证码表、UoW、身份会话、错误目录、Redis 登录限流与审计。SMTP 使用 465 implicit TLS 或 587 必需 STARTTLS；HTTPS 使用固定的 Resend `/emails` 接口。投递均设截止时间，验证证书/主机名，精确主机 allowlist，拒绝所有非公开 DNS 答案并绑定已校验 IP，禁用环境 HTTP 代理和重定向。只有 SMTP DATA 确认或 HTTPS 合法回执后才记录 sent。

全局配置由实时验证的 `super_admin` 管理，更新要求 revision CAS、Idempotency-Key、CSRF 和 Origin 检查。凭据只接受 `MAIL_CREDENTIAL_` 环境引用或 AES-256-GCM 密文；AAD 绑定 provider/key version。版本化主密钥只在环境中提供；读取结果只有 has_credential 和类型，不包含 locator、密文或明文。测试邮件只发给当前管理员已验证的本人邮箱，并有独立的持久幂等预约和 60 秒限流，外部 IO 不占业务事务。

验证码为加密随机六位数，五分钟有效；邮箱/IP/code 使用不同域的 HMAC。新迁移 `000007_email_auth_runtime.sql` 扩展原表的请求、IP、请求租户绑定，加入用户 email_verified_at 与测试投递表，不修改历史迁移。全局 request_key 与请求摘要绑定，相同 key 改邮箱/用途/端别返回冲突。PostgreSQL 串行执行邮箱/IP/用途限额：邮箱 60 秒冷却、每小时 5 次、单用途每小时 3 次、IP 每小时 30 次。无账号的请求同样使用真实、有界投递与相同限额，对外不暴露账号/租户是否存在。

验证时按 challenge 行锁持久增加错误次数，最多 5 次；错误响应在提交次数后返回。挑战绑定邮箱、purpose、audience、请求租户与签发时已解析的成员租户。消费、创建既有 Session、验证时间和审计在一个 UoW 内；重置则与密码更新及全部 Session 吊销原子提交。普通密码注册保持兼容，email_verified_at 为 NULL；只有显式提供并消费 register challenge 才标记验证。开发 reset token 和内存 resetStore 已移除。

Creator/Admin 复用 AuthView、AuthSession、ApiClient、i18n 和现有组件。能力不可用时禁用验证码登录/找回，发送失败不显示成功；普通注册保留，验证码注册为可选分支。会话 token 仍只在内存；仅 purpose/audience/邮箱绑定的挑战 ID、到期时间、冷却时间和请求 key 存入 sessionStorage，用于刷新/断网后重试原请求，不保存 code/password。Admin 配置保存清空凭据输入。ApiClient 写命令补齐当前内存 CSRF 头。

真实并发测试暴露 `identity.member` 向复制的 Principal 角色 slice 解码会写共享底层数组。解码前清空两个 slice，使每次实时身份校验拥有独立数组；不改变权限判据。

PR #126 的后续 CI 暴露两处验收环境缺陷：邮件 schema verifier 仍无条件插入已经由 `000007` 播种的单例；App HTTP 测试依赖前一 CI shell 中不会持久到下一步的认证密钥。前者改为兼容已有禁用单例，并把全部验证 fixture 放在显式回滚事务内；保留约束、权限、单次消费和冻结迁移数量断言。后者由测试生成独立的临时随机密钥与本地 Origin，不放宽生产配置检查或更改已应用迁移。

邮件最终传输边界再次验证邮箱、UUID、UTF-8 和邮件头 CR/LF/NUL。验证码内容只使用固定的用途标签，不把 HTTP purpose 值直接拼入邮件。真实 TLS SMTP 回归同时检查中文 MIME 主题、独立点号的 DATA 转义，以及正文中的 `Bcc:` 仍作为纯文本；两种提供商均在 DNS 前拒绝注入的邮件头。

## Alternatives considered

未复用开发明文重置 token，也未把日志写入当作发送成功。未采用 Redis 作为验证码权威状态，避免故障或进程重启清空消费次数。未为本地验收开放私网目标或关闭证书验证；测试专用 CA 和私有 dial seam 只在 `_test.go` 中把已校验地址接到真实 TLS socket。未知账号直接跳过网络投递会引入响应时间/状态差异，所以统一经过同一受限投递路径。SMTP 结果未知时不会自动重发同一预约。

## Consequences

需先应用新迁移。所有副本配置独立 `MAIL_OTP_HASH_KEY`；加密凭据需 `MAIL_ENCRYPTION_KEYS` 与 `MAIL_ENCRYPTION_KEY_VERSION`，环境引用需对应 `MAIL_CREDENTIAL_*`；出站主机需 `MAIL_ALLOWED_HOSTS`。更换 OTP HMAC 密钥会使未消费挑战失效；主密钥轮换需保留旧版本直到密文重新保存。部署配置说明保留在现有 `.env.example`。

源码与生成的 OpenAPI/Go/TS 契约同步更新。该改动涉及身份、凭据、持久化和迁移，保持人工评审边界；本笔记不构成生产部署或关闭 Issue 的证据。

## Verification

- `go build ./...`、定向 `go vet`、`go test ./internal/platform/mail ./internal/app`、`go run ./cmd/archguard`、双端 `pnpm run build` 与 i18n 检查通过。
- 真实 PG17/Redis7，完整迁移及幂等重放后运行：`MAIL_AUTH_PG_TEST=1 go test -race ./internal/platform/mail -run TestEmailPostgres -count=1 -v`。10 个场景通过：绑定/重启、全局请求 key 冲突、重复发送不重发、五次失败持久上限、并发单次消费、未知邮箱实际投递、注册验证、密码重置及全部旧 access/refresh 拒绝、审计故障回滚、匿名 IP 第 31 次拒绝、配置并发 CAS、过期/投递失败。实际测试中发现的共享角色数组 race 已修复并重跑通过。
- 实际组装 App/HTTP/PG 验证：`MAIL_AUTH_PG_TEST=1 go test ./internal/app -run TestMailAuthHTTPPostgres -count=1 -v`，配置 CAS、幂等回放、审计仅一次、凭据不回显、Creator/非全局权限、CSRF/Origin、未验证本人测试拒绝、已知/未知邮箱投递失败同为 503、旧开发 token 拒绝均通过。
- `go test -race ./internal/platform/mail -run 'TestSMTP|TestMailSSRF|TestHTTPS' -count=1` 通过。真实 TLS SMTP sink 验证 implicit TLS/STARTTLS 与证书验证；真实本地 HTTPS socket 验证 Resend 请求/回执/幂等头/拒绝重定向。不是外部供应商投递证明。
- 完整契约静态检查在基线还存在既有 `llm_fetch_upstream_models` 缺 Idempotency-Key 与错误目录漂移，依赖 #2 的独立修复；不据此降低验收。
- 本地 Chromium 启动因执行环境 `process_singleton_posix.cc socket() EPERM` 阻塞，未绕过；浏览器真实页面验收需交由已有 CI，标为 `NOT_VERIFIED`。外部 SMTP/Resend 供应商凭据、DNS/发件域名和本人真实邮箱最终收件验证均 `NOT_VERIFIED`，不能关闭 #98。
- `E2E_MAIL_UI_PG_TEST=1 python3 scripts/ci/mail_ui_acceptance.py` 在真实隔离 PG/Redis/API 通过。专属账号经真实注册/密码登录，只有角色通过显式 PG fixture 设置；Cookie/会话经匿名 stdin 交给 `mail_ui_browser.cjs`，无凭据文件，也不复用其他 CI 旅程的登录额度。`E2E_RUN_BROWSER=1` 供已有 Chromium CI 验证能力关闭、原身份 Cookie 恢复、管理员边界、实际表单写入、并发 API 写入导致的真实 CAS 409/草稿保留、真实主机校验 400 和 390px 布局。整个旅程保持邮件关闭并在 finally 恢复配置，不伪造投递或 HTTP 成功；此浏览器脚本本地仅语法检查，OTP 浏览器全链路仍未验证。
- 历史迁移 BEGIN/COMMIT 外层事务缺陷由 #105 另行修复；本次迁移成功不作为历史迁移事务原子性证据。上述注入失败验证的是当前业务 UoW 原子性。
- PR #126 CI 修复在包含 #2 契约依赖的交付基线上通过 `go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`、`node scripts/check-platform-contracts.mjs` 和邮件传输定向 race 测试。CodeQL 邮件内容注入线程需远端重新分析后判定，不能用本地功能测试代替 CodeQL 结论。
- CI 修复最终在隔离 PG17/Redis7 上再次运行全部邮件 PostgreSQL race 用例与 App HTTP 用例并通过。`psql -X -v ON_ERROR_STOP=1 -f migrations/post_r1/verify_email_schema.sql` 连续两次成功，前后设置完整行和 challenge 数量均不变。HTTP 验证特意清除父环境的 JWT/CSRF/幂等/游标/OTP 密钥及 Creator/Admin Origin 后启动，证明其独立配置真实生效。
