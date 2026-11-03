# Agent Note: 全局 root 显式切换租户并保留 Creator 所有权

Status: implemented

Task: INK-234
Scope: platform identity、tenant session switch、Creator resource authorization

## Problem

租户 operator 不能跨租户运维；仅在一个 tenant 授予 Creator 权限也不能满足 root 作为普通用户使用 Creator 的要求。直接让全局管理员绕过 tenant scope 会扩大业务资源访问面。

## Decision

按 [ADR-0030](../../reference/decisions/0030-global-root-administrator.md)，platform user 以 `global_roles` 保存 `super_admin`；Admin tenant catalog 只列活动租户，tenant switch 在授权和 CSRF 校验后创建目标 tenant 的新 session 并撤销旧 session。所有 Admin 请求仍绑定一个 tenant。Creator 使用同一邮箱身份，但每个活动 tenant 的 membership 含 `creator/operator/auditor`，Creator 业务仍由现有 owner scope 限制。注册和 CLI bootstrap 在 tenant 创建事务内投影启用的 global admin。

首次 root 账号通过用户指定的 DBX 连接写入；密码只在本机交给平台 bcrypt cost 12 实现生成哈希，SQL 不含明文。用户行、活动 tenant membership 与系统审计事件在一个 DBX 事务中提交。

## Alternatives considered

复用单 tenant operator 无法跨租户管理；放开所有 Repo 的 tenant 过滤会让资源查询隐式跨租户并破坏 owner 授权；只赋 `super_admin` 而没有 Creator membership 则无法使用作者端。显式切换与独立的 Creator owner 校验保留了各自边界。

## Consequences

Admin 全局权限只用于租户目录、切换和现有 Admin 角色能力；访问资源仍需明确 tenant。切换会使旧 Admin session 失效，前端 session epoch 变更并丢弃旧 tenant 缓存。root 的 Creator 身份覆盖各 tenant，但不能读写其他用户的 owner-scoped 创作资源。新 tenant 创建事务为启用的 super admin 增加三种 membership。

## Verification

DBX PostgreSQL 17.11 实测应用迁移 `000013`，并在 `inklore_r1` 单事务创建 `root@basui.com` / `root`；回读确认 `global_roles=[super_admin]`，5 个活动 tenant 各有一条 active membership，且三种角色均存在，每个 tenant 有一条成功系统 bootstrap 审计。HTTP 验收：username=root Admin 登录、email Admin 登录、email Creator 登录均返回200；tenant catalog 返回5项，切换到第二个 tenant 返回200，新身份可用，旧身份 GET `/v1/auth/me` 返回401。5字符口令返回400；6个ASCII和6个中文 Unicode 字符均进入凭据比对，错误口令返回401。Admin 页面中 `root` 通过客户端标识校验后发出登录请求；前端5个中文字符本地拦截、6个放行。root Creator读取同tenant另一用户的既有作品返回404。PostgreSQL回滚探针调用 `ProvisionGlobalAdmins`，确认新tenant获得恰好一条三角色root membership后主动回滚，并回读确认无残留tenant。Go build/vet、Creator/Admin 生产构建、i18n 924/924、Go archguard、请求 Schema生成/检查、工程笔记/Skills检查与`git diff --check`通过。完整 Creator注册HTTP流程和租户停用分支未单独执行；`check-design.cjs`仍因已修改/未登记冻结设计资产及 platform API 超过50KB而失败。
