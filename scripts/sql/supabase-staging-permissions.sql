-- INK-255 / ADR-0038: Supabase 专属测试 schema 的权限补充。
-- 不改变冻结迁移和校验和；只在独立的 inklore_ci schema 上重复原权限。
-- 专用于 Supabase 测试环境，不适用于生产或默认 public schema。
BEGIN;
SET LOCAL search_path TO inklore_ci,extensions,public;
DO $scope_guard$
BEGIN
  IF to_regclass('inklore_ci.schema_migrations') IS NULL THEN
    RAISE EXCEPTION 'expected migration metadata in private schema';
  END IF;
  IF (SELECT count(*) FROM inklore_ci.schema_migrations) <> 17 THEN
    RAISE EXCEPTION 'expected 17 migrations before staging grants';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='inklore_runtime' AND NOT rolcanlogin) OR
     NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='inklore_retention' AND NOT rolcanlogin) THEN
    RAISE EXCEPTION 'expected NOLOGIN roles';
  END IF;
END;
$scope_guard$;

-- 原权限迁移在 public 上声明角色；这里只针对私有测试 schema 补授权。
-- 撤销 000009 对 public 现存表与序列的显式授权，避免继承 public 默认 USAGE 后仍可访问无关业务。
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM inklore_runtime, inklore_retention;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM inklore_runtime, inklore_retention;
REVOKE ALL ON SCHEMA public FROM inklore_runtime, inklore_retention;
GRANT USAGE ON SCHEMA inklore_ci TO inklore_runtime, inklore_retention;
GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA inklore_ci TO inklore_runtime;
REVOKE DELETE ON ALL TABLES IN SCHEMA inklore_ci FROM inklore_runtime;
GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA inklore_ci TO inklore_runtime;
REVOKE UPDATE ON content_chapter_versions,content_writing_events,content_job_snapshots,
 outline_clue_events,worldview_fact_ledger_events,worldview_fact_payloads,
 agent_definition_versions,agent_tool_versions,agent_workflow_versions,agent_run_events,
 llm_task_policy_versions,llm_pricing_revisions,llm_usage_records,llm_usage_adjustments,
 platform_audit_events,platform_resource_revisions FROM inklore_runtime;
GRANT SELECT,DELETE ON ALL TABLES IN SCHEMA inklore_ci TO inklore_retention;
REVOKE DELETE ON worldview_fact_ledger_events FROM inklore_retention;
-- runtime没有物理DELETE：公开删除均逻辑停用/soft-delete。
-- retention只在专用worker身份运行：按租户/到期条件/引用顺序删除，不能提供给HTTP实例。
-- 账本事件在表触发器层还拒绝UPDATE/DELETE；擦除只针对payload分表。
-- 不配置ALTER DEFAULT PRIVILEGES自动授予未来表；每个新迁移必须显式复核权限。

-- 迁移版本表不能让运行时或数据清理身份修改。
REVOKE INSERT,UPDATE,DELETE ON inklore_ci.schema_migrations FROM inklore_runtime, inklore_retention;
-- API 身份必须经过独立部署角色授权，不允许 Supabase Data API 角色读取业务表。
REVOKE ALL ON SCHEMA inklore_ci FROM PUBLIC, anon, authenticated, service_role;
COMMIT;
