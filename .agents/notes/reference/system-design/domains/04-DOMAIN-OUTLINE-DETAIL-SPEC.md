> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 04 大纲域详细设计：R1 冻结实施契约

状态：INK-141 冻结目标，尚未实现。遵守[基线](../00-CONTRACT-BASELINE.md)，概览见[04](../04-DOMAIN-OUTLINE-SPEC.md)，DDL见[outline.sql](../../../../../docs/system-design/data/outline.sql)，机读契约见[outline](../../../../../docs/system-design/api/outline.json)、[planning](../../../../../docs/system-design/api/outline-planning.json)、[clues](../../../../../docs/system-design/api/outline-clues.json)、[类型](../../../../../docs/system-design/api/outline-types.json)。

## 1. 应用服务与Port

所有HTTP操作由OutlineApplication同名方法处理，方法名为operationId去掉outline_，签名(ctx, typed request)→typed data；末尾逐项列出。ctx固定tenant/actor/session/audience/request；仓储方法必须显式接收Scope。领域对象不依赖Gin/GORM，不把其他域DTO当可写实体。

| 能力/方法 | 输入→输出 | 调用者与职责 |
|---|---|---|
| NodeService.Create/Update/Delete/Restore | CreateNode/UpdateNode/RevisionCommand→Node | 本域HTTP；定义严格三层并锁master，不读取正文 |
| PlanningService.Create/Update/Delete/RestoreBeat/Scene | typed Beat/Scene请求→实体 | 本域HTTP；分开节拍与场景，不写chapter_versions |
| PlanningService.Reorder | parent node_id,expected_revision,resource,ordered_ids→OrderResult | UI拖拽/作者接受建议；一次只重排一组兄弟 |
| ClueService.Transition | clue,expected_revision,to_status,node_id,reason→Clue | 本域HTTP；状态变更与ClueEvent同事务 |
| OutlineReadService.ReadContext | Scope,novel,chapter UUID集合→Nodes/Beats/Scenes/Clues/Pacing | content.OutlinePort或agent.OutlineReadPort；允许按预算裁剪但返回truncated与读取revision |
| SnapshotService.CaptureSnapshot | UoW(read-only),Scope,novel→OutlineSnapshot | content worker经app；只查询本域表，返回类型化内存值，无外部网络 |
| SnapshotService.CloneSnapshot | UoW,Scope,target_novel,IDMap,OutlineSnapshot→IDMap | content复制最终事务；重置计划状态、映射本域与跨域ID |
| ScopePort.AssertResource | tenant,actor,novel,resource_kind,id→授权父链 | platform.ScopeAuthorizer；target、POV、地点按授权投影验证，不导入content/worldview |
| ScopePort.AssertResourceRevision | tenant,actor,target_kind=chapter,target_id,revision,hash→SnapshotScope | platform；验证历史hash不是任意客户端声称值，正文内容不出平台 |
| Clock/UUID/Idempotency/PGUoW/Outbox | 平台基础值与技术能力 | app装配；写入与幂等/outbox同事务 |

本域没有LLM、业务队列或Agent反向调用。全书复制是content作业；平台JobScheduler调度content worker，本域Snapshot/Clone只作短事务调用，前后都校验当前scope。CaptureSnapshot输出由消费者在事务外交FileStore保存，重试复用job_id对应文件；本域不持有下载URL或外部对象存储。

## 2. 实体字段与容量

所有表有UUIDv4 id、tenant_id→platform_tenants(id)、UTC timestamps；除clue_events外都有revision。Node/Beat/Scene/Clue响应与同名SQL列逐字段一致；active_position生成列只为延迟唯一约束，不暴露给客户端。deleted_at不替代状态枚举，正常GET排除已删除。

Node：novel_id、parent_id、level、target_id、title、summary、position、status、content_hash。title1..120，summary≤20000。master一书一个；其target=novel，position=0。volume子节点最多100；chapter子节点总计最多2000。volume父为master，chapter父为volume；scope验证chapter的当前volume与其volume节点target一致。普通PUT不改parent/level/target，不允许绕过绑定检查重挂树。

Beat：node_id必须chapter；title1..120、description≤2000、beat_type=setup/conflict/turn/climax/resolution，hook_type=none/danger/revelation/emotion，tension整数0..100，expected_words0..200000，position0..199；status=planned/written。每章≤200个未删Beat。

Scene：node_id必须chapter；title1..120、summary≤2000；pov_entity_id和location_entity_id可空但非空需平台验证同书entity；position0..199，status=planned/written。source_revision/source_hash/start_offset/end_offset四项全空表示未绑定正文；全非空则0≤start≤end≤200000，hash为64小写hex。AssertResourceRevision验证node.target_id的revision/hash确为存在的内容快照；选区长度越界由调用方正文控制器校验并由平台SnapshotScope返回的length约束复核，不能只信客户端200000上界。

Clue：title1..120、description≤2000，planted_node_id/planned_reveal_node_id/last_node_id都引用同书chapter Node；reason可空或1..1000。初始planned；planned_reveal_node_id可空，不用虚假的数字“第50章”引用会变化的章节。ClueEvent保存from_status/to_status/node_id/reason/actor_id/clue_revision，事件唯一(clue_id,clue_revision)，只追加。

Node.content_hash规范：以RFC8785 JCS规范化对象 {key: target_id小写UUID, level, title, content: summary, links: {parent_id: 小写UUID或null, novel_id: 小写UUID}} 后取UTF-8字节SHA256。数组不参与该对象，字符串不额外Unicode归一化，summary只将CRLF转LF。status/position/revision不在语义hash中，但每次成功revision变化仍写platform_resource_revisions(target_kind=outline_node,resource_id,revision,hash,content_length=summary全部码点数，含空白)，与节点更新同PGUoW。Outline ScopeProjectionPort.AppendRevision(UoW,target_kind,target_id,revision,hash,codepoint_length)强制传codepoint_length且平台不提供DEFAULT 0，只有空summary为0；它与语义hash在同UoW绑定。读取Node直接返回存储content_hash，灵感采纳使用platform.AssertResourceRevision核验，而不是重算旧节点当前文本。

## 3. FR-08 三级大纲用例

创建master：Scope验证作品可写→Idempotency Begin→以tenant+novel事务advisory lock防止空表并发初始化→检查无有效master→插Node(master,revision1)→outbox→幂等Complete。创建volume/chapter：先校验target归属，再锁master与parent；父状态不得archived/deleted；验证level相邻、目标唯一及数量→position尾插→parent和master revision各+1（同一行一次）→新Node+scope投影→commit。同一资源被并发创建由唯一索引转409 outline.invalid_state。

UpdateNode用expected_revision校验目标，自身与master必要时+1。draft→ready要求summary非空；ready修改title/summary且请求仍ready允许作者明确声明完成，未明确ready的编辑器提交draft。archived节点只读，PUT status=draft解档时允许修改内容；不能在archived父下新增孩子。归档不删除孩子，读取可以审计，但生成context默认排除archived树枝。

DeleteNode仅无有效child、Beat、Scene及任何未删Clue引用时允许；否则409 outline.invalid_state，UI列出安全计数并引导移除依赖。删除加deleted_at/revision并使对应scope不可普通访问。Restore要求30天内、parent未删且可写、target仍同书、level/target没有被新节点占用，尾插position。恢复冲突不偷偷改绑定/创建新ID。master恢复时只有原node且不可存在另一有效master。

正文移卷后的绑定修复使用POST /nodes/{node_id}/rebind，RebindNode={expected_revision,parent_id}，返回Node。仅chapter节点可调用；target_id保持不变，平台投影确认该章当前volume等于新parent(volume节点).target_id。锁master→旧/新parent按UUID→node；目标和新旧父均非archived/deleted，新父容量足够，CAS目标revision，将章纲尾插新父并压紧旧父顺序；所有受影响节点revision/hash/平台历史快照及master revision同事务更新。Clue/Scene/Beat引用原node.id不变，因此既有历史不会因修复绑定被迫删除。普通PUT仍不允许变parent。跨书/错误目标卷422，旧revision409，重复Key只移动一次。

UI树节点来自NodePage，点击下钻独立加载，不用硬编码“第一卷”等业务数据。title/summary表单用机读schema验证，但以服务器结果为最终事实；409保留输入，刷新后由作者合并。没有“总纲已就绪所以正文也完成”的状态推断。

## 4. FR-08 节拍、场景及原子重排

Create/UpdateBeat锁master→chapter node→Beat；验证node level/归属、每章容量、tension等字段。Beat PUT的node_id必须等于已有node_id；跨node移动不在PUT内隐式做，两张不同章卡片不能通过改node_id绕开重排。Create/UpdateScene同规则；pov/location只表示计划引用，不把实体姓名缓存成事实；实体日后删除则UI显示不可用引用并允许作者清空。

状态planned双向转换written由作者显式操作，且仅影响本域。written不是content.confirmed。场景锚点绑定历史revision/hash，正文编辑后不后台重算偏移；编辑器读取current body后比较绑定，出现差异显示“定位已过期”，禁用跳转但保留计划。重新选区用UpdateScene写新四元组。outline不读正文、不会在内容保存事件上自动改scene。

Reorder(node_id,resource,ordered_ids,expected_revision)中expected_revision绑定父Node。resource=nodes时父level为master/volume；beats/scenes时为chapter。ordered_ids必须等于全部有效同类孩子集合且没有重复，允许空集合仅在数据库确实为空时。先锁master→父→孩子UUID排序；位置按数组下标设置，目标revision各+1，父和master各+1且不重复，延迟唯一约束在COMMIT检查。不支持一次混排Beat与Scene，不把它们当成同一序列。position使用整数而非浮点rank，2000上界使全量校验与写入有界。

SQL约束负责tenant/novel复合FK、枚举、范围、active_position唯一；层级邻接和完整集合由Application在锁内验证。任何一步失败或连接断开未知提交，客户端重用原幂等键查结果，不发送新的随机排序覆盖。

## 5. FR-09 伏笔状态机与事件

| 当前 | 允许目标 | 必须条件 |
|---|---|---|
| planned | planted / abandoned | planted需node_id有效chapter；abandoned必须reason |
| planted | developing / revealed / abandoned | 推进/揭示需node_id；planned可直接揭示但必须先埋设 |
| developing | developing / revealed / abandoned | 可多次暗示，同node仍可事件但每次必须新expected_revision |
| revealed | closed / abandoned | closed需node_id且ordinal≥埋设与最后推进ordinal |
| closed、abandoned | 无 | 终态不可反转，重新计划创建新Clue |

转移事务锁master→涉及chapter nodes→clue；检查expected_revision、所有节点同书且有效，按当前大纲序号验证node不早于planted及last_node，计划回收节点不早于埋设节点。首次planted写planted_node_id；每次非abandoned写last_node_id；abandoned可node_id=null且reason必填。写Clue.status/reason/revision并追加ClueEvent，同事务增加master revision、outbox和幂等。客户端重复Key只返回已写事件的Clue，不重复追加。

UpdateClue只改title/description/planned_reveal_node_id，不改状态/历史来源。已planted时新计划回收节点不能早于埋设；closed/abandoned允许修改描述以整理，状态仍终态。DELETE记录tombstone并保留事件；restore30天内原状态原ID，终态不因恢复变可编辑状态。所有历史事件按created_at DESC,id DESC游标，actor属于platform_users，不回查任何正文。

如果作者重排节点导致历史顺序倒挂，不重写ClueEvent。GET pacing将受影响clue标为outline.clue_order_changed供作者重新审阅，并暂停对该线索的超期推断；后续Transition仍按当前序检查。系统不把历史真实操作的顺序偷偷“修正”为新目录顺序。

## 6. FR-09 节奏与超期读模型

Pacing非持久化实体；以master.revision为outline_revision，在一个REPEATABLE READ只读事务查询有效master→volume→chapter按position拼接。节点最多2000，按全书从0给ordinal；每chapter的tension为全部未删Beat.tension的算术均值，保留两位小数；无Beat为null。beat_count和chapter_id来自本域node.target_id。不能join content章节名，图标签使用Node.title。

连续5个非null章tension<40产生outline.low_tension，>90产生outline.high_tension；边界40/90不算，null中断连续段；超过5章合并最大连续区间。伏笔status=planted/developing/revealed且有planned_reveal_node，若至少一个其后chapter node.status=ready或其Beat/Scene=written，提示outline.clue_overdue；这是大纲计划进度提示，不声称正文已连载至某章。planned/closed/abandoned不预警。PacingWarning带node_ids与clue_id（节奏告警为null）。预警不阻塞任何合法保存。

结果最多2000点、2000条warnings；超出合并同类相邻段，clue预警按最近updated_at优先，返回warnings_truncated=true，UI显示还有更多并通过CluePage筛选。图和无障碍表使用同一Pacing响应，空图有“尚未配置节拍”的真实空态。

## 7. 查询索引与投影一致性

NodeRepo.Get/List始终tenant/novel，nodes_list支持updated_at/id分页；读树按parent/position，使用节点约束索引。BeatRepo/SceneRepo按tenant/novel/node_id/updated_at/id分页，不允许漏node_id拉全租户。ClueRepo.List支持status、planned_reveal_node_id过滤，clues_list与clues_target索引；ClueEventRepo按tenant/clue/created_at/id游标。node/beat/scene/clue scope校验在取数据前执行，外键不能代替授权。

任何本域修改更新master revision，缓存key=(tenant,novel,master_revision)；Redis可选缓存，失败直接PG读，CRUD不依赖Redis。outbox只广播resource_id/revision，不广播summary/场景/伏笔正文；WS丢失后重拉。app维护实体作用域与删除投影同UoW，权限不是最终一致。

## 8. 复制、删除保留与失败补偿

内部OutlineSnapshot={novel_id:UUID,outline_revision:Revision,nodes:Node[],beats:Beat[],scenes:Scene[],clues:Clue[]}，数组分别按位置/ID稳定排序且只含有效实体；IDMap是{kind:novel|volume|chapter|node|beat|scene|clue|entity|relation,source_id:UUID,target_id:UUID}[]，source+kind唯一。content worker调用CaptureSnapshot，输出Nodes/Beats/Scenes/Clues与master revision；不输出跨域正文，不要求outline回调content。content将typed snapshot保存在platform FileStore并持久化job引用；Parse/serialize在数据库事务外，快照只读事务≤10秒。最终CloneSnapshot接入同UoW创建新书完整树：Node.target_id按卷章IDMap重写；Scene实体引用按worldview clone IDMap，未复制设定则置空；scene锚点清空、Beat/Scene planned、Node draft、Clue planned且planted/last/reason清空，计划回收target按node新ID映射，不复制clue_events。克隆所有本域ID重新UUIDv4，失败整个目标书不可见，content job重试重用预分配映射。

软删除30天；原owner通过restore命令恢复，跨scope404。单项删除受引用约束；整书删除无需触碰本域表即可通过平台scope即时隔离。平台retention按已到期scope调度本域PurgeNovel：先clue_events、clues、scenes、beats，再chapter→volume→master节点；短批事务幂等删除，保留只含ID/数量的安全审计90天。投影广播失败只重放outbox，不撤销领域已提交状态。不存在通过修复任务直接改content/private表的后门。

## 9. 错误与UI验收

400为格式/游标，404为跨scope或不存在；409 outline.version_conflict刷新并合并；409 outline.invalid_state用于有依赖删除/终态迁移/唯一目标；422 outline.validation_failed用于层级、参数、次序、容量及锚点约束；503仅PG/平台必要依赖失败。错误details只给节点ID和field_errors，不带设定/正文。

1. 三层新建、修改、归档、恢复；尝试第四层、错误父级、跨书target，所有失败无孤立节点。
2. 新建Beat/Scene→拖拽→刷新，position相同；两客户端争同父revision只成功一次；交换位置不会中途撞唯一索引。
3. 使用含代理对的正文选区，UTF16转换为码点，绑定既有revision/hash；伪造hash拒绝；保存新正文后旧锚点失效但Scene计划保留。
4. 伏笔planned→planted→developing两次→revealed→closed，每次一条事件；重复Key无新事件；closed再推进409；放弃无reason422。
5. 连续5章边界、null间隔、章序重排和计划超期，图/表输出一致；预警不会冒充已确认正文进度。
6. 删除原scope后旧API不可读；恢复同ID新revision；复制不带入written状态、旧事件或正文锚点；worker重试不得重复克隆。

本轮是文档设计交付，验收步骤用于R1实施，不新增测试文件或声称功能已上线。

单项回收站：资源集合GET的deleted默认false；deleted=true仅原Owner在父作品未删除且30日保留期内列出已软删项目，使用同一实体Schema与deleted_at，并显示restore动作。这不授予读取已删除整书正文/设定的权限；整书回收站仅content Novel元数据，需先恢复整书才访问其子资源。cursor绑定deleted过滤条件。

## 10. 完整HTTP操作与应用方法索引

每行对应Application方法（机读operationId带域前缀）。非GET均要求Idempotency-Key，路径UUID必填，query星号为必填；DELETE使用query.expected_revision，PUT与状态命令使用JSON.expected_revision。成功响应为SuccessMeta+data。列表均使用common Limit/Cursor，类型定义见各域types分片。

| FR/应用用例 | HTTP | 请求Schema / query | 成功data |
|---|---|---|---|
| FR-08 outline.listnodes | GET /v1/outline/novels/{novel_id}/nodes | — / parent_id, level, deleted | 200 NodePage |
| FR-08 outline.createNode | POST /v1/outline/novels/{novel_id}/nodes | CreateNode / — | 201 Node |
| FR-08 outline.getNode | GET /v1/outline/nodes/{node_id} | — / — | 200 Node |
| FR-08 outline.updateNode | PUT /v1/outline/nodes/{node_id} | UpdateNode / — | 200 Node |
| FR-08 outline.deleteNode | DELETE /v1/outline/nodes/{node_id} | — / expected_revision* | 200 Node |
| FR-08 outline.reorder | POST /v1/outline/nodes/{node_id}/reorder | Reorder / — | 200 OrderResult |
| FR-03 outline.restoreNode | POST /v1/outline/nodes/{node_id}/restore | RevisionCommand / — | 200 Node |
| FR-08 outline.rebindNode | POST /v1/outline/nodes/{node_id}/rebind | RebindNode / — | 200 Node |
| FR-08 outline.listbeats | GET /v1/outline/novels/{novel_id}/beats | — / node_id*, deleted | 200 BeatPage |
| FR-08 outline.createBeat | POST /v1/outline/novels/{novel_id}/beats | CreateBeat / — | 201 Beat |
| FR-08 outline.getBeat | GET /v1/outline/beats/{beat_id} | — / — | 200 Beat |
| FR-08 outline.updateBeat | PUT /v1/outline/beats/{beat_id} | UpdateBeat / — | 200 Beat |
| FR-08 outline.deleteBeat | DELETE /v1/outline/beats/{beat_id} | — / expected_revision* | 200 Beat |
| FR-08 outline.listscenes | GET /v1/outline/novels/{novel_id}/scenes | — / node_id*, deleted | 200 ScenePage |
| FR-08 outline.createScene | POST /v1/outline/novels/{novel_id}/scenes | CreateScene / — | 201 Scene |
| FR-08 outline.getScene | GET /v1/outline/scenes/{scene_id} | — / — | 200 Scene |
| FR-08 outline.updateScene | PUT /v1/outline/scenes/{scene_id} | UpdateScene / — | 200 Scene |
| FR-08 outline.deleteScene | DELETE /v1/outline/scenes/{scene_id} | — / expected_revision* | 200 Scene |
| FR-03 outline.restoreBeat | POST /v1/outline/beats/{beat_id}/restore | RevisionCommand / — | 200 Beat |
| FR-03 outline.restoreScene | POST /v1/outline/scenes/{scene_id}/restore | RevisionCommand / — | 200 Scene |
| FR-09 outline.listclues | GET /v1/outline/novels/{novel_id}/clues | — / status, planned_reveal_node_id, deleted | 200 CluePage |
| FR-09 outline.createClue | POST /v1/outline/novels/{novel_id}/clues | CreateClue / — | 201 Clue |
| FR-09 outline.getClue | GET /v1/outline/clues/{clue_id} | — / — | 200 Clue |
| FR-09 outline.updateClue | PUT /v1/outline/clues/{clue_id} | UpdateClue / — | 200 Clue |
| FR-09 outline.deleteClue | DELETE /v1/outline/clues/{clue_id} | — / expected_revision* | 200 Clue |
| FR-09 outline.transitionClue | POST /v1/outline/clues/{clue_id}/transition | TransitionClue / — | 200 Clue |
| FR-09 outline.listClueEvents | GET /v1/outline/clues/{clue_id}/events | — / — | 200 ClueEventPage |
| FR-09 outline.pacing | GET /v1/outline/novels/{novel_id}/pacing | — / — | 200 Pacing |
| FR-03 outline.restoreClue | POST /v1/outline/clues/{clue_id}/restore | RevisionCommand / — | 200 Clue |
