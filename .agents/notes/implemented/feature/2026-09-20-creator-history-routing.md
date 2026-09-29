# Agent Note: Creator用普通路径承载登录来源与作品身份

Status: implemented

Task: INK-215
Scope: Creator navigation、App、会话入口、Vite与静态托管

## Problem

用户要求移除Hash路由。[登录回跳笔记](../bug-fix/2026-09-20-login-page-return-navigation.md)的独立页面与安全来源仍有效，但地址形式改为普通路径。编辑器作品ID原仅在App内存，刷新后无法恢复。

## Decision

按[ADR-0023](../../../../docs/decisions/0023-creator-history-routing.md)使用集中History模块，统一push/replace与popstate订阅。returnTo验证同源及页面白名单，作品ID放入editor查询参数。守卫/成功回跳替换历史项，旧Hash只做首次转换。Vite显式SPA，生产配置只给已知页面回退，API与静态资源独立。

## Alternatives considered

Hash无需托管回退，但用户明确不接受。第三方路由库功能完整，当前少量平级路由无需新增依赖。仅修改URL会导致刷新与作品恢复失败，因此同时约束部署和作品参数。

## Consequences

普通链接可直接打开与刷新；新增页面必须同步静态托管路径白名单。灵感仍为内存状态，刷新不保存。Vite默认SPA未知资源回退不代表生产行为；Nginx示例明确缺失资源404。Admin与Go运行链不变。

## Verification

[验收记录](../../../../docs/acceptance-INK-215.md)记录构建、Node内存断言、Chrome路径/刷新/历史/旧链接转换、开发与预览HTTP、Nginx配置和容器HTTP验证。未新增测试文件。真实认证成功回跳受现有身份接口500限制，未宣称端到端登录已通过；线上部署与TLS不在本轮实测范围。
