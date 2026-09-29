# ADR-0011：生产配置装载 —— 镜像只带模板，真实配置由部署侧只读挂载

- **状态**：Accepted
- **日期**：2026-09-17
- **决策者**：架构负责人（AI 协作）
- **关联任务**：INK-063
- **替代关系**：无

## 背景（Context）

`platform/config.Bootstrap` 规定：**生产环境必须提供 `configs/config.yaml`**，否则进程拒绝启动
（开发环境才允许「内置默认值 + 环境变量」降级）。这条守卫本身是对的——它拦住的是静默降级。

但发布链路与它不一致，实测（INK-059 的编排演练）暴露两个问题：

1. `Dockerfile` 只 `COPY` 了 `server` 与 `migrations`，镜像里**没有 `configs/`**；
   `docker-compose.prod.yml` 也不挂载配置。于是按发布文档执行
   `docker compose -p inklore-prod -f docker-compose.prod.yml up -d --wait` 必然失败：
   server 容器 `is unhealthy`，日志为「生产环境必须提供配置文件 configs/config.yaml」。
2. `.dockerignore` **没有**排除 `configs/config.yaml`：一旦用最直觉的方式 `COPY configs`，
   本地那份含 `encryption_key` 的配置就会被烤进镜像层，违反 [ADR-0004](./0004-secrets-only-from-env.md)。

## 决策（Decision）

1. **保留**「生产必须提供配置文件」守卫，不放松为「环境变量齐了就行」。
2. **镜像只搭载模板**：`Dockerfile` 复制 `configs/`，其中只允许 `config.example.yaml`；
   同时把 `configs/config.yaml` 写入 `.dockerignore`，**永远不进入构建上下文与镜像**。
3. **真实配置由部署侧只读挂载**：生产编排把宿主机 `./configs` 挂到容器 `/app/configs:ro`；
   部署前执行 `cp configs/config.example.yaml configs/config.yaml`。
4. **密钥仍只走环境变量**（ADR-0004）：`ENCRYPTION_KEY` 在 compose 中以 `${ENCRYPTION_KEY:?}` 必填，
   缺失时由 compose 直接报错，而不是让容器启动后因校验失败而反复重启。
5. **生产安全开关在编排层给出安全默认值**：生产环境校验要求 `auth.cookie_secure` 与
   `callback.enable_signature_verify` 同时为真，而配置模板出于通用性给的是 `false`。
   编排以 `COOKIE_SECURE: ${COOKIE_SECURE:-true}`、`CALLBACK_SIGNATURE_VERIFY: ${CALLBACK_SIGNATURE_VERIFY:-true}`
   提供安全默认值（同名环境变量可覆盖），使「模板 + 一条命令」可用，同时不放松校验。
6. **兑现「JWT_SECRET 未设置时回退 ENCRYPTION_KEY」**：该回退写在 `config.Validate()` 与
   `.env.example`，但启动期走 `Bootstrap→validate()`（运行期校验）不含它，组合根此前直接读
   `cfg.Auth.JWTSecret`，于是「只注入 ENCRYPTION_KEY」的部署被判为未配置令牌密钥而拒绝启动。
   现由组合根显式回退到 `cfg.Security.EncryptionKey`（与文档、与错误文案一致）。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 镜像内置一份可直接运行的 `config.yaml` | 真正开箱即用 | 把默认值（乃至密钥）烤进镜像层，且掩盖「配置未准备」这件事 | 与 ADR-0004 冲突，且让生产跑在模板值上 |
| 入口脚本启动时把 `config.example.yaml` 复制成 `config.yaml` | 一条命令即可起 | 把运维步骤藏进容器；镜像变成有状态初始化器，排障更难 | 隐式行为，破坏「配置即事实」 |
| 生产环境放开「必须提供配置文件」 | 少一份文件 | 削弱防静默降级的守卫；生产可能跑在默认值上 | 守卫是刻意设计的，不为此放松 |
| **镜像带模板 + 部署侧只读挂载（采纳）** | 一条命令可部署；密钥不入镜像；配置缺失在 compose 层即报错 | 首次部署多一步准备文件 | — |

## 后果（Consequences）

**正面**

- 发布文档里的部署与回滚命令**真的可用**（此前必然失败）；
- 密钥永不入镜像，且与「配置缺失必须显式失败」的守卫共存。

**负面 / 代价**

- 首次部署前必须准备 `configs/config.yaml`；若宿主目录不存在，Docker 会以 root 创建一个空目录（发布文档已写明该步骤）。
- 镜像内多出约 4 KB 模板文件，无运行时副作用。

**需要长期承担的义务**

- 新增配置项时，`config.example.yaml`、`.env.example` 与发布文档必须同步；
- 任何「把配置打进镜像」的提案都必须先推翻本 ADR，而不是顺手加一行 `COPY`。

## 验证方式（Verification）

1. `docker build` 后检查镜像内容：`/app/configs` 只含 `config.example.yaml`，**不含** `config.yaml`
   （本地同时存在 `configs/config.yaml` 时验证，用以证明 `.dockerignore` 生效）。
2. 按发布文档执行 `ENCRYPTION_KEY=<32 字节> APP_VERSION=<版本> SERVER_IMAGE=<仓库> docker compose -p inklore-prod -f docker-compose.prod.yml up -d --wait`
   → 退出码 0、server `healthy`、`curl /healthz` 返回 200 且 `data.version` 等于该版本。
3. 同一条命令仅改 `APP_VERSION` 回滚到上一版本 → 仍 `healthy`，`/healthz` 的 `version`/`git_commit` 随之切换。
4. 不提供 `ENCRYPTION_KEY` 时，compose 在插值阶段即报错（容器不会启动）。
5. 仅注入 `ENCRYPTION_KEY`（不设 `JWT_SECRET`）时的端到端实测：`AUTH_MODE=required` 启动日志为
   `认证中间件已装配 mode=required secret_configured=true`；无令牌 `GET /v1/novels` → **401**；
   `cmd/tokengen`（其文档与本回退一致）签发的令牌 → **200**。
