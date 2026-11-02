# 参与 InkLore 开发

感谢关注 InkLore。提交前请阅读 [AGENTS.md](AGENTS.md) 与 [仓库治理策略](docs/repository-governance.md)。

## 工作流

1. 从最新 `dev` 建立 `feat/<topic>`、`fix/<topic>`、`docs/<topic>` 或 `chore/<topic>` 分支。
2. 关联现有 Issue / INK 任务，说明范围、风险、验收条件与对运行路径的影响。
3. 遵守七领域边界、敏感数据规范与现有文档/ADR；不要擅自修改冻结资产或历史迁移。
4. 运行相关构建和静态门禁，明确未完成的 API、数据库、浏览器或 Agent 验收。
5. PR 指向 `dev`，Draft 用于进行中工作；需要人工审核的变更加 `manual-review` 标签。
6. 普通低风险文档 PR 可由 GitHub 在 CI 全绿、Review 对话解决后自动合并；其他非敏感代码必须经维护者添加 `auto-merge-approved` 标签，才能启用自动合并。
7. 安全、身份、密钥、Agent/LLM 底层、数据迁移、工作流等变更必须人工合并；可使用 `manual-review` 或 `do-not-merge` 标签阻止自动合并。
8. 合并后 GitHub 自动清理短期来源分支；向 `private` 合并仍需单独提出并人工审核 PR。

## 提交说明

推荐 Conventional Commits：`feat:`、`fix:`、`refactor:`、`docs:`、`chore:`、`ci:`。

不要提交明文密钥、个人数据、生成缓存或无关构建产物。不要因质量门禁失败而跳过或降级检查。
