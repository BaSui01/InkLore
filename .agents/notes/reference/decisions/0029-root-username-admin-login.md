# ADR-0029：管理员邮箱与root用户名双凭据登录

- **状态**：Accepted；按用户明确要求支持邮箱和root用户名登录
- **日期**：2026-09-27
- **决策者**：BaSui
- **关联任务**：INK-233
- **替代关系**：补充 ADR-0016/0021 的登录标识约束；不改变租户、角色或授权范围。

## 背景（Context）

R1账号以全局唯一邮箱标识，Admin audience按每个活动租户成员关系授予operator或auditor权限。用户要求管理员可使用用户名`root`或现有邮箱登录。仅让前端接受`root`会被登录Schema和Go邮箱解析拒绝；把root直接实现为全局超级角色则会绕过现有租户边界。

## 决策（Decision）

在platform用户增加可空、全局唯一的username列；R1当前仅允许管理员用户名`root`。登录请求恰好携带email或username之一，email仍是账户恢复与成员管理的规范地址。root用户名只在Admin audience可用，解析到绑定该用户名的现有platform用户后，继续执行原operator/auditor成员权限与tenant scope校验。CLI提供新管理员bootstrap绑定root，以及将root绑定到现有operator邮箱并设置密码的部署命令；密码仅由调用者经进程环境注入，不写入源码、参数或日志。公开Creator注册不接收username。

该决定创建普通operator身份，不新增`super_admin`角色、不授予跨租户访问，也不预置任何默认root账户或密码。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 仅用配置把字符串root映射到指定email | 无需扩展账户表，部署者可以改映射 | 每个请求依赖环境映射，别名唯一性与账号状态脱离数据库；成员身份变更难审计 | root是持久账号标识，应由platform用户表唯一约束管理 |
| 创建全局super_admin并绕过tenant成员角色 | 能跨租户管理，类似传统root权限 | 改变R1角色与租户边界，影响所有资源授权和审计路径 | 用户要求root作为用户名，未要求跨租户权限；沿用operator授权 |

## 后果（Consequences）

**正面**

- Admin登录既兼容现有email字段，也可用唯一username=root；邮箱登录方式不变。
- 数据库唯一索引消除重名root；username为NULL的既有账户无需迁移数据。
- 密码由部署CLI只通过环境变量接收，成功后hash入库并吊销旧session。

**负面 / 代价**

- 增加一次向前数据库迁移，并须更新严格请求Schema、嵌入Schema、成员装配与Admin表单。
- root被占用或对应账户不再拥有活动operator身份时，用户名登录失败关闭；改绑需重新执行受控CLI。

**需要长期承担的义务**

- root别名只能绑定已有operator成员的用户；每次登录仍执行常规tenant、membership与audience校验。
- 不得在seed、启动器、源码、文档或数据库中存明文默认口令；生产密码必须从Secret Store/环境注入。
- 任何新增普通用户名需单独裁决，不得隐式把root扩成全局权限角色。

## 验证方式（Verification）

校验email登录与username=root登录均成功使用同一用户密码；Creator audience使用root、username和email+username双字段均被拒；没有root alias、非operator成员、跨租户或audience错误均统一凭据失败；重复root分配被唯一索引拒绝。验证`go build ./...`、严格Schema生成/编译、双端构建和archguard。隔离PG迁移、邮箱/用户名真实HTTP请求和会话撤销须记录实测；不新增测试文件或仓库内默认凭据。
