# Agent Note: Go 1.27.1 与 Gin 1.12 维护升级

Status: implemented

Task: INK-188
Scope: Go 工具链、Gin HTTP Transport、Docker 构建基线

## Problem

项目原先声明 Go 1.25.0，Gin 已锁定 v1.12.0。Gin 版本没有可用升级，但 Go 1.25 已不在 Go 官方最近两个大版本的支持窗口内。工作区同时存在大量其他未提交业务和前端改动，不能借依赖升级之名做全量依赖刷新或整理无关文件。

## Decision

将模块语言基线提升为 `go 1.27.0`，通过 `toolchain go1.27.1` 固定开发与构建工具链，并将 Docker 构建阶段同步到 `golang:1.27.1-alpine`。Gin 保持 `github.com/gin-gonic/gin v1.12.0`，因为当前模块已是目标版本且 `go list -m -u` 没有显示更新。

保留当前 Gin 工程化运行路径：组合根使用 `gin.New`，Platform middleware 负责请求/Trace、CORS、JSON大小和统一错误；领域 Transport 通过 `gin.IRoutes` 注册；正式命令走严格 JSON 解码和统一 response；请求级生成使用带心跳和 `Last-Event-ID` 的 SSE；低频通知使用独立 WebSocket；标准库 `http.Server` 负责超时和优雅停机。没有为了追求表面上的“最新用法”改动冻结的路由、响应、SSE/WS职责或领域边界。

`go mod tidy` 仅将源码直接使用的 `pgx/v5` 与 `x/text` 从 indirect 调整为 direct，并补齐两条校验和；没有执行 `go get -u ./...`。

## Alternatives considered

- 只升级 Gin：未采用。当前已是 v1.12.0，没有可用更新，且不能解决 Go 1.25 的工具链支持问题。
- 直接执行 `go get -u ./...`：未采用。会引入 GORM、Redis、x/* 等无关升级，扩大当前工作区的回归面。
- 把 Gin 改为 `gin.Default()`：未采用。项目已有自定义请求契约、日志、恢复、CORS 和 SSE 约束，默认中间件会改变控制面。
- 将 SSE 改为 Gin 便捷渲染：未采用。当前事件序号、事件名、心跳和断线恢复需要显式控制。

## Consequences

本地与 Docker 构建使用 Go 1.27.1，开发机使用低版本 Go 时会按 `toolchain` 指令请求兼容工具链。Gin HTTP 运行路径不变，依赖升级的运行路径影响仅限 Go 编译器和模块图整理。未执行生产镜像发布、PG/Redis 联调或真实模型流；这些边界不由本维护任务覆盖。

## Verification

- `GOTOOLCHAIN=go1.27.1 go version`：`go1.27.1 linux/amd64`。
- `GOTOOLCHAIN=go1.27.1 go mod tidy`：退出码 0，依赖变化限于本任务说明范围。
- `GOTOOLCHAIN=go1.27.1 go build ./...`：退出码 0。
- `GOTOOLCHAIN=go1.27.1 go vet ./...`：退出码 0。
- `GOTOOLCHAIN=go1.27.1 go run ./cmd/archguard -reachability`：R1-R5 全部通过，可达性列表仅为信息项。
- `node scripts/check-project-skills.mjs`：退出码 0。
- `node scripts/agent-notes.mjs check`：退出码 0。
- `git diff --check`：发现其他并行工作区在前端文件末尾新增空行；未将其混入本任务。
- Docker 构建、前端构建、PG/Redis/模型运行路径：待本任务后续验收或发布环境验证。
