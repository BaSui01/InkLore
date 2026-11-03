# 19 · 旧文档清理与冻结交付

任务INK-141。用户明确要求删除旧文档，按system-design详细设计后整体重构；本轮不保留另一套旧目标设计供开发者二选一。

## 1. 实际删除范围

机器清单见[cleanup-manifest.json](cleanup-manifest.json)，共30个旧文件：archive中的旧产品分析与蓝图、旧docs/api、重复的Markdown DDL/CONTRACT分册、旧roadmap和release-and-rollback。删除发生在新领域规格、api与data替代落盘之后，不再次搬到新archive。

| 旧内容 | 唯一替代位置 |
| --- | --- |
| 历史产品分析、架构/前端蓝图 | 00/01/02/15/16/17/20 |
| 数字错误码、旧GET/POST流混合契约 | api/openapi.json、14、21 |
| Markdown中的独立DDL副本 | data/*.sql与data/README |
| 领域CONTRACT副本 | api源分片及生成目录 |
| 旧路线图 | 18冻结实施顺序 |
| 旧发布回滚 | 13部署/恢复 |

核心功能规格03..09和详细分册原文件已重写为R1，不叠加“旧版/新版”两套内容。14..18也重写，旧源码审计/待定提案不再指导R1实现。

## 2. 保留的非设计记录

依ADR-0017，.agents/notes/reference/decisions保留不可变选择依据，不新建history决策库；docs/backlog只保留活动任务与退役编号登记；docs/architecture只记录R1尚未实现及运行证据；project-management只是12治理导航。它们不拥有另一套API、DDL或UI目标。docs/semantic-recall-evaluation.md仅有退役说明，用于保持不可改写ADR-0010的历史引用可达，不保留旧评估正文。Skills与notes是工作流/因果记录，不覆盖R1契约，保持正在进行的独立任务记录不丢失。

## 3. 冻结产物

冻结内容包括需求、架构、七域用例/Port/仓储/状态、SQL、OpenAPI、错误目录、SSE/WS、双端UI与共享组件、运维、实施验收。源API→generated→入口与目录由工具编译；同名领域Schema保持外部ref，不被错误flatten。

验证命令：

~~~powershell
node docs/system-design/tools/build-contracts.cjs
node docs/system-design/tools/check-design.cjs --freeze
go build ./...
node scripts/check-project-skills.mjs
~~~

设计报告见implementation/design-validation.json。freeze-manifest.json存本轮SHA256，只用于检测漂移，不宣称历史防篡改或第三方认证。代码实现/真实PG约束/浏览器交互和性能验收另按18执行。

## 4. 后续维护

实施不得在组件里造路由、schema、状态或默认业务数据。只允许记录任务进度，不随意更改冻结契约；真实缺陷或新增需求通过R1.x/R2变更记录处理，同时修改受影响引用。旧编号001..140不重新利用，不把旧版本done当R1完成证据。
