# Agent Note: 全局密码下限改为六个字符

Status: implemented

Task: INK-232
Scope: internal/platform/identity、AuthView、platform API Schema 与身份契约

## Problem

作者、管理员、注册、密码找回、成员创建、CLI bootstrap 和改密共用一个身份服务，但前端校验、Go 验证器与冻结请求Schema都把新密码下限设为12个Unicode字符。要求放宽时若只改界面，会被严格请求Schema或服务端再次拒绝。

## Decision

按[ADR-0028](../../reference/decisions/0028-password-minimum-length.md)，所有新密码至少6个Unicode字符，最多72个UTF-8字节，bcrypt cost仍为12。Go验证器作为写入权威；所有创建/重置/改密路径经统一HashPassword/ValidatePassword，登录/注册/重置Schema同时改到minLength 6。旧有不少于12字符的密码继续可用，不触发用户迁移。

## Alternatives considered

只调前端能更快交付，但严格请求Schema仍会拦截短密码，而且CLI/成员服务也会沿用旧规则。降低bcrypt cost可以提升短密码校验速度，但会同时加快所有密码哈希的离线猜测，因此保留cost 12，仅按用户要求调整长度。

## Consequences

6至11个字符的密码可以用于所有新凭据写入。较短下限降低口令熵，登录限流和现有session撤销仍需有效；本次不引入默认用户、明文凭据或数据库迁移。root登录名/邮箱绑定是独立身份接线，不在INK-232中。

## Verification

`go build ./...`、`go vet ./...`、Creator/Admin 生产构建、`node scripts/build-request-schemas.cjs --check`、`go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts`（924/924）、`go run ./cmd/archguard`与`node scripts/agent-notes.mjs check`均通过。`node docs/system-design/tools/build-contracts.cjs`生成241个操作且issues为空；`check-design.cjs`仍报告平台 API 超过50KB、工作区冻结资产漂移及两份未登记 Agent Schema，未重封其他并行契约。后续在运行服务验证登录接口：5 ASCII 字符密码返回400 `platform.validation_failed`，6 个中文 Unicode 字符密码通过长度校验并到达 bcrypt 凭据比较（错误口令返回401 `auth.invalid_credentials`）；Admin 页面同样拦截5个中文字符、放行6个中文字符发请求。未新增测试文件；72 UTF-8字节上限的真实 HTTP 边界及密码写入/重置流程仍未逐项提交验证。
