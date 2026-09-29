# Agent Note: Vite 过期依赖预构建缓存的恢复方式

Status: implemented

Task: INK-187
Scope: frontend/.env、frontend/vite.config.ts 与本地 Vite 开发进程

## Problem

本地 Creator 开发页从 `:29000` 请求 Cytoscape 与 Tiptap 的 Vite 依赖预构建文件时，连续收到 `504 Outdated Optimize Dep`。运行中的 Creator Vite 进程已存在约 1.5 小时，缓存元数据仍只包含旧的公共依赖，而当前 Creator 模块图已经引用这些新增依赖。根 `.env` 为本地开发覆盖了端口 `29000/29001`，但这不代表架构契约的默认端口发生变化。

## Decision

停止旧的 Creator/Admin Vite 子进程，分别以 `npm run dev:creator -- --force` 和 `npm run dev:admin -- --force` 重启。让 Vite 自己强制重新生成依赖预构建，避免删除 `node_modules` 或修改业务源码、锁文件和构建配置。浏览器需要重新加载入口，以便取得新的依赖 hash；继续请求旧 hash 时返回 504 是 Vite 的过期请求语义，不是依赖包缺失。

## Alternatives considered

- 仅刷新浏览器：不能修复仍持有旧预构建缓存的长期运行进程。
- 永久设置 `optimizeDeps.force` 或把本次依赖全部硬编码进配置：会增加每次开发启动的预构建成本，且不能替代依赖变更后的进程重启。
- 删除整个 `node_modules` 并重装：影响范围大，无法解决单纯的 Vite 运行时缓存过期问题。

## Consequences

依赖或入口模块发生变化后，已有 Vite 进程可能短暂服务旧 hash；开发者应重启对应 Vite 实例并在浏览器刷新入口。双端共享 `frontend/node_modules/.vite` 时，Creator/Admin 都应一起重启，避免一端继续使用旧优化结果。R1 默认开发端口 `3000/3001` 不变；本地 `.env` 可按环境覆盖为 `29000/29001`。

## Verification

- `lsof -nP -iTCP:29000 -sTCP:LISTEN` 与 `lsof -nP -iTCP:29001 -sTCP:LISTEN`：两个新 Vite 进程正常监听。
- 当前 hash 的 `cytoscape.js`、`@tiptap_react.js`、`@tiptap_starter-kit.js`、`@tiptap_extension-placeholder.js`、`@tiptap_extension-character-count.js` 请求：全部 HTTP 200，响应体非空。
- `curl http://[::1]:29000/` 与 `curl http://[::1]:29001/`：均 HTTP 200。
- `npm run build:creator`：退出码 0；`npm run build:admin`：退出码 0。
- 本次涉及文档的定向 `git diff --check`：通过。
- 旧 hash 请求仍返回 `504 Outdated Optimize Dep`，符合 Vite 对过期预构建 URL 的预期行为；刷新入口后使用新 hash。

全仓 `git diff --check` 另外报告了用户工作区既有的 `frontend/apps/admin/src/routes/AuditTrailRoute.tsx`、`frontend/apps/admin/src/routes/LlmDashboardRoute.tsx`、`frontend/packages/i18n/src/locales/auth.ts` 和 `frontend/packages/ui/src/components/Input.tsx` 文件末尾空行；本任务未修改这些文件。
