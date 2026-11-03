> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 合规域应用与持久化详细设计 R1

状态：INK-141冻结实施目标。入口：[领域总纲](../08-DOMAIN-COMPLIANCE-SPEC.md)、[基线](../00-CONTRACT-BASELINE.md)、[DDL](../../../../../docs/system-design/data/compliance.sql)、[命令](../../../../../docs/system-design/api/compliance-commands.json)、[查询](../../../../../docs/system-design/api/compliance-queries.json)、[模型](../../../../../docs/system-design/api/compliance-schemas.json)。

## 1. 规则版本与自动机

RuleSet包含name、version_no、status、revision、checksum，draft→published→retired。每租户独立序号；创建可指定base_rule_set_id复制旧规则到新UUID集合。发布后规则不可增改删，修改必须复制draft。Rule包含kind=sensitive_literal/ai_phrase/ai_pattern、expression、severity=warning/block、category、advice、weight1..100、enabled。literal/phrase为1..64码点，pattern为1..256的Go RE2。R1不提供任意PCRE、模糊跳字或跨租户全局默认规则写入；初始化给每租户一个空published词库，明确空库不构成无风险证明。

AC构建使用Unicode rune序列，精确匹配，不做简繁转换、大小写折叠或NFKC，以免offset映射失真。ai_pattern使用Go regexp，byte范围经一次byte→rune前缀表换算。全量重叠命中保留，按start_offset、end_offset、rule_id排序，同rule+range去重；嵌套不同规则不合并。AC复杂度O(文本码点+命中数)，RE2总扫描上限100个enabled pattern，每规则表达式编译大小≤64KiB；超过上限422。正文最多200000码点、问题上限10000，超限整报告失败，不截断。

发布先读取draft revision与完整规则计算canonical checksum、编译临时snapshot，编译不持锁；然后事务获取pg_advisory_xact_lock(hashtextextended(tenant_id::text,8))，重验draft revision，旧published→retired、新draft→published，各revision+1，写outbox。任意规则变更同时增加rule_set revision，故发布不会漏掉并发编辑。进程收到通知加载指定UUID/checksum，编译完成才atomic pointer swap；已有请求持有旧不可变snapshot直至结束。每次新扫描都查PG当前published ID并确保本地快照相同；Redis仅加速通知，不能作为权威版本。新版本编译失败返回503 compliance.rule_snapshot_unavailable，不偷偷用旧版给新请求出报告；旧版在途请求可按绑定版本结束。通知丢失后下一次扫描仍加载正确版本。保留最近4版LRU，历史报告查询不要求重编译。

## 2. 报告、问题和审阅状态

Report绑定tenant、owner、novel_id、chapter_id、chapter_revision、content_hash、rule_set_id、mode=rules/deep、source_text及预算。HTTP入站先验证LF文本和hash，再调用platform ScopeAuthorizer.AssertChapterSnapshot验证身份；作品归属不符404，hash不匹配422，版本不是授权投影的快照409 compliance.snapshot_mismatch。输入不能伪造其他revision的证明。报告一经创建绑定不变，pending→running→succeeded/failed/cancelled，终态不重启。deep创建只返回pending，UI另建Agent run；rules走Scan同步200，同一内部执行流程不调用LLM。

Issue保存report_id、rule_id可空、detector=rule/llm、kind、severity、start_offset/end_offset、excerpt≤500、message≤2000、suggestion≤2000、confidence0..1、status=open/ignored、revision。offset是原报告全文码点范围，必须end>start且≤文本长度；长命中excerpt保留前500码点并明确excerpt_truncated=true，范围仍完整。模型证据不精确对应源切片不得入库。忽略命令要求reason1..500，事务锁report再issue，issue revision与report revision一起+1，同时insert IssueDecision(action=ignore/reopen,actor,reason)。ignored不减少原始risk_count；open_count单独计算。历史审阅记录不删除。

AI味分数只基于规则的可解释特征：score=min(100, round(100×Σ每种AI规则 min(命中数,3)×weight / max(100,码点数)))；sensitive_literal不计分，LLM问题不改变确定性分数。报告同时返回formula_version=r1、risk_count、open_count、block_count，score并非AI生成概率。模型补充语义问题需confidence和定位，无法定位的总体建议放summary，不创造伪问题。

Review为不可变回执，绑定report_id、report_revision、chapter_revision/hash、rule_set_id、actor、acknowledged_risk、note。报告有任何风险（含ignored）则acknowledged_risk必须true，无风险可false；报告必须succeeded。CreateReview在report锁下检查expected_revision，插入回执并将report revision+1；回执report_revision记录增加后的revision。后续ignore/reopen会再次增加revision并使旧回执失效。content的ComplianceReviewPort由app适配ValidateReviewReceipts(tenant,actor,exportSnapshots[],review_ids[])；逐项检查scope、报告成功、当前report revision等于回执、章节snapshot完全匹配、规则集仍为当前published，否则409 compliance.review_stale。该Port只读，不把词库风险当事实冲突。无需content引用compliance私表或接受客户端声明的风险布尔值。导出过程固定快照，校验在生成产物前完成，校验后发生正文变化不更改该次导出已冻结的内容。

## 3. 净化建议与Agent执行

Rewrite绑定report_id与report_revision、selected_issue_ids1..100、max_rounds1..3、budget、source snapshot。仅succeeded报告可创建；被选问题必须属于report且open，重复ID422。状态同Report。UI创建pending后POST Agent runs task_kind=compliance_rewrite携rewrite_id，Agent自己的CompliancePort调用ExecuteRewrite；compliance→GenerationPort→llm，绝不调用Agent或直接写content。

每轮在临时内存candidate上生成不重叠补丁，ValidatePatch校验原始range及old_text一致，再重跑固定rule_set规则扫描；目标风险下降即保留较好candidate，零选定风险结束，否则最多3轮。候选恶化则丢弃本轮，保留先前最佳结果；同文本hash重复立即stop_reason=no_improvement。最终以原始全文为基准输出patches及candidate_text/hash、remaining_risk_count、rounds_used、stop_reason=clean/max_rounds/budget/no_improvement。首轮无合法候选则failed，不伪装成功；预算只允许返回已验证的最佳候选，标budget且残余风险明确。patch最多100，range按start排序且无重叠；candidate_text≤200000。UI接受建议前比对chapter revision/hash，变化则409提示重新检测；作者经content正常保存，不存在合规“强制净化写正文”。

报告deep默认预算total_tokens30000/max_output4000/max_calls4，上限120000/8000/6；rewrite默认30000/4000/6。每次上下文含输出预留≤min(model limit,60000)；所有轮次、重新检测LLM与一次结构修复都占总预算；规则扫描零模型调用。预估完整处理不可容纳时422 compliance.analysis_too_large。reserve→LLM execution登记→settle由LLM usage复用；未知结果保留reservation并failed，不自动重发。

Agent ExecuteAudit/ExecuteRewrite入参resource_id、run_id、execution_key、fence、expected_revision，CAS认领并记录execution_fence；相同key重放，相同key更高fence恢复，其他key409 execution_conflict。CancelExecution/FailExecution由Agent调用，以fence和state CAS终态。24h无人认领pending置cancelled；90秒失联由Agent故障补偿；本域不建独立运行队列。完成时报告、issues、usage快照、outbox同事务；LLM调用外置。取消赢得CAS后迟到输出丢弃。

## 4. 应用方法、Port与事务

| 应用服务 | 方法/输入 | 输出/固定行为 |
| --- | --- | --- |
| RuleService | CreateRuleSet(name,base_id)、CreateRule/UpdateRule/DeleteRule(set_id,expected_revision,rule) | draft规则与set revision同事务；删除返回DeletedResult JSON |
| RuleService | PublishRuleSet(id,expected_revision) | 编译后事务版本切换；201创建/200命令 |
| AuditService | Scan(Snapshot)、CreateReport(Snapshot,Budget)、ExecuteAudit(Execution) | Report；deep模型只由Agent触发 |
| AuditService | ListReports/GetReport/ListIssues | scope+cursor；正文不出现在列表 |
| DecisionService | IgnoreIssue/ReopenIssue(report,id,expected_revision,reason) | Issue；report同步增revision，审计只追加 |
| ReviewService | CreateReview(report,expected_revision,ack,note)、ValidateReviewReceipts | Review / ValidatedReviews{chapter_id,revision,hash,review_id} |
| RewriteService | CreateRewrite、ExecuteRewrite、GetRewrite | typed Rewrite；生成结果仅建议 |

消费方定义ReportRepository/RuleRepository/DecisionRepository，基础设施PG实现；ScopeAuthorizer、UnitOfWork、IdempotencyStore、Outbox、Clock、AuditSink均来自platform。GenerationPort适配llm，任务key为compliance_audit/compliance_rewrite。RuleSnapshotCache本域接口复用platform Memory/Redis，但运行时完整自动机留进程内存。HTTP Transport只做绑定/认证/错误映射，领域不接触Gin/GORM。

所有写带Idempotency-Key；PUT/DELETE/状态命令expected_revision必填。锁顺序scope→rule_set或report→issue/rewrite；同报告多问题按UUID排序。幂等记录、资源写、审计、outbox同事务。创建rule使用expected_revision指向父set；更新rule使用expected_revision指向rule，同时锁set保障仍draft；发布针对set。failed Report记录安全failure_code而不暴露模型原文。通知失败可重放不回滚报告。

## 5. 仓储、索引与保留

物理表：compliance_rule_sets、compliance_rules、compliance_reports、compliance_issues、compliance_issue_decisions、compliance_reviews、compliance_rewrites。全部tenant FK、UUIDv4、timestamps；可变记录带revision；跨域chapter/novel/run是UUID弱引用。report源文本为个人敏感数据，日志/审计不得输出。索引：规则集tenant/status/version、规则set/updated_at/id、报告tenant/owner/chapter/updated_at/id、问题report/updated_at/id与report/status、审阅report/created_at/id、rewrite owner/report/updated_at/id。所有列表keyset分页20..100，append-only决策/审阅按created_at DESC,id DESC；其他按updated_at DESC,id DESC。

报告及建议默认30天保存，expire_at到期清除source_text、excerpt、candidate_text、patch old/new文本，记录redacted_at并将report revision+1使review失效；元数据和审计保留90天后去标识化/删除。失去作品scope即时拒绝原文读取。词库已发布历史被报告引用时不物理删，历史rule rows不可变。PG持久化权威，Redis失效不影响规则CRUD及纯规则扫描；AI admission不可用503。源码审计与运维日志只存ID、checksum、耗时、数量，不含正文。

## 6. 失败与验收

错误code固定：compliance.version_conflict409、invalid_state409、rule_duplicate409、invalid_rule422、rule_limit_exceeded422、snapshot_mismatch409、hash_mismatch422、too_many_issues422、invalid_model_output502、budget_exhausted429、review_required422、review_stale409、execution_conflict409、rule_snapshot_unavailable503、upstream_timeout504。通用错误响应使用common，禁止另设数字码。

1. 创建draft添加literal与ai_pattern，RE2不支持表达式422；同revision并发发布只一成功，旧published退役；发布失败仍保留旧版。
2. 两节点一个丢失通知，新扫描仍查PG版本并换snapshot；编译异常返回503而非用旧版出具新报告；在途旧扫描报告准确绑定旧set。
3. 文本“甲𠀀乙”对𠀀命中返回[1,2)，前端UTF16转码后高亮正确；多字节RE2同样定位；10001个问题整报告failed且无截断成功。
4. 伪造chapter_revision/hash失败；同文本不同chapter不能复用review。编辑正文后旧review导出409；新增规则发布也使旧review失效。
5. 有block仍可保存草稿、章节确认；未审阅风险导出422，明确ack后可导出，界面文案无“通过出版审核”。忽略必须理由，reopen仍保留先前决策。
6. 同key重放不重复问题/调用；取消与成功竞争只有一个终态；未知LLM结果不重发。max_rounds=4请求422、预算耗尽不产生第四轮。
7. 净化建议返回diff与残余风险，content正文hash在采纳前不变；作者保存后新revision必须重检。30天擦除后任何接口不返回原文。
## 7. 实施补充约束

规则扫描开始时绑定当时published UUID；deep报告在CreateReport时已绑定规则版本，ExecuteAudit/ExecuteRewrite必须加载该固定UUID和checksum，不能跟随最新head漂移。新版本发布后旧任务可完成，但旧规则回执不满足新的导出校验。用于初始化的空词库由租户引导操作者created_by记录，不设置内置用户。

draft规则DELETE物理移除规则，DeletedResult.revision为删除命令消费的旧revision+1，父rule_set revision同时增加；已发布规则永不物理删除。报告清理必须同步清空问题message/suggestion/excerpt与净化patches/candidate_text，不能仅清理source_text而遗留全文；净化candidate_hash和绑定来源hash可留作审计。报告和净化revision随擦除增加，issue亦增加revision；原Review保留不可变历史但校验必拒绝。
