# Agent Note: root作为管理员邮箱账户的唯一登录别名

Status: implemented

Task: INK-233
Scope: platform_users身份Schema、platform identity Login、cmd/admin凭据绑定

## Problem

R1登录只接受邮箱，Admin不能使用root用户名；硬编码root到邮箱的环境映射没有数据库唯一性，也无法与绑定账号的operator角色一起原子校验。直接创建跨租户super_admin会扩大用户没有要求的权限范围。

## Decision

按[ADR-0029](../../reference/decisions/0029-root-username-admin-login.md)，platform_users增加可空且唯一的username列，R1目前只允许值root。Login严格要求email或username二选一；Admin username root查找绑定用户后仍执行原audience、活动operator、tenant校验。Creator仍只允许email。首次`admin bootstrap -username root`可直接绑定；现有账号用`admin set-root -email EMAIL`绑定root、从`INKLORE_ROOT_PASSWORD`更新bcrypt密码并吊销所有旧session。CLI不把密码写入参数或输出。

## Alternatives considered

静态环境变量把root映射到某邮箱配置快，但缺少账号表级唯一约束和可靠的账号停用语义。新增全局super_admin可提供跨租户操作，但会越过R1 operator/auditor边界。选数据库唯一别名，复用operator角色与租户权限链。

## Consequences

管理员仍可用原邮箱登录；root别名唯一绑定一个数据库用户。新迁移000012为既有行写入NULL。调用`set-root`会改变目标用户密码并吊销其所有会话。2026-09-27检查时本地DB有2个活动operator但root_aliases=0，用户提供的邮箱未命中，故当时绑定事务未提交、未改动任何密码或会话。后续用户明确改为新建跨tenant全局管理员，见替代范围 ADR-0030 与 INK-234；新账号现以DBX插入。

## Verification

`go build ./...`、`go vet ./...`、Creator/Admin 生产构建、`node scripts/build-request-schemas.cjs --check`与`go run ./cmd/archguard`通过。2026-09-28真实HTTP验证 `username=root` Admin、email Admin、email Creator 均返回200；Admin浏览器表单用 root 标识通过客户端校验并发出请求。上述DBX新账号拥有唯一root alias；`set-root`对已有operator的重新绑定分支没有再次执行验证。`check-design.cjs`仍报告工作区冻结资产漂移、超限 platform API 与未登记的 Agent Schema。
