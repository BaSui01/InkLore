-- INK-228 / R1.x: Extend LLM model capabilities constraint to cover vision, image, rerank, video, and audio.
BEGIN;

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (
        SELECT conname, relname
        FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE t.relname IN ('llm_models', 'llm_task_policy_versions')
          AND c.contype = 'c'
          AND pg_get_constraintdef(c.oid) LIKE '%capabilities%'
          AND pg_get_constraintdef(c.oid) LIKE '%<@%'
    ) LOOP
        EXECUTE 'ALTER TABLE ' || quote_ident(r.relname) || ' DROP CONSTRAINT ' || quote_ident(r.conname);
    END LOOP;
END $$;

ALTER TABLE llm_models ADD CONSTRAINT llm_models_capabilities_check CHECK (
    cardinality(capabilities) > 0 AND capabilities <@ ARRAY['text','vision','image','embedding','rerank','video','audio','tools','structured','reasoning']::text[]
);

ALTER TABLE llm_task_policy_versions ADD CONSTRAINT llm_task_policy_versions_required_capabilities_check CHECK (
    required_capabilities <@ ARRAY['text','vision','image','embedding','rerank','video','audio','tools','structured','reasoning']::text[]
);

COMMIT;
