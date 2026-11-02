# 参与 InkLore 开发

感谢关注 InkLore。提交前请阅读 [AGENTS.md](AGENTS.md) 与 [仓库治理策略](docs/repository-governance.md)。

## 工作流

1. 从最新 `dev` 建立 `feat/<topic>`、`fix/<topic>`、`docs/<topic>` 或 `chore/<topic>` 分支。
2. 关联现有 Issue / INK 任务，说明范围、风险、验收条件与对运行路径的影响。
3. 遵守七领域边界、敏感数据规范与现有文档/ADR；不要擅自修改冻结资产或历史迁移。
4. 运行相关构建和静态门禁，明确未完成的 API、数据库、浏览器或 Agent 验收。
5. PR 指向 `dev`，Draft 用于进行中工作；需要人工审核的变更加 `manual-review` 标签。
6. 完成检查及对话后合并；合并后源分支会被自动清理。向 `private` 合并需单独发布 PR。

## 提交说明

推荐 Conventional Commits：`feat:`、`fix:`、`refactor:`、`docs:`、`chore:`、`ci:`。

不要提交明文密钥、个人数据、生成缓存或无关构建产物。不要因质量门禁失败而跳过或降级检查。
