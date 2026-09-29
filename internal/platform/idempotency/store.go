package idempotency

import (
	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"encoding/json"

	"gorm.io/gorm"
)

// All callers hold the five-part advisory transaction lock. Persisted processing
// rows are never stolen or deleted, including expired rows.
func claim(db *gorm.DB, req Request, hash string) (string, *Result, error) {
	var row struct {
		ID, RequestHash, State, Body string
		ResponseStatus               int
		Expired                      bool
	}
	found := db.Raw("SELECT id,request_hash,state,response_status,COALESCE(response_body::text,'') AS body,expires_at<=clock_timestamp() AS expired FROM platform_idempotency WHERE tenant_id=? AND actor_id=? AND method=? AND canonical_path=? AND idempotency_key=? FOR UPDATE", req.TenantID, req.ActorID, req.Method, req.Path, req.Key).Scan(&row)
	if found.Error != nil {
		return "", nil, found.Error
	}
	if found.RowsAffected == 1 {
		if row.State == "processing" {
			return "", nil, inProgress()
		}
		if row.State != "completed" {
			return "", nil, internal()
		}
		if !row.Expired {
			if row.RequestHash != hash {
				return "", nil, apperrors.Contract(409, "platform.idempotency_conflict", nil)
			}
			result, err := replayResult(row.ResponseStatus, []byte(row.Body))
			return row.ID, result, err
		}
		reset := db.Exec("UPDATE platform_idempotency SET request_hash=?,state='processing',response_status=NULL,response_body=NULL,operation_id=NULL,expires_at=clock_timestamp()+interval '24 hours',updated_at=clock_timestamp() WHERE id=? AND state='completed' AND expires_at<=clock_timestamp()", hash, row.ID)
		if reset.Error != nil {
			return "", nil, reset.Error
		}
		if reset.RowsAffected != 1 {
			return "", nil, inProgress()
		}
		return row.ID, nil, nil
	}
	// ON CONFLICT is defensive: all cooperative executors serialize via advisory lock.
	inserted := db.Raw("INSERT INTO platform_idempotency(tenant_id,actor_id,method,canonical_path,idempotency_key,request_hash,state,expires_at) VALUES(?,?,?,?,?,?,'processing',clock_timestamp()+interval '24 hours') ON CONFLICT(tenant_id,actor_id,method,canonical_path,idempotency_key) DO NOTHING RETURNING id", req.TenantID, req.ActorID, req.Method, req.Path, req.Key, hash).Scan(&row)
	if inserted.Error != nil {
		return "", nil, inserted.Error
	}
	if inserted.RowsAffected != 1 {
		return "", nil, inProgress()
	}
	return row.ID, nil, nil
}

func complete(db *gorm.DB, id, hash string, result Result) error {
	// Bind JSON as text, not bytea; the envelope and business writes share this commit.
	updated := db.Exec("UPDATE platform_idempotency SET state='completed',response_status=?,response_body=?::jsonb,expires_at=clock_timestamp()+interval '24 hours',updated_at=clock_timestamp() WHERE id=? AND request_hash=? AND state='processing'", result.Status, string(result.Body), id, hash)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return internal()
	}
	return nil
}

func replayResult(status int, raw []byte) (*Result, error) {
	var envelope struct {
		Code       string          `json:"code"`
		Data       json.RawMessage `json:"data"`
		MessageKey string          `json:"message_key"`
		Details    map[string]any  `json:"details"`
		RequestID  string          `json:"request_id"`
		TraceID    string          `json:"trace_id"`
	}
	if json.Unmarshal(raw, &envelope) != nil || !scope.ValidID(envelope.RequestID) || !tracePattern.MatchString(envelope.TraceID) {
		return nil, internal()
	}
	if status == 200 || status == 201 || status == 202 {
		if envelope.Code != "ok" || len(envelope.Data) == 0 {
			return nil, internal()
		}
	} else {
		if (status != 409 && status != 422) || !apperrors.IsContractStatus(envelope.Code, status) || envelope.MessageKey != "errors."+envelope.Code {
			return nil, internal()
		}
		// Even a persisted error cannot bypass the public details allowlist.
		_, safe := response.FailureBody(apperrors.Contract(status, envelope.Code, envelope.Details), envelope.RequestID, envelope.TraceID)
		var err error
		raw, err = json.Marshal(safe)
		if err != nil {
			return nil, internal()
		}
	}
	return &Result{Status: status, Body: raw, Replay: true}, nil
}
