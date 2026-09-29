-- R1角色权限设计。部署创建实际LOGIN账号后分别GRANT这些NOLOGIN角色；不在文件中放密码。
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='inklore_runtime') THEN CREATE ROLE inklore_runtime NOLOGIN; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='inklore_retention') THEN CREATE ROLE inklore_retention NOLOGIN; END IF;
END $$;
GRANT USAGE ON SCHEMA public TO inklore_runtime, inklore_retention;
GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO inklore_runtime;
REVOKE DELETE ON ALL TABLES IN SCHEMA public FROM inklore_runtime;
GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO inklore_runtime;
REVOKE UPDATE ON content_chapter_versions,content_writing_events,content_job_snapshots,
 outline_clue_events,worldview_fact_ledger_events,worldview_fact_payloads,
 agent_definition_versions,agent_tool_versions,agent_workflow_versions,agent_run_events,
 llm_task_policy_versions,llm_pricing_revisions,llm_usage_records,llm_usage_adjustments,
 platform_audit_events,platform_resource_revisions FROM inklore_runtime;
GRANT SELECT,DELETE ON ALL TABLES IN SCHEMA public TO inklore_retention;
REVOKE DELETE ON worldview_fact_ledger_events FROM inklore_retention;
-- runtime没有物理DELETE：公开删除均逻辑停用/soft-delete。
-- retention只在专用worker身份运行：按租户/到期条件/引用顺序删除，不能提供给HTTP实例。
-- 账本事件在表触发器层还拒绝UPDATE/DELETE；擦除只针对payload分表。
-- 不配置ALTER DEFAULT PRIVILEGES自动授予未来表；每个新迁移必须显式复核权限。
