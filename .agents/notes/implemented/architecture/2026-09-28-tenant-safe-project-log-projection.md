# Agent Note: 租户安全 HTTP 日志以白名单投影入库

Status: implemented

Task: INK-241
Scope: internal/platform/logger/project_projection.go、internal/platform/middleware/middleware.go 与 Admin 项目日志查询

## Problem

现有标准输出日志无法在管理端分页查询，但直接持久化通用 slog 会把任意业务参数或故障文本复制到数据库。审计事件已有独立的只追加事实语义，不适合承载高频项目日志。约束见 [ADR-0033](../../reference/decisions/0033-postgresql-safe-project-logs.md)。改前检索现有活跃 notes 未发现日志入库的相同机制记录。

## Decision

仅将已认证请求完成后的固定 `http_request` 事件作为租户内安全投影写入 `platform_project_logs`，持久化字段严格白名单：经模板校验的操作、公开错误码、状态、时间、耗时及请求关联值。请求结束日志必须使用认证过程中更新的 `c.Request.Context()`；不能复用请求开始前获取的旧 context。日志通过容量固定的队列写 PG，写入与到期清理均复用应用的 GORM 连接，并在专属 GORM Session 使用 `gormlogger.Discard` 禁止 SQL/故障日志递归；不用日志路径阻断业务。操作员/审计员读取时重新验证当前租户与成员资格。审计安全事实与项目日志查询始终分开。

## Alternatives considered

直接将 slog JSON 存成 JSONB 易于检索新字段，但无法保证后续任意参数不包含敏感正文；只读日志文件不需增加表，却不能满足用户要求的数据库持久化与多实例查询；复用审计表可省去表，但把可丢失的运维观测误作审计事实。故选择独立字段投影。

## Consequences

日志高并发或 PG 异常时可能丢失数据库投影，但标准输出/文件仍可用于诊断；系统启动、登录前和没有已验证租户上下文的 Worker 日志不在 Admin 查询范围内，不能声称全系统日志完整存档。既有 000015 迁移已在本地应用，校验和不能为了收紧 schema 而修改；旧 NULL tenant 行不经当前租户 API 暴露。

## Verification

`go build ./...` 与 `npm run build:admin` 退出 0；语言服务对改动文件无错误。补充 `platform-project-logs.json` 源接口后，重建请求映射并重启后端；`/healthz`、`/readyz` 200，匿名日志查询401，管理端已登录浏览器 GET 查询200、显示真实安全日志。DBX只读核对租户内日志由0增至2。跨租户隔离、90天清理和PG断线故障注入尚未在本轮复现，INK-241 保持 review。没有新增测试文件。
