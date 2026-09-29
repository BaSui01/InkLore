package enginepersistence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"InkLore/internal/agent/infrastructure/artifactstore"
	"InkLore/internal/platform/nulltype"

	"github.com/google/uuid"
)

type AgentPayloadStoreConfig struct {
	Store          artifactstore.Store
	ArtifactRefs   *artifactstore.ArtifactRefRepository
	ThresholdBytes int
}

type AgentPayloadExternalizer struct {
	store          artifactstore.Store
	refs           *artifactstore.ArtifactRefRepository
	saga           *artifactstore.Saga
	thresholdBytes int
}

type AgentPayloadExternalizeRequest struct {
	ArtifactType   string
	Source         string
	Phase          string
	PayloadJSON    string
	OwnerUserID    nulltype.Int64
	AgentRunID     nulltype.Int64
	AgentNodeRunID nulltype.Int64
}

type AgentExternalizedPayload struct {
	ArtifactType     string
	Source           string
	StorageObjectID  string
	ContentHash      string
	PayloadSizeBytes int64
	OwnerUserID      nulltype.Int64
	AgentRunID       nulltype.Int64
	AgentNodeRunID   nulltype.Int64
	ArtifactRefID    int64
	Reused           bool
}

type externalizedPayloadMarker struct {
	Backend          string `json:"artifact_backend"`
	ArtifactType     string `json:"artifact_type"`
	Source           string `json:"source"`
	StorageObjectID  string `json:"storage_object_id"`
	ContentHash      string `json:"content_hash"`
	Summary          string `json:"summary,omitempty"`
	PayloadSizeBytes int64  `json:"payload_size_bytes"`
	Externalized     bool   `json:"artifact_externalized"`
	Truncated        bool   `json:"truncated,omitempty"`
}

type agentPayloadExternalizer = AgentPayloadExternalizer
type agentPayloadExternalizeRequest = AgentPayloadExternalizeRequest
type agentExternalizedPayload = AgentExternalizedPayload

func NewAgentPayloadExternalizer(cfg AgentPayloadStoreConfig) *AgentPayloadExternalizer {
	if cfg.Store == nil {
		return nil
	}
	if cfg.ThresholdBytes <= 0 {
		cfg.ThresholdBytes = 32 * 1024
	}
	return &AgentPayloadExternalizer{
		store:          cfg.Store,
		refs:           cfg.ArtifactRefs,
		saga:           artifactstore.NewSaga(cfg.ArtifactRefs, cfg.Store),
		thresholdBytes: cfg.ThresholdBytes,
	}
}

func newAgentPayloadExternalizer(cfg AgentPayloadStoreConfig) *AgentPayloadExternalizer {
	return NewAgentPayloadExternalizer(cfg)
}

func (e *AgentPayloadExternalizer) maybeExternalize(ctx context.Context, req agentPayloadExternalizeRequest) (string, *agentExternalizedPayload, error) {
	return e.MaybeExternalize(ctx, req)
}

func (e *AgentPayloadExternalizer) MaybeExternalize(ctx context.Context, req AgentPayloadExternalizeRequest) (string, *AgentExternalizedPayload, error) {
	payload := defaultJSON(req.PayloadJSON)
	if e == nil || e.store == nil || len(payload) <= e.thresholdBytes || isExternalizedPayload(payload) {
		return payload, nil, nil
	}
	doc, err := artifactstore.NewJSONDocument(req.ArtifactType, req.Source, payload)
	if err != nil {
		return "", nil, err
	}
	if req.OwnerUserID.Valid {
		doc.OwnerUserID = req.OwnerUserID.Int64
	}
	if req.AgentRunID.Valid {
		doc.AgentRunID = req.AgentRunID.Int64
	}
	if req.AgentNodeRunID.Valid {
		doc.AgentNodeRunID = req.AgentNodeRunID.Int64
	}
	idempotencyParts := []string{
		req.ArtifactType,
		req.Source,
		fmt.Sprint(req.OwnerUserID.Int64),
		fmt.Sprint(req.AgentRunID.Int64),
		fmt.Sprint(req.AgentNodeRunID.Int64),
	}
	if phase := strings.TrimSpace(req.Phase); phase != "" {
		// Node output transitions must not reuse a waiting payload as terminal
		// output. Content hash remains part of the key so competing terminal
		// payloads retain the exact winner marker chosen by the node CAS.
		idempotencyParts = append(idempotencyParts, phase)
	}
	idempotencyParts = append(idempotencyParts, doc.ContentHash)
	if !req.AgentRunID.Valid && !req.AgentNodeRunID.Valid {
		idempotencyParts = append(idempotencyParts, uuid.NewString())
	}
	result, err := e.saga.Write(ctx, artifactstore.WriteRequest{
		Ref: artifactstore.ArtifactRefCreate{
			ArtifactType:   req.ArtifactType,
			Source:         req.Source,
			OwnerUserID:    req.OwnerUserID,
			AgentRunID:     req.AgentRunID,
			AgentNodeRunID: req.AgentNodeRunID,
			StorageBackend: artifactstore.BackendPostgres,
			ContentHash:    doc.ContentHash,
			SizeBytes:      doc.PayloadSizeBytes,
			IdempotencyKey: artifactstore.StableIdempotencyKey("agent_payload", idempotencyParts...),
		},
		Document: doc,
	})
	if err != nil {
		return "", nil, err
	}
	stored := result.Stored
	ext := &agentExternalizedPayload{
		ArtifactType:     req.ArtifactType,
		Source:           req.Source,
		StorageObjectID:  stored.ID,
		ContentHash:      stored.ContentHash,
		PayloadSizeBytes: stored.PayloadSizeBytes,
		OwnerUserID:      req.OwnerUserID,
		AgentRunID:       req.AgentRunID,
		AgentNodeRunID:   req.AgentNodeRunID,
		ArtifactRefID:    result.Ref.ID,
		Reused:           result.Reused,
	}
	marker := externalizedPayloadMarker{
		Externalized:     true,
		Backend:          stored.Backend,
		ArtifactType:     req.ArtifactType,
		Source:           req.Source,
		StorageObjectID:  stored.ID,
		ContentHash:      stored.ContentHash,
		PayloadSizeBytes: stored.PayloadSizeBytes,
		Summary:          summarizeJSONPayload(payload),
		Truncated:        len(payload) > payloadSummaryLimit,
	}
	markerJSON, err := json.Marshal(marker)
	if err != nil {
		return "", nil, fmt.Errorf("序列化 artifact marker 失败: %w", err)
	}
	return string(markerJSON), ext, nil
}

func (e *AgentPayloadExternalizer) CreateArtifactRef(ext *AgentExternalizedPayload, runID, nodeRunID int64) (int64, error) {
	if e == nil || e.refs == nil || ext == nil {
		return 0, nil
	}
	runRef := ext.AgentRunID
	if runID > 0 {
		runRef = nulltype.Int64{Int64: runID, Valid: true}
	}
	nodeRef := ext.AgentNodeRunID
	if nodeRunID > 0 {
		nodeRef = nulltype.Int64{Int64: nodeRunID, Valid: true}
	}
	if ext.ArtifactRefID <= 0 {
		return 0, fmt.Errorf("artifact ref id is required")
	}
	err := e.refs.AttachOwners(context.Background(), ext.ArtifactRefID, artifactstore.ArtifactOwners{
		OwnerUserID:    ext.OwnerUserID,
		AgentRunID:     runRef,
		AgentNodeRunID: nodeRef,
	})
	return ext.ArtifactRefID, err
}

func (e *AgentPayloadExternalizer) createArtifactRef(ext *AgentExternalizedPayload, runID, nodeRunID int64) (int64, error) {
	return e.CreateArtifactRef(ext, runID, nodeRunID)
}

func (e *AgentPayloadExternalizer) Compensate(ext *AgentExternalizedPayload, category string) error {
	if e == nil || e.saga == nil || ext == nil || ext.ArtifactRefID <= 0 || ext.Reused {
		return nil
	}
	return e.saga.Compensate(context.Background(), ext.ArtifactRefID, category, nil)
}

func (e *AgentPayloadExternalizer) compensate(ext *AgentExternalizedPayload, category string) error {
	return e.Compensate(ext, category)
}

func (e *AgentPayloadExternalizer) Resolve(ctx context.Context, value string) string {
	if e == nil || e.store == nil {
		return defaultJSON(value)
	}
	marker, ok := parseExternalizedPayloadMarker(value)
	if !ok || marker.StorageObjectID == "" {
		return defaultJSON(value)
	}
	doc, err := e.store.Get(ctx, marker.StorageObjectID)
	if err != nil || doc == nil || strings.TrimSpace(doc.PayloadJSON) == "" {
		return defaultJSON(value)
	}
	return doc.PayloadJSON
}

func (e *AgentPayloadExternalizer) resolve(ctx context.Context, value string) string {
	return e.Resolve(ctx, value)
}

func isExternalizedPayload(value string) bool {
	_, ok := parseExternalizedPayloadMarker(value)
	return ok
}

func parseExternalizedPayloadMarker(value string) (externalizedPayloadMarker, bool) {
	var marker externalizedPayloadMarker
	if strings.TrimSpace(value) == "" {
		return marker, false
	}
	if err := json.Unmarshal([]byte(value), &marker); err != nil {
		return marker, false
	}
	return marker, marker.Externalized && marker.StorageObjectID != ""
}

func summarizeJSONPayload(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= payloadSummaryLimit {
		return value
	}
	return value[:payloadSummaryLimit]
}

const payloadSummaryLimit = 512

func defaultJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}
