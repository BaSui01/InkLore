# auth API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| POST /v1/auth/captcha/verify | authVerifyCaptcha | FR-01 auth.verify_captcha | [platform.json](../platform.json) |
| GET /v1/auth/captcha | authCaptcha | FR-01 auth.captcha | [platform.json](../platform.json) |
| GET /v1/auth/email-capabilities | authEmailCapabilities | FR-01 auth.email | [platform-mail.json](../platform-mail.json) |
| POST /v1/auth/email-challenges | authRequestEmailChallenge | FR-01 auth.email | [platform-mail.json](../platform-mail.json) |
| POST /v1/auth/email-login | authEmailLogin | FR-01 auth.email | [platform-mail.json](../platform-mail.json) |
| POST /v1/auth/forgot-password | authForgotPassword | FR-01 auth.forgot_password | [platform.json](../platform.json) |
| POST /v1/auth/login | authLogin | FR-01 auth.login | [platform.json](../platform.json) |
| POST /v1/auth/logout | authLogout | FR-01 auth.logout | [platform.json](../platform.json) |
| GET /v1/auth/me | authMe | FR-01 auth.me | [platform.json](../platform.json) |
| PUT /v1/auth/password | authChangePassword | FR-01 auth.change_password | [platform.json](../platform.json) |
| POST /v1/auth/refresh | authRefresh | FR-01 auth.refresh | [platform.json](../platform.json) |
| POST /v1/auth/register | authRegister | FR-01 auth.register | [platform.json](../platform.json) |
| POST /v1/auth/reset-password | authResetPassword | FR-01 auth.reset_password | [platform.json](../platform.json) |
| GET /v1/auth/session-csrf | authSessionCSRF | FR-01 auth.csrf | [platform.json](../platform.json) |
| POST /v1/auth/switch-tenant | auth_switchTenant | FR-24 auth.switch_tenant | [platform.json](../platform.json) |
| POST /v1/auth/ws-ticket | authWSTicket | FR-17 auth.ws_ticket | [platform.json](../platform.json) |
