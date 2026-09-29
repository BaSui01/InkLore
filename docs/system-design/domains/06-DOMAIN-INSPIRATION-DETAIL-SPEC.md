# 灵感域应用与持久化详细设计 R1

状态：INK-141冻结实施目标。入口：[领域总纲](../06-DOMAIN-INSPIRATION-SPEC.md)、[统一契约](../00-CONTRACT-BASELINE.md)、[DDL](../data/inspiration.sql)、[API命令](../api/inspiration-commands.json)、[API查询](../api/inspiration-queries.json)、[文风与卡片命令](../api/inspiration-library.json)、[Schema](../api/inspiration-schemas.json)。

## 1. 聚合、不变量与状态机

Asset拥有规范文本、版权来源与Segment。asset状态ready/deleted，revision从1开始；正文导入后不可改写，纠正内容必须建新asset；只允许PUT元数据。删除保留deleted_at，恢复清空deleted_at。Segment是不可变、seq从1连续，全文码点区间互不重叠且覆盖规范文本；content_hash绑定整个asset。分片正文通过源文本切片返回，不重复存原文。Asset指向platform_files仅技术外键，应用先验证同tenant/owner；粘贴也由FilePort.StoreText存platform_files，不能另建对象存储。

Analysis拥有预算与报告发布状态。pending→running仅ExecuteAnalysis认领；pending/running→cancelled由Agent取消适配；running→succeeded/failed；终态不可重开。创建后24h未绑定run由清理器CAS置cancelled，failure_code=inspiration.unclaimed_expired。run_id是跨域UUID弱引用，execution_key是Agent提供的UUID。相同execution_key重复执行直接返回已完成结果或已有租约，异key抢占409 inspiration.execution_conflict。Agent lease失效只允许同key携更高fence恢复，数据库记录execution_fence；所有完成写带fence防止旧worker覆盖。running缺心跳90秒由Agent故障补偿调用FailExecution，不自建第二个调度器。

Report与analysis一对一、不可变，含summary、findings、patterns、style_metrics、ideas；不适用的数组为空，style_metrics为null。Finding维度opening_hook/conflict/pacing/character/cliffhanger，含observation、confidence0..1、evidence数组；Pattern含name、steps1..8、applicability、counterexample、evidence。StyleMetrics含mean_sentence_length、dialogue_ratio、sensory_bias及instructions。Ideas含title、category、body、rationale、evidence。最多100 findings、20 patterns、10 ideas；每条证据1..500码点，定位与原文比对通过才允许发布。模型生成不合Schema、证据超界或引文不符报inspiration.invalid_model_output，最多一次修复且计入预算。

StyleProfile聚合保存title、current_version_id、revision和删除状态。StyleVersion保存不可变version_no、instructions、metrics、source_report_id；PUT锁profile创建新版本+更新head同事务。调用方始终固定style_version_id，不使用动态“最新”隐式替换运行上下文。StyleVersion非任意prompt注入，instructions≤4000码点，装配时作为不可信风格数据而非系统指令。

Card保存category=opening/conflict/twist/cliffhanger/setting、body1..4000、rationale≤2000、可选source_report_id、evidence和source_kind=original/reference。状态suggested <-> archived，二者可deleted；恢复回suggested。去重fingerprint=SHA256(category+LF+NFKC(body).trim后连续空白折叠)，仅去重计算规范化，不改展示正文。tenant+owner+novel scope内活跃卡唯一，近似相似仅UI提示、不自动合并。编辑body后重算唯一约束。

Adoption保存card_id、target_domain=content/outline、target_id、target_revision、target_hash、state=active/revoked。它是作者复用建议的引用证明，不代表目标仍含该文本；目标随后改动不自动改写历史。记录时调用platform ScopeAuthorizer.AssertResourceRevision(tenant,actor,target_kind,target_id,revision,hash)核验tenant/actor/scope与目标版本投影，不能接受自报“已采纳”。撤销理由1..500，state仅active→revoked，不删除历史；再次采纳新目标版本新增引用。删除卡片不级联撤销采纳。

## 2. 应用服务及Port

所有方法参数含Context{tenant_id,actor_id,request_id,scope}；写方法由平台CommandExecutor包裹幂等、UnitOfWork、审计与outbox。下表方法名是应用契约，不暴露ORM。

| 方法 | 输入 | 输出与调用关系 |
| --- | --- | --- |
| ImportAsset | AssetCreate | FilePort.ReadAuthorized/StoreText→规范化→Chunker→AssetRepository.InsertWithSegments，返回Asset |
| UpdateAsset/DeleteAsset/RestoreAsset | id,expected_revision及元数据 | ScopeAuthorizer→CAS；删除先取消未终态分析的发布权；FilePort释放由outbox处理 |
| ListAssets/GetAsset/ListSegments | owner scope,cursor,limit | 仓储keyset查询；原文只授权creator可见 |
| CreateAnalysis | AnalysisCreate | 校验asset revision、可用性、kind约束、预算→pending Analysis |
| ExecuteAnalysis | analysis_id,run_id,execution_key,fence,expected_revision | Agent调用→认领→GenerationPort.Generate→ValidateEvidence→CompleteAnalysis；返回typed Report |
| CancelExecution/FailExecution | analysis_id,key,fence,reason | Agent调用CAS终态；reason只错误code，不存上游原文 |
| CreateStyle/ReviseStyle | StyleWrite,expected_revision | 校验报告归属；新版本与head同事务 |
| GetStyleVersion | style_id,version_id | Agent StyleReadPort调用→固定版本DTO，删除profile则新使用404 |
| CreateCard/UpdateCard/ArchiveCard | CardWrite,expected_revision | 校验source报告及evidence、去重→Card |
| RecordAdoption/RevokeAdoption | card_id,expected_revision,TargetRevision / adoption_id,expected_revision,reason | ScopeAuthorizer.AssertResourceRevision后写Adoption与卡revision，撤销只更新引用 |

消费方定义端口：inspiration的GenerationPort由app适配llm.Generate，入参task_kind、结构化schema、上下文片段、输出上限、execution_key、剩余token预算，返回typed模型结果+Usage；FilePort适配platform_files；ScopeAuthorizer适配platform；Clock、UnitOfWork、IdempotencyStore、Outbox复用platform。Agent自己的InspirationPort适配ExecuteAnalysis/GetReport/GetStyleVersion，不反向调用Agent。没有DraftProposalPort、ModerationPort或跨业务私表join。

## 3. 预算、执行与失败补偿

默认total_tokens=30000、max_output_tokens=4000、max_calls=8，可降低不可超过120000/8000/20；所有模型调用的input+output累计扣减，修复也计费。每次先持久化reservation和call序号，再经LLM幂等执行边界发出；reservation从remaining预扣，完成按实际结算；未知上游结果保留reservation且analysis失败upstream_outcome_unknown，禁止自动再次外呼。具体用量/调用登记复用llm的execution usage，不在本域复制provider日志。Analysis记录reserved_tokens、used_tokens、calls_used快照。无法容纳完整分析时在首次外呼前422 inspiration.analysis_too_large，不截掉尾部分片。

拆书采用逐片结构抽取后合并；计划调用数=分组数+1汇总+1修复预留，group按token估算装配，组上下文≤60000减输出预留。只有全分片成功且证据验证完成才发布Report；无半成品报告。ideas无asset允许原创；deconstruction/style必须asset，style样本≥1000码点。source已删除或revision变化导致提交阶段409 inspiration.source_changed并丢弃未发布结果；调用已发生的用量照实保留。

长外呼不持PG事务。claim事务只锁analysis，commit后生成；完成事务按asset→analysis顺序锁并重验授权/源revision/fence，insert report、CAS succeeded、幂等执行结果、outbox一起提交。取消赢CAS时迟到模型输出不得变成报告。网络响应丢失重放原HTTP结果；outbox通知失败重投，UI可轮询GET修复状态。FilePort先创建的暂存文件未被asset引用时24h清理；导入提交前失败不删除用户原上传文件。

## 4. SQL、查询与保留

DDL包含inspiration_assets、inspiration_segments、inspiration_analyses、inspiration_reports、inspiration_styles、inspiration_style_versions、inspiration_cards、inspiration_adoptions。全部UUIDv4+tenant FK+timestamps；可变表revision。域内关联tenant/id复合FK，平台文件/用户为技术FK，跨域作品/章节/run/目标为UUID不join。

列表资产、卡、档案、分析按tenant+owner(+novel)+updated_at DESC,id DESC，索引见DDL；segment按seq ASC,id ASC，version与adoption按created_at DESC,id DESC，游标签名绑定scope/filter/sort。报告按analysis唯一索引点查。GET列表默认limit20最大100，所有collection提供分页、不返回隐含全量。SQL所有SELECT/UPDATE含tenant与owner谓词，应用不靠RLS替代授权。数据约束由DB保障ID、枚举、范围、唯一性；JSONB结构由冻结Schema校验器在持久化前验证。

删除30天内可恢复；被删除源的证据显示source_unavailable，不将残存片段泄露给失权用户。30天后删除原文文件引用并擦除report中excerpt、card evidence excerpt及原文派生长文本，引用证明保留IDs/hash且不得含正文；90天审计保留actor去标识化摘要。恢复只能在未擦除且唯一指纹未被占用时成功，否则409 inspiration.restore_conflict。清理器每小时批量100条SKIP LOCKED，outbox按资源revision去重。

## 5. 错误与可复现验收

通用400/401/403/404/409/413/422/429/500/502/503/504遵循基线。领域错误：asset_duplicate/card_duplicate409、version_conflict409、source_changed409、invalid_state409、execution_conflict409、invalid_evidence422、invalid_model_output502、analysis_too_large422、budget_exhausted429、dependency_unavailable503、upstream_timeout504；错误details仅safe字段，重复可用operation_id承载已存在资源ID，不回传正文。

1. 导入含“甲𠀀乙”的素材，选择𠀀返回[1,2)，excerpt完全一致；同一文件与相同规范文本粘贴命中asset_duplicate，异tenant不相互泄露ID。
2. 重复相同Idempotency-Key同请求返回同asset/analysis ID及HTTP码；同键改title409且计数不增加。两次PUT同revision只有一次成功。
3. 创建pending不产生模型用量；UI创建Agent run后进入running，SSE来自run events；GET和重连不能再生成。
4. 模型虚构证据、超区间、预算耗尽分别产生明确failed；报告不存在，used_tokens仍完整；取消先提交后到结果不改变cancelled。
5. 修改文风产生version2，固定version1的Agent上下文保持原值；删除档案后新的读取拒绝，既有run快照仍可审计。
6. 两卡空白/NFKC等价正文触发唯一去重；原始正文保留。保存目标草稿后合法目标revision/hash可采纳，自造目标版本409；撤销后目标revision/hash不变。
7. 软删除后普通列表不显示；include_deleted可查看元数据并恢复，30天清理后原文不可恢复。跨owner/tenant访问各详情统一404。
## 6. 实施补充约束

Report与StyleVersion的内容版本在作者操作中不可变；保留清理是唯一擦除通道，必须增加其revision并保留redacted_at。已擦除的报告可返回空findings/patterns/ideas与redacted_at；已擦除StyleVersion及Card从普通查询中排除，详情返回404，不能构造不符合Schema的空instructions/body。源删除后的证据只显示source_unavailable，不返回已擦除excerpt；服务端移除对应evidence数组项，必要时移除无法满足证据最小数量的finding/pattern。清理不将Card状态或Adoption引用状态倒退，也不更改目标作品。

Asset文件内容通过FilePort读取并按固定LF规则规范化后切片；platform_files可保留原上传字节，asset_hash是规范化文本hash，不混用文件二进制hash。asset被清理时释放文件使用引用，platform按自身引用计数擦除bytea并保留被技术FK引用的最小元数据。source_url只记录来源，不发起网络抓取。StyleMetrics/instructions仅表达抽象风格参数，保存时拒绝超过500码点的逐字来源引文。
