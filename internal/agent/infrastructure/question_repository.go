package infrastructure

import (
	d "InkLore/internal/agent/domain"
	"context"
	"encoding/json"
	"time"
)

type questionRow struct {
	ID                  string
	TenantID            string
	OwnerUserID         string
	RunID               string
	NodeID              string
	Revision            int64
	QuestionVersion     int
	PolicyVersionID     string
	DefinitionVersionID string
	CreatedAt           time.Time
	SourceRevision      int64
	SourceHash          string
	TitleKey            string
	HelpKey             string
	ContextSummary      string
	ResponseSchema      json.RawMessage
	Options             json.RawMessage
	Status              string
	ExpiresAt           time.Time
	Answer              json.RawMessage
	AnswerHash          *string
	AnsweredAt          *time.Time
}

func (r questionRow) domain() (*d.Question, error) {
	q := &d.Question{ID: r.ID, TenantID: r.TenantID, OwnerUserID: r.OwnerUserID, RunID: r.RunID, NodeID: r.NodeID, Revision: r.Revision, QuestionVersion: r.QuestionVersion, PolicyVersionID: r.PolicyVersionID, DefinitionVersionID: r.DefinitionVersionID, CreatedAt: r.CreatedAt, SourceRevision: r.SourceRevision, SourceHash: r.SourceHash, Status: r.Status, ExpiresAt: r.ExpiresAt, Answer: r.Answer, AnswerHash: r.AnswerHash, AnsweredAt: r.AnsweredAt}
	q.TitleKey = r.TitleKey
	q.HelpKey = r.HelpKey
	q.ContextSummary = r.ContextSummary
	if err := json.Unmarshal(r.ResponseSchema, &q.ResponseSchema); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(r.Options, &q.Options); err != nil {
		return nil, err
	}
	return q, nil
}
func (r *RunRepositoryPostgres) LockRun(ctx context.Context, tenantID, runID string) (*d.AgentRun, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return nil, err
	}
	var row runRow
	result := db.Raw(`SELECT * FROM agent_runs WHERE tenant_id=? AND id=? AND deleted_at IS NULL FOR UPDATE`, tenantID, runID).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, d.ErrNotFound()
	}
	return row.toDomain(), nil
}
func (r *RunRepositoryPostgres) DatabaseTime(ctx context.Context) (time.Time, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return time.Time{}, err
	}
	var now time.Time
	err = db.Raw(`SELECT clock_timestamp()`).Scan(&now).Error
	return now, err
}
func (r *RunRepositoryPostgres) CreateQuestion(ctx context.Context, q *d.Question) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	schema, err := json.Marshal(q.ResponseSchema)
	if err != nil {
		return err
	}
	options, err := json.Marshal(q.Options)
	if err != nil {
		return err
	}
	return db.Exec(`INSERT INTO agent_run_questions(id,tenant_id,owner_user_id,run_id,node_id,revision,question_version,policy_version_id,definition_version_id,created_at,updated_at,source_revision,source_hash,title_key,help_key,context_summary,response_schema,options,status,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::jsonb,?::jsonb,'open',?)`, q.ID, q.TenantID, q.OwnerUserID, q.RunID, q.NodeID, q.Revision, q.QuestionVersion, q.PolicyVersionID, q.DefinitionVersionID, q.CreatedAt, q.CreatedAt, q.SourceRevision, q.SourceHash, q.TitleKey, q.HelpKey, q.ContextSummary, string(schema), string(options), q.ExpiresAt).Error
}
func (r *RunRepositoryPostgres) GetQuestion(ctx context.Context, tenantID, runID, questionID string) (*d.Question, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row questionRow
	result := db.Raw(`SELECT * FROM agent_run_questions WHERE tenant_id=? AND run_id=? AND id=?`, tenantID, runID, questionID).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, d.ErrNotFound()
	}
	return row.domain()
}
func (r *RunRepositoryPostgres) ListOpenQuestions(ctx context.Context, tenantID, runID string) ([]d.Question, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []questionRow
	if err = db.Raw(`SELECT * FROM agent_run_questions WHERE tenant_id=? AND run_id=? AND status='open' ORDER BY created_at,id`, tenantID, runID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]d.Question, 0, len(rows))
	for _, row := range rows {
		q, err := row.domain()
		if err != nil {
			return nil, err
		}
		result = append(result, *q)
	}
	return result, nil
}
func (r *RunRepositoryPostgres) CountQuestions(ctx context.Context, tenantID, runID string) (int, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return 0, err
	}
	var count int
	err = db.Raw(`SELECT COUNT(*) FROM agent_run_questions WHERE tenant_id=? AND run_id=?`, tenantID, runID).Scan(&count).Error
	return count, err
}
func (r *RunRepositoryPostgres) AnswerQuestion(ctx context.Context, q *d.Question, expectedRevision int64, answer json.RawMessage, hash, actorID string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	var row questionRow
	result := db.Raw(`UPDATE agent_run_questions SET status='answered',answer=?::jsonb,answer_hash=?,answered_by=?,answered_at=clock_timestamp(),updated_at=clock_timestamp(),revision=revision+1 WHERE tenant_id=? AND run_id=? AND id=? AND owner_user_id=? AND revision=? AND question_version=1 AND status='open' AND expires_at>clock_timestamp() RETURNING *`, string(answer), hash, actorID, q.TenantID, q.RunID, q.ID, actorID, expectedRevision).Scan(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return d.ErrVersionConflict(nil)
	}
	updated, err := row.domain()
	if err == nil {
		*q = *updated
	}
	return err
}
func (r *RunRepositoryPostgres) CloseQuestions(ctx context.Context, tenantID, runID, status string) error {
	if status != "cancelled" && status != "expired" {
		return d.ErrValidationFailed(nil)
	}
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	return db.Exec(`UPDATE agent_run_questions SET status=?,revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND run_id=? AND status='open'`, status, tenantID, runID).Error
}
func (r *RunRepositoryPostgres) ResumeQuestionNode(ctx context.Context, tenantID, nodeID string, revision int64, checkpoint json.RawMessage) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	result := db.Exec(`UPDATE agent_run_nodes SET status='ready',phase='think',checkpoint=?::jsonb,revision=revision+1,fencing_token=fencing_token+1,lease_owner=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE tenant_id=? AND id=? AND revision=? AND status='waiting_clarification'`, string(checkpoint), tenantID, nodeID, revision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return d.ErrVersionConflict(nil)
	}
	return nil
}
