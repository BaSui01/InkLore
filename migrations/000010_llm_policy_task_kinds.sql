-- ADR-0027 / INK-227 R1.x additive LLM migration; apply after 000009_permissions.
-- Existing immutable policy versions receive [] because no historical task-kind mapping existed.
-- Such versions remain usable by frozen legacy runs but are excluded from the Creator catalog.
-- New published versions require 1..16 task_kinds in the API/application contract.
BEGIN;
ALTER TABLE llm_task_policy_versions ADD COLUMN task_kinds text[] NOT NULL DEFAULT '{}' CHECK(cardinality(task_kinds)<=16);
COMMIT;
