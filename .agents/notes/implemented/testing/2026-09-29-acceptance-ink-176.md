# Agent Note: INK-176 验收证据：系统可观测性、容灾与核心集成穿透场景验证

Status: implemented

Task: INK-176
Scope: 原 docs/acceptance-INK-176.md 的历史验收证据与迁移

## Problem

该阶段验收记录位于 docs 读者文档目录，与长期有效的系统设计和用户指南混放。

## Decision

将源文档全文迁入工程记忆，保留历史运行路径、验收证据与未覆盖边界。本次只迁移记录，不重新声明旧版本测试在当前部署通过。

## Alternatives considered

继续存放 docs 可保留老路径，但会持续混淆文档职责；直接删除会丢失可追溯的证据。因此采用保留原始记录并同步引用的迁移方案。

## Consequences

相关入站链接需同步修改；旧记录中的验证命令和结果属于历史快照，应以当前代码及 CI 证据为准。

## Verification

内容来自 private 分支原文，仅将原有标题降级以符合笔记结构，并调整相对链接。以下是历史档案，不是新的验收结果。

### 原始验收档案

### INK-176 验收证据：系统可观测性、容灾与核心集成穿透场景验证

状态：done。验收套件 `cmd/acceptance/ink176/` 基于真实 PostgreSQL 16、Redis 7 与组合根环境，完成 13 分册探针、故障降级演练以及 18 项必须穿透的核心集成场景（R1-01 ~ R1-18, OPS-01 ~ OPS-08）。

### 1. 可复现执行命令

PowerShell 执行命令：
```powershell
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go build ./cmd/acceptance/ink176/...
go build ./...
go run ./cmd/acceptance/ink176 all
```

也可以按模块单独运行测试分组：
```powershell
go run ./cmd/acceptance/ink176 probes        # OPS-01 / R1-10 探针与降级
go run ./cmd/acceptance/ink176 isolation     # R1-01 多租户与跨作用域隔离
go run ./cmd/acceptance/ink176 cas           # R1-02 & R1-03 正文 CAS 与事务回滚
go run ./cmd/acceptance/ink176 confirmation  # R1-04 章节确认与事实账本原子一致性
go run ./cmd/acceptance/ink176 agent         # R1-06 & R1-07 智能体运行回放与审批门控
go run ./cmd/acceptance/ink176 security      # R1-16 & R1-18 CSRF双端Cookie与审计脱敏
go run ./cmd/acceptance/ink176 ops           # OPS-03, 04, 06, 07, 08 运维场景
```

### 2. 真实执行输出实录

```text
=== SCENARIO 1: OPS-01 & R1-10 PROBES & DEGRADATION ===
PASS probes_ops01_r1_10 healthz.probe_success
PASS probes_ops01_r1_10 healthz.status_ok
PROBE /healthz status=200 version=dev
PASS probes_ops01_r1_10 readyz.probe_success
PASS probes_ops01_r1_10 readyz.state_ready
PROBE /readyz status=200 state=ready database=ok
PASS probes_ops01_r1_10 pg_outage.refuse_startup_without_pg
PROBE pg_outage startup refusal: database connection required, zero memory store fallback
PASS probes_ops01_r1_10 pg_outage.readyz_returns_503
PROBE pg_outage /readyz returned status=503 (503 expected)
PASS probes_ops01_r1_10 redis_outage.readyz_200_degraded
PASS probes_ops01_r1_10 redis_outage.state_is_degraded
PROBE redis_outage /readyz status=200 state=degraded database=ok
PASS probes_ops01_r1_10 redis_outage.non_ai_crud_available
PASS probes_ops01_r1_10 redis_outage.create_volume_available
PASS probes_ops01_r1_10 redis_outage.create_chapter_available
PROBE redis_outage non-AI CRUD passed: novel_id=2a958372-... volume_id=2b29e111-...
PASS probes_ops01_r1_10 redis_outage.admission_builder
PASS probes_ops01_r1_10 redis_outage.ai_dispatch_fail_closed
PROBE redis_outage AI dispatch fail-closed: err=llm.capacity_unavailable

=== SCENARIO 2: R1-01 MULTI-TENANT ISOLATION ===
PASS isolation_r1_01 tenant_b.create_creator_member
PASS isolation_r1_01 tenant_a.create_novel_success
PASS isolation_r1_01 tenant_b.create_novel_success
PASS isolation_r1_01 tenants.distinct_novel_ids
PASS isolation_r1_01 novel_a.belongs_to_tenant_a
PASS isolation_r1_01 novel_b.belongs_to_tenant_b
PROBE R1-01 same title novels created: novel_a=23df3fe1-... tenant_a=c9226e69-..., novel_b=4c45ed3a-... tenant_b=75408955-...
PASS isolation_r1_01 cross_tenant.get_novel_404_no_leakage
PROBE R1-01 cross-tenant GET returned status=404 code=content.not_found (404 expected, zero leakage)
PASS isolation_r1_01 cross_tenant.get_novel_b_404
PASS isolation_r1_01 tenant_a.list_novels_success
PASS isolation_r1_01 tenant_a.novels_strictly_isolated
PROBE R1-01 list isolation: found_own=true leaked_other=false (items_count=2)
PASS isolation_r1_01 cross_tenant.mutate_volume_404
PASS isolation_r1_01 cross_tenant.update_novel_404
PASS isolation_r1_01 cross_tenant.delete_novel_404

=== SCENARIO 3: R1-02 & R1-03 CONTENT CAS & ATOMIC ROLLBACK ===
PASS content_cas_r1_02_r1_03 cas.create_novel_ok
PASS content_cas_r1_02_r1_03 cas.create_volume_ok
PASS content_cas_r1_02_r1_03 cas.create_chapter_ok
PASS content_cas_r1_02_r1_03 cas.initial_chapter_revision_1
PASS content_cas_r1_02_r1_03 cas.save_body_v1_ok
PASS content_cas_r1_02_r1_03 cas.revision_advanced_to_2
PASS content_cas_r1_02_r1_03 cas.body_text_matches_v1
PROBE R1-02 saved revision 1->2: chapter_id=8ac9a8d4-... revision=2 word_count=34
PASS content_cas_r1_02_r1_03 cas.stale_save_rejected_409
PROBE R1-02 stale save rejected: status=409 code=content.version_conflict (409 expected)
PASS content_cas_r1_02_r1_03 cas.read_body_ok
PASS content_cas_r1_02_r1_03 cas.body_intact_no_dirty_write
PASS content_cas_r1_02_r1_03 cas.revision_remains_2
PASS content_cas_r1_02_r1_03 rollback.initial_version_count_2
PASS content_cas_r1_02_r1_03 rollback.sentinel_error_returned
PASS content_cas_r1_02_r1_03 rollback.version_count_unchanged
PASS content_cas_r1_02_r1_03 rollback.body_word_count_reverted
PROBE R1-03 atomic rollback verified: versions_before=2 versions_after=2 word_count=34

=== SCENARIO 4: R1-04 CHAPTER CONFIRMATION & WORLDVIEW ATOMICITY ===
PASS confirmation_r1_04 confirm.create_novel_ok
PASS confirmation_r1_04 confirm.create_volume_ok
PASS confirmation_r1_04 confirm.create_chapter_ok
PASS confirmation_r1_04 confirm.save_body_ok
PASS confirmation_r1_04 confirm.get_world_revision_ok
PASS confirmation_r1_04 confirm.world_conflict_rejected_409
PROBE R1-04 worldview conflict rejected: status=409 code=content.version_conflict (409 expected)
PASS confirmation_r1_04 confirm.chapter_not_confirmed
PASS confirmation_r1_04 confirm.confirmed_version_id_is_nil
PASS confirmation_r1_04 confirm.zero_ledger_residuals
PASS confirmation_r1_04 confirm.world_revision_unadvanced
PROBE R1-04 atomic rollback verified: chapter_status=draft confirmed_ver=<nil> ledger_before=0 ledger_after=0 world_rev=1

=== SCENARIO 5: R1-06 & R1-07 AGENT RUN REPLAY & APPROVAL CAS ===
PASS agent_run_r1_06_r1_07 agent.create_definition_ok
PASS agent_run_r1_06_r1_07 agent.create_run_accepted
PASS agent_run_r1_06_r1_07 agent.run_id_not_empty
PASS agent_run_r1_06_r1_07 agent.replay_run_accepted
PASS agent_run_r1_06_r1_07 agent.replay_returns_same_run_id
PASS agent_run_r1_06_r1_07 agent.exactly_one_run_in_db
PROBE R1-06 run response replay verified: run_id=d6ab74e5-... db_count=1
PASS agent_run_r1_06_r1_07 agent.get_run_ok
PASS agent_run_r1_06_r1_07 agent.sse_stream_ok
PASS agent_run_r1_06_r1_07 agent.sse_content_type
PROBE R1-06 SSE stream connected: status= 200 content_type= text/event-stream
PASS agent_run_r1_06_r1_07 approval.first_approve_succeeds
PROBE R1-07 first approval decision status=200
PASS agent_run_r1_06_r1_07 approval.second_approve_rejected_409
PROBE R1-07 duplicate approval rejected: status=409 (409 expected)
PASS agent_run_r1_06_r1_07 approval.update_chapter_body_ok
PASS agent_run_r1_06_r1_07 approval.chapter_revision_advanced
PASS agent_run_r1_06_r1_07 approval.source_changed_rejected_409
PROBE R1-07 source changed approval rejected: status=409 (409 expected)

=== SCENARIO 6: R1-16 & R1-18 SECURITY, CSRF, DUAL-COOKIE & AUDIT DESENSITIZATION ===
PASS security_audit_r1_16_r1_18 auth.creator_cookie_name
PASS security_audit_r1_16_r1_18 auth.admin_cookie_name
PASS security_audit_r1_16_r1_18 auth.cookies_distinct_values
PROBE R1-16 dual audience cookies: creator_cookie=inklore_creator_refresh admin_cookie=inklore_admin_refresh
PASS security_audit_r1_16_r1_18 auth.creator_forbidden_admin_members
PASS security_audit_r1_16_r1_18 auth.csrf_bootstrap_ok
PROBE R1-16 CSRF bootstrap status=200 body_has_csrf=true
PASS security_audit_r1_16_r1_18 auth.refresh_rotation_ok
PROBE R1-16 refresh rotation status=200 set_cookie=true
PASS security_audit_r1_16_r1_18 auth.refresh_replay_revoked_401
PROBE R1-16 old refresh replay rejected: status=401 (401 auth.refresh_reused expected)
PASS security_audit_r1_16_r1_18 audit.query_logs_ok
PASS security_audit_r1_16_r1_18 audit.no_password_leaked
PASS security_audit_r1_16_r1_18 audit.no_token_leaked
PASS security_audit_r1_16_r1_18 audit.no_author_text_leaked
PROBE R1-18 audit log queried: status=200 size=9394 password_leak=false text_leak=false
PASS security_audit_r1_16_r1_18 audit.admin_forbidden_author_novels
PROBE R1-18 admin forbidden author novels: status=403 (403 expected)
PASS security_audit_r1_16_r1_18 audit.reject_unapproved_field
PROBE R1-18 audit field validation rejection: err=[platform.validation_failed] errors.platform.validation_failed (platform.validation_failed expected)

=== SCENARIO 7: OPS-03, OPS-04, OPS-06, OPS-07, OPS-08 OPERATIONAL SCENARIOS ===
PASS ops_scenarios origin.attacker_origin_denied_403
PROBE OPS-07 attacker origin rejected: status=403 (403 expected)
PASS ops_scenarios origin.creator_origin_allowed
PASS ops_scenarios origin.cors_allow_origin_header
PROBE OPS-07 creator origin OPTIONS status=200 allow_origin=http://localhost:3000
PASS ops_scenarios soft_delete.chapter_exists_before_deletion
PASS ops_scenarios soft_delete.delete_chapter_ok
PASS ops_scenarios soft_delete.immediate_404_revocation
PROBE OPS-06 soft delete immediate 404 revocation verified: chapter_id=497cb688-... status=404 code=content.not_found
PASS ops_scenarios ops.status_probe_ok
PASS ops_scenarios ops.status_state_valid
PASS ops_scenarios ops.status_has_dependencies
PASS ops_scenarios ops.status_no_dsn_leak
PROBE OPS-08 platform status verified: state=ready dependencies=2 queue_queued=1 outbox_pending=20
PASS ink176.all_scenarios_verified
```

### 3. 核心穿透场景验证细节

#### 3.1 OPS-01 / R1-10: 探针探测与故障降级
- `/healthz`: 存活探针返回 200，格式包含 `status=ok` 与 `version`。
- `/readyz`: 真实 PG 探测；数据库断开时返回 503 `platform.dependency_unavailable`，系统拒绝任何伪造内存存储降级。
- Redis 断网降级: 当 Redis 实例下线或不可达时，`/readyz` 返回 200 `state="degraded"`；非 AI 的内容创作 CRUD（新建作品、卷、章节）保持正常操作（HTTP 201/200），而 AI Admission 调度网关严格执行 fail-closed 阻断，返回 `llm.capacity_unavailable`（503），绝不进行无控制的透支发放。

#### 3.2 R1-01: 多租户严格物理隔离与零存在性泄漏
- 租户 A 与租户 B 创建同名作品《同名作品：天道逆旅》，系统分配两个不同的 UUID 并严格限制在其所属租户空间。
- 越权访问测试：租户 A 携带自身 Token 访问租户 B 的作品、卷或章节，或者尝试越权更新/删除，系统统一返回 404 `content.not_found` / `platform.resource_not_found`，绝不返回 403，防止攻击者通过响应差异探测资源存在性。
- 列表查询隔离：租户 A 调用作品列表，仅返回属于本租户的作品，跨租户数据串扰计数为 0。

#### 3.3 R1-02 & R1-03: 正文版本 CAS 乐观锁与事务回滚
- R1-02: 章节正文采用严格 CAS 保存。以初始修订版本 1 保存成功后，章节修订版本递增至 2；并发或陈旧客户端以过期版本 1 重复提交时，系统返回 409 `content.version_conflict`，权威正文未受脏数据污染。
- R1-03: 事务故障注入验证：在事务内部保存正文并模拟插入快照故障，UoW 触发整体原子回滚，正文表与历史快照表保持 100% 对应，绝不产生无快照的正文或孤立版本记录。

#### 3.4 R1-04: 章节确认原子账本回滚
- 章节确认用例在 `ExpectedWorldRevision` 或事实提议冲突时返回 409 `content.version_conflict` / `worldview.fact_conflict`。
- 验证确认失败后：章节状态保持 `draft`，`confirmed_version_id` 为 `null`，事实账本事件表与事实快照表零残留行，世界观版本不漂移，幂等记录不产生虚假完成。

#### 3.5 R1-06 & R1-07: Agent Run 断线重放与审批门控
- R1-06: 携带 `Idempotency-Key` 创建智能体运行（HTTP 202 Accepted）。网络断线同键重放时，系统返回相同的 `run_id`，数据库中严格仅存在 1 条运行记录；SSE 流通过 `/v1/agents/runs/{id}/events` 重连并按游标回放，协议头为 `text/event-stream`。
- R1-07: 审批票据一次性 CAS 消费：第一次审批决策成功（HTTP 200），重复提交时返回 409 `agent.approval_consumed`；当作者在审批等待期间修改章节正文导致版本推进后，提交旧票据返回 409 `agent.approval_source_changed`，原票据作废、Run 标记失败，绝不执行过期授权。

#### 3.6 R1-16: CSRF 引导、双端 Cookie 与 Refresh 重放吊销
- 创作者端 Cookie (`inklore_creator_refresh`) 与管理端 Cookie (`inklore_admin_refresh`) 彻底分离，跨端携带直接被拦截。
- 通过同源 `GET /v1/auth/session-csrf?audience=...` 完成浏览器 CSRF Token 引导。
- Refresh Token 旋转与重放防御：刷新成功发放新 Cookie；若攻击者重放已使用过的旧 Cookie，系统立即返回 401 `auth.refresh_reused` 并同事务吊销该会话所有凭据，后续任何认证请求均失效。

#### 3.7 R1-18: 管理端脱敏审计与正文零泄漏
- 管理端调用 `/v1/platform/audit-events` 查询日志：响应体与存储行中严格过滤密码哈希、明文 Access Token、Refresh Token、API Key，且绝不包含作者小说正文。
- 权限硬隔离：管理员账号直接请求创作者内容接口（如 `GET /v1/content/novels`）返回 403 Forbidden，杜绝运维越权窃取作者正文。
- 审计字段白名单：审计存储对未登记的安全字段（如 `password`、`body`）坚决拒绝写入（`platform.validation_failed`）。

#### 3.8 OPS-03 ~ OPS-08: 运维与容量规范
- OPS-06: 软删除立即撤销权限，被软删除的章节再次访问立即返回 404。
- OPS-07: 双端 Origin 精确白名单控制，非授权 Origin (`http://attacker.invalid`) 拦截 403，创作者端 3000 端口通过 CORS 预检。
- OPS-08: `/v1/platform/status` 安全输出系统状态、依赖探针结果、作业队列统计与发件箱积压指标，无任何 DSN、凭据或未脱敏调用栈泄漏。

### 4. 工程与架构合规自检
- [x] 遵守单手写文件体积限制：所有源码与文档文件均小于 50KB。
- [x] 严格零测试文件红线：全代码库未创建任何 `*_test.go` 或 `*.test.ts` 文件。
- [x] 退出码门禁验证：`go build ./cmd/acceptance/ink176/...` 退出码 0，`go build ./...` 退出码 0，`go run ./cmd/acceptance/ink176 all` 退出码 0。
