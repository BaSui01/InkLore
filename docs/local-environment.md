# 本地开发环境与数据库配置

本文档记录当前开发宿主机的本地基础设施运行信息、统一数据库密码凭据、环境变量配置与 DBX 客户端连接配置。

## 1. 本地基础设施概况

当前开发机直接使用宿主机原生系统服务提供数据库与缓存能力，未运行 Docker 容器（端口 5432 与 6379 均由宿主机服务占用）。

| 服务 | 版本 | 监听地址 | 默认账号 | 认证密码 | 业务数据库 / 库号 |
| --- | --- | --- | --- | --- | --- |
| PostgreSQL | 17.11 (Debian) | 127.0.0.1:5432 | ai_novel | `2QzwHFZbRxFyipIDul43A9DN4kaEepGZ` | `inklore_r1`（主库）、`ai_novel` |
| Redis | 7.0 | 127.0.0.1:6379 | （默认） | `2QzwHFZbRxFyipIDul43A9DN4kaEepGZ` | DB 0 |

- **PostgreSQL 扩展**：`vector` (0.8.0)、`pg_trgm` (1.6)、`pgcrypto` (1.3)。
- **数据迁移状态**：主库 `inklore_r1` 已应用 `000001_platform` 至 `000005_llm` 全部 5 组基线迁移，共 46 张业务表。

## 2. 统一随机密码

为避免特殊字符在各类 DSN URL、连接串及驱动解析中的转义异常，PostgreSQL 与 Redis 统一采用以下 32 位高强度字母数字随机密钥：

```text
2QzwHFZbRxFyipIDul43A9DN4kaEepGZ
```

## 3. 本地环境变量（.env）

项目根目录的 `.env`（已被 `.gitignore` 忽略）中对应的数据库与缓存配置如下：

```env
# PostgreSQL 权威数据源
DATABASE_DRIVER=postgres
DATABASE_DSN=postgres://ai_novel:2QzwHFZbRxFyipIDul43A9DN4kaEepGZ@127.0.0.1:5432/inklore_r1?sslmode=disable
DB_MAX_OPEN_CONNS=40
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=1800
DB_CONN_MAX_IDLE_TIME=600
DB_SLOW_THRESHOLD_MS=200

# 存储档位（数据库为唯一权威存储）
ALLOW_MEMORY_STORE=false

# Redis 缓存配置
REDIS_ENABLED=true
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=2QzwHFZbRxFyipIDul43A9DN4kaEepGZ
REDIS_DB=0
```

## 4. DBX 客户端配置

DBX 客户端（`/usr/bin/dbx`）与 DBX MCP Server（`@dbx-app/mcp-server`）的数据文件存储于 `~/.local/share/com.dbx.app/dbx.db`。已配置以下 3 个持久化连接，并已写入 MCP 全局策略白名单：

| 连接 ID | 连接名称 | 类型 | 地址 | 端口 | 数据库 | 用户名 |
| --- | --- | --- | --- | --- | --- | --- |
| `inklore-pg-r1` | InkLore PostgreSQL (inklore_r1) | postgres | 127.0.0.1 | 5432 | inklore_r1 | ai_novel |
| `inklore-pg-novel` | InkLore PostgreSQL (ai_novel) | postgres | 127.0.0.1 | 5432 | ai_novel | ai_novel |
| `inklore-redis` | InkLore Redis | redis | 127.0.0.1 | 6379 | 0 | - |

- **凭据存储**：密码已加密保存于 DBX `connection_secrets` 表中，打开 DBX 客户端即可直接连接。
- **MCP 白名单**：`allowedConnectionIds` 已包含上述 3 个 ID，支持 AI 工具与外部客户端调用。

## 5. 常用运维与验证命令

- **检查数据迁移状态**：
  ```bash
  go run ./cmd/migrate -cmd status
  ```
- **PostgreSQL 命令行连接**：
  ```bash
  PGPASSWORD=2QzwHFZbRxFyipIDul43A9DN4kaEepGZ psql -U ai_novel -h 127.0.0.1 -d inklore_r1
  ```
- **Redis 命令行测试**：
  ```bash
  redis-cli -a 2QzwHFZbRxFyipIDul43A9DN4kaEepGZ ping
  ```
