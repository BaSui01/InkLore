# ADR-0030：root全局管理员与Creator双权限

- **状态**：Accepted；用户明确要求root跨tenant管理并同时拥有普通用户权限
- **日期**：2026-09-28
- **决策者**：BaSui
- **关联任务**：INK-234
- **替代关系**：替代ADR-0029中“不新增super_admin角色”这一范围；保留email/username=root双凭据登录。

## 背景（Context）

R1 Admin身份由tenant成员关系中的operator/auditor授权，Creator使用同一platform user但只授予owner scope。用户要求新建root系统管理员，既能跨tenant执行Admin操作，也能作为普通Creator用户登录。仅直接写入一个tenant membership不能覆盖多个租户；放开全量Repo tenant过滤又会破坏数据库隔离与作品owner校验。

## 决策（Decision）

platform user增加仅含`super_admin`的可空全局角色集合；成员membership仍只保存creator/operator/auditor。Admin会话始终绑定一个tenant，root通过Admin端tenant catalog查看活动tenant并执行CSRF保护的会话切换。每个tenant中的root membership包含creator/operator/auditor，使其在当前tenant使用既有应用服务；Creator接口仍以root自己的owner_id过滤，不读取他人的正文或作品。业务请求与Repo继续强制一个明确tenant scope，不增加tenant通配查询。

现有及以后创建的tenant必须拥有root membership；新tenant创建时，在同一事务内为所有启用的global super_admin追加membership。root登录后默认选择一个具备相应audience角色的tenant；切换会在事务内核验全局角色与目标tenant，吊销旧session并创建绑定目标tenant的新session。Admin上下文按tenant ID重挂数据视图并刷新真实API。

初始root用户通过DBX事务直接写入email、username、bcrypt哈希、super_admin全局角色、所有现有活动tenant的三种membership与安全审计；明文密码只在本机通过隐藏输入交给Go bcrypt计算，不出现在SQL、DBX日志、源码或参数中。username=root仍只在Admin audience接受，email在Admin/Creator均可按成员身份登录。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 为root放开所有API与Repo的tenant过滤 | root登录后无需切换tenant | 全量列表和多对象写会隐式跨租户，owner与scope审计难以逐项维护 | 采用显式tenant切换，每个请求仍由现有tenant scope约束 |
| 把root设为单一tenant operator/creator | 复用现有会话和角色，改动很小 | 不满足跨tenant管理要求 | 用户明确要求全局跨tenant管理员 |
| 只加super_admin角色但不给Creator membership | 可统一管理租户运维 | 用户无法以同一身份使用普通创作者端 | 按用户要求同时 provision Creator membership，仍保留owner scope |

## 后果（Consequences）

**正面**

- root可在Admin端切换所有活动tenant，并通过邮箱在Creator端使用自己的作品空间。
- tenant切换更新HttpOnly refresh session、权限快照与前端数据作用域，不跨tenant复用缓存。
- 新注册creator tenant与CLI bootstrap tenant都会自动给root追加membership。

**负面 / 代价**

- 新增全局权限字段、一次向前迁移、两项API操作及双端session/租户选择接线。
- 全局root可改变每个tenant的模型、成员及运维配置；其账号被接管的影响显著大于tenant operator。
- 普通creator注册每次会增加一条root membership和最小审计/存储成本。

**需要长期承担的义务**

- 所有tenant创建路径必须同事务写入有效super_admin memberships；失败时tenant创建整体回滚。
- super_admin只能扩大Admin角色判定和tenant目录/切换能力，不得关闭tenant SQL过滤、资源owner校验或跨tenant404规则。
- global_roles变更须提升user revision并吊销全部session；tenant switch须校验CSRF/Origin、租户active和最新global role，并撤销旧session。
- root的Creator访问仍只能操作自身owner资源；不允许用global Admin身份读取其他作者正文、事实账本、run prompt或私有模型凭据。

## 验证方式（Verification）

执行000013迁移后核对DBX中唯一root email/username、super_admin global role、所有活动tenant memberships及审计记录；测试Admin email/username登录、Creator email登录、tenant catalog及切换、旧session失效、跨tenant直接ID拒绝、Creator owner隔离、更新global role后即时撤权以及新tenant自动membership。运行Go与双端构建、请求Schema嵌入检查、archguard、i18n和工程笔记检查。不新增测试文件；测试口令只经终端隐藏输入并仅保存bcrypt哈希。
