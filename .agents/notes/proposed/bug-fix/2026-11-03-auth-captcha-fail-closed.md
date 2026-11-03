# Agent Note: Staging 双端认证验证码安全降级与错误诊断

Status: proposed

Task: INK-272
Scope: frontend/packages/ui AuthView、GraphicCaptcha、authError 与 Creator 登录入口

## Problem

Render Staging 实际日志有两次 POST /v1/auth/login 在 0–2ms 内返回 400 platform.validation_failed，属于接入层或验证码校验阶段而非 bcrypt 401。未登录时 GET session-csrf 返回 401 是正常匿名会话恢复。用户截图说明错误横幅不区分验证码过期与其他格式错误。GraphicCaptcha 在获取服务端挑战失败后自行生成本地明文答案，没有 captcha_id；当前 Transport 忽略不完整的验证码字段，使图形挑战可能被客户端绕过。验证码登录的按钮和发送逻辑在未接线时还会 setTimeout 伪造已发送，生产找回密码实际返回 503。

## Proposal

图形挑战只能来自 /v1/auth/captcha 的真实 id+svg；加载失败显示明确可重试的提示并清空 id，禁止提交。服务端返回 details.field=captcha_id/captcha_code 时在 authError 里映射针对性文案，保持其他 400 错误语义。将未提供真实邮件后端回调时的“验证码登录”置灰，不再模拟发送成功；Creator 暂时隐藏不可工作的找回密码入口；注册合同尚未要求真实邮件 OTP，缺失回调时不阻塞注册。统一 AuthView、transport Session 和 i18n 复用，避免两个端各做一套。该修复不改变后端冻结 R1 optional captcha 合同，服务端强制挑战另案审议。

## Alternatives considered

- 错误时继续本地 Canvas 降级：可显示图案但不能建立服务器挑战，破坏真实校验，不采用。
- 将所有 400 一律解释为验证码错误：会掩盖输入 schema、Origin 及其它参数问题，不采用。
- 直接强制所有 API 用户提交验证码：变更既有冻结合同、CI/客户端与自动化集成脚本，不能未审先改。
- 仅要求用户“输入正确验证码”：无法修复客户端假挑战及错误提示混淆问题，不足。

## Acceptance criteria

GitHub CI 中 frontend TypeScript/双端 Vite 构建、i18n、notes、Go 契约均通过；服务端验证码失败时 UI 不显示本地可验证 Canvas，不提交无 captcha_id 请求；过期/错误码有对应用户提示；没有服务端邮件 OTP 时不伪造“已发送”；实际 Render 浏览器验证 Creator/Admin 正常密码登录、401 匿名恢复与失效验证码刷新，错误响应不泄露敏感凭据。

## Risks

当前服务端 captcha 输入是可选项，此 PR 只控制可信前端不绕过，不能声明全面防自动化。Render Free 休眠重启会清空进程内验证码，客户端需刷新重试。浏览器生产页面尚未执行自动验收前维持 review。历史会话曾暴露测试初始密码，应首次登录后修改。
