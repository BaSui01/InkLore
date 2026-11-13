-- The post-R1 invocation ledger is created after R1 blanket role grants.
-- Runtime read/write access must be explicit; never grant destructive DELETE.
REVOKE ALL ON llm_generation_invocations FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON llm_generation_invocations TO inklore_runtime;
REVOKE DELETE ON llm_generation_invocations FROM inklore_runtime;
GRANT SELECT, DELETE ON llm_generation_invocations TO inklore_retention;
