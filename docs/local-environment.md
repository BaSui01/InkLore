# 本地开发环境与数据库配置

本文是可公开的本地开发配置指南，不记录任何机器的真实数据库口令、API Key、Token、私有连接字符串或 DBX 凭据。

实际凭据只保存在开发者自己的环境变量、未纳入 Git 的 `.env` 或密钥管理器中。不要将已填写的配置复制到 Issue、PR、日志或仓库文档。

## 1. 本地依赖

项目开发环境使用 PostgreSQL 17（pgvector / pg_trgm）与 Redis 7。可在本机安装，也可使用仓库自带的 Docker Compose：

```bash
docker compose -f docker-compose.dev.yml up -d --wait
```

`docker-compose.dev.yml` 中的开发凭据仅用于隔离的本地演示，不得用于公网部署或任何正式环境。若复用已有数据卷，变更 PostgreSQL 主版本前必须完成备份与迁移。

## 2. 私密配置原则

- PostgreSQL 和 Redis 应采用不同的、随机生成的强密码，并限制可访问的来源地址。
- 不要在 Markdown、YAML、命令参数或 Git 历史中提交真实密码。
- 使用本机环境变量或 Secret Store 注入应用配置；`.gitignore` 已忽略项目根目录的 `.env`。
- 密码一旦误入 Git 历史，即使最新提交已删除，也需要先轮换实际凭据，再评估历史清理及其对分支和 PR 的影响。

## 3. 本地环境变量示例

复制根目录 `.env.example` 为本地 `.env`，并替换以下占位值。这里的字符串仅是示例，不能直接用于连接服务。

```dotenv
DATABASE_DRIVER=postgres
DATABASE_DSN=postgres://ai_novel:YOUR_LOCAL_PG_PASSWORD@127.0.0.1:5432/ai_novel?sslmode=disable
REDIS_ENABLED=true
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=YOUR_LOCAL_REDIS_PASSWORD
REDIS_DB=0
ALLOW_MEMORY_STORE=false
```

若密码包含特殊字符，需要在 PostgreSQL URI 中正确进行 URL 编码。生产环境必须配置独立密钥及安全连接参数；以上 `sslmode=disable` 仅供隔离的本地调试。

## 4. DBX 等数据库客户端

需要使用 DBX、psql 或 Redis CLI 时，先在本机客户端或密钥管理器中配置连接。客户端私有数据库、连接 ID、加密凭据文件与真实主机映射不得进入 Git。

## 5. 常用运维与验证命令

```bash
# 查看迁移状态（使用本地配置）
go run ./cmd/migrate -cmd status

# 检查 Docker 开发服务
docker compose -f docker-compose.dev.yml ps

# 停止开发服务（默认保留数据卷）
docker compose -f docker-compose.dev.yml down
```

执行真实数据库或缓存连接检查前，先在自己的终端安全注入认证变量。避免通过明文命令行参数传递密钥，以免泄露到 Shell 历史或进程列表。

## 6. 仓库公开前检查

清理当前文件不等于清理 Git 历史。任何包含真实凭据的旧提交、PR diff、缓存或 Fork 都需要单独评估。只有完成凭据轮换、可达历史审计和权限门禁审查后，才能将仓库设为公开。
