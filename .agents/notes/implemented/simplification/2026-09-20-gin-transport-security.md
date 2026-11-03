# Agent Note: Gin 入口安全与旧链删除

Status: implemented

Task: INK-192
Scope: 七域 HTTP Transport、app HTTP 装配、Agent 幂等授权与旧广播链

## Problem

Agent 缺 Identity 时曾信任身份头，缺幂等键时直接执行；各域 Body/反射解析和旧响应封包并存。旧 realtime 只有未装配适配器引用。已检索平台原子命令、身份锁序和worldview重放笔记，它们关于当前授权与CAS必须分离的约束继续有效。

## Decision

正式 Agent 构造要求全套依赖，命令只经幂等 Executor。回放前和事务内按当前会话/角色授权，资源授权通过消费方 TaskResourcePort.AuthorizeResource 由 app 使用 scope 与领域只读用例适配。它不读取旧 source_revision，避免CAS阻断合法回放。治理硬删除的回放仅检查当前租户operator，目标查询和CAS留给首次执行，幂等键自身绑定租户/actor/path。

七域 JSON 都调用 httpguard；LLM 模型输出只复用纯解析并映射原错误。Inspiration 命令改为只序列化data，封包由Executor完成，避免双层成功封包。Gin 原生Allow补OPTIONS；关联、白名单日志、无请求转储Recovery、精确CORS有序接线。默认不信任代理，配置非法CIDR拒绝启动；登录与注册现有路径同用ClientIP。

删除旧 response/json、middleware/auth、realtime hub/websocket/adapter及无消费者Go i18n；旧runtime broadcaster参数/Port一并删除。platform/auth仍被Agent运行主体与tokengen消费，不能随旧Gin认证一起删除；正式HTTP只使用identity。

## Alternatives considered

保留缺依赖fallback方便局部装配，但构造错误将成为提权入口。通过GetDefinition等目标查询授权看似更严格，却会让硬删除后的合法重放404；当前operator与幂等租户作用域已经界定管理写权限。复用Gin默认Recovery虽少代码，会转储请求/错误全文，因此关闭writer并只记固定字段。

## Consequences

重复入口被删除，当前撤权检查不依赖占位资源快照。真实模型/队列/文件适配器中的既有占位能力另登记INK-204，不以本轮HTTP验证代替其运行证据。认证扩展与冻结契约冲突尚待用户裁决，完整App现按缺失映射失败关闭。

## Verification

`go build ./...`、`go vet ./...`、archguard、notes/skills、Schema生成一致性通过。75个真实Gin组件请求与代理场景通过；依赖比较确认server删除realtime/i18n、新增requestschema/strictjson。隔离PG16迁移成功，App启动拒绝未声明captcha路由。真实PG组件23项通过，覆盖幂等事务/撤权/SSE续传/断线和底层FlushError。SSE必须控制Unwrap后的writer，Gin无错误Flush会遮蔽网络错误。EventService每次轮询/心跳重验RunAuthorizer，Artifact和审批读取也复用，列表跳过已无权资源并保持游标推进。完整事务与SSE证据见 [验收记录](../testing/2026-09-20-acceptance-ink-203.md)，未验收项保留review，不标全部完成。

本轮完整App只读PG17启动通过；中间件实测发现FromContext仅输出trace_id，因此访问与panic日志显式增加经Contract校验的request_id白名单字段。探针同时验证原始路径参数、查询、正文、Cookie、Authorization和panic文本不出现在日志，流响应后panic不追加JSON。详INK-203增量证据。

独立审阅追加：Agent/文件无体DELETE此前忽略body，Agent Query还接受重复revision。现统一使用RequireEmptyBody和严格revision查询检查；SSE移除未声明last_event_id查询后备，前端无该消费者。公共guard七场景通过，完整带身份命令仍等待隔离数据库验收。
