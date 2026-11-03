# Agent Note: Supabase 远程预检的 IPv4 Session Pooler 回退

Status: implemented

Task: INK-261
Scope: scripts/ci/supabase_staging_preflight.py 与 .github/workflows/supabase-staging-preflight.yml

## Problem

2026-11-03 已配置的 GitHub supabase-staging Environment Secret 在原 workflow 成功注入并通过 TLS 格式检查；真实 private Actions #41570140594 在 psql 阶段报 direct db.rjxtggttklpqbfjesrgp.supabase.co 的 IPv6 Network is unreachable。此项错误发生在密码验证之前，并不代表凭据错误或数据库离线。Supabase 官方文档说明免费计划直连通常为 IPv6，共享 Session Pooler 为 IPv4。

## Decision

保留 GitHub Environment 的受保护分支策略及现有数据库密钥，不把密码回写代码、PR、日志或上传的文件。在可信 workflow_dispatch 的 GitHub-hosted Runner 内解析仅供执行时使用的 `STAGING_DSN`，严格验证数据库 host、项目 ID、角色名、端口、TLS 参数。仅对 Supabase 官方域下有限的 IPv4 Session Pooler 地址建立真正的只读连接，以 `current_user`、`current_database`、17 个 R1 迁移、80 张私有表、所需扩展和角色权限证明目标正确，认证失败则保守拒绝。密码只能经子进程环境 `PGPASSWORD` 传递，禁止日志显示 DSN 和 psql stderr。

Session Pooler 实际集群索引不能从区域推断；有限候选只是认证验证过程，不代表可以按区域自动认定正确主机。若没有一个通过，必须回到 Supabase Dashboard > Connect 获取准确 Pooler 主机，再调整配置。未开启付费 IPv4 Add-on，也未更改 Supabase 权限。

## Alternatives considered

- 为 GitHub Runner 购买 Supabase IPv4 Add-on：产生持续费用，当前没必要。
- 把管理员账号或公开服务密钥替换到 GitHub Secret：违反权限边界，拒绝。
- 将密码输出为日志并在其他机器手工测试：造成明文泄露，拒绝。
- 外部第三方 IPv6 代理：引入未审核的密钥信任中介，拒绝。

## Consequences

普通 PR 不读取 Supabase Secret；只在受保护分支人工触发的远程预检解析凭据。代码属于 CI/凭据信任边界，需走人工审查和分支保护。不会永久改变数据库网络、角色或数据，且不将曾经的 IPv6 失败视为密码错误。

## Verification

GitHub Actions #41570140594：Confirm credentials exist 成功；Install PostgreSQL client 成功；Verify isolated Supabase schema without mutations 因直连 IPv6 无路由失败。Supabase MCP 已独立核验项目 ACTIVE_HEALTHY 与只读角色权限；GitHub Secret 列表确认变量已存在。修复后的 IPv4 运行结果要以合并后最新 Actions 为准，不能仅凭静态提交声明连接通过。
