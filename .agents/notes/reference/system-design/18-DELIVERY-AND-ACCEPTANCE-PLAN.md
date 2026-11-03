> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 18 · R1重构实施顺序与完成门禁

状态：冻结设计计划，功能未实施。执行状态以[任务看板](../planning/backlog.md)为准，本文不反复改为代码实况。用户要求先完成设计再整体重构；本轮INK-141仅交付文档、契约与旧设计清理。

## 1. 实施任务与顺序

历史INK-001..140不复用；INK-142是独立Skills任务，也不复用。R1实施从160开始，任务状态登记看板，默认todo。

| 任务 | 唯一交付范围 | 依赖 | 可独立验收 |
| --- | --- | --- | --- |
| INK-160 | platform PG迁移、UnitOfWork、scope、幂等 | 141 | CAS/跨scope/业务回滚/幂等唯一，DDL与API基座一致 |
| INK-161 | platform账号、会话、成员、CSRF与审计 | 160 | 双aud、refresh重放、最后operator、隐私边界 |
| INK-162 | platform文件、队列、outbox、WS/观测 | 160,161 | lease fencing、文件hash、消费去重、通知只安全摘要 |
| INK-163 | content作品/卷/章/版本/排序 | 160,161 | 两窗口/快照故障/回滚/软删真实DB |
| INK-164 | outline三级大纲/场景/伏笔/节奏 | 160,161 | 无环/同scope/码点范围/状态迁移/排序原子 |
| INK-165 | worldview角色/设定/图谱/账本 | 160,161 | 只追加事件、heads重建、逐项冲突、更正清理 |
| INK-166 | content确认用例与跨域Port装配 | 163,164,165 | 固定锁序、一事务确认、旧revision拒绝、无跨私表 |
| INK-167 | llm目录/SecretRef/策略/预算/Gateway | 160,161,162 | 策略版本、Redis故障、费率预留结算、严格schema |
| INK-168 | inspiration来源/分析/文风/灵感 | 162,167 | 来源码点、不可变风格、采纳核验不反写正文 |
| INK-169 | compliance规则/扫描/报告/净化 | 162,167 | AC版本切换、源修订、有限轮次、不假合规认证 |
| INK-170 | agent定义/工具/MCP/workflow配置 | 161,167 | schema/权限/域名与Secret允许列表、配置不可变版本 |
| INK-171 | agent TAO/PG运行/审批/SSE/取消 | 162,168,169,170 | 原run恢复、唯一终态、断流无重POST、未知副作用不重发 |
| INK-172 | content导入/导出/全文/统计 | 162,163,169 | 预览无写入、item幂等、快照导出、北京日/统计口径 |
| INK-173 | frontend共享身份/HTTP/Query/SSE/Schema/恢复库 | 161,162 | 独立app、严格number、字段错误、缓存隔离、IDB额度失败 |
| INK-174 | creator页面与编辑控制器 | 166,168,169,171,172,173 | 15全部页面、选区漂移、双命令保存、恢复与键盘 |
| INK-175 | admin页面与权限 | 167,170,171,173 | 16全部页面、配置实际生效、无正文越权、审计诊断 |
| INK-176 | 可观测性、备份/恢复/停机、容量验证 | 171,174,175 | 13逐项探针/故障/恢复/性能样本 |
| INK-177 | 旧库到R1映射导入及最终切换演练 | 176 | 无漏引用/hash变化、删除清单、回滚边界、双端bundlehash |

跨域任务166只装配公开Port，不把三域仓储合并。每域完成后才接入下一阶段，禁止用前端mock绕开未完成服务。既有代码可复用但仍逐条重验R1行为。

## 2. 每项验收证据模板

记录：INK/FR/operationId、Git版本与未提交工作区说明、环境/PG迁移版本、主体aud/role与脱敏tenant、请求方法/参数（去凭据）、响应HTTP/code/revision/关联ID、SQL前后行/约束、浏览器行为/截图、预期与实际、未覆盖项、运行路径影响。桩模型只证明协议；模型事实质量须人工报告且不能把一次输出推广为保证。

通用门禁：go build ./...；前端对应app构建；archguard跨域/大小/Emoji检查；OpenAPI静态/请求响应校验；SQL约束与查询计划；真实PG正反例；无新增 *_test.go/*.test.ts，无凭据，无假数据。仅“页面能打开”“接口200”“编译通过”不满足功能done。

## 3. 必须穿透的集成场景

| 编号 | 输入与故障 | 期望 |
| --- | --- | --- |
| R1-01 | A/B租户同名作品、越权UUID/API/SSE/file/cache | 全部scope隔离，不泄漏存在性 |
| R1-02 | 两页同revision保存；先改标题再保存正文失败 | 标题已保存真实显示，正文dirty保留；陈旧章409 |
| R1-03 | 正文写成功前故障于version插入 | 整笔回滚，正文和历史不分离 |
| R1-04 | 确认过程中worldview插入失败 | 章未confirmed、账本无半批、幂等记录不假完成 |
| R1-05 | 选区生成后编辑/切章/重复句 | 旧结果不可替换新范围；可复制/重新定位 |
| R1-06 | POSTrun响应丢失、SSE断开、7天游标过期 | 同键一个run；GET只回放；410取artifact不重生成 |
| R1-07 | 审批两个窗口、参数变化、源章变化、过期 | 仅本人一个决定，同run恢复，旧许可不能执行 |
| R1-08 | 上游已发送后workerkill | 未知结果失败/待对账，不自动多次计费 |
| R1-09 | cancel和succeed竞态、旧leaseworker迟到 | 一个终态和终态事件，fence挡旧提交 |
| R1-10 | Redis中断、PG中断分别操作 | AI前者503非AI可用；后者ready503无内存降级 |
| R1-11 | 导入中途失败后同键重放 | 已成功item不重复，剩余可恢复，预览不写正文 |
| R1-12 | 导出时作者继续编辑、有旧block报告 | 输出按版本快照；审阅绑定hash，无报告正常导出 |
| R1-13 | 发布新词库/模型策略时旧run在执行 | 旧快照不漂移，新run用新版本 |
| R1-14 | budget跨北京零点、unknown/partial上游费用 | 预留/实际/释放可对账，无重复结算/虚假免费 |
| R1-15 | 源资料/作品软删除及30天purge、备份恢复 | 权限立即撤销，payload按期擦除且不从备份复活 |
| R1-16 | 未登录刷新页面/双端cookie/refresh重放 | 同源CSRF引导；两aud不能互换；重放吊销 |
| R1-17 | 纯键盘/读屏/390px/暗浅色/英文长文 | 焦点、对比、布局、真实八态全部符合17 |
| R1-18 | 管理员查看run或诊断 | 仅脱敏摘要，诊断固定探针，不能冒名读取作者正文 |

## 4. 旧库切换

原始库先只读备份→隔离创建R1库→tenant/user/UUID映射→逐域转换→校验计数/引用/hash/账本heads→只读演练→短维护窗口最终增量→切两个静态站和API→探针/关键流程→留回滚时间窗。没有用户明确部署授权不得在真实库执行。回退不自动把R1新增写入倒灌旧schema，必须记录数据边界。

## 5. 本轮设计验收范围

INK-141验证资产一致性而非功能运行：编译OpenAPI入口、error/FR目录；校验所有本地refs、path parameters、mutation headers、CAS、operationId与权限元数据；DDL表Owner/引用静态核验；全文件链接/大小；旧文档真实删除清单；现后端go build。最终机器报告见[设计校验报告](implementation/design-validation.json)。没有执行生产部署/数据迁移，没有宣称DDL已在PG执行或所有UI已可用。

## R1.x Agent策略绑定与中途澄清增量门禁

依据[ADR-0027](../decisions/0027-proposed-creator-task-policy-catalog.md)，仅含Creator安全策略目录/Run不可变版本消费与Agent Question交互。INK-227登记本契约/设计交付；其实现任务为INK-228（策略目录Port、CreateRun版本冻结、Worker真实消费与000010迁移）、INK-229（Question状态/持久表、Answer CAS、PG job/checkpoint及SSE回放）、INK-230（Creator/Admin安全目录/问答UI与隔离PG/API/worker/浏览器端到端验收）。执行顺序：227设计review→228与229后端依赖完成→230跨域验收；每个实现阶段依次更新源契约/000010+000011迁移、Agent service/worker、双端UI，再执行隔离PG并发/恢复与浏览器验收。迁移000010先加LLM policy-version task_kinds（既有版本空映射），000011再加Agent Question与事件；两者只upgrade，保留旧字段与数据；回滚部署仅回应用，不执行down或删除已答Question/事件。单一task_key兼容只覆盖服务与客户端滚动升级期间，sunset在Creator升级验证后关闭并保留部署证据。

给定旧task_key Creator请求，兼容窗口内只接受tenant已启用策略并冻结版本，窗口关闭后拒绝缺version请求；未知/跨tenant拒绝且不创建Run/Job。给定策略停用/发布竞争，新Run要么绑定事务提交时current version，要么409且无Run/Job；Worker每次真实Gateway调用使用Run冻结版本，Admin发布不漂移已有运行。给定open question，页面退出/网络断开后GET恢复同Run，Last-Event-ID续传；合法回答本人CAS一次并恢复原job/checkpoint，重复键不重复执行，其他actor/scope拒绝；schema越界/伪造工具权限拒绝。取消、过期、撤权、来源变化竞态最终只有唯一终态/问答状态与原子持久事件；未知外部副作用不重发，未回答不伪造进度。须验证双端Creator/Admin UI、数据库迁移与上下行兼容、Agent API/Worker真实链路；文档、构建、静态检查不等于功能验收。



依据[ADR-0021](../decisions/0021-r1-auth-extensions.md)保留验证码、独立creator注册和密码找回入口。必须验证严格Schema、密码边界、角色注入拒绝、注册原子性、重置令牌失败重试/并发、会话撤销及开发/非开发投递边界；没有邮件供应商时不宣称生产找回已交付。原七域与Gin验收门禁继续有效。

## R1.x 密码最小长度调整（ADR-0028）

INK-232将统一新密码规则调整为6..72 UTF-8字节且至少6个Unicode字符；登录、Creator注册、找回重置、CLI bootstrap、成员创建/重置及本人改密都必须共用此规则。验证严格API/嵌入Schema一致，6字符接受、5字符拒绝、UTF-8超72字节拒绝，双端表单提示与后端身份服务一致；不自动重置已有密码，不引入默认账号口令。

## R1.x Admin root用户名登录（ADR-0029）

INK-233给platform_users增加可空唯一username列，并通过000012向前迁移；Auth Login Schema要求email/username恰好一个，公开注册仍只接收email。Admin允许输入email或root，后端用户名只在admin audience查询且结果必须通过普通operator membership/tenant授权。`cmd/admin bootstrap -username root`支持新数据库；`cmd/admin set-root -email EMAIL`为既有operator绑定root并更新口令、吊销旧session，口令仅从环境变量读取并立即清除。验证重复root、非operator、Creator用root、tenant隔离和旧email登录保留；不新增super_admin角色与默认凭据。
