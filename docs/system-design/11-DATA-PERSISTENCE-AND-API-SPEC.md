# 11 · 数据与HTTP契约总则

R1冻结；[API入口](api/openapi.json)、[公共Schema](api/common.json)、[实体DDL](data/README.md)是可检查资产；领域分册解释为什么与如何，不能另抄不一致字段表。

## 1. SQL组织与执行顺序

新库先platform.sql（pgcrypto/pg_trgm/vector扩展与tenant/user/scope），再content.sql、outline.sql、worldview.sql、inspiration.sql、compliance.sql、agent.sql、llm.sql。业务域相互不建立跨域私表FK；以UUID+scope验证与Port保持边界。同域引用尽量(tenant_id,id)复合FK防误租户，所有作用域查询带tenant。platform_sessions通过成员复合FK作用域，refresh_tokens与票据通过session继承scope，不能因表无tenant列跳过认证。

系统表例外固定：scope/revision投影与inbox用作用域复合主键；refresh_tokens/ws_tickets从不可变session FK继承tenant；租户/用户是根身份；不可变投影/token/审计只created_at，幂等/outbox状态由独立状态CAS并非公开revision。上述表不是公开业务聚合，不机械补无意义id/revision；业务聚合仍UUID+tenant+revision+timestamps。

设计SQL不写IF NOT EXISTS掩盖结构差异，重复执行应失败；正式迁移有版本记录与校验和。修改已有表只能增量迁移，不编辑已执行迁移。数据库默认now用于UTC瞬时，业务自然日必须timeutil转Asia/Shanghai。正文text走PG TOAST但不声称列表零IO：列表投影严禁SELECT *，只取标题/字数/revision/摘要标识。

## 2. 事务与并发

写聚合 revision从1增加，expected_revision必须；SQL UPDATE ... WHERE tenant_id=? AND id=? AND revision=? RETURNING ...。0行在同scope复查分404/409，不能先GET再无条件UPDATE。创建/软删除/重排/发布/审批同样有资源修订。批量重排先锁作品/结构revision，校验集合完整无重复且所有节点同scope，临时rank或一次CASE UPDATE避免unique序号中间冲突，事务内完成。

正文版本、确认账本、auth投影、幂等结果和outbox的原子性见02。外呼/模型/大文件不占事务；结果回写重新校验lease_epoch与source_revision。若作者在任务期间改正文，运行可以完成候选但artifact标source_stale，采纳需重新定位；不能用后台结果覆盖新正文。

批量导入：预览只解析，提交按冻结清单形成单个导入job；执行幂等项键(job_id,item_index)，记录created_id/状态，允许部分成功报告；不把整批HTTP失败当零写入。报告明确成功/失败/未处理，重试仅未成功项，已创建item不重复产生章节。导出按版本清单生成不可变snapshot，不边生成边读最新章混版本。

## 3. HTTP封包和错误

成功code=ok，data为声明类型；request_id UUID，trace_id32hex。错误code=domain.reason、message_key、details安全结构。不可用本地化文本做条件判断。列表data.items/next_cursor/has_more，游标签名绑定scope/filter/sort，默认limit20最多100。文件与SSE显式独立media，JSON解析器不处理其正常正文。

POST创建201、命令200、任务202，PUT/PATCH/DELETE200；错误规则见14。每个mutation须Idempotency-Key，认证会话端点用轮换事务例外。request_id由网关生成；外部X-Request-ID只接受合法UUID，否则重新生成；Traceparent合法则接续，不将X-Request-ID伪装trace_id。返回X-Request-ID/Traceparent方便关联，CORS只允许配置的两个应用Origin。

## 4. 输入约束与序列化

JSON拒绝未知字段（动态工具arguments除外，但必须通过绑定schema校验）；UUID格式/size限制先transport，跨实体规则在application。空值与未提供不同：PUT完整可修改字段，PATCH若某接口未声明则405，不开放通用任意JSONpatch。revision/int64范围≤9007199254740991，避免JS精度损失；大金额decimal字符串。枚举新增不是随意字符串；未知服务端枚举前端显示不可操作状态与安全原值，防止老客户端误写。

文本统一LF，禁NUL，不自动做NFKC改写小说文字。字数统计非空白码点，offset为码点，DOM选区转换见20；content_hash=SHA256 UTF8 canonical正文。标题长度按码点；密码按UTF8字节。错误field采用JSONPointer例如/temperature或/nodes/2/title，前端可定位字段。

## 5. 删除/保留与读缓存

软删除30天，恢复命令expected_revision，唯一名称冲突409提供重命名；到期处理按Owner删除正文/文件/payload，账本event保留不含原文的证明。run原始输入和artifact在对应资源清除时同步安排purge并停止授权读取。审计90天，SSE事件7天，幂等24h，下载导出7天；最长保留不能被日志副本绕过。

缓存key至少tenant/actor/permission_revision/resource/revision/query；操作成功精确失效，不全站invalidate。WS只是失效提示，以GET为准。敏感正文默认不进Redis，客户端只当前身份内存与受限IndexedDB。读返回的数据时间和revision真实，聚合统计延迟用as_of标明，不用0代替错误。

## 6. 重构导入旧库

目标是全新schema及UUID，不维持旧API。旧库冻结只读快照，mapping(old_table,old_id,new_id,tenant_id)记录映射；先users/tenant再作品/卷章/版本/设定/大纲/事实，校验行数、引用、正文hash和孤立资产。非法旧事实留迁移报告，不悄悄删掉。迁移工具单独验收，不以DDL文档存在即运行生产迁移。

## 7. 可复现验收

DB-01 空库按顺序建表且全部约束可查询；DB-02 双窗口CAS第二写409；DB-03版本插入故障回滚正文；DB-04跨域确认任一仓储故障无半账；DB-05重复幂等/导入item只有一份；DB-06跨租户同ID请求不泄漏；DB-07删除30天恢复和过期擦除按固定时钟复现；DB-08事件/用量投影重放收敛；DB-09API全部例子通过schema而非法payload被拒；DB-10游标不能在另一个scope复用。
