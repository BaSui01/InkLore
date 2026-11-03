> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 合规与行文质检域 R1 冻结设计

状态：INK-141 实施目标，R1 冻结。服从 [契约基线](00-CONTRACT-BASELINE.md)。实现契约见 [详细设计](domains/08-DOMAIN-COMPLIANCE-DETAIL-SPEC.md)、[SQL](../../../../docs/system-design/data/compliance.sql)、[查询API](../../../../docs/system-design/api/compliance-queries.json)、[命令API](../../../../docs/system-design/api/compliance-commands.json)、[模型](../../../../docs/system-design/api/compliance-schemas.json)。

## 1. 权威与产品语义

compliance拥有租户版本词库、检测报告、问题、审阅记录、净化建议请求。它是作者自查工具，不是出版审核机构，不承诺外部平台过审。severity只有warning/block：warning提示作者关注，block表示高风险、在导出审阅页优先展示；两者均不拦截存草稿。章节确认只受worldview事实冲突门约束，不调用合规门。导出要求作者明确完成风险审阅确认，记录报告ID、绑定章节revision/hash、规则版本及是否仍有未解决风险；不是要求风险为零。

正文与建议严格分离：本域无任何正文写Port。UI显示替换diff，作者接受后调用content保存并创建新正文版本。旧报告不可挪用为新版本证明。AI味指标是解释性文体特征分数，不是AI来源鉴定或概率。

## 2. 核心用例

| 用例 | 契约 | R1行为 |
| --- | --- | --- |
| COMP-01 维护词库 | operator创建rule-set草稿，增改删规则，publish | 每次发布形成不可变规则版本；异步通知各进程原子换AC快照 |
| COMP-02 快速扫描 | creator POST scans携chapter绑定与文本 | 同步200完整报告；只执行词面/句法规则，不调用LLM |
| COMP-03 深度检测 | POST reports创建pending，UI启动Agent task_kind=compliance_audit携report_id | Agent调用ExecuteAudit，报告终态后可分页读issues |
| COMP-04 忽略误报 | POST reports/{id}/issues/{id}/ignore携理由与issue revision | 记录不可变决策审计，标记ignored；reopen撤销忽略但保留理由历史 |
| COMP-05 风险审阅 | POST reports/{id}/reviews，携report revision与acknowledged_risk | 报告有block/warning时必须true；生成绑定快照的review回执 |
| COMP-06 净化建议 | POST rewrites携report、问题ID、预算、max_rounds | 创建pending；UI启动compliance_rewrite；最多3轮，仅输出建议及残余问题 |
| COMP-07 终止任务 | 在Agent界面取消run | Agent调用本域CancelExecution，终态只落一次；不在本域启动第二个作业 |

## 3. 固定约束

所有报告绑定novel_id、chapter_id、chapter_revision、content_hash及规范化文本。UI必须先保存当前草稿再创建正式报告；服务端重算SHA256，不匹配422。快照真实性通过platform授权投影中的章节revision/hash校验，服务端不能只相信客户端标签。投影必须与content正文保存同事务维护，脱敏保留hash/revision及scope，禁止包含正文。历史已授权的revision可用于历史报告，但审阅导出必须精确对应本次导出快照。

文本≤200000 Unicode码点、JSON≤2MiB。所有位置是UTF码点start_offset/end_offset左闭右开；不以byte或UTF16定位。编辑后旧报告显示过期，不自动重定位。规则类型sensitive_literal/ai_phrase/ai_pattern；severity=warning/block，关键词1..64码点，模式仅Go RE2语法1..256，拒绝反向引用与lookaround。规则集最多20000条；单报告最多10000条问题，超出返回422 compliance.too_many_issues且报告failed，禁止静默截断。

词库draft→published→retired；同租户最多一个published。发布时校验并编译，事务锁租户配置行切换版本，失败原版本不动。审计报告pending→running→succeeded/failed/cancelled；scans在请求内完成同样报告对象。问题open <-> ignored，采纳改写产生新报告，不把原问题伪装resolved。净化请求同一终态状态机，最多3轮、1..6调用、total_tokens1000..120000、每次输出256..8000、总上下文上限60000。

## 4. 复用与隔离

Agent自己的CompliancePort通过app/adapters调用本域ExecuteAudit/ExecuteRewrite；compliance的GenerationPort调用llm；没有compliance→agent或compliance→content。RuleSnapshotCache使用platform缓存通知与PG版本复核；通知丢失也不允许永久旧规则。content导出应用通过消费方声明的ComplianceReviewPort只读校验审阅回执，app/adapters适配本域ValidateReviewReceipts；不得信任客户端自由DTO。此只读能力方向由跨域基线统一登记。platform ScopeAuthorizer同时验证chapter scope和快照身份。SSE归Agent，WS仅报告完成/词库失效。

## 5. 验收要求

覆盖词库发布竞争与热更新失败回退、非BMP offset、同报告绑定不能跨章节使用、忽略理由必填且可追溯、AI味不作为作者来源判定、风险存在仍可存草稿/确认、导出旧review拒绝、重写预算与轮数硬上限、取消/迟到完成竞争、跨租户404。详细设计给出逐项输入与可验证结果。
