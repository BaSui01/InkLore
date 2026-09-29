package artifactstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"InkLore/internal/platform/tenantcontext"

	"gorm.io/gorm"
)

// DefaultMaxPayloadBytes 单条制品正文的体积上限（ADR-0008：超过上限应交给对象存储）。
const DefaultMaxPayloadBytes = 4 << 20

// ErrPayloadTooLarge 制品正文超过实现层上限；按 ADR-0008 明确报错，绝不静默截断。
var ErrPayloadTooLarge = errors.New("artifact payload exceeds the configured limit")

// PostgresStore 把制品正文落到 PostgreSQL（INK-047）。
//
// 语义与 MemoryStore 保持一致：同 id 不同 content_hash 视为幂等冲突；
// 未命中的 Get/Delete 返回 ErrNotFound；数据库连接由组合根持有，Close 不做任何事。
type PostgresStore struct {
	db              *gorm.DB
	maxPayloadBytes int
}

// PostgresStoreOptions 覆盖实现层默认值。
type PostgresStoreOptions struct {
	MaxPayloadBytes int
}

// NewPostgresStore 构造制品库；db 为空时返回 nil（调用方据此退化为内存实现）。
func NewPostgresStore(db *gorm.DB, options ...PostgresStoreOptions) *PostgresStore {
	if db == nil {
		return nil
	}
	store := &PostgresStore{db: db, maxPayloadBytes: DefaultMaxPayloadBytes}
	for _, option := range options {
		if option.MaxPayloadBytes > 0 {
			store.maxPayloadBytes = option.MaxPayloadBytes
		}
	}
	return store
}

var _ Store = (*PostgresStore)(nil)

// Put 写入制品正文；同 id 同内容幂等，同 id 不同内容返回冲突错误。
func (s *PostgresStore) Put(ctx context.Context, doc Document) (StoredDocument, error) {
	if s == nil || s.db == nil {
		return StoredDocument{}, fmt.Errorf("artifact store is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return StoredDocument{}, err
	}
	ensureDocumentDefaults(&doc)
	if doc.PayloadSizeBytes > int64(s.maxPayloadBytes) {
		return StoredDocument{}, fmt.Errorf("%w: %d > %d bytes", ErrPayloadTooLarge, doc.PayloadSizeBytes, s.maxPayloadBytes)
	}

	payloadJSON := strings.TrimSpace(doc.PayloadJSON)
	if payloadJSON == "" && doc.Payload != nil {
		if encoded, err := json.Marshal(doc.Payload); err == nil {
			payloadJSON = string(encoded)
		}
	}
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	metadataJSON := "{}"
	if len(doc.Metadata) > 0 {
		if encoded, err := json.Marshal(doc.Metadata); err == nil {
			metadataJSON = string(encoded)
		}
	}

	var existing string
	// 注意：Raw(...) 返回的是 *gorm.DB，它永远不是 nil；必须检查 .Error 字段。
	if scanErr := s.db.WithContext(ctx).Raw("SELECT content_hash FROM artifact_documents WHERE id = ? AND tenant_id = ?", doc.ID, tenantcontext.OrDefault(ctx)).Scan(&existing).Error; scanErr != nil {
		return StoredDocument{}, scanErr
	}
	if strings.TrimSpace(existing) != "" && existing != doc.ContentHash {
		return StoredDocument{}, fmt.Errorf("artifact idempotency conflict: document id already has different content")
	}

	err := s.db.WithContext(ctx).Exec(
		"INSERT INTO artifact_documents (id, tenant_id, artifact_type, source, content_hash, backend, payload, payload_json, "+
			"text_content, metadata, owner_user_id, agent_run_id, agent_node_run_id, workflow_id, task_id, "+
			"artifact_ref_id, payload_size_bytes, retention_expires_at, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?, ?::jsonb, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT (id) DO UPDATE SET artifact_type = EXCLUDED.artifact_type, source = EXCLUDED.source, "+
			"content_hash = EXCLUDED.content_hash, backend = EXCLUDED.backend, payload = EXCLUDED.payload, "+
			"payload_json = EXCLUDED.payload_json, text_content = EXCLUDED.text_content, "+
			"metadata = EXCLUDED.metadata, payload_size_bytes = EXCLUDED.payload_size_bytes, "+
			"retention_expires_at = EXCLUDED.retention_expires_at, updated_at = EXCLUDED.updated_at",
		doc.ID, tenantcontext.OrDefault(ctx), doc.Type, doc.Source, doc.ContentHash, BackendPostgres, payloadJSON, payloadJSON, doc.Text,
		metadataJSON, nullableInt64(doc.OwnerUserID), nullableInt64(doc.AgentRunID), nullableInt64(doc.AgentNodeRunID),
		nullableInt64(doc.WorkflowID), nullableInt64(doc.TaskID), nullableInt64(doc.ArtifactRefID),
		doc.PayloadSizeBytes, doc.RetentionExpiresAt, doc.CreatedAt, doc.UpdatedAt,
	).Error
	if err != nil {
		return StoredDocument{}, err
	}
	return StoredDocument{
		ID:               doc.ID,
		Backend:          BackendPostgres,
		ContentHash:      doc.ContentHash,
		PayloadSizeBytes: doc.PayloadSizeBytes,
		CreatedAt:        doc.CreatedAt,
	}, nil
}

type artifactDocumentRow struct {
	RetentionExpiresAt *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Payload            []byte
	Metadata           []byte
	ID                 string
	ArtifactType       string
	Source             string
	ContentHash        string
	PayloadJSON        string
	Text               string `gorm:"column:text_content"`
	OwnerUserID        *int64
	AgentRunID         *int64
	AgentNodeRunID     *int64
	WorkflowID         *int64
	TaskID             *int64
	ArtifactRefID      *int64
	PayloadSizeBytes   int64
}

// Get 按 id 读取制品正文。
func (s *PostgresStore) Get(ctx context.Context, id string) (*Document, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("artifact store is not initialized")
	}
	var row artifactDocumentRow
	if err := s.db.WithContext(ctx).Raw(
		"SELECT id, artifact_type, source, content_hash, payload, metadata, payload_json, text_content, "+
			"owner_user_id, agent_run_id, agent_node_run_id, workflow_id, task_id, artifact_ref_id, "+
			"payload_size_bytes, retention_expires_at, created_at, updated_at "+
			"FROM artifact_documents WHERE id = ? AND tenant_id = ?", id, tenantcontext.OrDefault(ctx)).Scan(&row).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.ID) == "" {
		return nil, ErrNotFound
	}
	doc := &Document{
		ID:                 row.ID,
		Type:               row.ArtifactType,
		Source:             row.Source,
		ContentHash:        row.ContentHash,
		PayloadJSON:        row.PayloadJSON,
		Text:               row.Text,
		PayloadSizeBytes:   row.PayloadSizeBytes,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		RetentionExpiresAt: row.RetentionExpiresAt,
	}
	if len(row.Payload) > 0 {
		var payload any
		if err := json.Unmarshal(row.Payload, &payload); err == nil {
			doc.Payload = payload
		}
	}
	if len(row.Metadata) > 0 {
		metadata := map[string]any{}
		if err := json.Unmarshal(row.Metadata, &metadata); err == nil {
			doc.Metadata = metadata
		}
	}
	if row.OwnerUserID != nil {
		doc.OwnerUserID = *row.OwnerUserID
	}
	if row.AgentRunID != nil {
		doc.AgentRunID = *row.AgentRunID
	}
	if row.AgentNodeRunID != nil {
		doc.AgentNodeRunID = *row.AgentNodeRunID
	}
	if row.WorkflowID != nil {
		doc.WorkflowID = *row.WorkflowID
	}
	if row.TaskID != nil {
		doc.TaskID = *row.TaskID
	}
	if row.ArtifactRefID != nil {
		doc.ArtifactRefID = *row.ArtifactRefID
	}
	return doc, nil
}

// Delete 删除制品正文；未命中返回 ErrNotFound。
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("artifact store is not initialized")
	}
	result := s.db.WithContext(ctx).Exec("DELETE FROM artifact_documents WHERE id = ? AND tenant_id = ?", id, tenantcontext.OrDefault(ctx))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Health 探测存储可用性。
func (s *PostgresStore) Health(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("artifact store is not initialized")
	}
	var one int
	return s.db.WithContext(ctx).Raw("SELECT 1").Scan(&one).Error
}

// Close 不做任何事：数据库连接由组合根持有与释放。
func (s *PostgresStore) Close(ctx context.Context) error {
	return nil
}

// nullableInt64 把 0 视为「未设置」，写 NULL 而不是 0。
func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
