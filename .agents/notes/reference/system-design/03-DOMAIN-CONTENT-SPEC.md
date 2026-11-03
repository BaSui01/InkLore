> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 03 内容域：R1 冻结目标

状态：INK-141 冻结设计，尚未实现。全局语义服从[唯一契约基线](00-CONTRACT-BASELINE.md)。实施细节见[内容详细设计](domains/03-DOMAIN-CONTENT-DETAIL-SPEC.md)，建表使用[data/content.sql](../../../../docs/system-design/data/content.sql)，接口使用[content](../../../../docs/system-design/api/content.json)、[commands](../../../../docs/system-design/api/content-commands.json)、[jobs](../../../../docs/system-design/api/content-jobs.json)与[类型](../../../../docs/system-design/api/content-types.json)。这些是同一设计的互补视图，不提供旧路由兼容。

## 1. Owner、调用方向与范围

content唯一拥有作品、卷、章节纯文本、每次保存版本、写作统计事件和文件作业。场景计划唯一Owner是outline；编辑器场景栏调用outline接口，正文仍整章保存到content。content不拥有角色、不提取模型事实、不调用agent。作者让AI写作时，UI调用agent生成建议，采纳才调用SaveBody，确认才调用ConfirmChapter。

content应用定义OutlinePort、WorldviewFactPort、ExportReviewPort，app适配到outline、worldview、compliance应用服务；后三者不反调content。全部应用复用platform.ScopeAuthorizer、PGUnitOfWork、IdempotencyStore、Outbox、FileStore、JobScheduler、Clock。ScopeAuthorizer拥有作品、卷、章归属及revision/hash授权投影；content写入时由app投影适配器在相同UoW更新，不靠最终一致性允许越权窗口。

## 2. 核心模型与状态

| 实体/表 | 权威数据 | 状态和关键约束 |
|---|---|---|
| Novel / content_novels | owner_id、title、summary、category、target_words、word_count | active可写；archived只读；deleted只允许原Owner回收站元数据与恢复 |
| Volume / content_volumes | novel_id、title、summary、position | deleted_at空为有效；一书最多100卷；卷不能为空删除（须先移走/删除章） |
| Chapter / content_chapters | volume_id、body、current_version_id、confirmed_version_id、hash | draft→confirmed；任何正文保存/回滚→draft；每书最多2000有效章 |
| Version / content_chapter_versions | title/body/hash/word_count快照、chapter_revision、来源 | 正常业务仅追加；与本次正文保存同事务 |
| WritingEvent / content_writing_events | added_words、removed_words、source | 仅save/rollback追加；导入与复制不虚增写作量 |
| ContentJob / content_jobs | import/export/copy、lease、结果file/novel ID | PG持久化后台状态，独立于agent |
| ImportItem / content_import_items | 解析预览条目、正文、hash、warnings | 不自动写入章节；作者显式选择后提交 |
| JobSnapshot / content_job_snapshots | 固定版本正文与卷章排序 | 作业渲染只读取自身快照，不在长解析期间持事务 |

归档命令active→archived；restore对archived→active，对deleted→previous_status。delete记录previous_status并撤销普通作品scope，30日内restore保留全部原ID并增加revision；30日到期不可恢复，删除正文、版本、作业快照与PG文件，由各域保留其允许的去标识化证据。历史事实不会因编辑或回滚自动撤销。

## 3. UI动作、用例、接口与验收

| FR / UI动作 | API和类型 | SQL / 可复现验收 |
|---|---|---|
| FR-02 书架筛选、开书、改信息 | GET/POST /v1/content/novels；CreateNovel→Novel；PUT /novels/{novel_id} UpdateNovel | novels；同名可存在，空书首次开书只有Novel及scope/worldrevision初始化，刷新可见 |
| FR-02 归档、复制 | POST /novels/{novel_id}/archive、copy；RevisionCommand或CopyNovel→Novel/Job | novels/jobs/snapshots；归档拒绝编辑；复制成功新ID、全章draft、不复制已确认事实账本 |
| FR-03 删除与回收站恢复 | DELETE /novels/{novel_id}?expected_revision；POST restore | novels；跨Owner404；旧revision409；删除后其它域读写即刻404；恢复不复活单独删除的卷章 |
| FR-04 卷章添加、移卷拖拽、回收 | volumes/chapters CRUD；organization及reorder Reorder→Organization | volumes/chapters；完整成员排列原子提交；漏ID、重ID、跨书ID整批失败且目录不变 |
| FR-05 编辑/自动保存 | GET/PUT /chapters/{chapter_id}/body SaveBody→Body | chapters/versions/writing_events；同revision竞争只有一方成功，另一方收到current_revision并停止自动提交 |
| FR-06 历史、差异、回滚 | versions、diff；POST rollback Rollback→Body | versions；回滚新建版本，现有历史不覆盖，状态draft，旧事实不变化 |
| FR-07 审阅事实后确认 | POST /chapters/{chapter_id}/confirm Confirm→Confirmation | chapters + worldview账本/heads/revisions同PGUoW；任一冲突全部回滚 |
| FR-18 上传后导入预览、选择、提交 | POST novels/{novel_id}/imports CreateImport→Job；GET imports/{job_id}/preview；POST commit CommitImport→Job | jobs/import_items/chapters/versions；预览零章节副作用，提交不会因超时重试重复建章 |
| FR-19 导出与下载 | POST novels/{novel_id}/exports CreateExport→Job；GET jobs/{job_id} | jobs/snapshots + platform FileStore；TXT/Markdown/DOCX/EPUB均可下载；所有章绑定同快照 |
| FR-20 作品内全文检索 | GET novels/{novel_id}/search?q=... SearchPage | chapters pg_trgm；中文子串、转义百分号、跨租户隔离、码点定位正确 |
| FR-20 北京时间写作统计 | GET novels/{novel_id}/statistics?from=YYYY-MM-DD&to=... Statistics | writing_events；UTC16:00跨北京时间自然日；导入复制不计新增创作字数 |

所有修改携Idempotency-Key；所有PUT/DELETE和基于当前状态的命令携expected_revision。POST创建201，命令200，后台受理202。列表cursor分页与排序服从基线，目录聚合接口专为拖拽提供position顺序。归档、删除、恢复、复制、导入提交、取消均真实状态，不以客户端动画代替服务器结果。

## 4. 编辑器与失败体验

正文UTF-8、LF，≤200000码点，空格/空行/缩进保留，不自动替换标点。码点字数不含Unicode空白。textarea控制器把UTF16 selection转换为码点，所有建议定位绑定source_revision和content_hash。

停输2秒写IndexedDB，按tenant/user/novel/chapter隔离，7天TTL、每章3份、主体100MiB软上限；退出清除，shared-device默认关闭。停止输入30秒自动保存、连续输入60秒最大间隔，每章单在途请求。409停止自动保存，显示base/local/server三方合并；用户确认后用新幂等键和最新revision保存。网络未知结果重试原键，不猜测已保存。存储拒绝/配额不足持续显示告警并允许下载本地文本。

## 5. 导入导出与复用

文件统一platform FileStore PG bytea；上传和下载复用平台鉴权文件端点，JSON只传file_id，不传base64或外链。输入≤20MiB，导出≤100MiB。平台JobScheduler负责lease/heartbeat调度；content worker处理自己的jobs，不能将文件导入转成agent run。HTTP受理、worker每阶段、最终发布前都验证actor当前scope。

导出复用ExportReviewPort读取compliance审阅事实：没有报告可正常导出；已有匹配快照的block报告需作者本人显式ack并传review_ticket，绑定revision/hash。合规是风险提示，不是第三方出版审核。收到新报告、票据失效或源快照变动时拒绝发布并要求重新审阅，绝不跨表查询compliance。

## 6. 冻结交付验收

验收以类型化API和真实SQL为准：书架到导出形成闭环，所有UI成功反馈来自权威响应；幂等重放不新增版本/章/事实；作用域撤销后旧作业不能发布；重启worker可接续可重试阶段；append-only事实与回滚行为一致；不写业务代码或测试文件来替代本轮设计交付。
