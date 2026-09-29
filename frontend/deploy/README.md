# Creator 普通路径部署

依据 [ADR-0023](../../docs/decisions/0023-creator-history-routing.md)，Creator 使用 History 路由，首页 `/`，独立登录页 `/login`，工作台 `/outline`、`/worldview`、`/analysis`、`/editor?novelId=…`。Admin 是独立应用，不能把其构建产物或路由挂到此站点。

开发：`npm --prefix frontend run dev:creator -- --strictPort`。Vite SPA 回退支持直接访问和刷新；`/v1` 和 `/healthz` 先进入既有后端代理。

构建预览：`npm --prefix frontend run build:creator` 后，执行 `npm --prefix frontend run preview -- --host 127.0.0.1 --strictPort`。若开发服务占用端口，可显式传 `--port` 选择未占用的验收端口。

生产静态托管示例为 [creator.nginx.conf](creator.nginx.conf)。它是完整 Nginx 配置：

1. 将 `frontend/dist` 部署到 `/usr/share/nginx/html`，只放 Creator 产物。
2. 按部署网络修改 API upstream（示例 `127.0.0.1:8080`）。容器中的 localhost 指容器本身，不能盲用；应改成实际后端地址。
3. 监听端口默认 3000；本地 29000 是 `.env` 开发覆盖，不改变双端隔离约定。
4. 将配置作为 Nginx 主配置挂载，执行 `nginx -t`，再按部署流程启动。需要 HTTPS 时由部署入口配置 TLS，并同步后端合法 Origin 与可信代理配置。
5. 已知前端页面返回 `index.html`；API、健康检查和静态资源不使用 HTML 回退。SSE 禁用缓冲，WebSocket 保留 Upgrade。

验收：直接请求 `/login`、`/outline` 必须返回应用 HTML；未知 `/assets/missing.js` 必须 404；`/v1/...` 必须得到后端响应而非前端 HTML。新增 Creator 视图时同步配置中的路径白名单。此示例不等于已部署线上。
