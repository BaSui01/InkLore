# Agent Note: INK-215 · Creator普通路径路由验收

Status: implemented

Task: INK-215
Scope: 原 docs/acceptance-INK-215.md 的历史材料迁移与链接维护

## Problem

阶段性交付证明、契约差异、外部 UI 调研等历史记录混放在 docs 读者文档目录，容易让读者误认成最新有效的规范或功能事实。

## Decision

将原始材料迁入工程记忆，保留其来源、历史约束、验证记录和未完成边界。这是记录整理，不代表原记录涉及的能力已在当前云端完成验收。

## Alternatives considered

继续置于 docs 可维持旧入口但扩大文档噪声；删除历史材料会丢失决策依据与复现信息。因此保留其原有正文并收敛为 notes 记录。

## Consequences

原路径不再作为权威入口；引用必须更新。外部研究中的 UI 功能分析仅作为参考，不对第三方功能或当前 InkLore 的实现状态背书。

## Verification

保留源文档文本，按工程笔记结构调整标题和相对链接；禁用的表意图符号按仓库笔记规范去除。本轮没有重跑源记录中的历史命令。

### 历史原始资料

### INK-215 · Creator普通路径路由验收

### 实施范围

依据[ADR-0023](../../reference/decisions/0023-creator-history-routing.md)。Creator首页/，独立登录/login，注册/register，找回密码/forgot-password，工作台/outline、/worldview、/analysis，编辑器/editor?novelId=作品ID。原Hash入口首次加载转换为普通路径。Admin保持独立，不改其路由。

统一navigation模块管理History导航、popstate与应用内通知。登录守卫及成功回跳replace历史项；returnTo仅同源已知非鉴权页面，保留作品参数。App从地址读取作品ID，移除仅在内存保存当前作品的路由实现。首页灵感仍仅在App内存保留。无新增npm依赖或Go运行路径变化。

### 命令与结果

- `go build ./...`：退出0。
- `npm --prefix frontend run build`：Creator/Admin TypeScript和Vite构建退出0。
- 使用项目TypeScript内存转译navigation.ts并以Node assert断言：外站/双斜线/反斜线/未知路径/API/鉴权页returnTo拒绝；编辑器参数保留；认证路由往返；History通知；旧Hash转换；无参数登录清除旧目标。全部通过，无新增测试文件。此项是路由级断言，不是服务端登录验收。
- Vite开发29000与生产构建预览29002：GET /、/login?returnTo=%2Foutline、/outline、/editor?novelId=…均200且包含应用入口；/v1/not-a-route返回404且不是应用HTML。
- Vite默认SPA回退会对未知/assets/missing.js返回HTML（初次HTTP断言因此失败）；未把Vite开发回退当作生产资源规则。生产Nginx配置独立收紧资源与页面白名单。
- `docker run --rm -v <creator.nginx.conf>:/etc/nginx/nginx.conf:ro nginx:stable-alpine nginx -t`：退出0。
- 临时Nginx容器挂载真实dist，配置仅将监听端口改为127.0.0.1:29003、API上游改为本机29080：首页/登录/大纲/编辑器均200应用HTML；缺失assets、未知页面、未知API均404且非应用HTML。容器验收后停止并自动删除；预览进程已停止。该结果不代表线上已部署或TLS/SSE/WS完整验收。

### Chrome实际操作

| 操作 | 可观察结果 |
| --- | --- |
| 根首页点击登录 | /login?returnTo=%2F，hash为空，独立登录页 |
| 登录页刷新 | 地址与登录表单保留，不404 |
| /login?returnTo=%2Foutline切换注册并刷新 | /register?returnTo=%2Foutline，显示注册页面 |
| 浏览器后退/前进 | 正确回到登录/注册页面，returnTo保留 |
| 打开旧/#/login?returnTo=outline | 自动转换/login?returnTo=%2Foutline，hash为空 |
| 直接打开/editor?novelId=真实验收ID | 未登录转/login，returnTo包含完整/editor?novelId=… |
| 返回根首页 | 无需登录正常显示 |

### 剩余边界

本地身份恢复接口此前返回500，真实成功登录后的作品加载与自动回跳尚未实网复验。本轮已验证路由消费returnTo的往返逻辑，但不声称认证成功；INK-215保持review。部署示例需按实际网络配置API上游和TLS，不会自动部署。
