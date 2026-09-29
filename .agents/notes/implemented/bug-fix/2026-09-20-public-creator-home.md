# Agent Note: 创作者首页公开访问与个人数据按需加载

Status: implemented

Task: INK-213
Scope: Creator App、HomeRoute、HomeHeader、useHomeBookshelf

## Problem

原首页复用了工作台登录守卫，首次访问被重定向登录。用户明确纠正：首页应该无需登录。已有文学首页笔记中的登录前提被本次展示行为修正；服务端资源权限不变。

## Decision

首页在会话初始化和受保护路由判断前渲染，守卫排除 home。书架查询只在已登录时启用，匿名显示登录引导。创建调用现有登录弹窗，取消不丢首页灵感。会话纪元变化更换 QueryClient 并清理旧缓存，失去身份时关闭创建对话框。

## Alternatives considered

另建公开站点可完全隔离展示，但会复制首页；仅移除 App 守卫会让匿名发送作品请求并出现错误。因此保留同一首页，分别约束公开展示和私人数据。无需更改后端鉴权或新增依赖。

## Consequences

公开首页不依赖身份恢复成功；个人作品仍需登录。灵感仅保留当前页面内存，刷新不保存。工作台深链仍带 returnTo 要求登录。

## Verification

Chrome真实匿名验证根路径、#/home刷新、创建弹出登录、Escape后灵感保留，零content/novels请求；#/outline跳#/login?returnTo=outline。go build ./...与双端生产构建通过。当前本地session-csrf返回500，因此本轮登录后恢复及退出回归未验证，任务保留review。运行路径仅前端展示和查询条件改变，未写测试文件。
