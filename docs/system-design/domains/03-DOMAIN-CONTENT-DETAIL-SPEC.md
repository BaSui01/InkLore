# 03 内容域详细设计：R1 冻结实施契约

状态：INK-141 冻结目标，尚未实现；不描述旧源码。上位规则为[基线](../00-CONTRACT-BASELINE.md)，概念入口为[内容域](../03-DOMAIN-CONTENT-SPEC.md)。字段权威见[类型](../api/content-types.json)，路由见[资源](../api/content.json)、[命令](../api/content-commands.json)、[作业](../api/content-jobs.json)，DDL见[content.sql](../data/content.sql)。

## 1. 分层、签名与共享契约

Transport仅鉴权/解码/参数上限/错误映射；Application编排scope、幂等、锁、仓储与Port；Domain执行状态/字数/排序/确认规则；Infrastructure实现本域表、纯格式编解码。所有方法首参Context含tenant_id、actor_id、audience、session_id、request_id，所有写方法第二参Command含expected_revision（创建不需要）及幂等Key。签名约定App.<operationId去掉content_>(ctx, request) -> 对应响应data；末尾API表逐个列出请求与返回，不能把CRUD委托给不带scope的通用Repository。

| 消费方接口 | 参数→返回 | 被调用者/拒绝与边界 |
|---|---|---|
| ContentReadService.ReadChapterSnapshot/ReadContext | Scope,novel,chapter_ids(≤100),max_bytes(≤2MiB)→Novel/Chapter及body/hash/version_id、truncated | agent.ContentPort通过app调用；本服务只读本域，可经OutlinePort/WorldviewReadPort补充指定资源；agent负责其模型Token预算，content不反调agent |
| ScopePort.AssertNovel/AssertResource | tenant,actor,novel/resource,capability→Scope | platform.ScopeAuthorizer；跨scope404；归档可read/export/copy但不可write |
| ScopePort.AssertTrashOwner | tenant,actor,novel,resource→Scope | 仅原Owner回收站访问；不能恢复30天已擦除数据 |
| ScopeProjectionPort.Apply / AppendRevision | UoW,resource revision/hash,codepoint_length(必填),parent,lifecycle→void | app适配写platform当前及历史授权快照，同事务立即撤权 |
| WorldviewFactPort.EnsureWorld | UoW,scope,novel_id→WorldRevision | worldview.EnsureWorld；开书初始化revision=1；不是GET时偷偷建行 |
| WorldviewFactPort.CommitFacts | UoW,scope,novel,chapter,version,hash,expected_world_revision,proposals→事件IDs/world_revision | worldview应用；不能开启独立事务；账本冲突原样上抛 |
| OutlinePort.ReadContext/CaptureSnapshot/CloneSnapshot | scope,novel,或UoW,target,IDMap,snapshot_file_id→typed snapshot/IDs | outline应用；克隆重建节点/节拍/场景/伏笔，映射引用 |
| WorldviewPort.CaptureSnapshot/CloneSnapshot | scope,novel,或UoW,target,IDMap,snapshot_file_id→entity IDMap | worldview应用；只复制静态档案/关系，动态事实与账本不复制 |
| ExportReviewPort.ValidateSnapshot | actor,novel,chapter revision/hash集合,review_ticket?→ReviewDecision | compliance应用；无报告允许；匹配block报告必须本人ack；不把“无报告”伪装成通过检测 |
| FilePort.Read/Put/Delete | scope,file_id或bytes/media/hash/retention→FileRef | platform.FileStore；PG bytea，tenant/owner复核；拒绝外链/Base64 |
| JobPort.Register/Claim/Heartbeat/Notify | domain=content,job_id,lease_token→Lease | platform.JobScheduler调度本域Claim回调；业务状态仍content_jobs |
| IdempotencyPort.Begin/Complete | UoW,tenant,actor,method,path,key,request_hash→lease/replay | platform；资源、版本、outbox和幂等结果同commit |

所有端口写入传platform不透明UoW，不传gorm.DB。正文选择范围(source_revision,hash,start,end)供agent/compliance使用时，消费方通过platform.AssertResourceRevision验证；content对每次章revision写入同步平台历史快照。scope授权投影只包含归属/版本/hash/content_length，不承载正文。Owner每次AppendRevision必传codepoint_length=规范化正文的全部Unicode码点数（含空白），平台存为content_length且无DEFAULT 0；只有空正文长度为0，不可用word_count代替。长度与同一revision的hash在同UoW绑定。长作业无法永久持有授权；每阶段重验membership与scope，授权撤销不再输出敏感文件。

## 2. 参数与字段映射

UUID均v4；created_at/updated_at UTC TIMESTAMPTZ，API Timestamp为UTC Z。全部可变行revision正整数从1；成功状态变化+1，不允许客户端设置id/owner/tenant/position/hash/word_count等派生字段。title1..120、summary≤2000、category≤60；字符计数均Unicode码点。每书有效卷≤100、有效章≤2000，每章正文≤200000；作品target_words≤100000000，word_count≤400000000。JSON≤2MiB，空正文合法，禁止超限静默截断。

| API实体 | SQL映射/额外内部字段 |
|---|---|
| Novel全部字段 | content_novels同名列；previous_status仅deleted时记录原active/archived |
| Volume全部字段 | content_volumes同名；active_position是软删除过滤后的唯一约束辅助列，不返回 |
| Chapter元数据+Body.body | content_chapters同名列；current_version_id/confirmed_version_id为本域复合FK，延迟检查解决初建环 |
| Version全部字段（列表VersionSummary排除body，单版本GET才取body） | content_chapter_versions同名，chapter_revision是创建该快照时的revision；可变章后续确认/重排不伪造新body版本 |
| DailyStat/Statistics | writing_events按北京时间聚合；net_words=added_words-removed_words；save_count=事件数；缺日补0；current_word_count取novels |
| Job | content_jobs同名对外列；available_at/lease_token为调度内部；volume_ids/include_drafts/include_planning/include_worldview、commit_volume_id/commit_item_ids/expected_novel_revision、两个跨域snapshot_file_id和organization_snapshot_file_id为持久化命令输入；后者以Organization及类型化IDMap数组保存全部卷（含空卷）与源/目标UUID映射 |
| ImportItem | content_import_items同名；selected只在commit受理时CAS置true，UI选择暂存本地，服务端预览不可编辑 |
| JobSnapshot | content_job_snapshots固定章/版本/title/body/hash/卷章position；独立副本用于渲染，避免版本清理或编辑干扰 |
| SearchHit / Diff | 查询派生，不建表；snippet与diff文本是已授权正文切片，不写日志 |

正文传输CRLF正规化为LF，除此不改空格/缩进/空行/标点；规范化后SHA256计算UTF-8 bytes，word_count统计非Unicode White_Space码点。空白表固定采用Unicode15.1 White_Space属性，不使用数据库locale或浏览器分词。

## 3. FR-02/03 作品生命周期

CreateNovel事务顺序：校验creator当前成员资格→Begin幂等→INSERT novel(active,revision=1,word_count=0)→app建立scope及其owner投影→worldview.EnsureWorld→outbox novel.created→Complete幂等→COMMIT。不自动插入空卷/章；开书向导下一步按真实volumes/chapters POST创建。书架默认active，可按status=archived/deleted筛选；deleted只回收站元数据，不返回正文。

UpdateNovel仅元数据，不允许写status/owner/word_count。Archive只从active到archived，禁止活动import commit；已有只读export可继续，但worker仍验证scope。Restore(archived)回active，Restore(deleted)回previous_status；恢复投影同事务可用，旧单独删除的卷/章仍deleted。DeleteNovel从active/archived→deleted，记录时间与previous_status，撤销一般scope，outbox请求作业取消与下游清理；即使通知未送达，下游scope立即拒绝访问。软删除是权限闭环，不靠级联逐表“标记完成”防越权。

卷/章DELETE保留30天并释放排序位置；删非空卷409 content.invalid_state。同书已删章不计novel.word_count，恢复加回；恢复卷/章在其父尾部重新分配position且加revision，不挤占现有有效项；容量不足422。30天内恢复同ID。到期retention worker使用平台Capability，仅按系统确定的tenant/novel tombstone清理，不能从UI伪造scope；先作业/事件引用、再版本、章、卷、作品（FK延迟检查），各域通过平台调度分别擦除数据；幂等墓碑记录确保部分失败可续做，不通过content直接DELETE其它域表。

CopyNovel为202作业。源可active/archived；请求包含title、include_planning、include_worldview和expected_revision。受理时预分配target_novel_id但书架不可见。worker捕获源content与两域同一REPEATABLE READ UoW只读快照，转换/文件写出在事务外；source_revision与快照绑定；copy/export在捕获时novel.revision必须等于job.source_revision，不相等则failed且error_code=content.version_conflict，不偷偷使用提交后新版本。跨域CaptureSnapshot Port在只读UoW中返回类型化内存快照，content在事务外序列化到PG FileStore，file_id保存到jobs，正文在job_snapshots；Organization与全部source_id/target_id/kind映射写organization_snapshot_file_id，空卷也保留。快照总量≤100MiB，超限作业失败明确content.snapshot_too_large。完成时一次短PGUoW建立新书scope→卷章新ID/版本source=copy且全draft→worldview.CloneSnapshot→outline.CloneSnapshot→job succeeded/outbox。IDMap全部由作业预分配且重试复用。没有include_worldview则场景POV/地点置空；复制世界观时映射实体引用。历史versions、事实账本、写作统计、生成运行、合规报告、源授权均不复制。复制伏笔保留计划文本，状态统一planned，清空埋设/回收事件；不得把源小说事实伪装为新书已发生事实。

## 4. FR-04 组织与并发

CreateVolume/CreateChapter在novel锁内追加有效兄弟尾部；章创建同事务插入空正文Version(source=create)、current_version_id，revision=1，并创建scope。卷章元数据更新CAS自身revision并更新novel.revision，章title更新同事务产生source=save版本，body不变且统计事件added/removed=0。

GET organization返回完整有效volumes/chapters及novel_revision，按卷position/章position排序；普通列表仍updated_at/id游标，不混用目录游标。Reorder携完整volume_ids和全部有效chapter_id→volume_id/position。所有ID唯一、同书、非deleted；卷ID集合、章ID集合与锁后数据库集合完全相等；各卷章position从0连续，无洞。锁顺序scope→novel→所有受影响章按UUID排序；更新卷/章位置、相关revision与scope，novel只+1。数据库active_position延迟唯一约束避免交换位置中途冲突。任何检查或CAS失败整批回滚。正文移卷不反向驱动outline改变计划；app同步平台parent投影，outline UI提示旧绑定需作者调用rebindNode选择对应新卷纲，保留原节点及伏笔引用。

## 5. FR-05/06 保存、版本、diff与回滚

SaveBody锁scope→novel→chapter；校验novel active、chapter有效、expected_revision；规范化body与计算hash/word_count；即使内容相同，新幂等键的显式保存仍创建版本并增加revision，原幂等重试不新增。插入Version(source=save)，更新chapter.body/current_version/hash/count/status=draft（confirmed_version_id保留旧确认来源），按差值更新novel.word_count并增加novel.revision，追加writing_event、同步平台当前及历史revision/hash、outbox并完成幂等。任何一步失败没有“正文成功但版本缺失”。版本不按最近50条删除；有效作品全部保留，直到主动删除到期擦除。

Diff校验两version都属于当前授权章。先LF行级Myers，再对变化块做码点级Myers，返回equal/insert/delete的old/new offset及text；总计算上限5000000步，超过则变化块作为delete+insert且coarse=true，仍精确可还原两端，不截断原文。响应最多400000个hunk（实现应合并相邻同kind），两端正文合计≤400000码点。前端双栏渲染，不能把码点offset误当UTF16。

Rollback必须先由UI处理未保存本地草稿：保存或明确舍弃，不能后台静默覆盖。服务校验目标version同章并按当前expected_revision执行Save流程，source=rollback、restored_from_version_id目标；回滚新version、新chapter revision、status=draft，写作统计记录实际文本变化。历史版本不改；已确认事实不自动retract。metadata/排序/确认产生新的chapter revision时，即便body未变也写授权历史revision/hash，供跨域严格定位。

写作事件added/removed来自diff中的非空白码点数，替换相同长度文字仍计新增和删除；不是仅取新旧总字数差。UTC时间由Clock产生，北京自然日查询先把from00:00与to+1日00:00按Asia/Shanghai转UTC范围再用created_at索引过滤；分组使用AT TIME ZONE 'Asia/Shanghai'。范围1..366日且to>=from。导入/复制/初建不记writing_event，避免刷字数。

## 6. FR-07 章节确认原子账本

Confirm请求={expected_revision,expected_world_revision,fact_proposals≤100}。作者确认UI展示固定正文版本和fact提议列表，逐条选择assert/supersede/retract、target_event_id、resolution和理由；不会在确认按钮后台另跑模型。

固定顺序：scope授权并保持权限版本→novel锁→chapter锁→worldview revision锁。确认只允许当前draft且有current_version；空正文允许确认但无事实提议。应用把chapter/version/hash作为可信来源传WorldviewFactPort.CommitFacts，同UoW插入ledger/payload、更新heads、角色投影、worldrevision；然后content改confirmed、confirmed_version_id=current_version_id、chapter.revision+1、novel.revision+1、scope投影/幂等/outbox一起提交。worldrevision每次成功确认+1，即使提议为空，用于记录本次审阅边界。确认不再生成相同body版本，源版本是作者实际审阅的版本。

过期章409 content.version_conflict；过期世界观或逐条事实冲突409 worldview.fact_conflict。details只给current_revision或安全冲突标识，禁止正文和SQL。已confirmed新幂等键再确认返回409 content.invalid_state，避免隐式重复入账；同键完成重放返回原Confirmation。用户改正文后状态draft，旧facts保留；重新确认若相同key已有head，必须明确supersede/retract或不提交该事实。不存在全局allow_override。

## 7. FR-18/19 长作业、调度与文件补偿

ContentJob状态：queued→running→preview_ready（import）→committing→running(stage=commit)→succeeded；export/copy为queued→running→succeeded。非终态可→cancelling→cancelled；技术/业务不可恢复失败→failed；预览24h未提交→expired。succeeded/failed/cancelled/expired不可回running。stage和progress只按真实解析条目/输出字节/落库阶段计算，不伪造定时增长；preview_ready时progress=60，提交后完成100。

HTTP只短事务校验与创建jobs/幂等/outbox，POST返回202 Job。平台JobScheduler以domain=content注册回调，content仓储用FOR UPDATE SKIP LOCKED领取本域候选，lease30秒、每10秒心跳、lease_token(UUID)围栏；每次写进度/终态须匹配token及未取消状态。attempts表示技术重试数0..3（初次不计），退避1/2/4秒；worker崩溃租约到期重新领取。文件解析/格式化完全在事务外。worker开始、读取文件、commit/发布前分别验证原actor会员版本、novel scope；不得把管理员权限当作者权限。

幂等边界分层：HTTP键24h；job_id作为永久内部execution_key；FileStore.Put使用job_id+artifact_kind产生确定性文件唯一键；本域jobs行锁/CAS确保一次最终提交，生成章ID由job_id与预分配UUID映射稳定保存（commit items对应创建ID可由item.id直接作为新chapter.id，v4不变）。事务未知结果先查job终态，不再次创建章节。取消与最终事务争夺同job行锁；取消先赢则worker不得发布，成功先赢则取消返回当前succeeded。取消命令携job.expected_revision；超时用户先GET再重试新revision。

输入通过平台上传端点获得file_id。CreateImport验证file owner/tenant、MIME与扩展一致、size≤20MiB，TXT只UTF-8（允许BOM），UTF-16/非法UTF-8返回content.unsupported_encoding。DOCX用ZIP+OOXML标准库解包：≤2000 entries、解压总量≤100MiB、压缩比≤100、禁路径穿越、DTD/外部实体、宏及外部链接获取；图片/嵌入对象不转正文，发现则明确content.unsupported_document拒绝以免静默丢失。段落次序保留，表格按单元格TAB/行LF输出；标题优先Heading1，其次严格整行“第…章”标题，缺标题整份作为一章并warnings=heading_inferred；章节空正文warning=empty_chapter。标题>120、章>200000码点、条目>2000整体解析失败，不截断。

预览items只写content_import_items，GET分页可审阅title/body/hash/warnings，禁止请求时临时重新解析。CommitImport.expected_revision绑定job.revision，expected_novel_revision绑定当前作品，volume_id为既有有效同书卷，selected_item_ids1..2000无重复且属于本job。受理事务CAS preview_ready→committing并持久化选择；worker短事务锁scope→novel→job，复查novel revision、卷、容量，按ordinal追加章/初始Version(source=import)，更新novel统计与scope，然后job成功。与受理后作者编辑竞争时409意义以job.error_code=content.version_conflict终止，不部分导入；作者重新发起新导入作业，原预览仍在保留期可读取。

CreateExport.expected_revision绑定novel；volume_ids为空表示全书，非空必须无重复且同书；include_drafts=false只选confirmed，并保留章序，不因跳过草稿伪造连续原编号。worker在短REPEATABLE READ快照捕获所有待导出chapter，ValidateSnapshot确认合规审阅；没有报告无需票据，匹配block报告必须本人ack review_ticket且所有章revision/hash一致。渲染完成在发布前再验证相同快照的审阅资格和scope，源正文后续变化不混入产物；票据针对快照而非“当前随便一版”。

TXT：UTF-8无BOM，卷标题/章标题与正文间LF空行；Markdown：卷H1、章H2，正文原样并转义导致非预期结构的标题前缀；DOCX：标准OOXML正文宋体12pt、24pt行距，标题黑体16pt、章分页，XML转义；EPUB3：mimetype首个非压缩条目、container.xml、package.opf、nav.xhtml与章XHTML，UTF-8，不访问外部资源，无封面时不伪造图片。四格式都写title/作者显示名/语言zh-CN元数据，输出bytes>100MiB立即停止并失败content.export_too_large。FileStore落PG后job记录output_file_id；HTTP下载只走平台鉴权文件接口，7天产物TTL，每次下载再验证scope。

FileStore成功而job提交失败：重试查execution_key复用相同文件，失败/取消作业后台Delete孤儿文件；job标成功但WS通知失败仅outbox重放，文件不重渲染。预览/源临时文件24h过期，输出文件7天，job无敏感内容摘要保留90天；source_file若为平台用户独立上传则仅解除job引用，由平台引用计数/retention决定清除，不能误删其它作业引用文件。源作品删除后立即失去下载scope，到期擦除所有关联快照/输出。

## 8. FR-20 查询、索引与错误恢复

仓储每个方法显式Scope参数，禁止FindByID裸查询。NovelRepo.List用tenant/owner/status/updated_at/id；VolumeRepo.List、ChapterRepo.List元数据不SELECT body。HistoryRepo.List用tenant/chapter/created_at/id；JobRepo.List限定actor，operator不读取作者作业结果。所有查询不join跨域私表。

Search q2..100码点，trim外层空白但内部精确，按case-insensitive字面子串搜当前有效章title/body，不解释%、_为通配符。pg_trgm GIN缩小候选，参数化ILIKE转义ESCAPE处理；2字符也能正确退化到本书过滤扫描。按updated_at DESC,id DESC分页，不按不稳定分数；返回每章第一处body匹配码点[start,end)，只命中title时offset=0/end=0且snippet=title。snippet最多500码点，无HTML高亮注入；点击先校验chapter_revision/hash，过期重新检索。软删除及归档scope遵守授权；归档可搜，删除不可搜。

错误恢复固定：400改请求/游标；404清除本地不可访问资源；409版本冲突刷新并人工合并；409幂等进行中按Retry-After:1查询/同键重试；422展示字段限制；429作业容量等待；503依赖失败可同键重试；500只显示request_id。每租户同时运行content jobs≤4、每actor非终态≤10，超限content.job_capacity_exceeded；Redis故障不影响这组PG约束。

## 9. 验收步骤与事务故障点

1. 开书→卷→章→保存，GET重读确认所有typed字段和revision；相同Key重放不增加versions，异载荷同Key409。
2. 两客户端同revision保存：一方200，另一方409；冲突界面三方合并，不丢本地副本；断网与IndexedDB拒绝均有真实告警。
3. 目录拖拽跨卷，完整排列成功；重复/遗漏/跨书ID均整批不变；删除/恢复位置尾插且word_count正确。
4. 版本A→B→回滚A得到C，C为draft；diff hunk可重构A/B；既有world facts保持，UI提示仍需事实修订。
5. Confirm在worldview写入后、content确认前模拟事务失败（实施验收注入点），两域均无提交；并发worldrevision更改得到409，不产生孤立账本。
6. TXT/DOCX预览零正文副作用；20MiB边界、异常ZIP、超长章明确失败；commit重试只有一组新章；取消/成功竞态以job终态为准。
7. 四种导出下载验证格式与固定版本；无报告正常，block未ack拒绝，ack过期拒绝；撤销scope后旧链接404。worker重启、lease过期、FileStore写后崩溃均能按job_id恢复或清理。
8. 北京时间23:59:59与次日00:00:00分别归天；搜索中文与特殊字符、删除隔离、空结果、游标篡改；UI图表不使用静态业务数组。

这些是R1实施验收条款，本轮交付不运行或新增业务测试文件。

单项回收站：资源集合GET的deleted默认false；deleted=true仅原Owner在父作品未删除且30日保留期内列出已软删项目，使用同一实体Schema与deleted_at，并显示restore动作。这不授予读取已删除整书正文/设定的权限；整书回收站仅content Novel元数据，需先恢复整书才访问其子资源。cursor绑定deleted过滤条件。

## 10. 完整HTTP操作与应用方法索引

每行对应Application方法（机读operationId带域前缀）。非GET均要求Idempotency-Key，路径UUID必填，query星号为必填；DELETE使用query.expected_revision，PUT与状态命令使用JSON.expected_revision。成功响应为SuccessMeta+data。列表均使用common Limit/Cursor，类型定义见各域types分片。

| FR/应用用例 | HTTP | 请求Schema / query | 成功data |
|---|---|---|---|
| FR-02 content.listNovels | GET /v1/content/novels | — / status | 200 NovelPage |
| FR-02 content.createNovel | POST /v1/content/novels | CreateNovel / — | 201 Novel |
| FR-02 content.getNovel | GET /v1/content/novels/{novel_id} | — / — | 200 Novel |
| FR-02 content.updateNovel | PUT /v1/content/novels/{novel_id} | UpdateNovel / — | 200 Novel |
| FR-03 content.deleteNovel | DELETE /v1/content/novels/{novel_id} | — / expected_revision* | 200 Novel |
| FR-04 content.listvolumes | GET /v1/content/novels/{novel_id}/volumes | — / deleted | 200 VolumePage |
| FR-04 content.createVolume | POST /v1/content/novels/{novel_id}/volumes | CreateVolume / — | 201 Volume |
| FR-04 content.getVolume | GET /v1/content/volumes/{volume_id} | — / — | 200 Volume |
| FR-04 content.updateVolume | PUT /v1/content/volumes/{volume_id} | UpdateVolume / — | 200 Volume |
| FR-03 content.deleteVolume | DELETE /v1/content/volumes/{volume_id} | — / expected_revision* | 200 Volume |
| FR-04 content.listchapters | GET /v1/content/novels/{novel_id}/chapters | — / deleted | 200 ChapterPage |
| FR-04 content.createChapter | POST /v1/content/novels/{novel_id}/chapters | CreateChapter / — | 201 Chapter |
| FR-04 content.getChapter | GET /v1/content/chapters/{chapter_id} | — / — | 200 Chapter |
| FR-04 content.updateChapter | PUT /v1/content/chapters/{chapter_id} | UpdateChapter / — | 200 Chapter |
| FR-03 content.deleteChapter | DELETE /v1/content/chapters/{chapter_id} | — / expected_revision* | 200 Chapter |
| FR-02 content.archiveNovel | POST /v1/content/novels/{novel_id}/archive | RevisionCommand / — | 200 Novel |
| FR-03 content.restoreNovel | POST /v1/content/novels/{novel_id}/restore | RevisionCommand / — | 200 Novel |
| FR-03 content.restoreVolume | POST /v1/content/volumes/{volume_id}/restore | RevisionCommand / — | 200 Volume |
| FR-03 content.restoreChapter | POST /v1/content/chapters/{chapter_id}/restore | RevisionCommand / — | 200 Chapter |
| FR-04 content.getOrganization | GET /v1/content/novels/{novel_id}/organization | — / — | 200 Organization |
| FR-04 content.reorderOrganization | POST /v1/content/novels/{novel_id}/organization/reorder | Reorder / — | 200 Organization |
| FR-05 content.getBody | GET /v1/content/chapters/{chapter_id}/body | — / — | 200 Body |
| FR-05 content.saveBody | PUT /v1/content/chapters/{chapter_id}/body | SaveBody / — | 200 Body |
| FR-06 content.listVersions | GET /v1/content/chapters/{chapter_id}/versions | — / — | 200 VersionPage |
| FR-06 content.getVersion | GET /v1/content/chapters/{chapter_id}/versions/{version_id} | — / — | 200 Version |
| FR-06 content.diffVersions | GET /v1/content/chapters/{chapter_id}/diff | — / from_version_id*, to_version_id* | 200 Diff |
| FR-06 content.rollbackChapter | POST /v1/content/chapters/{chapter_id}/rollback | Rollback / — | 200 Body |
| FR-07 content.confirmChapter | POST /v1/content/chapters/{chapter_id}/confirm | Confirm / — | 200 Confirmation |
| FR-02 content.copyNovel | POST /v1/content/novels/{novel_id}/copy | CopyNovel / — | 202 Job |
| FR-20 content.search | GET /v1/content/novels/{novel_id}/search | — / q* | 200 SearchPage |
| FR-20 content.statistics | GET /v1/content/novels/{novel_id}/statistics | — / from*, to* | 200 Statistics |
| FR-18 content.createImport | POST /v1/content/novels/{novel_id}/imports | CreateImport / — | 202 Job |
| FR-19 content.createExport | POST /v1/content/novels/{novel_id}/exports | CreateExport / — | 202 Job |
| FR-18 content.listJobs | GET /v1/content/novels/{novel_id}/jobs | — / — | 200 JobPage |
| FR-18 content.getJob | GET /v1/content/jobs/{job_id} | — / — | 200 Job |
| FR-18 content.cancelJob | POST /v1/content/jobs/{job_id}/cancel | RevisionCommand / — | 200 Job |
| FR-18 content.previewImport | GET /v1/content/imports/{job_id}/preview | — / — | 200 ImportPreview |
| FR-18 content.commitImport | POST /v1/content/imports/{job_id}/commit | CommitImport / — | 202 Job |
