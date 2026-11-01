# 验证矩阵

下列命令在仓库根执行；运行前读取现有脚本，避免脚本已变更而照抄过期命令。命令退出非零时检查原因，区分本次引入与已有问题，不能隐瞒或把失败标成通过。

| 影响面 | 必需检查 | 通过意味着什么 |
| --- | --- | --- |
| 所有交付 | go build ./...；git diff --check | 当前工作区可构建、已跟踪 diff 无空白错误；不证明行为正确 |
| Go / 分层 / 装配 | go vet ./...；go run ./cmd/archguard -reachability；gofmt -l internal cmd | 静态质量、护栏与可达性；配置仍需核对消费方 |
| 前端 | pnpm --dir frontend run build | 当前脚本分别构建 Creator/Admin；之后必须在真实应用地址核对行为 |
| i18n | go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts | 词条对齐；还需检查页面是否使用词条 |
| 技能与指南 | node scripts/check-project-skills.mjs | 技能元数据、长度、Emoji 与本地文件链接；不验证外站与语义 |
| 工程笔记 | node scripts/agent-notes.mjs check；归档变动另执行 node scripts/agent-notes.mjs check --base HEAD | 格式/任务关联/链接/哈希；基线检查拒绝旧封印变化，不判定理由真假或漏写笔记 |
| API / DB | 在已授权的开发环境发真实请求，查返回与持久化结果 | 正常、非法输入、跨租户、重启留存、并发冲突按任务选择；不操作生产样本 |
| SSE / Agent / LLM | 正常生成、无 Provider、上游中断、用户取消；核对持久化终态 | 不用静态数组/假进度替代真实路径；桩上游不证明真实模型质量 |
| 发布 / 回滚 | 按发布手册核对版本、healthz、readyz 与迁移兼容 | 构建通过与服务就绪分别记证据；实际发布需要任务授权 |

已有综合入口：[check.ps1](../../../../scripts/check.ps1)，可执行 pwsh -File scripts/check.ps1 -Reachability -IncludeFrontend。按任务选择，不为纯文档改动启动数据库和模型请求。不得创建或修改 *_test.go / *.test.ts。

运行路径：业务改动可比较 go list -deps ./cmd/server 前后结果；纯文档与离线检查工具明确“没有修改 Go 包、启动入口、依赖和接线，运行路径不受本任务影响”。不要把其他并行改动算作自己的变化。

完整工作区已有失败时，记录原始命令及归因；任务保持 review 或说明外部阻塞，不能降低 DoD。通过的编译无需无理由重复；文档终态更新后重跑轻量文档校验即可。
