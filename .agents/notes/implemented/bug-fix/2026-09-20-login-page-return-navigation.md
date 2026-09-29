# Agent Note: 独立登录页保留来源路由

Status: implemented

Task: INK-214
Scope: CreatorSessionContext、App、CreatorLoginRoute及登录入口

## Problem

用户要求登录应跳转独立页面，成功后返回原页面。[公开首页笔记](2026-09-20-public-creator-home.md)中的弹窗选择由本次用户指令替代，匿名首页和后端鉴权结论仍有效。

## Decision

Creator移除AuthModal，openAuthPage生成独立鉴权路由并附带returnTo。返回目标限制为已有Creator视图；登录、注册与找回密码切换保留来源。App无参数登录时清除历史returnTo，成功认证消费当前目标。成功提交只经AuthView一次回调，找回密码完成不当作登录成功。首页灵感提升到App内存，以跨登录路由保留。

## Alternatives considered

弹窗可保持首页组件挂载，但不符合用户指定导航方式，因此移除。任意URL返回便于扩展，但会引入外跳风险；沿用已有Creator路由白名单。浏览器持久化灵感可跨刷新，但会持久存储输入，本次仅保存当前App内存。

## Consequences

公开首页保持无需登录；创建与登录入口转到完整页面。注册与找回密码不会丢返回目标，刷新可从URL恢复returnTo；灵感不跨整页刷新。共享AuthView仍作为独立页面的表单实现，首页不内嵌认证UI。无Go运行链、后端认证和依赖变化。

## Verification

Chrome点击首页登录进入#/login?returnTo=home，页面无首页和dialog；切换注册保留returnTo；连续后退恢复首页输入；访问outline跳#/login?returnTo=outline。go build ./...、npm --prefix frontend run build及定向diff检查退出0。当前浏览器请求session-csrf返回500，因此真实成功登录自动回跳仍待补验，不以路由验证冒充认证成功。未新增测试文件。
