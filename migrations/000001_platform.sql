-- R1设计DDL：先执行本文件，再执行各业务域；本轮仅设计，不在工作库执行。
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE platform_tenants (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(120) NOT NULL,
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','suspended')),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE platform_tenants IS 'Owner platform；租户身份与启停，不存业务正文';
CREATE TABLE platform_users (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email varchar(254) NOT NULL,
 display_name varchar(60) NOT NULL, password_hash text NOT NULL,
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX platform_users_email_unique ON platform_users(lower(email));
COMMENT ON TABLE platform_users IS 'Owner platform；bcrypt cost12 hash，禁止明文密码与默认密码';
CREATE TABLE platform_memberships (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 user_id uuid NOT NULL REFERENCES platform_users(id), roles text[] NOT NULL,
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,user_id), UNIQUE(tenant_id,id),
 CHECK(cardinality(roles)>0 AND roles <@ ARRAY['creator','operator','auditor']::text[])
);
COMMENT ON TABLE platform_memberships IS 'Owner platform；有效角色实时查验；移除最后一个operator必须业务事务拒绝';
CREATE TABLE platform_sessions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, user_id uuid NOT NULL,
 audience text NOT NULL CHECK(audience IN ('creator','admin')), csrf_hash char(64) NOT NULL,
 membership_revision bigint NOT NULL, expires_at timestamptz NOT NULL, revoked_at timestamptz,
 last_seen_at timestamptz NOT NULL DEFAULT now(), revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(tenant_id,user_id) REFERENCES platform_memberships(tenant_id,user_id), UNIQUE(tenant_id,id)
);
CREATE INDEX platform_sessions_active ON platform_sessions(tenant_id,user_id,expires_at) WHERE revoked_at IS NULL;
COMMENT ON TABLE platform_sessions IS 'Owner platform；JWT只带session ID，服务端验证吊销/成员状态';
CREATE TABLE platform_refresh_tokens (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), session_id uuid NOT NULL REFERENCES platform_sessions(id),
 token_hash char(64) NOT NULL UNIQUE, used_at timestamptz, expires_at timestamptz NOT NULL,
 replaced_by uuid REFERENCES platform_refresh_tokens(id), created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE platform_refresh_tokens IS 'Owner platform；原始刷新token不入库；用过token再次出现吊销session';
CREATE TABLE platform_resource_scopes (
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id), resource_id uuid NOT NULL,
 resource_kind text NOT NULL, owner_id uuid NOT NULL REFERENCES platform_users(id), novel_id uuid,
 current_revision bigint NOT NULL CHECK(current_revision>0), content_hash char(64), deleted_at timestamptz,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,resource_id)
);
CREATE INDEX platform_scope_owner ON platform_resource_scopes(tenant_id,owner_id,resource_kind,resource_id) WHERE deleted_at IS NULL;
COMMENT ON TABLE platform_resource_scopes IS 'Owner platform；各域同UoW维护授权投影，不含正文，禁止异步权限滞后';
CREATE TABLE platform_resource_revisions (
 tenant_id uuid NOT NULL, resource_id uuid NOT NULL, revision bigint NOT NULL CHECK(revision>0),
 content_hash char(64) NOT NULL, content_length bigint NOT NULL CHECK(content_length>=0),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,resource_id,revision),
 FOREIGN KEY(tenant_id,resource_id) REFERENCES platform_resource_scopes(tenant_id,resource_id)
);
COMMENT ON TABLE platform_resource_revisions IS 'Owner platform；不可变revision/hash证明，用于分析报告/采纳绑定真实性';
CREATE TABLE platform_files (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 owner_id uuid NOT NULL REFERENCES platform_users(id), novel_id uuid,
 filename varchar(180) NOT NULL, media_type varchar(100) NOT NULL, purpose text NOT NULL,
 payload bytea, size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 0 AND 104857600), sha256 char(64) NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0), expires_at timestamptz, deleted_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), CHECK(payload IS NOT NULL OR deleted_at IS NOT NULL),
 CHECK(payload IS NULL OR (octet_length(payload)=size_bytes AND sha256=encode(digest(payload,'sha256'),'hex')))
);
CREATE INDEX platform_files_owner ON platform_files(tenant_id,owner_id,created_at DESC,id DESC);
COMMENT ON TABLE platform_files IS 'Owner platform；权限下载、PGbytea，上传20MiB/导出100MiB；自动导出文件7天过期';
CREATE TABLE platform_idempotency (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 actor_id uuid NOT NULL REFERENCES platform_users(id), method varchar(10) NOT NULL,
 canonical_path varchar(512) NOT NULL, idempotency_key uuid NOT NULL, request_hash char(64) NOT NULL,
 state text NOT NULL CHECK(state IN ('processing','completed')),
 response_status integer CHECK(response_status BETWEEN 200 AND 599), response_body jsonb,
 operation_id uuid, expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,actor_id,method,canonical_path,idempotency_key),
 CHECK(state<>'completed' OR (response_status IS NOT NULL AND response_body IS NOT NULL))
);
CREATE INDEX platform_idempotency_expiry ON platform_idempotency(expires_at);
COMMENT ON TABLE platform_idempotency IS 'Owner platform；逻辑命令24h重放，scope/授权先于缓存返回；成功同业务事务完成';
CREATE TABLE platform_jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 owner_id uuid NOT NULL REFERENCES platform_users(id), domain text NOT NULL,
 job_kind text NOT NULL, resource_id uuid NOT NULL, execution_key varchar(200) NOT NULL,
 state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','leased','retry_wait','succeeded','failed','cancelled')),
 payload jsonb NOT NULL DEFAULT '{}', attempt integer NOT NULL DEFAULT 0 CHECK(attempt BETWEEN 0 AND 4),
 available_at timestamptz NOT NULL DEFAULT now(), lease_owner uuid, lease_epoch bigint NOT NULL DEFAULT 0,
 lease_until timestamptz, cancel_requested_at timestamptz, last_error_code varchar(100),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,domain,execution_key), UNIQUE(tenant_id,id)
);
CREATE INDEX platform_jobs_claim ON platform_jobs(available_at,id) WHERE state IN ('queued','retry_wait');
CREATE INDEX platform_jobs_lease ON platform_jobs(lease_until) WHERE state='leased';
COMMENT ON TABLE platform_jobs IS 'Owner platform；通用调度租约，不取代agent_run/领域job业务状态；SKIP LOCKED领取';
CREATE TABLE platform_outbox (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 producer text NOT NULL, event_type varchar(100) NOT NULL, aggregate_id uuid NOT NULL,
 aggregate_revision bigint NOT NULL CHECK(aggregate_revision>0), payload jsonb NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','delivered','dead_letter')),
 attempt integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz,
 UNIQUE(tenant_id,producer,aggregate_id,aggregate_revision,event_type)
);
CREATE INDEX platform_outbox_pending ON platform_outbox(available_at,id) WHERE state='pending';
COMMENT ON TABLE platform_outbox IS 'Owner platform；业务提交同事务写，payload仅ID/修订/安全摘要；不得放正文和凭据';
CREATE TABLE platform_inbox (
 consumer varchar(100) NOT NULL, event_id uuid NOT NULL REFERENCES platform_outbox(id),
 processed_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(consumer,event_id)
);
COMMENT ON TABLE platform_inbox IS 'Owner platform；消费投影与inbox插入同事务；去重而非假设只投递一次';
CREATE TABLE platform_audit_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 actor_id uuid, audience text NOT NULL, action varchar(100) NOT NULL,
 resource_id uuid, outcome text NOT NULL CHECK(outcome IN ('succeeded','denied','failed')),
 request_id uuid NOT NULL, trace_id char(32) NOT NULL, safe_changes jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX platform_audit_query ON platform_audit_events(tenant_id,created_at DESC,id DESC);
COMMENT ON TABLE platform_audit_events IS 'Owner platform；只追加90天，配置变更仅安全字段，不存正文/Key/令牌';
CREATE TABLE platform_ws_tickets (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), session_id uuid NOT NULL REFERENCES platform_sessions(id),
 ticket_hash char(64) NOT NULL UNIQUE, expires_at timestamptz NOT NULL, consumed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE platform_ws_tickets IS 'Owner platform；60秒一次性票据，原值不入库不记access log';
