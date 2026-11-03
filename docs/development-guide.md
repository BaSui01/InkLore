# 开发与维护

先读 [AGENTS](../AGENTS.md)，定位真实调用链，再参照 [当前架构](architecture.md)。
任务和验收目标来自 [GitHub Issues](https://github.com/BaSui01/InkLore/issues)。

## 本地开发

Go 版本以 go.mod 为准；前端以 package.json、pnpm-lock.yaml 和 CI 配置为准。
API、Creator、Admin 默认端口为 8080、3000、3001，实际地址以配置为准。

```bash
go build ./...
go vet ./...
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build
make ci NOTES_BASE=<本次修改前的真实提交>
```

配置参考 [本地环境](local-environment.md)、[docker-compose.dev.yml](../docker-compose.dev.yml) 和 .env.example。
仅在已授权的隔离环境操作数据库或模型，不复制真实密钥进仓库。

## 按影响面验证

| 改动 | 检查 |
| --- | --- |
| Go / 装配 | build、vet、gofmt、archguard；按风险增加针对性测试 |
| 前端 | 双端类型检查和构建，受影响页面的请求、状态和错误恢复 |
| API / 数据 | 字段一致性，迁移、权限、事务和冲突的实际结果 |
| Agent / 模型 | 上下文、工具、审批、恢复、产物、费用与失败状态 |
| 文档 / 技能 | 链接、技能元数据、notes 结构与历史封印 |
| 契约资产 | 语义和错误目录同步，失败不得通过改写历史迁移掩盖 |

```bash
node scripts/check-project-skills.mjs
node scripts/agent-notes.mjs check --base <本次修改前的真实提交>
node docs/system-design/tools/check-design.cjs
node scripts/check-platform-contracts.mjs
```

后两项检查暴露契约差距，应记录实际结果。构建、静态检查、隔离迁移和业务验收分别标注；缺环境时写 NOT_VERIFIED。

## 协作资料

按需使用 [开发技能](../.agents/skills/inklore-development/SKILL.md)、[决策技能](../.agents/skills/inklore-decisions/SKILL.md)、[维护技能](../.agents/skills/inklore-maintenance/SKILL.md)。
历史设计、研究和验收在 [工程记忆](../.agents/notes/README.md)，无需每次加载完整历史。
