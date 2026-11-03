\set ON_ERROR_STOP on
-- This gate intentionally uses ONLY GitHub Actions' throwaway Postgres.
-- No production or Supabase connection strings are accepted.
-- 工作流先核对实际迁移身份与校验和，再传入仓库迁移数量。
SELECT set_config('inklore_ci.expected_migrations', :'expected_migrations', false);
DO $$
DECLARE
  n integer;
  bad integer;
BEGIN
  IF current_database() <> 'inklore_ci' THEN
    RAISE EXCEPTION 'integration gate requires isolated inklore_ci database';
  END IF;
  SELECT count(*) INTO n FROM schema_migrations;
  IF n = 0 OR n <> current_setting('inklore_ci.expected_migrations')::integer THEN
    RAISE EXCEPTION 'migration count differs from verified repository stream, found %', n;
  END IF;
  IF to_regclass('public.llm_channel_models') IS NULL
     OR to_regclass('public.llm_task_policy_versions') IS NULL
     OR to_regclass('public.agent_runs') IS NULL THEN
     RAISE EXCEPTION 'missing Agent / LLM routing tables';
  END IF;
  SELECT count(*) INTO bad
  FROM pg_constraint c
  JOIN pg_class t ON t.oid=c.conrelid
  WHERE t.relname='llm_channel_models'
    AND c.contype='f'
    AND pg_get_constraintdef(c.oid) LIKE '%(tenant_id,%';
  IF bad < 2 THEN
    RAISE EXCEPTION 'tenant-scoped model/channel foreign keys missing';
  END IF;
  RAISE NOTICE 'Agent/LLM isolated schema and tenant FK gate passed (% migrations)', n;
END
$$;
