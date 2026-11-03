> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 02 · 总体架构与跨域执行

状态：R1冻结设计。全局值遵循[契约基线](00-CONTRACT-BASELINE.md)，决策见[ADR-0016](../decisions/0016-r1-reconstruction-design-freeze.md)。

## 1. 运行形态

同一Go模块、单一业务服务二进制：role=all（默认）同时HTTP和Worker；role=api/worker只分进程职责，可横向扩容但不拆服务接口。所有实例连接同PG与Redis。两前端独立构建，creator和admin分别静态产物和域名；开发端口3000/3001，API8080。Nginx代理各站/v1至同后端，后端独立判audience与权限。

~~~mermaid
flowchart TB
 C[Creator 3000] --> H[HTTP Transport]
 A[Admin 3001] --> H
 H --> APP[各域 Application]
 APP --> P[消费方 Port]
 P --> W[app adapters 唯一装配]
 W --> D[领域能力]
 APP --> U[platform UnitOfWork]
 U --> DB[(PostgreSQL16)]
 WORK[同仓 Worker + PG lease] --> APP
 D --> REDIS[(Redis7 配额与缓存)]
~~~

## 2. 域依赖与权威

- content→outline读规划、worldview读设定与确认落账、compliance读导出审阅；绝不反向调用agent生成。
- agent→content/worldview/outline/inspiration/compliance读取授权上下文与调用建议能力，→llm模型。工具产生的小说变更必须形成候选，由作者通过content/worldview命令采纳。
- inspiration/compliance→llm模型能力；不调用agent，Agent作为调用者执行已创建分析任务。outline/worldview/llm不依赖业务域。
- 跨域读实体归属校验走platform授权投影，投影只含tenant/resource/owner/state/revision，不含业务正文。每次资源新建/删除与scope投影同UoW，不允许异步产生越权时间窗。
- domain只值对象、聚合、规则与Port声明，不含Gin/GORM/Redis/HTTP供应商代码。application编排规则、事务与输出DTO；transport只解码验证调用映射；infrastructure实现仓储/外部协议，不被其他域直接import。

## 3. 关键运行路径

### 3.1 保存

Transport验证JWT→ScopeAuthorizer确认章归属→解析IdempotencyKey和expected_revision→Content.SaveBody→UnitOfWork开始→claim幂等记录→按tenant/id/revision条件更新章（0行=冲突）→插version快照→scope revision/统计outbox→完成幂等记录→提交→返回typed Chapter。正文长字符串不参与跨域共享缓存。版本快照插入失败整笔回滚。

### 3.2 章节确认

Content.Confirm先取已保存章的source revision/hash并验证用户提供的expected_revision；同UoW固定顺序锁scope、novel、chapter、worldview_revision。调用WorldviewFactPort.CheckAndAppend(txctx,expected_world_revision,proposals,resolutions)；有冲突整笔拒绝，无部分事实成功。content更新confirmed_version_id、confirmed_at与revision；账本events、heads投影、confirmation receipt、outbox同提交。Port实现禁止自行开启REQUIRES_NEW或commit。

### 3.3 生成

UI创建或选择资源→POSTagents/runs持久化输入快照/queue记录→202返回run ID→UI GETevents订阅→Worker claim→scope复检→策略快照/预算预留→TAO/工具→持久化event→SSE从日志投递→产物→预算结算→终态CAS。运行中被撤销作品权限：下个工具/模型调用前终止，已保存产物变不可读。订阅取消不取消作业；cancel命令设置cancelling，worker在安全检查点终结。

### 3.4 拆书/质检

先创建领域analysis/report/rewrite记录，快照与来源hash冻结；再创建Agent run(task_kind,resource_id)，Agent工具调用对应Domain ExecutionPort。领域内部调用LLMGatewayPort返回结构化建议，不能写content/worldview。事务仅保存阶段产物；模型调用在事务外。幂等执行键由run/node/attempt logical invocation定义，技术重试不得产生重复报告版本。

## 4. 共享能力与隔离

platform只放技术能力：auth、scope、errors、response、unitofwork、jobs、outbox、files、cache、limiter、logger、metrics、timeutil、schema、crypto。领域“通用service/repository”不得上提。复用两步：先找稳定相同语义，再把调用输入输出写入Port；只是字段相似不能共享聚合。包级允许依赖表见03，前端复用见20。

UnitOfWork签名：Within(ctx, func(txctx context.Context) error) error。SQL仓储只从TxResolver(txctx)获得当前事务；没有事务则只读用DB，写命令必须拒绝无UoW。外部HTTP调用、SSE等待与大文件解析不得在Within中。Postgres隔离READ COMMITTED+显式锁/版本条件；serialization/deadlock错误仅事务无外部副作用时最多重试3次，保留同幂等键。

## 5. 失败与降级

PG不可用：写/读权威请求503，readyz失败；绝不切内存仓储或演示正文。Redis不可用：AI admission关闭、非AICRUD可用，UI显示生成暂不可用；已有额度租约无法续期则在下一调用前终止，不静默无限额度。上游超时：Gateway区分已接收输出与未发送请求；未知计费结果记unknown，不自动换渠道重发。Worker崩溃：lease到期恢复，读取节点执行记录，不重复已提交工具副作用。投影失败：outbox保留重试，核心已提交结果不回滚；UI显示数据截至时间。

## 6. 重构迁移边界

在隔离新库按R1 DDL建表；旧库只读导入，不原地把BIGSERIAL强转UUID。映射表记录old table/id→new UUID，校验租户/引用/正文hash；只在独立切换窗口换流量。转换器与证据由18实施任务负责。本轮不执行删除数据、迁移或部署。新旧API不双写，回滚切回旧服务与旧只读快照，R1上线后的新增数据需人工决定迁回，不能承诺无损自动降级。
