# Agent Note: 会话初始化必须覆盖 StrictMode 的重复调用

Status: implemented

Task: INK-211
Scope: frontend/packages/transport/src/http/session.ts 的 bootstrap 与 refresh 协调

## Problem

首页验收自动登录后，React StrictMode 重复执行 provider effect，原 bootstrap 各自 GET CSRF 并直接 executeRefresh。同一个 HttpOnly 刷新令牌并发消费，网络证据为一次 200、一次 401，随后作品请求 401、再次刷新 403。用户看到刚登录就退回登录页。

检索已有身份撤权/锁序笔记，它约束服务端重放与撤权，仍然适用；本次是调用方并发缺陷，不改变服务端保护规则。

## Decision

bootstrap 保存一个在途 Promise；同时进入的调用共享整个 CSRF/refresh 初始化。内部通过现有 refresh 单飞入口轮换。finally 清理 bootstrapPromise，允许未来失败恢复重试，而非永久缓存失败或匿名快照。

## Alternatives considered

移除 StrictMode 可以隐藏开发场景，但不能处理多消费者同时初始化。放宽服务端刷新令牌重放保护会削弱安全契约，因此不采用。只在单个 React provider 增加布尔标记不能覆盖其他调用者，故将并发约束放到已有 AuthSession。

## Consequences

初始化现在共享一次轮换，不改 token 存储、Cookie、密码、后端身份或权限模型。已有 refresh 的单飞行为复用；不宣称修复多标签页同时刷新或所有登录退出竞争，它们是不同的协调边界。

## Verification

[首页验收](../../../../docs/acceptance-INK-206.md)记录真实 Chrome 刷新前后请求：修复后 CSRF 200 和 refresh 200 各一次，书架读取 200，重载仍在首页并保留作品。双端构建和 go build 通过；未编写测试文件。运行影响局限于共享前端身份初始化，服务端安全规则保持原行为。
