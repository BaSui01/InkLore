-- INK-273 baseline: LLM-owned retention ledger for frozen policy versions.
-- Agent entities pin frozen versions; instead of letting the LLM deletion
-- check query Agent-owned private tables (a forbidden llm -> agent reverse
-- dependency), Agent registers each pin here through the existing
-- agent -> llm application boundary inside its own creation transaction.
-- Do not modify or re-run frozen R1 000001-000017 or post_r1 000001.

CREATE TABLE llm_policy_version_references (
  tenant_id uuid NOT NULL,
  policy_version_id uuid NOT NULL,
  reference_kind text NOT NULL CHECK(reference_kind IN ('run','question','guide_session')),
  reference_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(tenant_id, reference_kind, reference_id, policy_version_id)
);
CREATE INDEX llm_policy_version_references_version
 ON llm_policy_version_references(tenant_id, policy_version_id);
COMMENT ON TABLE llm_policy_version_references IS
 'LLM-owned pins on frozen policy versions, registered by agent at entity creation. Rows are retained for the lifetime of the referencing entity; pre-existing rows in already-deployed environments must be backfilled from agent tables before relying on deletion refusal.';

REVOKE ALL ON llm_policy_version_references FROM PUBLIC;
GRANT SELECT,INSERT,DELETE ON llm_policy_version_references TO inklore_runtime;
