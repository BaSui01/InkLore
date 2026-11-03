> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 04 大纲域：R1 冻结目标

状态：INK-141 冻结设计，尚未实现。遵守[基线](00-CONTRACT-BASELINE.md)，细节见[详细设计](domains/04-DOMAIN-OUTLINE-DETAIL-SPEC.md)，执行DDL见[data/outline.sql](../../../../docs/system-design/data/outline.sql)，契约见[outline](../../../../docs/system-design/api/outline.json)、[planning](../../../../docs/system-design/api/outline-planning.json)、[clues](../../../../docs/system-design/api/outline-clues.json)和[类型](../../../../docs/system-design/api/outline-types.json)。

## 1. Owner与边界

outline唯一拥有三级大纲、剧情节拍、场景计划、伏笔状态与节奏读模型。总纲/卷纲/章纲分别是Node.level=master/volume/chapter，不把节拍当作章纲。Scene属于outline，字段summary是计划摘要，start_offset/end_offset是可选正文锚点；没有scene.body，没有content场景表。

outline不依赖content、worldview、agent、llm或compliance。跨域UUID只作标识，ScopeAuthorizer验证novel/volume/chapter/entity归属；场景锚点调用平台AssertResourceRevision而非正文仓储。content通过自己的OutlinePort调用本域ReadContext、CaptureSnapshot、CloneSnapshot；agent通过自己的OutlineReadPort读取计划，生成的建议经作者通过本域API接受，不直接改库。

## 2. 聚合与不变量

一个作品最多一个未删除master节点；master.target_id=novel_id、parent_id=null。volume节点parent为master，target_id为平台验证的同书卷；chapter节点parent为volume节点，target_id为同书且属于对应卷的章节。每个同level目标唯一。层级固定三层，无任意目录树；正文移卷后大纲显示绑定不一致提示；作者通过POST /nodes/{node_id}/rebind将章纲移到新卷纲，保留节点和伏笔引用ID，不自动跟随篡改计划。

| 对象 | 表 | 状态/参数 |
|---|---|---|
| Node | outline_nodes | draft双向转换ready；任意非删除状态→archived；archived→draft；title1..120，summary≤20000，position从0连续 |
| Beat | outline_beats | planned双向转换written由作者显式改变；每章≤200，tension0..100，expected_words0..200000 |
| Scene | outline_scenes | planned双向转换written；每章≤200；码点锚点四项全部空或完整填写；不持正文 |
| Clue | outline_clues | planned→planted→developing→revealed→closed；未终态可→abandoned；closed/abandoned终态 |
| ClueEvent | outline_clue_events | 追加记录from/to/node/reason/actor，状态历史独立于当前投影 |

任何修改锁master并递增其revision一次，形成outline_revision；目标实体也CAS递增，若目标就是master不重复增加。重排校验完整兄弟集合，position唯一延迟约束在提交时生效。父子同tenant/novel复合FK，跨业务域没有FK或join。

## 3. 功能到UI与验收

| FR / UI动作 | API类型和端点 | SQL/验收 |
|---|---|---|
| FR-08 总纲→卷纲→章纲下钻与编辑 | /v1/outline/novels/{novel_id}/nodes GET/POST；/nodes/{node_id} GET/PUT/DELETE；CreateNode、UpdateNode→Node | nodes；二级挂二级、跨书target、重复chapter细纲422/409；删除有孩子节点409 |
| FR-08 节拍卡片、章末钩子 | /novels/{novel_id}/beats，/beats/{beat_id}；Beat的beat_type/hook_type/tension | beats；200张上限；作者切written不意味着章节已确认 |
| FR-08 场景侧栏、POV、地点、选区锚点 | scenes CRUD；CreateScene/UpdateScene→Scene | scenes；source_revision/hash伪造拒绝，正文变化后定位标为过期，计划摘要继续保留 |
| FR-08 拖拽顺序 | POST /nodes/{node_id}/reorder Reorder→OrderResult | nodes/beats/scenes；resource指定集合，expected_revision对应父node，任一外部ID整批失败 |
| FR-09 伏笔埋设、推进、揭示、回收、放弃 | clues CRUD；POST /clues/{clue_id}/transition TransitionClue→Clue；GET events | clues/clue_events；无章节不能埋设，不能先回收后埋设，终态不可倒退 |
| FR-09 节奏图及超期提示 | GET /novels/{novel_id}/pacing→Pacing | 按nodes顺序与beats实时聚合；无节拍tension=null；连续5章低于40/高于90形成提示，不阻止保存 |

列表遵守cursor分页updated_at DESC,id DESC；客户端按position渲染已加载集合；pacing提供完整最多2000章的有序点。图与等价表共享同一响应，不硬编码演示数组。与正文确认无自动状态联动，confirmed不是大纲状态。

## 4. 复用、失败与完成定义

HTTP鉴权→ScopeAuthorizer→应用服务→PGUnitOfWork→域仓储；IdempotencyStore/outbox共事务。原子修改失败整体回滚；outbox只发低频失效通知。无LLM/文件解析在事务内。本域同步操作无自有长作业；全书复制由content作业调用Snapshot/Clone能力，通过平台JobScheduler和FileStore实现，消费者承担长任务授权检查，本域每次Port调用仍再次验证scope。

软删除保留30天，单项删除提供同资源restore命令；恢复同ID新revision且重校验父存在、target scope和容量。整书删除由授权投影即时隔离所有本域读取，到期通过平台retention调度清理。引用中的UUID审计保留不等于可绕过权限读取。验收要求三层、场景、节拍、伏笔与节奏全链路有真实持久化及冲突反馈。
