-- INK-255 / ADR-0038: harden immutable-history trigger functions in Supabase staging.
-- These functions are SECURITY INVOKER but should not inherit a mutable caller search_path.
-- Apply only after all 17 native migrations have completed in inklore_ci.
BEGIN;
ALTER FUNCTION inklore_ci.content_reject_history_update() SET search_path TO pg_catalog, inklore_ci, extensions;
ALTER FUNCTION inklore_ci.outline_reject_event_update() SET search_path TO pg_catalog, inklore_ci, extensions;
ALTER FUNCTION inklore_ci.worldview_reject_ledger_mutation() SET search_path TO pg_catalog, inklore_ci, extensions;
ALTER FUNCTION inklore_ci.worldview_reject_payload_update() SET search_path TO pg_catalog, inklore_ci, extensions;
COMMIT;
