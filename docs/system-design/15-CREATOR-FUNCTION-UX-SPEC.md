# 创作者工作台功能与交互合同 R1

状态：INK-141 冻结目标，尚未实现。唯一跨册契约为 [00 基线](00-CONTRACT-BASELINE.md)。共享模块及状态所有权见 [20](20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)，视觉与键盘见 [17](17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)。本文冻结页面路由与交互，接口能力名是应用用例语义，不是额外 HTTP 路由；HTTP 字段与操作以 [API 入口](api/openapi.json) 的对应域 Schema 为准，域适配器完成明确映射。

## 1. 应用边界、身份与数据

Creator 是端口3000、strictPort 的独立 React19 应用，仅 creator audience，必须拥有 creator 成员角色及作品scope。多角色用户仍不能从本端进入管理页面。允许按ADR-0021公开注册creator独立租户；无运维菜单、管理端切换、自动投稿、协同编辑、图片或语音入口。登录 /login 的wire为email或保留用户名root、password、tenant_id、audience=creator；username只映射root且仍需活动creator membership和作品owner scope；email合法邮件格式且≤254字符，tenant_id可省略，提供时为UUIDv4；密码至少6个Unicode字符且≤72 UTF-8字节（ADR-0028），账号字段按身份Schema验证；登录成功access只在内存，HttpOnly刷新session由服务端设置。刷新页面先GET /v1/auth/session-csrf?audience=creator获取nonce，再POST /v1/auth/refresh恢复session后读资源；401单飞刷新一次，失败进入重新登录；刷新和退出附CSRF与合法Origin。/account 显示脱敏用户/租户/会话、改密码和退出；本人改密码PUT /v1/auth/password提交current_password、new_password、expected_revision=principal.user_revision，成功吊销全部会话后重新登录；退出主动清除该主体恢复副本和缓存。切租户须结束当前主体缓存上下文并重新认证，绝不复用旧作品ID。

每个页面使用 PageGate(scope, permission, queryState)，路由参数 UUIDv4，使用 ReactRouter7 loaders 验证session与作品scope，随后 TanStackQuery5读取。无效ID本地错误，合法但不存在/跨scope统一404；不自动改选第一本书。跨域数据由各域查询组合，UI不构造数据库join。页面聚合失败分区呈现，不让一个统计请求阻断正文。

所有列表 limit默认20、1..100，游标随筛选和排序整体重置，默认 updated_at DESC,id DESC。URL只含对象ID、页签、非敏感筛选；全文查询词及正文留组件内存，避免浏览器历史泄露。所有操作展示北京时间，传输 UTC Z；标题trim后1..120码点、显示名1..60、简介0..2000、正文≤200000码点、JSON≤2MiB，超限拒绝不截断。本文字段清单中 id/tenant_id/created_at/updated_at 为所有实体的共同读字段，mutable实体另有 revision；tenant_id来自session，不可表单任填。外域引用为UUID。

## 2. 全页面状态与恢复合同

每个后续页面都完整应用以下八态，页面条目的补充优先于通用展示但不移除任何状态。

|状态|进入条件|渲染与允许动作|恢复验收|
|---|---|---|---|
|loading|无成功快照且查询进行|结构骨架、aria-busy，不显示0/空业务数据|读成功后保持焦点，不自动执行主操作|
|empty|成功且items空|页面专属首个动作和解释|创建后使用返回ID，刷新仍存在|
|error|首次读取失败|错误码、request_id、重试；保留已输入表单|重试仅失败请求，500不得冒充empty|
|offline|网络不可达|持续条；缓存只读，正文可本地编辑；所有远端命令禁用|联网先重读revision再恢复提交，不离线排队危险命令|
|stale|有成功快照但刷新失败/通知新revision|显示最后成功时间与过期标记；表单保留，提交先核对基线|旧revision不能静默覆盖新数据|
|partial|独立分区部分失败|成功区可读，失败区独立重试；关键前置缺失禁相应命令|重试不会重置正文/其他筛选|
|forbidden|403明确权限撤回|清除该scope缓存、取消读取、收起敏感面板|恢复权限后重新PageGate；跨scope404不显示实体名|
|ready|权威读取成功|启用已授权操作|按钮与服务端权限双重校验|

写表单 clean→dirty→validating→submitting→clean；422回dirty并定位字段，409回conflict，网络响应丢失回unknown。所有创建/更新/删除/命令必有UUID Idempotency-Key；PUT/DELETE及状态命令带expected_revision，DELETE放query。unknown同键同载荷重试，不生成新键；409 operation_in_progress按Retry-After核对；同键异载荷拒绝。表单成功以服务器返回的revision与字段为准，不用乐观成功Toast代替落库。删除影响对话框默认焦点“取消”，取消不发请求。

## 3. 页面清单与逐页合同

以下能力采用 Domain.Method 命名；每页“追踪”将功能→组件→接口能力→权威Owner→验收直接串联。所有主次操作除特别声明均要求creator及当前作品scope。检索、报告、建议不会直接写正文。

### C01 书架 /books 与开书 /books/new

字段：Novel 的title、summary、category、target_words、status、revision、updated_at；列表显示书名、分类、当前字数和更新时间。title1..120码点、summary0..2000、category0..60、target_words为0..100000000整数，四字段均按CreateNovel提交；status为active/archived/deleted，不用“已发布”替代。
组件：BookGrid + FilterBar + SchemaForm + ResourceActions。主：创建、继续写作；次：修改元数据、复制作品、归档/恢复活跃、软删除。复制弹窗提交expected_revision/title/include_planning/include_worldview，POST novels/{novel_id}/copy返回202 Job；按job_id读取直至succeeded，再使用target_novel_id进入新作品；复制正文/结构快照不复制运行审批、凭据、原作品所有权或既有确认事实，生成草稿并重新确认。创建返回novel_id后进入该作品结构页；空书架提供开书；无章节时进入新建章而非空白伪章节。离线可查看已缓存书名，不提交开书；统计partial不隐藏卡片。删除30天内去回收站，明确撤销scope及下属文件影响。
追踪 C01 / FR-02：BookGrid→Content.ListNovels/CreateNovel/UpdateNovel/ArchiveNovel/DeleteNovel→content；验收标题空/121码点拒绝，双击创建一次，同键重放同ID，第二本继续绝不进入第一本，列表500保留error。

### C02 结构 /books/:novelId/structure

字段：Volume(title,summary,position,revision)，Chapter(volume_id可空,title,position,status,word_count,revision)，Scene摘要从outline读取。position从0开始；每作品最多100卷/2000章，卷position0..99、章0..1999。GET organization返回novel_revision/volumes/chapters；POST organization/reorder提交expected_revision=novel_revision、volume_ids及完整chapters[{chapter_id,volume_id,position}]，同时完成归卷/重排，重复/跨作品ID拒绝。卷删除保留章节并移至未分卷；章删除独立确认。组件StructureTree + NodeForm + SceneLinkList。主新建章节，次新建卷、重命名、移动、排序、删除。键盘上移/下移/移入卷与拖拽等效。空树只有未分卷结构说明；outline失败partial仅禁场景联动；409重载并保留拟定顺序。
追踪 C02 / FR-04：StructureTree→Content.GetOrganization/CreateVolume/CreateChapter/ReorderOrganization/DeleteVolume/DeleteChapter；SceneLinkList→Outline.ListScenes→各域Owner；验收两标签同时排序后一请求409、无半树提交，刷新顺序一致，跨作品章不可拖入。

### C03 写作 /books/:novelId/chapters/:chapterId

字段：GET /v1/content/chapters/{chapter_id}/body返回Body{chapter,body}；chapter含title/status/revision/content_hash/word_count/current_version_id/confirmed_version_id。正文仅原生textarea。结构树240、正文640..800、检查器320，按17降栏。组件ChapterPage→EditorController→PlainTextEditor，并挂SelectionToolbar、SaveIndicator、ContextPanel、RunInspector、RecoveryBanner。主保存，次AI建议、版本、确认事实、专注。统一保存协调器先PUT chapters/{id}提交title与expected_revision，再用其返回revision PUT chapters/{id}/body提交body；每步独立幂等键，只有发生变化的部分才提交。同章共用串行锁，标题已存而正文失败时显示“标题已保存，正文未保存”，正文持续dirty；不宣称两个命令同事务。仅正文保存按content版本规则产生chapter_versions；confirmed正文修改后回draft。404保留可下载本地稿，不自动转写下一章。
空正文是有效草稿，不显示演示文本。context partial不阻止手写；正文GET失败禁止以空字符串初始化保存。offline继续编辑并持续显示本地保护状态；stale进入三方比较；forbidden撤除服务器正文，仅把正在编辑的本人未提交稿保留为显式导出操作，不再联网写入。正文与异步回包绑定tenant/user/novel/chapter/generation防错章。
追踪 C03 / FR-05、FR-12：EditorController→Content.GetBody/UpdateChapter/SaveBody→content；验收中文输入法、emoji作为用户文本的代理对、组合字符、换行往返不破坏码点定位，网络500不显示已保存，A/B CAS冲突不覆盖，具体保存及恢复规则见4。UI自身不使用emoji图标。

### C04 版本 /books/:novelId/chapters/:chapterId/versions

字段：Version(id,chapter_id,chapter_revision,title,body,content_hash,word_count,source,restored_from_version_id,created_at)，source=create|save|rollback|import|copy；当前chapter revision独立显示。GET diff提交from_version_id/to_version_id，返回offset_unit=unicode_code_point、hunks与coarse，coarse时标粗粒度，不宣称逐字精确。组件VersionList+DiffView+SourceBadge。主比较，次rollback以当前expected_revision和version_id提交，新版本draft且旧事实不撤销。empty无历史；一侧失败partial禁比较/回滚，不显示全文删除；离线只读已缓存快照；stale提示当前正文已变更。
追踪 C04 / FR-06：DiffView→Content.ListVersions/GetVersion/RollbackChapter→content；验收A→B→回滚A生成第三个版本、revision递增、事实账本原记录仍可追溯，409保留比较。

### C05 三级大纲与场景 /books/:novelId/outline

字段：Node(parent_id,level,target_id,title,summary,position,status,revision)，level=master|volume|chapter，status=draft|ready|archived，summary≤20000，position从0开始；Scene(node_id,title,summary,pov_entity_id,location_entity_id,position,status,source_revision,source_hash,start_offset,end_offset)，summary≤2000、status=planned|written；Beat(node_id,title,description,beat_type,hook_type,tension,expected_words,position,status)，description≤2000、tension0..100、expected_words0..200000。beat_type=setup|conflict|turn|climax|resolution，hook_type=none|danger|revelation|emotion。组件OutlineTree+OutlineForm+SceneBoard+RhythmChart与等价表。主保存，次新增下级、同父重排、场景/节拍编辑、AI扩展；节点不提供改变parent_id的任意移动。POST nodes/{node_id}/reorder携resource=nodes|beats|scenes、ordered_ids与expected_revision。父子必须符合三级且同作品；场景锚点码点区间与source_hash同源。空树引导总纲，曲线partial不阻止节点保存；AI产物逐字段接受后调用outline写能力。
追踪 C05 / FR-08：OutlineTree/SceneBoard→Outline.ListNodes/SaveNode/ReorderNodes/ListScenes/SaveScene/SaveBeat；生成→Agent.CreateRun→outline/agent各自Owner；验收四级节点、环路和异作品父引用422，场景保存不修改正文，建议采纳409要求重新比较。

### C06 伏笔 /books/:novelId/foreshadowing

字段：Clue(title,description,status,planted_node_id,planned_reveal_node_id,last_node_id,reason,revision)及ClueEvent。状态planned→planted→developing→revealed→closed，另有abandoned；页面动作明确计划、埋设、推进、揭示、关闭、放弃，合法转换以服务端状态机裁决，不能任意下拉跳态。transition提交expected_revision/to_status/node_id/reason，引用大纲节点UUID，不伪装chapter_id；reason按Schema限制，需理由的动作不得空。组件ForeshadowingBoard+Timeline+TransitionDialog。主推进状态，次创建/编辑计划、查看节点/事件、筛选；empty引导创建，来源不可读partial仅显示安全受限摘要，stale先重读。
追踪 C06 / FR-09：TransitionDialog→Outline.ListClues/CreateClue/UpdateClue/TransitionClue/ListClueEvents/GetPacing→outline；验收未埋设直接回收被拒、并发回收只一条成功、历史仍可读。

### C07 角色 /books/:novelId/characters

字段：Character(name,description,aliases,role_type,life_status,realm_rank,status,revision)。name1..60、description≤20000、aliases最多20；role_type=protagonist|supporting|antagonist|other，status=active|archived。life_status=alive|missing|dead|revived与realm_rank0..10000只读来自事实投影，普通档案表单不能直接编辑，事实更正走C10。组件CharacterList+CharacterForm+RelatedFactsPanel。主创建/保存，次归档、查看关系/事实、软删/恢复。删除保留事实来源；关系/事实列表仅辅助显示已读引用，不声称全量无引用，最终删除由服务端约束裁决。关联partial不阻止手工编辑；失败保留输入。
追踪 C07 / FR-10：CharacterForm→Worldview.ListCharacters/CreateCharacter/UpdateCharacter/DeleteCharacter→worldview；验收重名不以客户端名字当ID去重，角色改名关系仍引用原UUID，越权404，失败保留表单。

### C08 设定 /books/:novelId/world

字段：Lore(category,title,description,status,revision)，category=faction|place|system|item|history|other，description≤20000、status=active|archived。组件EntryList+SchemaForm+ReferencePanel。主保存，次分类筛选、归档、软删/恢复。ReferencePanel组合已有relations/facts列表显示已读关联，明确分页范围；没有单独GetReferences接口，缺失/partial不能声称无人引用。删除确认保留“服务端将再次检查引用”，最终409约束拒绝保留实体，不因辅助列表未全量而伪造级联能力。
追踪 C08 / FR-10：SchemaForm→Worldview.ListLore/CreateLore/UpdateLore/DeleteLore/RestoreLore与ListRelations/ListFacts→worldview；验收引用冲突显示冲突对象安全摘要，409重读比对，失败不清空描述。

### C09 图谱 /books/:novelId/graph

字段：Relation(subject_id,predicate,object_id,description,weight,status,revision)，predicate1..60、description≤2000、weight0..100，status=active|retired；端点同作品UUID。GET graph返回nodes/edges/truncated/world_revision，最多500节点/1000边；truncated=true明确提示局部图，关系表独立游标分页查全。组件RelationGraph(SVG/Canvas)+RelationTable+RelationForm同一选中ID；主建立关系，次编辑/退休/删除/恢复，图布局仅本地偏好。empty无关系不造节点；节点partial不猜正文；offline禁写；删除409不移除服务器未删边。
追踪 C09 / FR-10：RelationForm→Worldview.ListRelations/CreateRelation/UpdateRelation/DeleteRelation→worldview；验收键盘表格完成图上全部写操作，大图分页不宣称全量，删除CAS冲突不移除服务器未删边。

### C10 事实与确认 /books/:novelId/facts

字段：FactEvent(subject_id,predicate,fact_key,value,action,target_event_id,resolution,reason,source_chapter_id,source_version_id,source_content_hash,world_revision,seq,payload_erased)；action=assert|supersede|retract，resolution=none|correction|retcon；world_revision独立于chapter revision。组件FactLedger+FactProposalReview+ConflictResolution+ConfirmationSummary。主Content.ConfirmChapter，次Worldview.CheckFacts、SupersedeFact、RetractFact；原记录无编辑/删除。确认expected_revision/expected_world_revision/fact_proposals最多100条；逐项resolution/reason，无全局override。手工修正同时携fact head expected_revision和expected_world_revision，reason1..1000。payload_erased只显示去标识证明，不推测被擦正文。
确认前必须成功读取正文来源版本、当前世界revision及所有待审提案；任何partial/stale禁确认。离线只读本地摘要，不能入账。FactProposalReview由content页面组合，content应用调用WorldviewFactPort，同一UoW提交；UI只有一个ConfirmChapter命令，不先写正文再串行写账本。
追踪 C10 / FR-07、FR-11：FactProposalReview→Content.ConfirmChapter；FactLedger→Worldview.ListFactEvents/CheckFacts/SupersedeFact/RetractFact→content/worldview；验收故障整批回滚无“已确认但未入账”，409保留提案并刷新世界revision，回滚正文不撤事实。

### C11 拆书与灵感 /books/:novelId/inspiration

页签资料、分析、文风、灵感卡；深链tab=assets|analyses|styles|cards。Asset(title,source_kind,file_id,author,source_url,source_note,rights_status,state,revision)，source_kind=file|paste，paste_text1..200000、source_note1..2000、rights_status=owned|licensed|public_domain|unknown。Analysis(asset_id,asset_revision,kind,brief,state,run_id,budget,usage)，kind=deconstruction|style|ideas，brief≤4000，state=pending|running|succeeded|failed|cancelled；报告单独GET analyses/{id}/report。Style(title,current_version_id,state)，StyleVersion(instructions,metrics,source_report_id,version_no)，instructions1..4000；metrics类型化mean_sentence_length/dialogue_ratio/sensory_bias/instructions。Card(title,category,body,rationale,source_kind,source_report_id,evidence,state,revision)，body1..4000、rationale≤2000、evidence最多20，category=opening|conflict|twist|cliffhanger|setting，state=suggested|archived|deleted。组件FilePicker+AssetList+AnalysisPanel+StyleProfile+StyleVersions+InspirationCard+AdoptionHistory。主导入/粘贴资料与发起分析；次创建/更新/删除恢复文风、比较不可变文风版本、灵感卡CRUD/归档/恢复、采纳与撤销采纳记录。输入txt/docx≤20MiB，真实内容再验；上传成功不等于领域资料就绪。
卡片采纳先由content/outline目标写接口成功保存，随后POST cards/{card_id}/adoptions记录expected_revision/target_domain/target_id/target_revision/target_hash；登记失败只重试登记，不重复写目标。撤销adoption提交理由，只改变记录state=revoked，不撤销目标正文。文风更新产生StyleVersion，历史比较使用version_id；卡片来源evidence保留asset/segment码点锚点和source_unavailable。资料empty引导上传；分析error保留原文件与运行详情；partial失败模块独立重试；源文件删除/版本过期后标stale并禁止当作新事实。先POST /v1/inspiration/analyses创建pending分析记录（201），再POST /v1/agents/runs携task_kind=inspiration_analyze与resource={resource_type:analysis,resource_id:analysis_id,source_revision:analysis.revision}（202）；所有AI分析经Agent.CreateRun，不直接POST长流；完成产物归inspiration，采纳到正文必须经过EditorController与Content.SaveBody。
追踪 C11 / FR-13、FR-14：FilePicker→Platform.UploadFile；AnalysisPanel→Inspiration.CreateAsset/ListAssets/CreateAnalysis/GetAnalysis/GetReport/ListStyles/CreateStyle/UpdateStyle/ListStyleVersions/ListCards/CreateCard/UpdateCard/SetCardState/RecordAdoption/RevokeAdoption + Agent.CreateRun→platform/inspiration/agent；验收上传响应丢失同键不重复文件、分析失败可核对原run、卡片采纳不写worldview。

### C12 质检 /books/:novelId/quality

字段：Report(chapter_id,chapter_revision,content_hash,rule_set_id,mode,state,summary,ai_flavor_score,risk_count,open_count,block_count)，mode=rules|deep，state=pending|running|succeeded|failed|cancelled。问题通过GET reports/{report_id}/issues独立游标分页返回Issue(kind,severity,start_offset,end_offset,message,suggestion,state,revision)，state=open|ignored；message为领域文本按纯文本渲染，不是错误message_key。组件ReportList+FindingsTable+IssueDecisionHistory+ReviewForm+SourceAnchor+DiffView。主规则扫描POST scans或深度检查；次定位、ignore/reopen（expected_revision+reason1..500）、审阅、创建净化。扫描及深度检查都绑定已保存chapter_revision/content_hash并提交text，正文≤200000。深度POST reports先建pending再Agent.CreateRun(task_kind=compliance_audit,resource={resource_type:report,resource_id:report_id,source_revision:report.revision})。净化POST rewrites提交report_id/expected_revision/selected_issue_ids（1..100）/max_rounds（1..3）/budget，再CreateRun(task_kind=compliance_rewrite,resource={resource_type:rewrite,resource_id:rewrite_id,source_revision:rewrite.revision})；读取Rewrite.patches/candidate_text/candidate_hash，作者接受才进入draft。ReviewForm提交expected_revision/acknowledged_risk/note≤1000到reports/{id}/reviews，Review不可变绑定报告/章节/规则修订，供导出review_ticket使用；报告完成或已有review不自动把chapter设confirmed。
empty引导检查；运行中真实状态，不倒计时造进度；单报告失败error；建议基线失配stale允许阅读但禁直接覆盖，重新定位后人工确认合并；规则/模型区partial不宣称“全部通过”。
追踪 C12 / FR-15：ReportList→Compliance.ListReports/GetReport/ListIssues/ListIssueDecisions/ListReviews；规则→Compliance.Scan；深检→Compliance.CreateReport→Agent.CreateRun；净化→Compliance.CreateRewrite→Agent.CreateRun；审阅→Compliance.CreateReview；采纳→EditorController→Content.SaveBody→各自Owner；验收原文变更后旧建议无法直接替换重复句，报告完成不意味着章节confirmed。

### C13 工作流 /books/:novelId/workflows

字段：Creator目录GET /v1/agents/catalog/workflows、/catalog/definitions及/catalog/task-policies均只返回当前tenant可用安全投影，不读Admin Workflow.manifest或system_instruction。Creator分别选择Definition、task_kind、不可变task_policy_version_id与可选Workflow。CreateRun七个创作分支必填task_kind/task_policy_version_id/resource/instruction/definition_id，workflow_id按分支可选；Creator不提交task_key，服务端从版本派生。resource保留source revision/hash/selection绑定。策略目录失败或空时禁运行；策略变更409保留输入并刷新重选。旧task_key仅兼容升级窗口服务端fail-closed解析，随后关闭，不做代码映射。
empty为无可用工作流，不能伪造默认模板；schema加载失败禁启动；预算partial禁模型运行但保留输入；429显示retry_after_seconds，503 capacity_unavailable保留手工编辑。
追踪 C13 / FR-16：SchemaForm→Agent.CatalogWorkflows/CatalogDefinitions/CreateRun→agent；验收一个POST持久化run后再订阅，刷新页只读取不重启，离页继续运行。

### C13 创建运行与安全策略选择（R1.x）

Run表单分别从Creator Definition、Workflow与task-policy安全目录读取动态选项；目录获取失败/为空时禁止创建，不回退静态列表。task_kind必须属于definition版本的task_kinds且同时被策略版本支持。提交七个创作分支之一的CreateRun请求时必须带`task_policy_version_id`，不得提交`task_key`；服务端返回冻结`policy_version_id`与派生`task_key`。策略在目录读取后更新/停用时409 `agent.task_policy_version_changed`，保留用户输入并刷新后要求重新确认。旧task_key仅滚动升级期间由服务端当前tenant启用目录兼容解析，客户端升级后sunset拒绝旧请求；不得按task_kind或角色硬映射。Admin diagnostic维持独立受控入口。

### C14 Agent中途澄清

Run/Node状态增加`waiting_clarification`，区别于只处理工具副作用的`waiting_approval`。Creator RunInspector展示题目title/help locale key、选项或文本字段、context摘要与服务器期限；渲染不支持HTML。schema仅安全单选/多选/文本，question≤8KiB/深度8，题面≤4KiB/20选项/答案≤4KiB；无可验证schema或权限字段时拒绝，绝不按模型自由文本当命令。

回答发送`POST /v1/agents/runs/{run_id}/questions/{question_id}/answer`，Idempotency-Key及`expected_revision`、`question_version`、schema验证后的answer必需。只有发起Run本人仍持作品scope时可答；单题CAS成功一次，重放同键回原答复，别键并发返回question_already_answered。答案是untrusted data，不覆盖正文/事实/源版本。事件仅放answer_hash而GET Question恢复时仅本人可读。

退出/刷新/断线不取消Run。重新进入先GET Run/Question再带Last-Event-ID重连；回答原question会恢复同一Run/node/checkpoint/job，不POST第二个Run，不重放已完成工具副作用。Question过期（1..15分钟且不晚于Run deadline）、来源变化或权限撤销不得继续；服务端到期写run.failed，撤权立即拒绝读取/回答并清敏感缓存。取消/答复/到期竞争按PG锁序+CAS；等待计入Run 30分钟deadline且不显示伪造进度。


字段：Run(id,status,revision,created_at,owner_user_id,task_kind,task_key,resource,nodes,policy_version_id,definition_version_id,workflow_version_id,error_code)，来源基线在resource.source_revision/source_hash；usage只来自usage.updated事件。Node含node_key/role/status/phase/attempt；Artifact含kind/target_resource_id/source_revision/content_hash/text_content/file_id/is_partial，is_partial禁止当完整草稿直接采纳。组件RunList+RunInspector+ArtifactReview。主查看/接受完整草稿，次显式cancel与复制错误标识；queued→running→waiting_approval→running，running可succeeded/failed/cancelling→cancelled；终态不可重开。重新运行必须用户新意图、新run/幂等键。
事件GET断流是连接状态，不改run为failed。重连Last-Event-ID、seq去重；410先GET run权威快照，告知历史过期；离页只取消订阅，后台继续。原始隐藏推理不呈现，只展示reasoning.summary。产物接受前核对章节/来源修订，草稿进入EditorController而非后台写正文。
追踪 C14 / FR-17：RunInspector→POST /v1/agents/runs（202）+ GET /v1/agents/runs/{run_id}/events + Agent.GetRun/CancelRun/GetArtifacts→agent；验收断网重订阅无第二次模型调用、乱序重复seq不重复追加、取消与成功竞争以终态CAS为准。

### C15 我的审批 /approvals

字段：Approval(id,run_id,node_id,owner_user_id,tool_version_id,arguments_hash,arguments_json,source_revision,expires_at,status,revision)及脱敏参数摘要。组件 ApprovalList + ApprovalCard + RunInspector。主本人批准，次本人拒绝/查看影响；按钮写明工具、副作用与目标，15分钟有效，剩余时间仅辅助，服务端时间决定。仅发起run本人且仍有作品权限可决定；operator/auditor角色不提供代批。approve/reject携expected_revision、arguments_hash、source_revision和幂等键；批准status=consumed、拒绝rejected，另有expired/cancelled。恢复原run不创建新run。
empty为无待审批；详情partial缺参数摘要禁批；offline禁批；stale重新读取票据；过期收起按钮显示run失败原因，拒绝不伪装成功执行。
追踪 C15 / FR-17：ApprovalCard→Agent.ListApprovals/GetApproval/ApproveApproval/RejectApproval→agent；验收另一主体403/404、双标签只能消费一次、批准后相同run_id继续且已完成副作用不重跑。

### C16 文件导入导出 /books/:novelId/files

字段：File(id,filename,media_type,purpose,size_bytes,sha256,revision,expires_at,deleted_at)，Job(id,kind,status,stage,progress,source_file_id,output_file_id,source_revision,target_novel_id,expires_at,review_ticket)；kind=import|export|copy，status=queued|running|preview_ready|committing|cancelling|succeeded|failed|cancelled|expired。组件FilePicker+ImportPreview+JobList+DownloadAction。主导入/导出，次按job详情查看进度、取消、下载或删除未绑定文件。导入txt/docx≤20MiB，POST novels/{id}/imports携expected_revision/file_id/format；preview_ready后GET imports/{job_id}/preview分页，勾选1..2000项，commit携job expected_revision、expected_novel_revision、volume_id、selected_item_ids。导出txt/markdown/docx/epub，提交expected_revision/format/volume_ids/include_drafts/review_ticket；review_ticket取已完成且基线仍匹配的质检审阅，过期或不匹配必须重新审阅。输出≤100MiB，从Job.output_file_id鉴权下载。文件multipart/二进制，不Base64 JSON或外链；解析不持DB事务，commit原子批次失败不假装部分成功。
empty给文件类型说明；解析中关闭页面后台继续；下载error允许按file_id重新下载；产物过期stale需新导出，不能把旧版本当最新。导出采用content快照，不锁住写作；创建后编辑不改变已有产物。
追踪 C16 / FR-18、FR-19：FilePicker→Platform.UploadFile/DownloadFile；ImportPreview→Content.CreateImport/PreviewImport/CommitImport/CreateExport/ListJobs/GetJob/CancelJob→platform/content；验收21MiB前后端均拒、导出标题显示快照修订、取消下载不删除服务器产物，未授权文件404。

### C17 作品内检索 /books/:novelId/search

字段：q（2..100码点），SearchHit(chapter_id,version_id,title,content_hash,chapter_revision,snippet,start_offset,end_offset)；查询词不入URL、不遥测。组件SearchBox+SearchResults+SourceAnchor，主搜索，次跳转定位；300ms防抖只优化GET，不缓存他租户结果。仅检索当前作品；empty真实无匹配，error保留输入，stale定位先校chapter_revision/content_hash，失配重查，不按重复字符串猜位置。
追踪 C17 / FR-20：SearchResults→Content.SearchNovel→content；验收同词两处定位按码点区间、修订变化标过期、跨作品过滤不可绕过scope。

### C18 写作统计 /books/:novelId/statistics

字段：Statistics(novel_id,from,to,days,current_word_count)，DailyStat(date,added_words,removed_words,net_words,save_count,timezone)，timezone=Asia/Shanghai；无confirmed_chapters字段，不虚构确认计数。组件DateRangeFilter+MetricCards+WritingChart+AccessibleTable。主查询，次查看每日明细；from≤to，窗口≤366天，传参按Schema日期格式。仅统计已保存事实，输入区字数另标本地未保存；empty无记录，partial未知值不填0，offline显示快照时间。
追踪 C18 / FR-20：WritingChart→Content.GetWritingStatistics→content；验收北京时间23:59/00:00分日正确、回滚产生真实增减不充当新增创作、无数据不显示虚构连续天数。

### C19 作品设置 /books/:novelId/settings 与回收站 /recycle-bin

字段：Novel元数据及revision、deleted_at；RecycleItem为Novel列表视图，不新增wire实体。purgeAt仅UI按deleted_at加30天推算提示，服务端到期与恢复结果为准，不发送purge_at字段。组件 NovelSettings + DangerZone + RecycleList。主保存元数据/恢复，次归档、删除；30天内恢复同ID新revision，到期正文/文件擦除，不能承诺永久恢复。写作目标target_words为0..100000000整数，summary≤2000、category≤60，不塞入summary模拟字段。恢复前确认依赖与scope，冲突显示具体安全原因。
empty无已删作品；partial影响清单缺失禁删除；stale到期倒计时不得覆盖服务器判定；forbidden清除回收列表。
追踪 C19 / FR-03：DangerZone/RecycleList→Content.UpdateNovel/DeleteNovel/ListDeletedNovels/RestoreNovel→content；验收删除后原深链不可写、恢复同ID且revision增加、超期不可恢复，保留事实仅为去标识化审计证明。

## 4. 编辑器控制器冻结算法

### 4.1 状态与保存

EditorController持有base={title,body,revision,content_hash}、draft、titleDirty/bodyDirty、selection、edit_generation、save_status、inflightPayload、pendingLatest、localRecoveryStatus；TanStackQuery持有服务器快照，二者不能双向无限同步。初始化仅一次；后续服务器更新进入stale比较，不用useEffect覆盖dirty。

保存状态clean→dirty→saving_metadata→saving_body→clean，未改标题跳过metadata、未改正文跳过body；保存期间新输入保持对应dirty。提交冻结draftSnapshot和base revision。metadata PUT成功先更新title基线及revision，body PUT使用该revision与独立幂等键；正文失败保留bodyDirty且显示部分保存，不回滚已成功标题。仅body成功按content规则创建版本；metadata-only不宣称新增正文版本。单章最多一个在途写，后续保存合并为pendingLatest，前一响应只更新base，不能覆盖更晚draft。30秒停输自动保存，持续输入最长60秒触发；Ctrl/Cmd+S立即请求但不并发。IME composition期间只更新输入，不格式化、不保存半个组合；compositionend后恢复计时。服务端返回revision作为下一次CAS，不自行+1冒充确认。

409 version_conflict停止自动提交并保留base/local/remote三份，DiffView分段展示三方差异；用户明确接受合并后以最新remote revision提交新键。401暂停保存、保留当前稿并认证；认证主体变化不得恢复自动保存。超时unknown固定原键/载荷重放核对，成功后才提交pendingLatest；不能根据正文相同猜成功。

### 4.2 本地恢复与容量

IndexedDB数据库按版本迁移；key=(tenant_id,user_id,novel_id,chapter_id)，记录recovery_id、base_revision、base_hash、draft、saved_at、checksum。停输2秒写最新副本；每章最多3份，7天TTL，每主体100MiB软上限。超过软上限先清过期，再按saved_at淘汰未打开章的最旧备份；当前章最新未同步稿不静默淘汰，仍无空间则停止本地写、显示持续告警和下载纯文本。写入失败与IndexedDB不可用相同处理，远端自动保存继续但不宣称本地保护成功。

进入章比较恢复副本与服务器base；相同可忽略，较新或异基线展示“查看/恢复到编辑区/下载/丢弃”。恢复只改draft，不自动远端覆盖。shared-device默认关闭恢复，用户开启须明确知晓本地留存；退出清除此主体全部副本。关闭页尽力触发本地flush，beforeunload只能提示，不保证断电最后2秒。多标签通过BroadcastChannel发送chapter revision和保存状态摘要、不发token/正文；服务端CAS仍为唯一并发裁决。

### 4.3 选区与AI采纳

UI选区使用textarea selectionStart/End UTF16索引；发请求前以Array.from(text.slice(0,index)).length转换码点，若索引切代理对则收敛至合法边界。锚点为{chapter_id,source_revision,content_hash,start_offset,end_offset,selected_text_hash}，[start,end)；源稿必须先成功保存才启动依赖正文的AI。映射回textarea时累计每码点UTF16长度，组合字符不改变码点定义。

选区动作：续写/扩写以task_kind=chapter_draft、改写以chapter_rewrite，检查进入C12 scans或compliance_audit；不发送未定义task_kind。候选产物留在右侧。接受替换或插入时再次验证chapter/source_revision/hash与选区hash；正文有变化则禁直接应用并打开DiffView人工合并。不能使用当前可变光标替代请求时锚点。应用只更新draft，显式保存/自动保存才创建content版本；撤销一次恢复采纳前draft及选区。格式化LF、缩进调整必须预览且可撤销，不静默trim正文。

## 5. 事务、查询与失败验收边界

前端不实现事务或写表。Content.SaveBody负责正文+chapter_versions+幂等+outbox原子提交；ConfirmChapter经WorldviewFactPort同UoW遵守scope→novel→chapter→worldview_revision锁序；agent队列和审批CAS由agent负责。前端失败补偿是核对权威状态、同键重放和保留草稿，绝不发送反向删除“补偿”未知成功的写入。

页面查询要求Owner仓储提供scope过滤和游标排序索引：作品(tenant_id,owner_id,updated_at,id)，卷章(tenant_id,novel_id,position,id)，版本(tenant_id,chapter_id,created_at,id)，事件(tenant_id,run_id,seq)，各建议/报告(tenant_id,novel_id,source_revision,updated_at,id)，全文使用作品scope内全文/pg_trgm查询。此为读性能验收要求，实际表名/索引以data分册为准，UI不另建权威缓存表。

验收记录按C01..C19逐页同时覆盖八态、字段边界、成功后重新GET、跨租户404、同键重放、过期revision409、权限撤回清缓存、键盘与读屏。特别矩阵：2个主体×2个作品×2个标签；网络断在提交前/提交后响应前/SSE中；Redis故障仅AI admission503而非手写CRUD失败；PG失败显示不可写；7天事件过期恢复run快照；2秒恢复窗口、30/60秒保存、100MiB配额均实测时间与存储结果。本册定义实施验收要求，不声称已经执行。
