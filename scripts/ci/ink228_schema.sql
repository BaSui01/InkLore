\set ON_ERROR_STOP on
-- This gate intentionally uses ONLY GitHub Actions' throwaway Postgres.
-- No production or Supabase connection strings are accepted.
DO $$
DECLARE
  n integer;
  bad integer;
BEGIN
  IF current_database() <> 'inklore_ci' THEN
    RAISE EXCEPTION 'integration gate requires isolated inklore_ci database';
  END IF;
  SELECT count(*) INTO n FROM schema_migrations;
  IF n <> (SELECT count(*) FROM (VALUES
    ('000001'),('000002'),('000003'),('000004'),('000005'),('000006'),
    ('000007'),('000008'),('000009'),('000010'),('000011'),('000012'),
    ('000013'),('000014'),('000015'),('000016'),('000017')) AS t(v)) THEN
    RAISE EXCEPTION 'expected 17 versioned migrations, found %', n;
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
  RAISE NOTICE 'INK-228 isolated schema and tenant FK gate passed (% migrations)', n;
END
$$;
