# INK-161 · R1.1后端身份与审计验收

日期：2026-09-18，北京时间，未提交工作区。用户在无冲突验收后明确批准四项最小勘误，[ADR-0018](decisions/0018-r1-minimal-contract-errata.md)生效。本记录覆盖INK-161的15个后端操作；状态唯一来源为[backlog](backlog.md)。下载416定义已补，实际文件下载归INK-162；真实Chrome与产品前端仍有独立未验证边界。

## 运行路径

cmd/server → app.New → R1组合根，除技术探针外现挂载15个业务操作：login、session-csrf、refresh、logout、me、ws-ticket、本人改密，成员list/get/create/update/reset-password/revoke-sessions，以及审计list/export。本人改密新增到真实幂等与UoW链，导出走当前权限与全量安全缓冲；NoMethod返回JSON405与Allow。旧业务路由、公开注册未挂载；WS连接与消费归INK-162。

新增 [identity](../internal/platform/identity/types.go)、[membership](../internal/platform/membership/types.go)、[audit](../internal/platform/audit/types.go)、[cursor](../internal/platform/cursor/cursor.go)均由 [app](../internal/app/platform.go)装配。身份与成员命令复用INK-160的真实PG UoW；成员命令经幂等执行器，安全审计与业务同提交。旧代码/旧import尚未全部退役。

## 可复现命令

~~~powershell
docker run --detach --rm --name inklore-r1-acceptance-161 --publish 127.0.0.1:55432:5432 --env POSTGRES_HOST_AUTH_METHOD=trust --env POSTGRES_DB=inklore_r1_acceptance_161 pgvector/pgvector:pg16
docker run --detach --rm --name inklore-r1-redis-161 --publish 127.0.0.1:56379:6379 redis:7-alpine
$env:INKLORE_ACCEPTANCE_DSN='host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable'
go build ./...
go run ./cmd/acceptance/ink161
go run ./cmd/acceptance/ink160
go run ./cmd/archguard
node scripts/check-r1-platform.mjs
node docs/system-design/tools/check-design.cjs
node scripts/check-project-skills.mjs
node scripts/agent-notes.mjs check
git diff --check
docker stop inklore-r1-acceptance-161 inklore-r1-redis-161
~~~

CLI在任何自动迁移前先读current_database并拒绝名称不以inklore_r1_acceptance开头的库。使用随机合成账号、密码、密钥和loopback HTTP；不创建禁用的测试文件、不访问模型、不写用户现有容器。trust仅用于临时回环端口数据库。

## 行为证据

R1.1最终真实PG16、Redis7与HTTP验收输出24组PASS，退出0；15个已挂操作全部有真实成功响应覆盖，JSON按完整Draft2020-12、NDJSON逐行按Audit验证。INK-160的26组回归同时通过。每组含多项断言：

| 范围 | 已验证行为 |
| --- | --- |
| 响应契约 | 使用冻结基线已选定的jsonschema/v6 v6.0.3，逐条校验真实响应；15操作均有成功覆盖；缺字段/额外封闭字段/错类型/非法UUID与日期/数值边界反例被拒；未知及暂缓路由404不计成功覆盖 |
| 代理Cookie容器 | 独立3000/3001代理真实后端，4组HTTP脚本验收通过：双Cookie、无Origin恢复/跨aud拒绝、轮换/重复退出、旧refresh重放吊销；手工请求头不算真实浏览器证据 |
| 双aud/令牌 | 两个独立HttpOnly、SameSite=Strict、Path=/v1/auth Cookie；refresh明文不进JSON；JWT改签拒绝；creator不能进入admin资源 |
| 同源恢复 | session-csrf缺Origin且same-origin+正确Host或Referer可恢复；跨aud、wrong Origin、same-site/cross-site、缺metadata拒绝；POST不继承GET特例 |
| refresh/CSRF | 固定nonce可恢复，错误CSRF/Origin不轮换；成功换token不换session；旧token重放401后新access也401，另一aud会话仍有效 |
| WS票据 | 冻结audience请求体成功，缺字段400、交叉aud403；库只存hash且有效期60秒 |
| logout | 清Cookie、旧access失效；无Cookie重复退出仍200 |
| 登录失败 | 未知账号、错密、非成员同401/code，近似时延由每次bcrypt12比较保证，未做统计时序侧信道测量 |
| 成员保护 | 最后operator不能降权；确定性409可重放；operator不能给自己增加creator |
| 重置与隔离 | 单活跃租户重置使两aud会话失效；多活跃租户403且user revision不变；跨tenant成员404；普通revoke仅本tenant，另一tenant仍可用 |
| 分页与权限 | 真实updated_at/id keyset不重复；HMAC游标拒篡改/跨actor；auditor只读 |
| 并发最后operator | 两个operator同时自降权恰一成功、一409，PG至少留一个active operator |
| 审计 | 安全字段落库与读取，created_at/id keyset、filter绑定游标；refresh重放有审计；正文/密码/令牌不进安全变化；password字段被拒 |
| 本人改密 | 缺头/缺当前密码400、错密401、user revision冲突409可缓存、同键异载荷409；成功一次递增并吊销跨tenant/aud所有会话；旧JWT不可重放，新会话同载荷重放且保持新会话有效 |
| 改密故障与竞争 | 注入审计写失败后密码/hash/revision及全部会话均回滚；两个会话并发改密恰一200一401、无锁升级死锁、只提交一次 |
| 审计导出 | NDJSON逐行Schema；改密事件仅一次且无密码、resource_id为user；32天或10001条422无文件头，10000条完整返回；默认31天且与90天可见窗相交 |
| 未声明方法 | 已知普通及参数路径405标准封包，Allow包含实际注册方法与OPTIONS；同路径OPTIONS200，未知路径PATCH404 |
| 限流 | Redis正常时第6次同IP/账号失败429带Retry-After；Redis连接拒绝时内存仍5/min，已有会话读取不受影响 |
| 锁序 | 两tenant的operator互为目标用户并发写均成功；刷新与禁用竞争无死锁，最终旧JWT失效、session已吊销 |

Redis故障反例会输出指向本机拒绝端口的连接错误，不含密码、访问令牌或用户正文。所有密码均为临时合成输入，不在证据中列出。

## 新增契约与浏览器验收入口

- 离线验证器反例：`go run ./cmd/acceptance/contracts/check`；只预加载三个冻结本地JSON，禁止网络和外部文件引用，不输出响应值或凭据。该包只被验收CLI使用，不进入cmd/server运行路径。
- 浏览器诊断入口及两控制台启动步骤见 [fixture README](../cmd/acceptance/ink161-browser/README.md)。主代理已启动该二进制并验证真实代理链。
- 启动fixture后在另一控制台执行 `pwsh -File scripts/accept-ink161-browser-http.ps1`，4组通过；这是带真实Cookie容器的HTTP模拟，不能当Chrome证据。每端口登录入口只可消费一次，之后要做浏览器验收须重启fixture。
- 初次HTTP模拟失败源于PowerShell WebRequestSession保留上个端口的Origin；改为每次清空自定义头后4组通过。没有因此修改后端授权。
- Chrome连接不可用时不修改用户profile或假称浏览器通过；验收入口不属于产品页面，也不在正式server挂载。

## 部署bootstrap

[cmd/admin](../cmd/admin/main.go)提供部署专用bootstrap；先执行R1迁移，再从stdin或短期INKLORE_BOOTSTRAP_PASSWORD接收密码，参数不接受密码。创建tenant、首个operator和安全审计同事务提交；stdout仅tenant/user/member ID。完整命令实测通过，不只是调用库函数。

进程环境沿用INK-160四个独立密钥与两个精确Origin要求。可通过REDIS_ADDR、REDIS_PASSWORD或已加载Redis非密钥地址配置接入Redis；Redis未配置/不可用时登录使用严格进程内滑窗。生产Cookie设Secure；本轮临时HTTP环境仅检验非生产Cookie行为。

## 发现与修复

只读审阅发现ws-ticket最初误用空DTO、无Cookie logout提前401、同源GET不带Origin时无法恢复，均在公共transport修正并加入真实HTTP断言。成员Owner修正跨tenant互为目标用户时的锁顺序：tenant后将actor和target用户按UUID排序锁定，再进行当前身份授权。该顺序已做真实双连接竞争验收。

R1.1审阅发现Allow仅扫描注册路由会漏掉中间件支持的OPTIONS，已修正并用真实PATCH/OPTIONS对照。幂等原结果按完整JSON语义（含原request_id/trace_id）比较；PG JSONB重排空白和键顺序不作为业务结果变化，未修改冻结JSONB存储。缺少current_password与空字符串区分：前者400，后者按错误当前密码401。

## 剩余边界

INK-161后端交付按18分册的双aud、refresh重放、最后operator、隐私边界逐项验收；浏览器/产品UI在后续前端与端到端门禁继续验证，不以本次后端done抵扣。已完成现有15操作真实响应的Draft2020-12自动校验，不代表全R1的233操作都已验收。冻结成功封包顶层允许额外属性，额外字段反例只证明关闭additionalProperties的typed data与Error对象；没有悄悄收紧冻结Schema。真实浏览器尝试被环境阻断：Chrome连接器找不到DevToolsActivePort，系统中Chrome进程数为0。仍未验证产品前端UI、生产TLS、会话/审计物理清理、Redis集群容量或时序侧信道统计。

成员bcrypt12在幂等外层事务内计算时仍持锁；这有吞吐代价，未以预哈希客户端字段绕过校验。角色变化会吊销本租户会话，访问授权每次重读PG；JWT角色不是权威。审计按90日窗过滤，物理清理属后续平台任务。

工程理由见 [身份与成员锁序笔记](../.agents/notes/implemented/architecture/2026-09-18-r1-identity-revocation-and-lock-order.md)。
