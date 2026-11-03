# Agent Note: Render Free 的 Staging 运行范围和真实验收边界

Status: implemented

Task: INK-255
Scope: render.yaml、deploy/render、免费云端隔离验收工作流

## Problem

Railway 新账号无法启动免费试用，付费 Hobby 超出当前零预算。原有 Railway Staging PR 的 Nginx 与私有服务拓扑依赖付费容器，无法直接声称存在云端 Creator/Admin。仓库虽已有 PostgreSQL 17 + Redis 7 临时 CI 与 Supabase 私有 inklore_ci schema，仍无持久在线浏览器入口。

## Decision

在独立 Render Blueprint 中明确定义一台 Free Go Web Service、两个静态站点和一个 Free Key Value。PostgreSQL 继续使用已验证的 Supabase 专用 inklore_ci schema；蓝图不创建新库、不能在启动时执行迁移，数据库连接只能由 Render 安全环境变量提供。JWT、CSRF、幂等、游标四把签名密钥由 Render 独立生成；外部模型密钥不配置。免费 Key Value 只允许内部网络、关闭持久化、使用 noeviction。宿主服务从环境变量读取 redis:// 内部连接 URL，严禁在解析失败时回退其他 Redis。

静态站点采用到公开 API Web Service 的 Render Rewrite 保持相对 /v1 路由，避免重写现有认证前端；Blueprint 使用候选 onrender.com 地址，但实际分配的域名、Origin、Cookie、POST/SSE 重写均必须浏览器核验。所有服务固定 dev 与 autoDeployTrigger=off，避免开发推送触发不受控构建。资源计划明确 free，拒绝默认付费计划。仅准备代码与无密钥校验，不等于云端资源已经创建。

## Alternatives considered

- Railway Hobby：用户明确没有预算，故本轮不采用。原 Railway PR 保留作未来可回滚参考。
- 三台 Render Free Web 容器运行 API/Creator/Admin：会消耗共计约三倍 Free instance-hour 配额，不符合零预算优先级。
- 将 Supabase 视为 HTTP 运行平台：Supabase PostgreSQL 只提供数据库，不承载此 Go HTTP 进程。
- 直接把浏览器 API 指向独立 Go 域名：前端刷新 Cookie 与 CSRF 均使用 same-origin 合同，跨站直连会破坏现有鉴权。
- 继续只跑 GitHub Actions：可完成可重复集成验收，但不是可供用户直接访问的持久站点。

## Consequences

Render Free Go Web Service 空闲 15 分钟会休眠，后台 Worker 无法保证 24 小时连续运行。免费 Key Value 重启后会清空状态，只能用于允许丢失的临时测试。Free 静态站点外部 Rewrite 不等同于已经证明 POST Cookie 或 WebSocket 代理行为，需要端到端验证。候选 onrender.com 域名不是已分配域名；如名字已占用需同步变更 API 的 Creator/Admin exact Origin 和 Rewrite。现有 Supabase 生产库/已有 public 表保持不变。所有 Key 仅由 Render 生成或通过安全控制台输入，不写进 GitHub。

## Verification

本轮在提交前以 YAML 解析器检查 render.yaml：4 个资源、1 个 Free Web、2 个静态站、1 个 Free Key Value、3 个 Web autoDeployTrigger=off、全部引用 dev。CI 追加 Docker 镜像实际构建、/app/server 与入口脚本存在、缺 DATABASE_DSN 时必须拒绝启动。最终 GitHub Actions 状态需以工作流实际结果为准。Render 未连接时不能宣称已创建云端实例、网址、测试账号、浏览器行为或付费模型调用。
