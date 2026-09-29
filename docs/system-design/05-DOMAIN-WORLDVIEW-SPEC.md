# 05 世界观域：R1 冻结目标

状态：INK-141 冻结设计，尚未实现。遵守[基线](00-CONTRACT-BASELINE.md)。实施见[详细设计](domains/05-DOMAIN-WORLDVIEW-DETAIL-SPEC.md)、[真实SQL](data/worldview.sql)、[资源API](api/worldview.json)、[关系API](api/worldview-relations.json)、[事实API](api/worldview-facts.json)、[类型](api/worldview-types.json)。

## 1. Owner与调用方向

worldview唯一拥有角色、设定词条、关系三元组、事实事件与当前heads。无业务域依赖；不得查content表、反调正文、从模型直接提取并确认事实。content.ConfirmChapter通过消费方WorldviewFactPort调用worldview.CommitFacts，同一platform PGUnitOfWork内原子完成章节确认与账本。agent只读取上下文并提出FactProposal，作者逐条审阅才进入确认命令。

角色和词条共用worldview_entities作为同域统一节点身份；kind固定character/lore。API仍提供Character/Lore不同明确类型，Lore.title映射entities.name。关系端点拥有复合FK，同书实体才可连接；跨业务域来源章/版本仅UUID，不跨域FK或join。

## 2. 数据与状态机

| 模型/表 | 规则 |
|---|---|
| WorldRevision / worldview_revisions | 每书一行，revision从1；任一角色/设定/关系/事实变更整事务+1；last_seq分配不可变事件序号 |
| Character/Lore / worldview_entities | active双向转换archived；deleted_at软删除；角色life_status与realm_rank是事实heads投影，不允许普通PUT绕过事实账本修改 |
| Relation / worldview_relations | active双向转换retired；同三元组最多一个有效active；禁自环；weight0..100 |
| FactEvent / worldview_fact_ledger_events | assert/supersede/retract只INSERT；seq每书连续递增；target指当时head事件；SQL触发器阻止UPDATE/DELETE |
| FactPayload / worldview_fact_payloads | 值、理由、actor、来源hash；业务不可变，30日到期擦除载荷允许DELETE；保留账本UUID/key/digest证明 |
| FactHead / worldview_fact_heads | 唯一键novel/subject/predicate/fact_key；active/retracted，可重放投影；revision CAS |

角色初始alive、realm_rank=0；允许alive→missing/dead，missing→alive/dead，dead→revived（需retcon及理由），revived→missing/dead。dead→alive必须明确retcon理由；境界下降必须retcon。不是仅靠名称文本推断角色身份。撤回life_status/rank后根据剩余active head重算；没有有效head回到初始默认。角色静态档案与动态事实分栏展示，作者不能通过“编辑角色”偷偷更改已确认事实。

## 3. 用例、API类型、UI和验收

| FR / UI动作 | API | SQL / 验收 |
|---|---|---|
| FR-10 新建/编辑角色、别名、角色类型 | /v1/worldview/novels/{novel_id}/characters与/characters/{character_id}；CreateCharacter/UpdateCharacter→Character | entities；角色名1..60，aliases≤20；普通PUT没有life_status/rank写字段 |
| FR-10 分类设定百科 | lore CRUD；CreateLore/UpdateLore→Lore | entities(kind=lore)；category=faction/place/system/item/history/other；正文description≤20000 |
| FR-10 关系图与等价表 | relations CRUD；GET /novels/{novel_id}/graph→Graph | relations/entities；跨书边404，自环422，节点≤500/边≤1000且truncated明确标记 |
| FR-07 审阅确认事实 | content Confirm的fact_proposals使用FactProposal；GET world-revision；POST facts/check→CheckResult | revisions/ledger/payload/heads；预检无账本副作用；最终确认重检版本，非缓存判定 |
| FR-11 活跃事实与来源审计 | GET /novels/{novel_id}/facts→FactPage；fact-events→FactEventPage | heads/ledger/payload；分页历史稳定按seq；旧事实保留来源版本，不冒充当前正文 |
| FR-11 撤回/替代 | POST /facts/{fact_id}/retract或supersede；RetractFact/SupersedeFact→FactMutation | fact_id为head.id；要求head expected_revision与expected_world_revision；并发旧票据409，不覆盖别人修订 |
| FR-03 删除/恢复档案 | DELETE资源?expected_revision；POST restore | entities/relations；软删除保留30天；被活跃关系或head引用的实体禁止删除，先显式退休/撤回 |

## 4. 事实确认与审查界限

FactValue是text/number/boolean/entity有标签联合，不接受任意JSON。FactProposal最多100条；一批同subject/predicate/fact_key只能一条。assert无target；supersede/retract必须精确target_event_id、resolution=correction或retcon、reason1..1000；retract.value=null。没有全局allow_override。冲突返回结构化subject/key/current_event/proposal_index/reason，HTTP409 worldview.fact_conflict，整批无任何写入。

life_status使用text枚举，realm_rank用number整数，item_status用text完整/损坏/销毁机器值intact/damaged/destroyed，location/relationship用entity；custom允许明确FactValue并要求稳定snake_case fact_key。一致性是确定性校验，不调用LLM、不声称证明小说逻辑绝对正确。模型事实仅建议，作者负责审阅。

## 5. 权限、删除与复用验收

所有查询带tenant+ScopeAuthorizer；operator不能因运维角色读取作者角色/事实正文。worldview只依赖平台UUID/Clock/PGUoW/授权/幂等/outbox/FileStore。全书复制的Snapshot/Clone由content作业调用，本域不启动agent；复制静态设定和关系、不复制账本与动态死亡/等级事实。30日到期擦除实体描述和payload，事件保留去标识化UUID/key/digest；正常业务和retention均不能改事件本体。验收含事务失败回滚、head重建、重复幂等请求、删除后作用域隔离、事件不可变、图表同源与角色动态投影。
