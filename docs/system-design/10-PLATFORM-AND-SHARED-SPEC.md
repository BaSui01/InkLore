# 10 · 公共基座实现设计

状态R1冻结，未实现。详见[全局契约](00-CONTRACT-BASELINE.md)、[平台DDL](data/platform.sql)、[平台接口](api/platform.json)。platform是技术Owner，不拥有章节、大纲、模型策略等业务。

## 1. 服务与Port契约

| 能力 | 输入→输出 | 核心不变量 | 依赖/失败 |
| --- | --- | --- | --- |
| Auth.Authenticate | bearer,audience→Principal(tenant,user,roles,session) | 校验签名/过期/session撤销/成员状态，roles不单信JWT | PG，auth.unauthenticated |
| Scope.Assert | Principal,resource ID,action→Scope | 同tenant+owner，明确管理资源依role；不可见404 | PG投影，platform.resource_not_found |
| Scope.AssertChapterSnapshot / AssertResourceRevision | tenant,actor,kind,resource,revision,hash→SnapshotProof | 当前权限有效且历史revision/hash存在；返回content_length码点长度，不返回正文或另造receipt_id | platform_resource_revisions |
| UnitOfWork.Within | ctx,fn(txctx)→error | 所有域仓储复用同连接事务；嵌套复用，不自行commit | PG，不在事务内外呼 |
| Idempotency.Execute | actor,method,path,key,request,fn→cached result | 授权在缓存之前；同键异载荷拒绝；幂等+业务同commit | PG，platform.idempotency_conflict |
| FileStore.Put/Open/Delete | scope,stream,metadata→FileDescriptor/reader | 输入20MiB输出100MiB，hash/类型/授权；不返回内部URL | PGbytea |
| Jobs.Enqueue/Claim/Heartbeat/Suspend/Wake/Complete | domain,kind,resource,key→JobLease | lease30秒/心跳10秒；Complete只接受内部JobLease或AuthorizedCommand proof，不能由HTTP输入伪造 | PG SKIP LOCKED |
| Outbox.Publish/Consume | aggregate revision,event→event ID | 同业务事务；consumer inbox去重 | PG，投影可恢复 |
| Schema.Compile/Validate | schemaDraft2020-12,hash,value→field errors | 禁网络$ref，只已注册本地schema；编译按hash缓存 | 类型/required/enum/additional全部校验 |
| Cursor.Encode/Decode | actor,filter,sort,key→opaque | HMAC签名，绑定作用域，24小时有效 | 环境密钥；非法400 |
| Clock.Now/BeijingDay | instant→UTC/北京日边界 | 不用系统本地时区 | 标准库 |

领域通过Port使用能力；PG事务对象只在platform/infrastructure解析context，不暴露到domain结构体。错误要包装cause用于安全日志，不直接当HTTP message。

## 2. 身份会话与成员

部署通过cmd/admin bootstrap创建tenant+首个operator，密码交互stdin或短期环境，不在参数/日志。UI允许按ADR-0021公开注册creator独立租户。POSTauth/login收email或username=root、password、tenant_id、audience；账号不存在、密码错、非成员统一401且近似时延；IP+账号摘要Redis滑窗5次/分钟，Redis故障登录使用进程内严格限流但不影响既有session读取，AI配额仍failclosed。

登录返回access_token(15分钟)、csrf_token和Principal；服务端Set-Cookie对应audience的刷新token。浏览器仅内存存access与csrf，401只允许单飞刷新一次，刷新失败保留未保存文本但清除服务端缓存并进入登录覆盖层，不自动重复写命令。登录成功后旧界面重新GET后才能合并保存。

刷新令牌每次轮换：FOR UPDATE锁refresh row，已used则吊销session并401；未使用则插新token hash、标old used/replaced_by、提交后Set-Cookie。响应丢失的旧token不能无限重放，用户重新登录；安全优先于透明重试。csrf不跨session复用；刷新页面初次恢复需要GETauth/session-csrf（同源Origin/Sec-Fetch-Site检查，按HttpOnlycookie解析session返回nonce；nonce固定由独立CSRF密钥HMAC(session_id,audience,created_at)派生，库仅hash用于核验），再POSTrefresh。API冻结采用GETsession-csrf，禁止先读localStorage token。logout吊销会话及后代refresh，清cookie，幂等200；两端session分开退出，全部会话吊销由成员禁用触发。

成员管理operator可创建本租户账号/成员、改roles/状态、重置密码（一次临时密码只显示一次，不写审计）。禁止最后一个active operator被移除/降权；锁tenant后统计检查。禁止operator给自己增加正文访问权限；creator scope只有本人新建/显式转移的作品，R1不提供转移命令。成员信息对auditor只读，creator仅自己。管理员密码重置仅允许账号只有一个active租户成员关系，否则403 auth.cross_tenant_account，改走本人改密或部署CLI；创建成员不允许复用其他租户已有email，409 auth.account_unavailable，避免越租户接管。密码重置或本人改密吊销该用户全部会话，必须重新登录；不发电子邮件，不新增SMTP依赖。

## 3. 授权投影

每个业务聚合创建时同UoW UpsertScope(resource_kind,owner,novel,revision,hash)。章/大纲节点保存必须显式提交AppendRevision(resource_id,revision,hash,codepoint_length)，字段无默认值，含空白的码点长度只在真实空正文才为0；与正文/节点相同UoW追加resource_revisions；分析报告可引用历史revision，但必须拥有当前章的读取权限。soft-delete同时Scope.deleted_at，恢复同ID的新scope revision；过期清除正文时清除历史hash投影可返回不可用，不能允许已删除作品通过report正文泄露。生成运行持有输入snapshot的删除按13保留策略清除，安全审计仅留ID和操作事实。

SQL条件必须tenant_id + resource_id + owner允许，不通过前端传user_id决定owner。Repo方法强制ctx，批量IN查询每项都scope过滤。跨scope404；管理动作权限不足403。下载文件、SSE重连、WS票据消费、幂等结果重放和缓存命中都重新授权。

## 4. 文件

上传multipart(file,purpose,novel_id可选)；接收流计数，超过20MiB立即413。允许TXT UTF8/UTF16 BOM、DOCX application/vnd.openxmlformats-officedocument.wordprocessingml.document、JSON（仅明确导入工具）；不允许SVG/HTML作为任意内联内容。检测magic+扩展+mime，DOCX禁宏/外部关系，解压后100MiB/10000条、压缩比≤100；path traversal拒绝。文件名去路径/控制符，download Content-Disposition attachment、nosniff，禁止按上传MIME直接内联HTML。

上传只创建FileDescriptor不创建Novel；任务引用必须同tenant/owner与purpose。无业务引用的输入文件24h清理；已绑定来源保留至资源清除；生成导出7天到期可重新生成。下载GET按权限返回二进制，错误仍JSON封包；客户端按Content-Type和HTTP判断，不能下载错误JSON成DOCX。Range支持单段bytes，非法416 platform.range_invalid；ETag为sha256不暴露原文，If-None-Match只用于已鉴权读。

## 5. 幂等与outbox

规范化JSON键排序、去无关空白，上传摘要含文件SHA256/元数据；HMAC-SHA256密钥独立环境注入。Idempotency-Key必须UUID；scope/path包含实际ID，query仅保留影响语义字段（expected_revision包括在hash）。事务claim唯一索引；并发读未完成409+Retry-After。成功封包保留原request_id/trace_id并加Idempotent-Replay:true，新请求关联号留响应header便于追踪。请求校验前的400/401不缓存；执行后确定性409/422可写completed，5xx回滚不缓存；审批来源失效采用CommandOutcome(commit=true,public_error=409)先提交票据/节点/run失效及事件，再缓存409，不能throw导致失效回滚。其它确定性校验失败无业务写；外部副作用用领域ExecutionReceipt另查不自动重做。身份会话端点不用通用幂等缓存，避免明文token进响应缓存。

outbox消费者最多8次，1/2/4/8/16/32/60/60秒退避；失败标dead_letter并告警，operator可读问题但R1 UI不提供任意重放载荷。重新派发由CLI按event_id执行既定consumer，写审计。订阅WS只用安全ID/topic/revision，正文读取仍走授权HTTP。

## 6. 验收

PL-01 两aud令牌交叉访问返回403且无业务写；PL-02 跨租户文件/幂等/SSE均404；PL-03 refresh重复使用吊销session，日志无token；PL-04 当前scope与快照hash不一致拒绝报告；PL-05 重复保存同幂等键只一version；PL-06 kill worker后lease重领且fencing旧worker不能完成；PL-07 zipbomb/路径穿越/超限文件拒绝且无孤儿正文；PL-08 移除最后operator409；PL-09 私有ref schema不能联网；PL-10 PG故障不内存降级；PL-11 outbox重复投递投影不重复；PL-12 任一API错误响应无SQL、token或正文。
