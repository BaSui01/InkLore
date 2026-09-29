# Agent Note: 登录页采用克制的编辑式视觉

Status: implemented

Task: INK-231
Scope: frontend/packages/ui/src/components/AuthPageLayout、AuthView、GraphicCaptcha 与认证词条

## Problem

Creator 登录页使用蓝色主按钮、米色字段、网格渐变底纹和悬浮卡片，品牌与表单叠加了多层装饰；验证码又沿用蓝紫高饱和色，整体偏向自动生成的 SaaS 登录模板。Admin 复用同一认证组件，也继承了这些问题。

## Decision

共享认证页改为深墨绿品牌栏与浅中性色表单区，桌面采用左右分栏、小屏转为上下结构。登录专属语义色限定在 `.auth-shell`，避免改变 Creator/Admin 已登录业务页面的主题。移除尚未接通的 Creator 社交登录入口，并让 SVG 与 Canvas 验证码继承认证页色板；中英文登录文案改为直接的身份/操作提示。

## Alternatives considered

改写 Creator/Admin 全局主题 token 会同时影响编辑器、治理表格与图表，超出截图所示登录页范围。保留居中卡片但只换按钮颜色，无法消除网格、渐变和多层浮卡造成的模板观感。

## Consequences

Creator 与 Admin 的全屏认证入口保持行为和双端隔离，仅共用布局与局部色板。登录主题变量在认证根节点定义，因此不会把其他业务页面强制改成绿色主题。当前验证覆盖桌面页面；窄屏布局由响应式规则实现，本轮未完成真实移动设备截图验收。

## Verification

`npm --prefix frontend run build:creator`、`npm --prefix frontend run build:admin`、`go build ./...` 均退出 0；`go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts` 报 924/924 一致；目标文件 `git diff --check` 通过。浏览器分别检查 `http://127.0.0.1:29000/login?returnTo=%2F` Creator 与 `http://127.0.0.1:29001/#/login` Admin 桌面页面；认证控件可见、验证码配色已收敛，未提交登录或创建管理员账号。
