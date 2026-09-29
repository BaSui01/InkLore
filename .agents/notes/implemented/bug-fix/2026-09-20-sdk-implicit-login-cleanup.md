# Agent Note: 共享业务 SDK 隐式登录与硬编码凭据清理

Status: implemented

Task: INK-207
Scope: frontend/packages/transport/src/api/sdk.ts

## Problem

此前 ApiSDK 包含一个内部的 ensureAuthenticated 方法，在未认证时会静默使用硬编码的默认凭据（author@inklore.local / Password123!）进行后台自动登录。这既违反了零硬编码安全铁律，又掩盖了真实的未授权 401 拦截与前端登录回跳流程。

## Decision

1. 彻底删除 ApiSDK 中的 ensureAuthenticated 方法及其内置默认密码；
2. 移除所有业务方法（如 content.listNovels, outline.createScene 等）内部对 ensureAuthenticated 的调用；
3. 严格由上层 AuthSession 与 UI 会话上下文管理登录态，由底层 ApiClient 负责 Bearer Token 注入与 401 onSessionInvalid 触发。

## Alternatives considered

保留 ensureAuthenticated 但改为检查登录态抛出异常：ApiSDK 作为纯业务 API 封装，底层已经有 ApiClient 拦截 401 并触发 onSessionInvalid 回调，若在 SDK 层二次冗余判断会与底层统一拦截机制冲突，直接删除更干净彻底。

## Consequences

- 创作者端与管理端 API 调用行为更加透明可预测，未登录时正确由 ApiClient 捕获 401 并触发用户登录引导；
- 前端代码中不再残留任何测试用的默认用户与密码。

## Verification

1. 编译构建：cd frontend && npm run build:creator && npm run build:admin 顺利通过；
2. 全仓检索 ensureAuthenticated 与硬编码密码，确认无残留；
3. 后端门禁与系统检查全部通过。
