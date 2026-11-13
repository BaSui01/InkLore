-- #104: a closed unanswered question must not fabricate answered_at.
-- Existing answered rows retain their full answer binding constraint.
DO $$
DECLARE constraint_name text;
BEGIN
  FOR constraint_name IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'agent_run_questions'::regclass AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%answered_at IS NULL%'
  LOOP
    EXECUTE format('ALTER TABLE agent_run_questions DROP CONSTRAINT %I', constraint_name);
  END LOOP;
END $$;
ALTER TABLE agent_run_questions ADD CONSTRAINT agent_run_questions_unanswered_empty
 CHECK (status = 'answered' OR (answer IS NULL AND answer_hash IS NULL AND answered_by IS NULL AND answered_at IS NULL));
