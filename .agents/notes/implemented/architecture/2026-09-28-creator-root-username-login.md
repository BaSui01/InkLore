# Agent Note: Creator username root 登录保持作者权限边界

Status: implemented

Task: INK-235
Scope: Creator AuthView、CreatorSession、platform Login 与认证契约

## Problem

Creator登录页使用HTML email字段，root被浏览器校验拦截；后端也只允许Admin audience使用保留username。用户要求在截图对应的Creator页面直接使用root用户名和密码。

## Decision

按[ADR-0031](../../reference/decisions/0031-creator-root-username-login.md)，Creator/Auth共享登录表单允许email或唯一保留名root，Creator路由将root发送为`username`，其他标识发送为`email`。platform Login在两种audience接受username=root，但Creator仍要求有效活动Creator membership，资源请求继续使用既有tenant和owner scope。注册、找回密码仍仅接收email。

本机Creator/Admin origin采用`127.0.0.1`，与用户当前浏览器URL一致；CORS保持逐origin精确匹配。

## Alternatives considered

要求作者先从Admin端登录或仍将root映射到邮箱都会让当前Creator页继续拒绝用户名。允许任意username或自动切换audience则会扩大认证面；实现限定root身份标识，沿用原有audience和资源授权。

## Consequences

Creator与Admin使用同一个root账号和密码，但分别创建各自audience的会话。Creator用户名登录不继承Admin API访问能力；作品仍由owner scope隔离。Captcha、密码最小长度、邮箱注册及找回流程不变。

## Verification

`go build ./...`、`go vet ./...`、Creator/Admin生产构建、OpenAPI与嵌入Schema检查、i18n 924/924、archguard、notes/skills通过。真实HTTP在Origin `http://127.0.0.1:29000`下验证Creator username/email和Admin username/email均200；Creator token访问Admin tenant目录403，任意非root username在Schema层400。Chromium打开用户截图所用Creator地址，填写root、口令和当前验证码后成功收到200 session并跳转到创作首页，页面显示root主体与退出按钮。排障发现共享AuthSession曾把省略的tenant_id发成空字符串，且双端路由丢弃captcha_id；请求体修正为仅发送有效可选字段后端到端通过。无新增测试文件、数据库字段或密码明文。
