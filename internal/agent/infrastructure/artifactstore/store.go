package artifactstore

import (
	"InkLore/internal/platform/timeutil"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BackendPostgres 是制品正文的存储后端标签（ADR-0008：统一落 PostgreSQL JSONB）。
const BackendPostgres = "postgres"

type Store interface {
	Put(ctx context.Context, doc Document) (StoredDocument, error)
	Get(ctx context.Context, id string) (*Document, error)
	Delete(ctx context.Context, id string) error
	Health(ctx context.Context) error
	Close(ctx context.Context) error
}

type Document struct {
	UpdatedAt          time.Time      `json:"updated_at"`
	CreatedAt          time.Time      `json:"created_at"`
	Payload            any            `json:"payload,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	RetentionExpiresAt *time.Time     `json:"retention_expires_at,omitempty"`
	Type               string         `json:"type"`
	Source             string         `json:"source"`
	ID                 string         `json:"id,omitempty"`
	ContentHash        string         `json:"content_hash"`
	PayloadJSON        string         `json:"payload_json,omitempty"`
	Text               string         `json:"text,omitempty"`
	BinaryRefs         []BinaryRef    `json:"binary_refs,omitempty"`
	WorkflowID         int64          `json:"workflow_id,omitempty"`
	ArtifactRefID      int64          `json:"artifact_ref_id,omitempty"`
	AgentNodeRunID     int64          `json:"agent_node_run_id,omitempty"`
	PayloadSizeBytes   int64          `json:"payload_size_bytes"`
	AgentRunID         int64          `json:"agent_run_id,omitempty"`
	TaskID             int64          `json:"task_id,omitempty"`
	OwnerUserID        int64          `json:"owner_user_id,omitempty"`
}

type BinaryRef struct {
	StorageBackend string `json:"storage_backend"`
	StorageKey     string `json:"storage_key"`
	URL            string `json:"url,omitempty"`
	MIMEType       string `json:"mime_type,omitempty"`
	Hash           string `json:"hash,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
}

type StoredDocument struct {
	CreatedAt        time.Time
	ID               string
	Backend          string
	ContentHash      string
	PayloadSizeBytes int64
}

func NewJSONDocument(artifactType, source, payloadJSON string) (Document, error) {
	payloadJSON = defaultJSON(payloadJSON)
	payload, err := decodeJSONPayload(payloadJSON)
	if err != nil {
		return Document{}, err
	}
	return Document{
		Type:             artifactType,
		Source:           source,
		Payload:          payload,
		PayloadJSON:      payloadJSON,
		ContentHash:      HashString(payloadJSON),
		PayloadSizeBytes: int64(len(payloadJSON)),
	}, nil
}

func HashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func defaultJSON(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}

func decodeJSONPayload(value string) (any, error) {
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil, fmt.Errorf("解析 artifact JSON payload 失败: %w", err)
	}
	return payload, nil
}

func ensureDocumentDefaults(doc *Document) {
	now := timeutil.Now().UTC()
	if doc.ID == "" {
		doc.ID = uuid.NewString()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = now
	}
	if doc.ContentHash == "" {
		if doc.PayloadJSON != "" {
			doc.ContentHash = HashString(doc.PayloadJSON)
		} else {
			doc.ContentHash = HashString(doc.Text)
		}
	}
	if doc.PayloadSizeBytes <= 0 {
		if doc.PayloadJSON != "" {
			doc.PayloadSizeBytes = int64(len(doc.PayloadJSON))
		} else {
			doc.PayloadSizeBytes = int64(len(doc.Text))
		}
	}
}

type MemoryStore struct {
	docs map[string]Document
	mu   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{docs: map[string]Document{}}
}

func (s *MemoryStore) Put(ctx context.Context, doc Document) (StoredDocument, error) {
	if err := ctx.Err(); err != nil {
		return StoredDocument{}, err
	}
	ensureDocumentDefaults(&doc)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.docs[doc.ID]; ok && existing.ContentHash != doc.ContentHash {
		return StoredDocument{}, fmt.Errorf("artifact idempotency conflict: document id already has different content")
	}
	s.docs[doc.ID] = doc
	return StoredDocument{
		ID:               doc.ID,
		Backend:          BackendPostgres,
		ContentHash:      doc.ContentHash,
		PayloadSizeBytes: doc.PayloadSizeBytes,
		CreatedAt:        doc.CreatedAt,
	}, nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.docs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &doc, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, id)
	return nil
}

func (s *MemoryStore) Health(ctx context.Context) error {
	return ctx.Err()
}

func (s *MemoryStore) Close(ctx context.Context) error {
	return ctx.Err()
}

var ErrNotFound = fmt.Errorf("artifact document not found")
