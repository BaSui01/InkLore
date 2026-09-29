package agentrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"InkLore/internal/agent/infrastructure/artifactstore"
)

// AgentPayloadCleanupCandidate is an owner-local proof that an Agent payload
// ref is safe to hand to artifactstore's reconciler.
type AgentPayloadCleanupCandidate struct {
	Ref artifactstore.ArtifactRef
}

// ListAgentPayloadCleanupCandidates returns old Agent payload refs proven to be orphaned.
func (r *Repository) ListAgentPayloadCleanupCandidates(ctx context.Context, olderThan time.Time, limit int, execute ...bool) ([]artifactstore.ArtifactRef, error) {
	if r == nil || r.db == nil || r.db.GORM() == nil {
		return nil, fmt.Errorf("agent payload cleanup repository is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	doExecute := len(execute) > 0 && execute[0]
	items := make([]artifactstore.ArtifactRef, 0)
	const query = `
		SELECT ar.id, ar.artifact_type, ar.owner_user_id, ar.workflow_id,
		       ar.generation_task_id AS task_id, ar.agent_run_id, ar.agent_node_run_id,
		       ar.source, ar.storage_object_id, ar.storage_key, ar.storage_backend,
		       ar.content_hash, ar.size_bytes, ar.status, ar.idempotency_key,
		       ar.attempt_count, ar.last_error_category, ar.ready_at,
		       ar.created_at, ar.updated_at
		FROM artifact_refs ar
		WHERE ar.updated_at <= ?
		  AND ar.artifact_type LIKE 'agent_%'
		  AND ar.status IN ('failed', 'cleanup_pending', 'ready')
		  AND NOT EXISTS (
			SELECT 1 FROM agent_artifacts aa WHERE aa.artifact_ref_id = ar.id
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM agent_runs run
			WHERE (run.input_json LIKE ('%"storage_object_id":"' || ar.storage_object_id || '"%')
			    OR run.output_json LIKE ('%"storage_object_id":"' || ar.storage_object_id || '"%'))
			  AND (ar.agent_run_id IS NULL OR run.id = ar.agent_run_id)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM agent_run_nodes node
			WHERE (node.input_json LIKE ('%"storage_object_id":"' || ar.storage_object_id || '"%')
			    OR node.output_json LIKE ('%"storage_object_id":"' || ar.storage_object_id || '"%'))
			  AND (ar.agent_node_run_id IS NULL OR node.id = ar.agent_node_run_id)
		  )
		ORDER BY ar.updated_at ASC, ar.id ASC
		LIMIT ?`
	if err := r.db.GORM().WithContext(ctx).Raw(query, olderThan, limit).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent payload cleanup candidates 失败: %w", err)
	}
	proven := items[:0]
	for i := range items {
		if items[i].Status == artifactstore.StatusReady {
			if !doExecute {
				continue
			}
			marked, err := r.markReadyAgentOrphan(ctx, items[i].ID, olderThan)
			if err != nil {
				return nil, err
			}
			if !marked {
				continue
			}
			items[i].Status = artifactstore.StatusCleanupPending
		}
		proven = append(proven, items[i])
	}
	return proven, nil
}

func (r *Repository) markReadyAgentOrphan(ctx context.Context, refID int64, cutoff time.Time) (bool, error) {
	if refID <= 0 {
		return false, nil
	}
	result := r.db.GORM().WithContext(ctx).Exec(`
		UPDATE artifact_refs
		SET status = 'cleanup_pending', last_error_category = 'agent_orphan', updated_at = CURRENT_TIMESTAMP
		WHERE artifact_refs.id = ? AND artifact_refs.updated_at <= ? AND artifact_refs.status = 'ready'
		  AND artifact_refs.artifact_type LIKE 'agent_%'
		  AND NOT EXISTS (SELECT 1 FROM agent_artifacts aa WHERE aa.artifact_ref_id = artifact_refs.id)
		  AND NOT EXISTS (
			SELECT 1 FROM agent_runs run
			WHERE (run.input_json LIKE ('%"storage_object_id":"' || artifact_refs.storage_object_id || '"%')
			    OR run.output_json LIKE ('%"storage_object_id":"' || artifact_refs.storage_object_id || '"%'))
			  AND (artifact_refs.agent_run_id IS NULL OR run.id = artifact_refs.agent_run_id)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM agent_run_nodes node
			WHERE (node.input_json LIKE ('%"storage_object_id":"' || artifact_refs.storage_object_id || '"%')
			    OR node.output_json LIKE ('%"storage_object_id":"' || artifact_refs.storage_object_id || '"%'))
			  AND (artifact_refs.agent_node_run_id IS NULL OR node.id = artifact_refs.agent_node_run_id)
		  )`, refID, cutoff)
	if result.Error != nil {
		return false, fmt.Errorf("标记 Agent orphan cleanup_pending 失败: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func IsAgentPayloadCleanupCandidate(ref artifactstore.ArtifactRef) bool {
	return strings.HasPrefix(ref.ArtifactType, "agent_") &&
		(ref.Status == artifactstore.StatusFailed || ref.Status == artifactstore.StatusCleanupPending) &&
		ref.StorageObjectID != ""
}
