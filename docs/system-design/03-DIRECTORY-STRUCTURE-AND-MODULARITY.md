# 03 · 代码目录、依赖和复用边界

状态：R1冻结设计；现仓库尚未完成重构。目录树是目标，不是对当前文件存在的声明。

~~~text
cmd/server/main.go                 HTTP/Worker启动，只有组合根调用
cmd/migrate/main.go                版本化迁移
cmd/admin/main.go                  引导租户/账号/密码重置/会话吊销
cmd/archguard/main.go              分层与安全静态守卫
internal/app/                     Config加载、生命周期、组件装配
internal/app/adapters/            各消费方Port的跨域适配
internal/platform/{auth,scope,unitofwork,jobs,outbox,files,...}/
internal/{content,outline,worldview,inspiration,compliance,agent,llm}/
  doc.go                          Owner、可用Port、禁止依赖
  transport/http/                 router/request/response/handler
  application/                    用例、DTO、ports、事务编排
  domain/                         聚合、状态、值对象、错误
  infrastructure/{persistence,provider}/
frontend/apps/creator/            独立index/Vite/main/router/pages
frontend/apps/admin/              独立index/Vite/main/router/pages
frontend/domains/{七域}/           API映射/查询/控制器/业务组件
frontend/shared/{api,stream,ui,schema,i18n,theme,auth}/
frontend/package.json             workspace命令、锁版本依赖
migrations/                       实施后增量迁移，设计SQL不直接生产执行
~~~

## 1. 允许依赖表

| 发起层 | 可以依赖 | 禁止 |
| --- | --- | --- |
| transport | 本域application、platform认证/response | 直接SQL、他域transport、模型SDK |
| application | 本域domain/ports、platform技术接口 | 他域infrastructure、读取他域私表 |
| domain | 标准库、稳定值对象；必要platform错误类型 | Gin、GORM、SDK、路由DTO |
| infrastructure | 本域domain/application ports、platform适配 | 反向调用transport、私自业务审批 |
| app/adapters | 双方application ports/公开domain值类型 | 把私库句柄传给调用方 |
| frontend shared | 基础库、shared内部下层 | domains、apps、业务实体查询 |
| frontend domain | shared、本域控制器/公开跨域读DTO | 任一app入口、另一域内部state |
| frontend app | 本端路由/页面组合、domains/shared | 另一app入口/页面 |

跨域依赖DAG按00，静态守卫按导入而非目录名猜测；跨域只读值对象是明确例外，写命令仍由消费方Port，不能传ORM模型。

## 2. 文件拆分与命名

Go目录snake/lowercase，类型PascalCase，schema字段snake_case；HTTPDTO与聚合不得一个struct兼用。Go手写单文件<50000字节，按用例保存/确认/导出拆，不把一个handler文件装全部路由。React组件PascalCase、hooks useX、查询keys集中queryKeys.ts。JSON Schema由API类型生成供前端读取，生成物标头和生成命令，不同时手写第二份DTO。

每域最低文件职责：router.go只注册；request_*.go做字段绑定；*_service.go用例；ports/接口命名按需要；repository_*.go按聚合SQL；errors.go机器码与HTTP无关；error_mapper.go在transport映射。源码注释解释不变量/事务边界，不抄一份完整设计。

## 3. 复用决策表

| 问题 | 一次实现的位置 | 领域负责的部分 |
| --- | --- | --- |
| 解码、错误封包、request/trace | platform/httpguard/response | 领域code映射与字段约束 |
| UUID/分页/时间 | platform/types/cursor/timeutil | 默认排序与业务自然日口径 |
| CAS/幂等/事务 | platform/unitofwork/idempotency | resource revision更新、业务快照 |
| 后台队列/租约/outbox | platform/jobs/outbox | handler任务、节点状态、补偿规则 |
| 文件大小/hash/下载 | platform/files | 来源资产、导入分章、导出格式 |
| 模型调用/预算/限流 | llm GatewayPort | Agent编排、业务输出schema |
| 规则schema校验 | platform/schema | 工具/模型/表单允许字段和业务校验 |
| scope投影与授权 | platform/scope | Owner在同事务登记资源和修订 |
| 组件状态、对话框、表单、流解析 | frontend/shared | 业务页面组合与domain控制器 |

禁止把“大纲与正文都含title”作为共用聚合理由；禁止用any/JSONB作为省略领域建模手段；工具arguments/results可以动态schema但必须绑定版本并校验。

## 4. 组件装配与生命周期

app.New读取配置→验证PG/签名与SecretRef→初始化DB/Redis/SchemaCompiler→构造platform服务→按DAG构造域服务→装配Port→注册路由/worker handler→ready=true。Close停止接收任务、取消/交接lease、排空事件写入、关HTTP/DB。不得在包init建立连接或注册有副作用后台协程；所有goroutine经safego+context，panic留审计但不吞失败终态。

前端路由懒加载域页面，共享包tree-shake；Admin bundle不包含编辑器页面，Creator bundle不包含Key/运维页面。相同UI组件复用源代码而不是共享一个运行应用壳。检查产物文件和直接URL访问，不能仅检查侧栏按钮。
