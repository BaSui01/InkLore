> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 05 世界观域详细设计：R1 冻结实施契约

状态：INK-141 冻结目标，尚未实现。上位[基线](../00-CONTRACT-BASELINE.md)，概览[05](../05-DOMAIN-WORLDVIEW-SPEC.md)，DDL[worldview.sql](../../../../../docs/system-design/data/worldview.sql)，API[资源](../../../../../docs/system-design/api/worldview.json)、[关系](../../../../../docs/system-design/api/worldview-relations.json)、[事实](../../../../../docs/system-design/api/worldview-facts.json)及[类型](../../../../../docs/system-design/api/worldview-types.json)。

## 1. 边界与应用服务

本域只依赖platform；不导入content/outline/agent/llm/compliance。跨域chapter/version/novel标识使用UUID值，不携带对方可写对象。Transport→Application→Domain←Infrastructure，Domain无数据库/HTTP能力。HTTP方法与operationId去掉worldview_同名应用入口，输入/输出见末尾完整契约表。

| 方法/Port | 入参→返回 | 谁调用谁/事务 |
|---|---|---|
| WorldviewApplication.EnsureWorld | UoW,Scope,novel_id→WorldRevision | content开书经WorldviewFactPort调用，初始化revisions一次，不由GET懒写 |
| CharacterService/LoreService.Create/Update/Delete/Restore | ctx,typed request→Character/Lore | 作者UI调用；统一entities身份、本域schema分型 |
| RelationService.Create/Update/Delete/Restore | ctx,typed request→Relation | 作者UI调用；端点均由本域实体仓储验证 |
| FactService.CheckFacts | Scope,expected_world_revision,FactProposal[]→CheckResult | content UI预审/agent建议预检；只读，不保留事务到确认 |
| FactService.CommitFacts | UoW,Scope,novel/source_chapter/source_version/source_hash,expected_world_revision,proposals→event IDs/world revision | content.ConfirmChapter经app适配；复用同PGUoW，拒绝独立commit |
| FactService.Retract/Supersede | Scope,head_id,expected_revision,expected_world_revision,value?/reason/resolution→FactMutation | 世界观UI直接命令；在本域新UoW，追加事件而不是改旧事件 |
| WorldviewReadService.ReadContext | Scope,novel,entity IDs,token/record budget→typed entities/heads/relations | content、agent定义消费方Port调用；预算裁剪返回truncated及world_revision |
| SnapshotService.CaptureSnapshot | read-only UoW,Scope,novel→WorldviewSnapshot | content复制worker调用；只读本域、返回内存DTO，不解析文件 |
| SnapshotService.CloneSnapshot | UoW,target Scope,IDMap,WorldviewSnapshot→entity IDMap | content复制最终事务，重建实体/关系，重置动态投影，不复制账本 |
| ScopePort.AssertNovel/AssertTrashOwner/AssertResourceRevision | tenant,actor,scope identity/revision/hash→Scope | platform鉴权投影，禁止借机读取正文；历史来源由content可信传参+平台校验 |
| PGUoW/Idempotency/Outbox/Clock/UUID/FileStore | 平台技术能力 | app装配；幂等与事件/heads/响应同事务；snapshot文件由content消费者持有 |

creator audience与owner scope不可由客户端覆盖。operator/auditor默认看不到实体description、事实value或reason。Scope鉴权与worldrevision锁内检查均不可省；跨scope404，不能用“先返回实体，再检查其novel”泄漏存在性。

## 2. SQL与API字段对应

所有表UUIDv4、tenant FK、UTC created_at/updated_at；mutable行revision>0。所有业务外键都带tenant/novel，仅指向本域表。

| 表 | API及字段映射 | 不变量 |
|---|---|---|
| worldview_revisions | WorldRevision同名id/tenant/novel/revision/timestamps；last_seq内部字段 | 一书一行，锁是本域写入总序；任何本域成功变更整事务revision只+1 |
| worldview_entities | Character同名name/description/aliases/role_type/life_status/realm_rank/status；Lore.title双向转换name、category；kind不由客户端更新 | kind=character要求name≤60，life_status/rank非空，category空；kind=lore要求category非空，角色字段空 |
| worldview_relations | Relation所有字段同名 | subject/object复合FK同书，禁自环，有效active三元组唯一 |
| worldview_fact_ledger_events | FactEvent同名身份/source IDs/action/target/resolution/seq/world_revision；payload_digest内部SHA256 | 不存事实原文、理由或用户姓名；不可UPDATE/DELETE |
| worldview_fact_payloads | FactEvent.value由value_kind及对应值列构造；reason/actor_id/source_content_hash同名；payload_erased=载荷不存在 | 事件正常写入必有一条载荷，业务不可变；retention可删除以满足擦除 |
| worldview_fact_heads | FactHead同名identity/subject/predicate/key/event_id/status/revision；value从head.event_id对应payload只读派生 | 每个语义槽只有一个head；retract保留head并使value=null |

FactValue分型：text={kind,text_value}1..2000；number={kind,number_value}整数0..10000；boolean={kind,boolean_value}；entity={kind,entity_id}。同一值不得同时出现多个分支字段；SQL用value_kind+num_nonnulls强制单值，JSON Draft2020-12 oneOf+additionalProperties:false阻止任意object。UUID引用实体先校验同书且未删。

FactEvent正常retract.value=null，payload仍保存reason/actor/hash；擦除后payload_erased=true且value/reason/actor/hash均null，source UUID/seq/action/target/key/digest保留，不再声称可还原事实原文。event_id不等于head.id，API /facts/{fact_id}明确使用head.id；proposal.target_event_id使用账本事件ID。

## 3. FR-10 角色、设定、关系

CreateCharacter默认life_status=alive、realm_rank=0、status=active；作者传name1..60、description≤20000、aliases≤20个非空1..60字符、role_type=protagonist/supporting/antagonist/other。aliases先去完全相同重复值，保留作者大小写，不按显示名合并不同角色，重名合法。CreateLore传title1..120、description≤20000、category=faction/place/system/item/history/other，status=active。

PUT明确全字段更新，expected_revision必填；角色PUT只开放静态字段和status，不开放life_status/rank。active双向转换archived都需CAS。archived保留图谱与审计但生成默认context不选；恢复active不改变事实。创建/修改实体同UoW更新platform entity scope投影，outline只借平台验证POV/地点UUID；实体删除后该投影立即不可用。

DeleteEntity要求无未删active relation且无active fact_head引用（subject或entity值），否则409 worldview.invalid_state；UI引导先退休关系、显式retract事实，再删除。若仅已撤回事件引用，允许软删，不丢账本。30天恢复同ID/revision+1；不自动恢复用户另行删除/退休的关系。RestoreRelation要求两端都有效同书，且同三元组无active竞争；失败409不改任何行。

CreateRelation传subject_id、predicate1..60、object_id、description≤2000、weight0..100；predicate是作者维护的关系标签，不等于FactPredicate。subject/object可character或lore，均在本域entities复合FK下，不能引用其它书。状态active双向转换retired；PUT允许修改端点但在锁worldrevision后重新验证全部不变量。删除是deleted_at，退休是领域状态，两者语义不同。静态关系不自动改事实relationship槽；UI分开“设定关系”与“已确认关系事实”。

Graph默认depth=1；有center_id时广度优先无向遍历两端邻接，depth最多2；无center时按实体updated_at/id取全图有界首批。最多500节点/1000边，均当前scope，边两端必须已包含，超过限制truncated=true；排序稳定，UI提示使用center缩小范围或关系分页列表，不伪装完整全图。图返回world_revision，布局是客户端派生不入事实表，无障碍等价列表使用同一nodes/edges。空图展示真实空态。

## 4. FR-07 事实提议与确定性校验

FactProposal字段必填（允许null由schema指定）：subject_id、predicate、fact_key、value、action、target_event_id、resolution、reason。上限100，一批(subject,predicate,key)不重复。fact_key必须snake_case [a-z][a-z0-9_]{0,119}，不得放人名/正文；核心predicate要求fact_key同predicate，custom才使用作者稳定机器key。

| predicate | 合法value | 约束 |
|---|---|---|
| life_status | text alive/missing/dead/revived | subject必须character；初始alive；通常alive→missing/dead，missing→alive/dead，dead→revived，revived→missing/dead；复活必须逐条明确retcon及理由 |
| realm_rank | number 0..10000 | subject必须character；非递减正常，下降必须retcon |
| item_status | text intact/damaged/destroyed | subject必须category=item的lore；intact→damaged/destroyed，damaged→destroyed，恢复/重造必须retcon |
| location | entity | 对象必须category=place的有效lore；同一subject当前地点唯一 |
| relationship | entity | 对象必须有效同书实体且不等于subject；单槽关系是已确认重点关联，多种命名关系使用custom不同key |
| custom | 任一明确FactValue | 不推断自然语言真假；不同值仍要求supersede，不因“自定义”绕过head一致性 |

assert必须target_event_id=null、resolution=none、value非空，且语义槽没有active head；若head已retracted则允许新assert并沿用同head.id/revision+1。首次head不存在则创建revision=1。supersede必须target等于当前active head.event_id、value非空、resolution=correction/retcon、reason1..1000。retract同样精确target且value=null、reason必填；命令的默认resolution=correction；已retracted再撤回新Key409，不无限追加空撤回。

correction只允许符合正常状态迁移/非下降约束的事实修正；retcon明确允许非正常迁移并保留理由。不能把reason为空、target旧值或者不存在实体通过retcon放行，不能使用allow_override批量绕过。同一assert值与已有head相同也冲突，避免误判“重复事实”能静默确认；客户端应省略已知事实或明确supersede。

CheckFacts在REPEATABLE READ事务内取worldrevision+所需heads+实体；expected_world_revision不符409 worldview.fact_conflict。非版本冲突以CheckResult.valid=false及conflicts返回200（尚未写库）；每项conflicts有subject_id/fact_key/current_event_id/proposal_index/reason。check POST虽只读仍遵守非GET Idempotency-Key入口契约，重放不改变任何领域revision。最终CommitFacts再次锁与计算，绝不相信check结果缓存就是写入许可。

## 5. 同PGUoW确认与修订事务

content锁scope→novel→chapter，然后调用本域；本域按worldview_revisions锁→涉及entities按UUID排序→heads按(subject,predicate,key)排序。纯worldview修改锁scope→worldrevision→实体/heads，不再锁novel/chapter，因此不会与content形成逆序锁环。

CommitFacts必须携已有opaque UoW。验证当前membership/scope与来源chapter/version/hash的可信平台历史投影，检查expected_world_revision；Domain.Inspect生成全量ConflictSet，任何冲突raise worldview.fact_conflict使外层事务整体回滚。正常则分配new_world_revision=old+1、连续seq(last_seq+1..+N)，逐条插immutable ledger event、payload与digest，upsert heads（expected event精准比较），重新计算受到影响的character life/rank投影并对实体revision+1，更新last_seq/worldrevision、平台投影、outbox。返回event_ids和new_world_revision给content，只有content最终COMMIT才能持久化。本域不得在异常中捕获冲突继续写其余条目。

空proposal批次也增加worldrevision一次；N=0时last_seq不变，这是确认快照边界而非空事实事件。worldrevision初始1，字符静态修改也会递增，防止作者在旧角色设定基础上确认新事实。Outbox、幂等响应与所有领域修改同一次提交。

直接Retract/Supersede根据head.id查同scope头，锁worldrevision与head，校验请求expected_revision=head.revision、expected_world_revision=worldrevision；target自动取锁后head.event_id，插新event，source_chapter/version/hash置null表示人工修订而不是伪造章节出处；原事件仍保留真实来源。撤回后head.status=retracted/value=null，角色投影回默认或其他仍有效语义槽。相同Key重放同FactMutation（同event.id），不同payload同Key409。没有“更新事件的有效标志”或DELETE ledger。

## 6. 投影重建、擦除及补偿

事件本体SQL触发器拒绝UPDATE/DELETE，业务数据库角色仅SELECT/INSERT。载荷采用独立行是为了协调“不可变账本”和“软删除到期擦除”，不是把旧事件改成新事实。正常业务payload也仅SELECT/INSERT，retention专用角色才可DELETE；event.updated_at始终等于created_at，删payload不修改event。

payload_digest = SHA256(每事件独立随机UUIDv4盐的bytes + RFC8785 canonical JSON{value,reason,actor_id,source_content_hash})；盐存payload并与其一并擦除，防止保留digest被字典攻击还原敏感事实。事件本体只保留没有直接身份含义的UUID、predicate/key、action/resolution与digest；不得将原正文、角色姓名、任意用户文本写fact_key或审计code。payload_erased后的账本只证明发生过哪个序号/修订链，不能验证被擦除原文。

head重建由平台维护作业调度本域ReplayHeads，持scope系统capability与worldrevision租约：读取该书按seq递增的事件，断言target链前驱正确，重建临时head集合；成功后短事务校验读取worldrevision未变，替换本域heads投影，保留head既有ID并revision+1，仅修复投影不追加业务event。新headID由旧头语义键映射恢复；没有旧头时新UUID且任何客户端旧revision无效。角色动态状态按重建heads重算，worldrevision+1与outbox同事务。若发现ledger链损坏或未到期payload缺失，停止作业、报worldview.projection_corrupt，不能猜测值并继续发布。

整书删除后scope立即阻止读写，30天内payload保留支持恢复；到期平台retention调用PurgeNovel，先删除heads/relations/entities敏感内容及payload（包括reason/actor/hash/salt），worldrevision记录仅保留必要调度/去标识化元数据；ledger事件保留，无跨域删除。已擦除作品不再重建可读heads。单个实体软删被引用保护，因此到期可以删除静态档案而不破坏active fact。安全审计摘要最多90天，不保存原始description/value。

补偿只针对技术投影/通知：outbox发送失败重放；内联head/character投影更新失败整个交易回滚，不能提交“账本成功但head失败”。长snapshot文件序列化由content在事务外完成；FileStore写后消费方作业失败由content清理孤儿file，worldview不私建第二队列。

## 7. 仓储、查询、索引与限制

WorldRevisionRepo.Lock/NextSeq必须tenant+novel；EntityRepo.Get/List/BatchGet按tenant/novel/kind与updated_at DESC,id DESC，批量最多500；GraphRepo.Neighbors分别走subject与object索引，不做全表反向扫描。RelationRepo.List按更新游标，active唯一索引防重复边。HeadRepo.GetByKey/LockBatch按唯一语义键，HeadRepo.List按tenant/novel/updated_at/id并可过滤subject/status；LedgerRepo.Append/Page按tenant/novel/seq DESC，cursor绑定actor/filter，事件游标不用会变化的head排序。

Ledger与payload/head/entity的join只发生本域；source章标题通过UI独立content请求展示，本域只返回UUID来源。跨scope来源不可见时UI展示“来源不可访问”，不会要求worldview拿管理员权限代查正文。

容量：每书有效entities≤10000、relations≤50000、active heads≤100000；每次事实提议≤100，图节点500/边1000，列表limit1..100默认20。创建及批次锁worldrevision后检查容量，422 worldview.validation_failed给safe field_errors。Redis不可用直接PG读写，不能用Redis锁代替PG本域总序。

## 8. 复制与静态资料复用

内部WorldviewSnapshot={novel_id:UUID,world_revision:Revision,characters:Character[],lore:Lore[],relations:Relation[]}，数组按id稳定排序，IDMap={kind:entity|relation,source_id:UUID,target_id:UUID}[]；Character读取类型包含投影字段，但Clone明确忽略其life_status/realm_rank。CaptureSnapshot输出character/lore静态字段、有效relations、worldrevision；只包含当前授权书，不读content。content统一捕获跨域只读快照并在事务外交平台FileStore保存。CloneSnapshot参与content的最终UoW：预分配新entities/relations UUID，映射全部端点，创建new worldrevision初始1（EnsureWorld已存在则在本次clone提交内按规则增加一次），字符life_status=alive、realm_rank=0；不复制ledger、heads、payload、删除项或原actor来源。description可能描写源角色设定，作为作者主动复制静态文本保留，但UI动态状态明确是新书初始状态。无法映射的外域来源字段置空，不能跨书引用原scope。

## 9. 验收与错误恢复

1. 两种实体表单的字段边界、角色重名、aliases、归档/恢复；试图PUT life_status被schema拒绝，不能绕过账本。
2. 关系自环/跨书端点/重复active三元组失败；图与关系表同源，500/1000上限显示truncated，不暴露未授权节点。
3. 作者提交alive→dead，再提交dead→alive无retcon失败；带retcon和精确target成功；realm下降同理，reason/target缺失均失败。
4. 同worldrevision两个确认只有一个提交；跨content事务在账本插入后失败，两域均回滚；空提议确认worldrevision仍前进一次。
5. supersede/retract后旧事件完全不变，head指新事件；重复幂等键不多一条事件；并发旧head.revision返回409。
6. 从ledger顺序重建得到同active/retracted语义；篡改投影可修复，缺载荷/断链停止，不猜值。DELETE/UPDATE ledger被SQL触发器阻止。
7. 30天删除到期payload/盐/静态文本擦除，ledger仍是UUID/digest证明且payload_erased=true；重新开书不能恢复旧事实原文。
8. Clone后新ID、新书授权、动态初始状态、无事实历史；任一clone错误回滚整个新书，可安全重试。

错误：400格式/游标、404不存在或scope隐藏、409 worldview.version_conflict用于实体/head CAS、409 worldview.fact_conflict用于世界观版本/事实冲突、409 worldview.invalid_state用于引用删除/重复三元组、422 worldview.validation_failed用于类型/范围/容量、503依赖失败。recovery按API x-errors执行，错误只给安全标识、current_revision和field_errors。本轮只交付设计与静态契约核验，不增加测试文件。

单项回收站：资源集合GET的deleted默认false；deleted=true仅原Owner在父作品未删除且30日保留期内列出已软删项目，使用同一实体Schema与deleted_at，并显示restore动作。这不授予读取已删除整书正文/设定的权限；整书回收站仅content Novel元数据，需先恢复整书才访问其子资源。cursor绑定deleted过滤条件。

## 10. 完整HTTP操作与应用方法索引

每行对应Application方法（机读operationId带域前缀）。非GET均要求Idempotency-Key，路径UUID必填，query星号为必填；DELETE使用query.expected_revision，PUT与状态命令使用JSON.expected_revision。成功响应为SuccessMeta+data。列表均使用common Limit/Cursor，类型定义见各域types分片。

| FR/应用用例 | HTTP | 请求Schema / query | 成功data |
|---|---|---|---|
| FR-10 worldview.listcharacters | GET /v1/worldview/novels/{novel_id}/characters | — / deleted | 200 CharacterPage |
| FR-10 worldview.createCharacter | POST /v1/worldview/novels/{novel_id}/characters | CreateCharacter / — | 201 Character |
| FR-10 worldview.getCharacter | GET /v1/worldview/characters/{character_id} | — / — | 200 Character |
| FR-10 worldview.updateCharacter | PUT /v1/worldview/characters/{character_id} | UpdateCharacter / — | 200 Character |
| FR-10 worldview.deleteCharacter | DELETE /v1/worldview/characters/{character_id} | — / expected_revision* | 200 Character |
| FR-10 worldview.listlore | GET /v1/worldview/novels/{novel_id}/lore | — / deleted | 200 LorePage |
| FR-10 worldview.createLore | POST /v1/worldview/novels/{novel_id}/lore | CreateLore / — | 201 Lore |
| FR-10 worldview.getLore | GET /v1/worldview/lore/{lore_id} | — / — | 200 Lore |
| FR-10 worldview.updateLore | PUT /v1/worldview/lore/{lore_id} | UpdateLore / — | 200 Lore |
| FR-10 worldview.deleteLore | DELETE /v1/worldview/lore/{lore_id} | — / expected_revision* | 200 Lore |
| FR-03 worldview.restoreCharacter | POST /v1/worldview/characters/{character_id}/restore | RevisionCommand / — | 200 Character |
| FR-03 worldview.restoreLore | POST /v1/worldview/lore/{lore_id}/restore | RevisionCommand / — | 200 Lore |
| FR-10 worldview.listrelations | GET /v1/worldview/novels/{novel_id}/relations | — / deleted | 200 RelationPage |
| FR-10 worldview.createRelation | POST /v1/worldview/novels/{novel_id}/relations | CreateRelation / — | 201 Relation |
| FR-10 worldview.getRelation | GET /v1/worldview/relations/{relation_id} | — / — | 200 Relation |
| FR-10 worldview.updateRelation | PUT /v1/worldview/relations/{relation_id} | UpdateRelation / — | 200 Relation |
| FR-10 worldview.deleteRelation | DELETE /v1/worldview/relations/{relation_id} | — / expected_revision* | 200 Relation |
| FR-03 worldview.restoreRelation | POST /v1/worldview/relations/{relation_id}/restore | RevisionCommand / — | 200 Relation |
| FR-10 worldview.graph | GET /v1/worldview/novels/{novel_id}/graph | — / center_id, depth | 200 Graph |
| FR-11 worldview.getWorldRevision | GET /v1/worldview/novels/{novel_id}/world-revision | — / — | 200 WorldRevision |
| FR-11 worldview.listFacts | GET /v1/worldview/novels/{novel_id}/facts | — / subject_id, status | 200 FactPage |
| FR-11 worldview.listFactEvents | GET /v1/worldview/novels/{novel_id}/fact-events | — / subject_id | 200 FactEventPage |
| FR-11 worldview.checkFacts | POST /v1/worldview/novels/{novel_id}/facts/check | CheckFacts / — | 200 CheckResult |
| FR-11 worldview.retractFact | POST /v1/worldview/facts/{fact_id}/retract | RetractFact / — | 200 FactMutation |
| FR-11 worldview.supersedeFact | POST /v1/worldview/facts/{fact_id}/supersede | SupersedeFact / — | 200 FactMutation |
