<div align="center">

# InkLore · 墨境

**让创作拥有结构，让灵感自由生长。**

基于 Go、React、PostgreSQL 的模块化 AI 小说创作平台，面向小说作者、创作智能体与多模型治理。

[架构设计](docs/system-design/00-INDEX.md) · [开发指南](docs/development-guide.md) · [任务看板](docs/backlog.md) · [贡献指南](CONTRIBUTING.md)

</div>

---

## 项目概览

InkLore 采用 **模块化单体架构**，以七个业务领域和统一平台基座组织创作与 AI 能力。设计目标、已实现代码与实际验收进度严格区分；不以编译通过代替真实业务验收。

| 子系统 | 职责 |
| --- | --- |
| Creator Studio | 小说创作、章节编辑、大纲、世界观与智能体交互 |
| Admin Console | 模型、渠道、任务策略、运行审计与平台治理 |
| Go API / Worker | 领域用例、HTTP 接口、持久任务、Agent 调度与流式事件 |
| PostgreSQL / Redis | 业务持久化、事务与运行时基础设施 |

**技术栈**：Go · Gin · GORM · PostgreSQL 17 · Redis 7 · React 19 · TypeScript · Vite · pnpm

## 工程入口

建议先阅读 [AGENTS.md](AGENTS.md) 和 [系统设计索引](docs/system-design/00-INDEX.md)，再按任务进入对应领域。

```bash
# 后端构建与静态检查
go build ./...
go vet ./...

# 前端（Node 22、pnpm 11.25.0）
cd frontend
pnpm install --frozen-lockfile
pnpm run build

# 完整本地门禁（在仓库根目录；需真实 Git 基线）
make ci NOTES_BASE=<base-commit>
```

开发约定端口：**API 8080 · Creator 3000 · Admin 3001**。本地 PostgreSQL / Redis 配置见 [docker-compose.dev.yml](docker-compose.dev.yml)。仅使用示例环境文件，不提交真实密钥。

## 设计与实现事实

| 文档 | 用途 |
| --- | --- |
| [系统设计索引](docs/system-design/00-INDEX.md) | 权威目标设计入口 |
| [接口契约](docs/system-design/api/openapi.json) / [数据设计](docs/system-design/data/README.md) | API 与数据库约束 |
| [Creator UX](docs/system-design/15-CREATOR-FUNCTION-UX-SPEC.md) / [Admin UX](docs/system-design/16-ADMIN-FUNCTION-UX-SPEC.md) | 双端界面与交互 |
| [实施记录](docs/architecture.md) / [任务看板](docs/backlog.md) | 已实现事实、阻塞与验收证据 |
| [ADR](docs/decisions/README.md) / [工程记忆](.agents/notes/README.md) | 重要取舍与演进过程 |

## 协作与分支

- `private` 为稳定主分支，`dev` 为开发集成分支。
- 业务、文档与基础设施变更先从功能分支提交 PR 到 `dev`；向 `private` 发布时单独提出 PR。
- PR 必须满足分支保护与 CI，禁止将 Draft、未确认的架构或数据变更直接自动合并。
- 合并后自动删除短期 PR 分支，保护长期集成分支。

详见 [贡献指南](CONTRIBUTING.md) 与 [仓库治理策略](docs/repository-governance.md)。

## GitHub 协作入口

[Issues](https://github.com/BaSui01/InkLore/issues) · [Pull Requests](https://github.com/BaSui01/InkLore/pulls) · [Projects 看板](https://github.com/users/BaSui01/projects/1) · [Actions](https://github.com/BaSui01/InkLore/actions) · [Insights](https://github.com/BaSui01/InkLore/pulse) · [依赖关系](https://github.com/BaSui01/InkLore/network/dependencies) · [安全](https://github.com/BaSui01/InkLore/security) · [Wiki](https://github.com/BaSui01/InkLore/wiki)

[GitHub 功能与安全治理指南](docs/github-platform.md)；Wiki 发布前的页面草稿见 [docs/wiki](docs/wiki/Home.md)。

## 状态声明

InkLore 持续开发中。文档中的目标能力不代表全部已上线；涉及数据库迁移、跨租户授权、计费、模型调用与 Agent 恢复的功能必须以独立可复现验收为准。
