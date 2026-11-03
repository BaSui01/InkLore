# ADR-0031：Creator 登录支持 root 用户名

- **状态**：Accepted；按用户最新明确要求，Creator 登录页也接受用户名与密码
- **日期**：2026-09-28
- **决策者**：BaSui
- **关联任务**：INK-235
- **替代关系**：仅替代 ADR-0029 与 ADR-0030 中“root用户名只限Admin audience”的认证入口范围；全局角色、tenant membership、owner scope 与Creator不能进入Admin的约束不变。

## 背景（Context）

root 已可用用户名/邮箱进入 Admin，也可用邮箱进入 Creator。Creator 页面仍使用 `type=email`，用户输入 `root` 会先被浏览器原生邮箱校验拦截；即使解除前端限制，后端 Login 也只在 Admin audience 接受 username。用户明确要求在当前 Creator 页面使用用户名和密码登录。

## 决策（Decision）

保留唯一 username `root`，允许它与密码在 `admin` 和 `creator` 两种 audience 登录。Admin 仍需要活动tenant中的全局 `super_admin` 或operator/auditor授权；Creator 必须在选定tenant拥有活动creator membership，所有创作资源仍通过现有owner scope授权。username 只解析账号身份，不提升 audience、tenant 或资源权限。

Creator登录表单改用文本型邮箱/用户名标识框，将 `root` 编码为 Login API 的 `username` 字段，其余输入仍作为 `email`。公开注册、找回密码和账号邮箱字段继续只接受email；未更改API字段结构或数据库Schema。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| Creator仅允许邮箱，要求操作员改用Admin端 | 不改变先前audience契约 | 当前Creator页面会继续拒绝用户要求的`root`登录 | 用户明确要求在Creator当前登录页使用用户名和密码 |
| 让任意用户名或Creator username获得Admin能力 | 可扩展更多账户别名 | 混淆登录标识与权限，破坏audience隔离和最小权限 | username仅识别唯一root账户，权限仍取决于活动membership与owner scope |

## 后果（Consequences）

**正面**

- root可在Creator登录页用用户名/密码或邮箱/密码进入自己的作者工作区。
- 既有Creator/Admin成员授权、跨tenant显式切换和Creator作品owner限制不变。

**负面 / 代价**

- Creator登录表单标签由“电子邮箱”变为“电子邮箱或用户名”，而注册/找回仍为邮箱，需清楚区分表单模式。
- root用户名可在两个audience输入，但任一资源请求仍由服务端按session、audience、membership和owner scope校验。

**需要长期承担的义务**

- username仍是保留值`root`，不可创建任意Creator username，也不进入公开注册、密码找回或邮箱绑定字段。
- 创建Creator session前必须查询目标Creator tenant membership；不得用`super_admin`跳过Creator owner scope。
- username存在与否的认证失败语义不得泄露跨tenant账户或作品存在性。

## 验证方式（Verification）

生成Schema确认Login username分支适用于两种audience；真实Creator页面填写`root`通过浏览器验证并发送username请求；真实HTTP验证username Creator、email Creator、username Admin、email Admin均可成功，其他username/creator-only访问Admin失败。验证Creator对非本人作品仍返回跨scope 404，Creator 仍不能进入Admin；运行Go build/vet、双端生产构建、request schema检查、i18n、archguard、notes/skills检查。不新增测试文件或明文口令。
