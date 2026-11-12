-- INK-273 baseline: one-time backfill of pre-existing Agent pins onto the
-- LLM-owned reference ledger, so ledger-only policy deletion refusal also
-- covers entities created before this stream existed. Applying it here keeps
-- the upgrade path coordinated with the migration runner instead of relying
-- on a manual operator step. This is a single idempotent schema-level copy;
-- runtime llm code never reads Agent tables, and new creations register
-- through the agent -> llm application port with ON CONFLICT dedupe.
-- Do not modify or re-run frozen R1 000001-000017 or earlier post_r1 files.

INSERT INTO llm_policy_version_references (tenant_id, policy_version_id, reference_kind, reference_id)
SELECT r.tenant_id, r.policy_version_id, 'run', r.id
FROM agent_runs r
ON CONFLICT DO NOTHING;

INSERT INTO llm_policy_version_references (tenant_id, policy_version_id, reference_kind, reference_id)
SELECT g.tenant_id, g.policy_version_id, 'guide_session', g.id
FROM agent_novel_creation_sessions g
ON CONFLICT DO NOTHING;
