package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/uow"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type NovelGuideRepository struct{ manager *uow.Manager }

func NewNovelGuideRepository(manager *uow.Manager) (*NovelGuideRepository, error) {
	if manager == nil {
		return nil, errors.New("agent novel guide: UOW manager required")
	}
	return &NovelGuideRepository{manager: manager}, nil
}

func (r *NovelGuideRepository) db(ctx context.Context, write bool) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if write {
		db, err = r.manager.Require(ctx)
	} else {
		db, err = r.manager.DB(ctx)
	}
	if err != nil {
		return nil, err
	}
	return db.Session(&gorm.Session{Logger: logger.Discard}), nil
}

func (r *NovelGuideRepository) CreateNovelGuide(ctx context.Context, session *d.NovelGuideSession) (bool, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return false, err
	}
	turns, err := marshalGuideTurns(session.Turns)
	if err != nil {
		return false, err
	}
	result := db.Exec(`INSERT INTO agent_novel_creation_sessions
		(id,tenant_id,owner_user_id,created_at,updated_at,revision,start_key,start_hash,definition_version_id,policy_version_id,model_name,max_output_tokens,temperature,system_instruction,initial_idea,turns,status,expires_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::jsonb,?,?) ON CONFLICT (tenant_id,owner_user_id,start_key) DO NOTHING`,
		session.ID, session.TenantID, session.OwnerUserID, session.CreatedAt, session.UpdatedAt,
		session.Revision, session.StartKey, session.StartRequestHash, session.DefinitionVersionID,
		session.PolicyVersionID, session.ModelName, session.MaxOutputTokens, session.Temperature, session.SystemInstruction, session.InitialIdea, string(turns), session.Status, session.ExpiresAt)
	return result.RowsAffected == 1, result.Error
}

type novelGuideRow struct {
	ID                  string
	TenantID            string
	OwnerUserID         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Revision            int64
	Status              string
	InitialIdea         string
	Turns               []byte
	CurrentQuestion     []byte
	Proposal            []byte
	CreatedNovelID      sql.NullString
	DefinitionVersionID string
	PolicyVersionID     string
	ModelName           string
	SystemInstruction   string
	MaxOutputTokens     int
	Temperature         float64
	StartKey            string
	StartRequestHash    string
	StartResponse       []byte
	ErrorCode           string
	ExpiresAt           time.Time
}

const novelGuideColumns = `id,tenant_id,owner_user_id,created_at,updated_at,revision,status,initial_idea,turns,
	current_question,proposal,created_novel_id,definition_version_id,policy_version_id,model_name,max_output_tokens,temperature,system_instruction,start_key,start_hash,start_response,error_code,expires_at`

func (r *NovelGuideRepository) GetNovelGuide(ctx context.Context, tenantID, ownerID, id string, lock bool) (*d.NovelGuideSession, error) {
	db, err := r.db(ctx, lock)
	if err != nil {
		return nil, err
	}
	query := "SELECT " + novelGuideColumns + " FROM agent_novel_creation_sessions WHERE tenant_id=? AND owner_user_id=? AND id=? AND expires_at>clock_timestamp()"
	if lock {
		query += " FOR UPDATE"
	}
	var row novelGuideRow
	if err = db.Raw(query, tenantID, ownerID, id).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, d.ErrNotFound()
	}
	return row.toDomain()
}

func (r *NovelGuideRepository) GetNovelGuideByStartKey(ctx context.Context, tenantID, ownerID, key string, lock bool) (*d.NovelGuideSession, error) {
	db, err := r.db(ctx, lock)
	if err != nil {
		return nil, err
	}
	query := "SELECT " + novelGuideColumns + " FROM agent_novel_creation_sessions WHERE tenant_id=? AND owner_user_id=? AND start_key=? AND expires_at>clock_timestamp()"
	if lock {
		query += " FOR UPDATE"
	}
	var row novelGuideRow
	if err = db.Raw(query, tenantID, ownerID, key).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, d.ErrNotFound()
	}
	return row.toDomain()
}

func (r *NovelGuideRepository) UpdateNovelGuide(ctx context.Context, session *d.NovelGuideSession, expectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	turns, err := marshalGuideTurns(session.Turns)
	if err != nil {
		return err
	}
	question, err := marshalGuideJSON(session.CurrentQuestion)
	if err != nil {
		return err
	}
	proposal, err := marshalGuideJSON(session.Proposal)
	if err != nil {
		return err
	}
	var startResponse any
	if len(session.StartResponse) > 0 {
		startResponse = string(session.StartResponse)
	}
	var createdNovel any
	if session.CreatedNovelID != "" {
		createdNovel = session.CreatedNovelID
	}
	result := db.Exec(`UPDATE agent_novel_creation_sessions
		SET updated_at=?,revision=?,status=?,turns=?::jsonb,current_question=?::jsonb,proposal=?::jsonb,
			created_novel_id=?,start_response=?::jsonb,error_code=?,expires_at=?
		WHERE tenant_id=? AND owner_user_id=? AND id=? AND revision=? AND expires_at>clock_timestamp()`,
		session.UpdatedAt, session.Revision, session.Status, string(turns), question, proposal,
		createdNovel, startResponse, nullableGuideText(session.ErrorCode), session.ExpiresAt,
		session.TenantID, session.OwnerUserID, session.ID, expectedRevision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return d.ErrVersionConflict(map[string]any{"current_revision": expectedRevision})
	}
	return nil
}

func (r *NovelGuideRepository) BeginNovelGuideCommand(ctx context.Context, command *d.NovelGuideCommand) (*d.NovelGuideCommand, bool, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return nil, false, err
	}
	result := db.Exec(`INSERT INTO agent_novel_creation_commands
		(id,tenant_id,session_id,owner_user_id,command_type,idempotency_key,request_hash,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,'processing',clock_timestamp(),clock_timestamp()) ON CONFLICT DO NOTHING`,
		command.ID, command.TenantID, command.SessionID, command.OwnerUserID, command.CommandType,
		command.IdempotencyKey, command.RequestHash)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		copy := *command
		return &copy, true, nil
	}
	var row novelGuideCommandRow
	result = db.Raw(`SELECT id,tenant_id,session_id,owner_user_id,command_type,idempotency_key,request_hash,status,response,error_code
		FROM agent_novel_creation_commands WHERE tenant_id=? AND owner_user_id=? AND session_id=? AND command_type=? AND idempotency_key=? FOR UPDATE`,
		command.TenantID, command.OwnerUserID, command.SessionID, command.CommandType, command.IdempotencyKey).Scan(&row)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if row.ID == "" {
		return nil, false, d.ErrNotFound()
	}
	return row.toDomain(), false, nil
}

type novelGuideCommandRow struct {
	ID, TenantID, SessionID, OwnerUserID, CommandType, IdempotencyKey, RequestHash, Status, ErrorCode string
	Response                                                                                          []byte
}

func (row novelGuideCommandRow) toDomain() *d.NovelGuideCommand {
	return &d.NovelGuideCommand{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, OwnerUserID: row.OwnerUserID,
		CommandType: row.CommandType, IdempotencyKey: row.IdempotencyKey, RequestHash: row.RequestHash,
		Status: row.Status, Response: row.Response, ErrorCode: row.ErrorCode}
}

func (r *NovelGuideRepository) CompleteNovelGuideCommand(ctx context.Context, tenantID, ownerID, sessionID, commandType, key string, response []byte) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	result := db.Exec(`UPDATE agent_novel_creation_commands SET status='completed',response=?::jsonb,error_code=NULL,updated_at=clock_timestamp()
		WHERE tenant_id=? AND owner_user_id=? AND session_id=? AND command_type=? AND idempotency_key=? AND status='processing'`,
		string(response), tenantID, ownerID, sessionID, commandType, key)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return d.ErrVersionConflict(nil)
	}
	return nil
}

func (r *NovelGuideRepository) FailNovelGuideCommand(ctx context.Context, tenantID, ownerID, sessionID, commandType, key, errorCode string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	result := db.Exec(`UPDATE agent_novel_creation_commands SET status='failed',response=NULL,error_code=?,updated_at=clock_timestamp()
		WHERE tenant_id=? AND owner_user_id=? AND session_id=? AND command_type=? AND idempotency_key=? AND status='processing'`,
		errorCode, tenantID, ownerID, sessionID, commandType, key)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return d.ErrVersionConflict(nil)
	}
	return nil
}

func (row novelGuideRow) toDomain() (*d.NovelGuideSession, error) {
	var turns []d.NovelGuideTurn
	if err := json.Unmarshal(row.Turns, &turns); err != nil {
		return nil, err
	}
	if turns == nil {
		turns = []d.NovelGuideTurn{}
	}
	session := &d.NovelGuideSession{ID: row.ID, TenantID: row.TenantID, OwnerUserID: row.OwnerUserID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision, Status: d.NovelGuideStatus(row.Status),
		InitialIdea: row.InitialIdea, Turns: turns, CreatedNovelID: row.CreatedNovelID.String, DefinitionVersionID: row.DefinitionVersionID,
		PolicyVersionID: row.PolicyVersionID, ModelName: row.ModelName, SystemInstruction: row.SystemInstruction, MaxOutputTokens: row.MaxOutputTokens, Temperature: row.Temperature, StartKey: row.StartKey, StartRequestHash: row.StartRequestHash,
		StartResponse: row.StartResponse, ErrorCode: row.ErrorCode, ExpiresAt: row.ExpiresAt}
	if len(row.CurrentQuestion) > 0 && string(row.CurrentQuestion) != "null" {
		var question d.NovelGuideQuestion
		if err := json.Unmarshal(row.CurrentQuestion, &question); err != nil {
			return nil, err
		}
		session.CurrentQuestion = &question
	}
	if len(row.Proposal) > 0 && string(row.Proposal) != "null" {
		var proposal d.NovelGuideProposal
		if err := json.Unmarshal(row.Proposal, &proposal); err != nil {
			return nil, err
		}
		session.Proposal = &proposal
	}
	return session, nil
}

func marshalGuideTurns(turns []d.NovelGuideTurn) ([]byte, error) {
	if turns == nil {
		turns = []d.NovelGuideTurn{}
	}
	return json.Marshal(turns)
}
func marshalGuideJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}
func nullableGuideText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
