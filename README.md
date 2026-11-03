<div align="center">

# InkLore · 墨境

**让创作拥有结构，让灵感自由生长。**

基于 Go、React、PostgreSQL 的模块化 AI 小说创作平台，面向小说作者和小型创作团队。

[当前架构](docs/architecture.md) · [开发指南](docs/development-guide.md) · [Issues](https://github.com/BaSui01/InkLore/issues) · [贡献指南](CONTRIBUTING.md)

</div>

## 项目概览

InkLore 以作品、大纲、人物与世界观、章节编辑、AI 建议审阅和导出为产品目标。
当前代码与实际验收分别记录，尚未完成的链路见 [源码能力基线](.agents/notes/reference/audits/2026-11-03-source-baseline.md)。

| 子系统 | 职责 |
| --- | --- |
| Creator Studio | 作者工作台、章节编辑、大纲、世界观和 Agent 交互 |
| Admin Console | 模型、渠道、策略、运行审计和平台管理 |
| Go API / Worker | 领域用例、HTTP、持久任务、调度和事件 |
| PostgreSQL / Redis | 业务持久化、事务及运行时基础设施 |

技术栈和版本以 go.mod、frontend/package.json、锁文件和部署配置为准。

## 开发入口

先读 [AGENTS](AGENTS.md)，按任务定位源码；无需先完成历史阶段或按旧编号顺序施工。

```bash
go build ./...
go vet ./...
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build
make ci NOTES_BASE=<本次修改前的真实提交>
```

默认开发端口：API 8080、Creator 3000、Admin 3001。实际地址以配置为准。
本地依赖见 [docker-compose.dev.yml](docker-compose.dev.yml)，使用示例配置，不提交真实密钥。

## 文档与协作

- [文档导航](docs/README.md)：开发、当前架构、接口、数据库和运维说明。
- [接口与数据资产](docs/system-design/00-INDEX.md)：工具消费的机器契约与语义检查。
- [工程记忆](.agents/notes/README.md)：调查、实现取舍与历史设计。
- [Issues](https://github.com/BaSui01/InkLore/issues) / [Milestones](https://github.com/BaSui01/InkLore/milestones)：任务、依赖和验收状态。

功能分支向 dev 提交 PR；private 使用独立发布 PR。遵守分支保护、人工审查和用户授权。
设计可以持续演进，已应用迁移与有效安全机制保持兼容。构建成功不等于产品链路通过验收。
