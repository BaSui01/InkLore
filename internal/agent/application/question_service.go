package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	d "InkLore/internal/agent/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"github.com/google/uuid"
)

// nodeCheckpoint is resumed without repeating the Think call which asked the
// question. Answers remain untrusted context; no authoritative source is changed.
type nodeCheckpoint struct {
	UserModel      string            `json:"user_model,omitempty"`
	Observation    string            `json:"observation"`
	CallHistory    []string          `json:"call_history,omitempty"`
	QuestionID     string            `json:"question_id,omitempty"`
	Clarifications []json.RawMessage `json:"clarifications,omitempty"`
}

func (s *RunService) GetQuestion(ctx context.Context, p identity.Principal, runID, questionID string) (*d.Question, error) {
	if !uow.InTransaction(ctx) {
		var question *d.Question
		err := s.manager.WithinReadSnapshot(ctx, func(tx context.Context) (err error) {
			question, err = s.GetQuestion(tx, p, runID, questionID)
			return err
		})
		return question, err
	}
	if !scope.ValidID(runID) || !scope.ValidID(questionID) {
		return nil, d.ErrValidationFailed(nil)
	}
	if err := s.AuthorizeRun(ctx, p, runID); err != nil {
		return nil, err
	}
	q, err := s.repo.GetQuestion(ctx, p.TenantID, runID, questionID)
	if err != nil {
		return nil, err
	}
	if q.OwnerUserID != p.UserID {
		return nil, d.ErrNotFound()
	}
	return q, nil
}

// suspendQuestion commits the question, continuation, Run, event and release of
// the same Job lease together. A failed CAS rolls back every mutation.
func (s *RunService) suspendQuestion(ctx context.Context, lease *d.RunJobLease, node d.AgentRunNode, in d.AskQuestionInput, checkpoint nodeCheckpoint) error {
	return s.jobs.WithRunJobLease(ctx, lease, func(tx context.Context) error {
		run, err := s.repo.LockRun(tx, lease.TenantID, lease.RunID)
		if err != nil {
			return err
		}
		if run.Status != d.RunStatusRunning || node.DefinitionVersionID == nil {
			return d.ErrVersionConflict(nil)
		}
		now, err := s.repo.DatabaseTime(tx)
		if err != nil {
			return err
		}
		if !run.DeadlineAt.After(now) {
			_, err = s.Terminate(tx, run.TenantID, run.ID, run.Revision, d.RunStatusFailed, "", "agent.question_expired", false, nil)
			return err
		}
		count, err := s.repo.CountQuestions(tx, run.TenantID, run.ID)
		if err != nil {
			return err
		}
		if count >= d.MaxRunQuestions {
			_, err = s.Terminate(tx, run.TenantID, run.ID, run.Revision, d.RunStatusFailed, "", "agent.loop_detected", false, nil)
			return err
		}
		questions, err := s.repo.ListOpenQuestions(tx, run.TenantID, run.ID)
		if err != nil {
			return err
		}
		if len(questions) != 0 {
			return d.ErrVersionConflict(nil)
		}
		sourceRevision := int64(1)
		sourceHash := HashString(run.Instruction)
		if run.SourceRevision != nil {
			sourceRevision = *run.SourceRevision
		}
		if run.SourceHash != nil {
			sourceHash = *run.SourceHash
		}
		expires := now.Add(d.QuestionWaitLimit)
		if run.DeadlineAt.Before(expires) {
			expires = run.DeadlineAt
		}
		q := &d.Question{ID: uuid.NewString(), TenantID: run.TenantID, OwnerUserID: run.OwnerUserID, RunID: run.ID, NodeID: node.ID, Revision: 1, QuestionVersion: 1, PolicyVersionID: run.PolicyVersionID, DefinitionVersionID: *node.DefinitionVersionID, CreatedAt: now, SourceRevision: sourceRevision, SourceHash: sourceHash, Status: "open", ExpiresAt: expires, AskQuestionInput: in}
		checkpoint.QuestionID = q.ID
		encoded, err := json.Marshal(checkpoint)
		if err != nil {
			return err
		}
		if err = s.repo.SaveNodeCheckpoint(tx, run.TenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodePhaseObserve, node.Iteration+1, d.NodeStatusWaitingClarification, encoded, nil); err != nil {
			return err
		}
		if err = s.repo.CreateQuestion(tx, q); err != nil {
			return err
		}
		if err = s.gateway.RecordPolicyVersionReference(tx, run.TenantID, run.PolicyVersionID, "question", q.ID); err != nil {
			return err
		}
		if _, err = s.repo.UpdateRunStatus(tx, run.TenantID, run.ID, run.Revision, d.RunStatusWaitingClarification); err != nil {
			return err
		}
		payload := d.QuestionRequiredPayload{QuestionID: q.ID, QuestionVersion: 1, NodeID: node.ID, AskQuestionInput: q.AskQuestionInput, SourceRevision: q.SourceRevision, SourceHash: q.SourceHash, ExpiresAt: q.ExpiresAt}
		if err = s.questionEvent(tx, q, "question.required", payload, now); err != nil {
			return err
		}
		return s.jobs.SuspendRunJob(tx, lease, expires)
	})
}

// AnswerQuestion returns a committed public conflict separately from an ordinary
// error. HTTP idempotency must persist the invalidation and its 409 atomically.
func (s *RunService) AnswerQuestion(ctx context.Context, p identity.Principal, runID, questionID string, in d.QuestionAnswerInput) (out *d.Question, committed *pe.AppError, err error) {
	if !scope.ValidID(runID) || !scope.ValidID(questionID) || in.ExpectedRevision < 1 || in.QuestionVersion != 1 {
		return nil, nil, d.ErrValidationFailed(nil)
	}
	if err = s.AuthorizeRun(ctx, p, runID); err != nil {
		return nil, nil, err
	}
	err = s.manager.Within(ctx, func(tx context.Context) error {
		binding, e := s.repo.GetRunOnly(tx, p.TenantID, runID)
		if e != nil {
			return e
		}
		if e = s.jobs.LockRunJobCommand(tx, p.TenantID, binding.PlatformJobID); e != nil {
			return e
		}
		run, e := s.repo.LockRun(tx, p.TenantID, runID)
		if e != nil {
			return e
		}
		if run.OwnerUserID != p.UserID {
			return d.ErrNotFound()
		}
		// Reauthorize inside the mutation boundary; cached responses also pass the HTTP authorizer.
		if e = s.AuthorizeRun(tx, p, runID); e != nil {
			return e
		}
		q, e := s.repo.GetQuestion(tx, p.TenantID, runID, questionID)
		if e != nil {
			return e
		}
		if q.OwnerUserID != p.UserID {
			return d.ErrNotFound()
		}
		if q.Status == "answered" {
			return pe.Contract(409, "agent.question_already_answered", nil)
		}
		if q.Status == "expired" {
			return pe.Contract(409, "agent.question_expired", nil)
		}
		if q.Status != "open" || run.Status.IsTerminal() {
			return pe.Contract(409, "agent.question_binding_mismatch", nil)
		}
		if q.Revision != in.ExpectedRevision {
			return d.ErrVersionConflict(map[string]any{"current_revision": q.Revision})
		}
		answer, e := q.ValidateAnswer(in.Answer)
		if e != nil {
			return e
		}
		now, e := s.repo.DatabaseTime(tx)
		if e != nil {
			return e
		}
		invalidate := func(code, status string) error {
			if e := s.repo.CloseQuestions(tx, p.TenantID, runID, status); e != nil {
				return e
			}
			if _, e := s.Terminate(tx, p.TenantID, runID, run.Revision, d.RunStatusFailed, "", code, false, nil); e != nil {
				return e
			}
			committed = pe.Contract(409, code, nil)
			return nil
		}
		if !q.ExpiresAt.After(now) || !run.DeadlineAt.After(now) {
			return invalidate("agent.question_expired", "expired")
		}
		node, e := s.repo.GetNodeByID(tx, p.TenantID, runID, q.NodeID)
		if e != nil {
			return e
		}
		var checkpoint nodeCheckpoint
		if e = json.Unmarshal(node.Checkpoint, &checkpoint); e != nil {
			return e
		}
		if run.Status != d.RunStatusWaitingClarification || node.Status != d.NodeStatusWaitingClarification || checkpoint.QuestionID != q.ID || node.DefinitionVersionID == nil || *node.DefinitionVersionID != q.DefinitionVersionID || run.PolicyVersionID != q.PolicyVersionID || run.SourceRevision == nil || *run.SourceRevision != q.SourceRevision || run.SourceHash == nil || *run.SourceHash != q.SourceHash {
			return invalidate("agent.question_binding_mismatch", "cancelled")
		}
		unchanged, e := s.resources.VerifySourceUnchanged(tx, p, run.TaskKind, s.buildRunView(run, nil).Resource, q.SourceRevision, q.SourceHash)
		if e != nil {
			return e
		}
		if !unchanged {
			return invalidate("agent.question_binding_mismatch", "cancelled")
		}
		if e = s.repo.AnswerQuestion(tx, q, in.ExpectedRevision, answer, HashString(string(answer)), p.UserID); e != nil {
			// A question can cross its deadline between validation and the database CAS.
			var conflict *pe.AppError
			if errors.As(e, &conflict) && conflict.Code == "agent.version_conflict" {
				currentTime, timeErr := s.repo.DatabaseTime(tx)
				if timeErr != nil {
					return timeErr
				}
				if !q.ExpiresAt.After(currentTime) {
					return invalidate("agent.question_expired", "expired")
				}
			}
			return e
		}
		checkpoint.QuestionID = ""
		checkpoint.Clarifications = append(checkpoint.Clarifications, answer)
		encoded, e := json.Marshal(checkpoint)
		if e != nil {
			return e
		}
		if e = s.repo.ResumeQuestionNode(tx, p.TenantID, node.ID, node.Revision, encoded); e != nil {
			return e
		}
		if _, e = s.repo.UpdateRunStatus(tx, p.TenantID, runID, run.Revision, d.RunStatusRunning); e != nil {
			return e
		}
		if e = s.jobs.WakeRunJob(tx, p.TenantID, run.PlatformJobID); e != nil {
			return e
		}
		if e = s.questionEvent(tx, q, "question.answered", d.QuestionAnsweredPayload{QuestionID: q.ID, QuestionVersion: 1, AnswerHash: *q.AnswerHash, ActorID: p.UserID, AnsweredAt: *q.AnsweredAt}, now); e != nil {
			return e
		}
		out = q
		return nil
	})
	return
}
func (s *RunService) questionEvent(ctx context.Context, q *d.Question, name string, payload any, now time.Time) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	seq, err := s.repo.AllocateNextSeq(ctx, q.TenantID, q.RunID)
	if err != nil {
		return err
	}
	return s.repo.AppendEvent(ctx, q.TenantID, &d.AgentRunEvent{ID: uuid.NewString(), TenantID: q.TenantID, RunID: q.RunID, CreatedAt: now, UpdatedAt: now, Seq: seq, EventName: name, Payload: raw, ExpiresAt: now.Add(7 * 24 * time.Hour)})
}

// checkWaitingQuestion is called by a real worker after reclaiming a suspended
// job. It never repeats Generate while a persisted question is open.
func (s *Scheduler) checkWaitingQuestion(ctx context.Context, lease *d.RunJobLease) error {
	return s.jobs.WithRunJobLease(ctx, lease, func(tx context.Context) error {
		run, err := s.repo.LockRun(tx, lease.TenantID, lease.RunID)
		if err != nil {
			return err
		}
		// An answer may have won the race with the job claim. Keep that continuation runnable.
		if run.Status != d.RunStatusWaitingClarification {
			return s.jobs.SuspendRunJob(tx, lease, time.Now().UTC())
		}
		qs, err := s.repo.ListOpenQuestions(tx, run.TenantID, run.ID)
		if err != nil {
			return err
		}
		if len(qs) != 1 {
			return errors.New("waiting run must have exactly one open question")
		}
		now, err := s.repo.DatabaseTime(tx)
		if err != nil {
			return err
		}
		if !qs[0].ExpiresAt.After(now) || !run.DeadlineAt.After(now) {
			if err = s.repo.CloseQuestions(tx, run.TenantID, run.ID, "expired"); err != nil {
				return err
			}
			return s.completeRun(tx, lease, run.Revision, d.RunStatusFailed, "agent.question_expired")
		}
		return s.jobs.SuspendRunJob(tx, lease, qs[0].ExpiresAt)
	})
}
