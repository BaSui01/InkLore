# ADR-0016：R1完整重构设计定稿与单一文档基线

> 文档位置部分被 [ADR-0017](0017-preserve-decision-records-and-single-design-entry.md) 替代：第1条及替代范围中的history搬迁/执行记录位置不执行，其余架构与R1契约保持有效。

状态：Accepted（设计决策，尚未实施）。日期2026-09-18。任务INK-141。依据：用户明确要求删除旧文档、以system-design完成详细设计后整体重构；本决策记录本轮选定方案，不能解释为代码已通过验收。

## 决策

1. docs/system-design是唯一施工设计入口；docs/README只导航。旧架构实况、路线图、历史产品分析、兼容设计和旧API副本在内容被新规范覆盖后删除。以前ADR保留于decisions/history供追溯，历史冲突条款由本决策替代。当前任务/证据统一18分册，历史INK编号不复用。
2. 七域模块化单体及Owner不变；依赖DAG明确扩展至inspiration/compliance的模型能力和Agent只读上下文。content只调用outline/worldview和compliance导出审阅Port，agent统一执行模型任务；content不调用agent。platform ScopeAuthorizer提供作品权限投影，避免叶子域反查content私表。见00-CONTRACT-BASELINE。
3. 公开接口按域重建，不保持/v1/novels旧路由或旧POST流兼容。UUIDv4；字符串domain.reason错误；类型化JSON封包；int64 revision/expected_revision CAS；副作用命令24h幂等。api/openapi.json是机读接口唯一入口，SQL以data/为准。
4. 生成改为POST持久化run，再GET SSE订阅同请求执行流；PG队列与租约，7天事件回放，本人审批恢复原run，断连接收不取消生成，显式取消。WS仅低频通知，不传JWT查询参数，改一次性短票据。
5. content保存正文与版本同事务。确认跨content/worldview使用platform不透明UnitOfWork，由各域仓储在同一PG事务写自有表；账本只追加，fact_heads可变投影。平台授权投影/outbox/幂等与业务提交保持同事务，拒绝跨域裸SQL。
6. 单一PG16业务/文件/队列权威，Redis仅缓存/配额。输入20MiB，输出100MiB以PGbytea存储；不同时提供S3与磁盘第二方案。Redis故障拒绝新AI调用而保留非AI读写。全文pg_trgm，向量1536维，Token预算60000。
7. 私有部署本地账号与多租户会话；creator/operator/auditor，JWT15分钟+HttpOnly刷新轮换30天；不依赖隐式通配权限，不公开注册。两独立前端app，3000/3001 strictPort。IndexedDB限定保留，编辑自动保存和版本冲突有完整恢复交互。
8. 新增库限定：后端JSONSchema Draft2020-12校验库santhosh-tekuri/jsonschema/v6；前端ReactRouter7、TanStackQuery5、Ajv8+formats3；已有技术栈保留。只引入文档设计，不在本轮安装依赖或改lockfile。

## 替代范围

旧ADR-0001的单体/分层原则继续保留，域清单依R1；0002的SSE/WS职责保留但具体路由/生命周期由R1替代；0003北京时区与0004环境凭据不变；0006认证策略、0007确认协议、0009权限/审批细节、0014依赖DAG、0015兼容提案在与R1冲突处由本决策替代。旧ADR正文不改，移入历史目录并由索引标记历史适用范围。没有任何历史“现状限制”可覆盖R1接口契约。

## 备选与取舍

- 保持旧接口兼容会留下两套错误与流式协议，整体重构无需付出此复杂度，因此不选。
- Redis队列/S3文件会再引入持久化一致性面；R1规模用PG任务与受限文件足够，容量超过阈值必须另开R2，不在施工中切换。
- 跨域Saga用于确认会让已确认标记与事实暂时不一致；模块化单体共享PG可使用Port封装同事务，因此选择原子确认。
- 前端自定义缓存/表单引擎维护成本高；复用Query/Ajv并限定边界，业务状态机仍属于各域控制器。

## 验收

文档：OpenAPI引用/operationId/参数/响应、DDL执行顺序/Owner/约束、功能至接口至组件映射、全链接/单文件体积、状态终态与幂等语义均检查。实现：另按18各任务验收，不以设计检查替代DB/模型/浏览器证据。冻结并非保证永无缺陷；R1.x变更必须记录原因、契约影响、迁移与验收并保持旧基线可追溯。
