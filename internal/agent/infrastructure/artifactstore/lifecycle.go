package artifactstore

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"InkLore/internal/platform/nulltype"
)

const (
	StatusPending        = "pending"
	StatusReady          = "ready"
	StatusCleanupPending = "cleanup_pending"
	StatusFailed         = "failed"
	StatusDeleted        = "deleted"
	StatusActiveLegacy   = "active"
)

type ArtifactLocator struct {
	StorageObjectID string
	StorageKey      string
	StorageBackend  string
	ContentHash     string
	SizeBytes       int64
}

type ArtifactOwners struct {
	OwnerUserID    nulltype.Int64
	WorkflowID     nulltype.Int64
	TaskID         nulltype.Int64
	AgentRunID     nulltype.Int64
	AgentNodeRunID nulltype.Int64
}

func StableIdempotencyKey(scope string, parts ...string) string {
	values := append([]string{strings.TrimSpace(scope)}, parts...)
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	prefix := strings.Trim(strings.ToLower(strings.TrimSpace(scope)), "-_: ")
	if prefix == "" {
		prefix = "artifact"
	}
	return prefix + ":" + hex.EncodeToString(sum[:])
}

func IsLifecycleStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case StatusPending, StatusReady, StatusCleanupPending, StatusFailed, StatusDeleted, StatusActiveLegacy:
		return true
	default:
		return false
	}
}

func NormalizeErrorCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	var b strings.Builder
	for _, r := range category {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '-' || r == ' ':
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	if b.Len() == 0 {
		return "artifact_operation_failed"
	}
	return b.String()
}
