package errors

import (
	"encoding/json"
	"regexp"

	"github.com/google/uuid"
)

var reasonKey = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,119}$`)
var factKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,119}$`)

var fieldPointer = regexp.MustCompile(`^(/[a-zA-Z0-9_~.-]+)+$`)

// SafeDetails projects only typed machine metadata. Callers must never put user
// text in reason keys or JSON pointers; private payloads have no response field.
func SafeDetails(input map[string]any) map[string]any {
	out := map[string]any{}
	raw, err := json.Marshal(input)
	if err != nil {
		return out
	}
	var details struct {
		CurrentRevision   int64  `json:"current_revision"`
		RetryAfterSeconds *int64 `json:"retry_after_seconds"`
		OperationID       string `json:"operation_id"`
		Fields            []struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		} `json:"field_errors"`
		Conflicts []struct {
			ResourceID      string  `json:"resource_id"`
			Reason          string  `json:"reason"`
			CurrentRevision int64   `json:"current_revision"`
			SubjectID       string  `json:"subject_id"`
			FactKey         string  `json:"fact_key"`
			CurrentEventID  *string `json:"current_event_id"`
			ProposalIndex   *int    `json:"proposal_index"`
		} `json:"conflicts"`
	}
	if json.Unmarshal(raw, &details) != nil {
		return out
	}
	validRevision := func(v int64) bool { return v > 0 && v <= 9007199254740991 }
	validID := func(s string) bool {
		id, err := uuid.Parse(s)
		return err == nil && id.Version() == 4 && id.Variant() == uuid.RFC4122 && id.String() == s
	}
	if validRevision(details.CurrentRevision) {
		out["current_revision"] = details.CurrentRevision
	}
	if details.RetryAfterSeconds != nil && *details.RetryAfterSeconds >= 0 && *details.RetryAfterSeconds <= 86400 {
		out["retry_after_seconds"] = *details.RetryAfterSeconds
	}
	if validID(details.OperationID) {
		out["operation_id"] = details.OperationID
	}
	fields := []map[string]string{}
	for _, v := range details.Fields {
		if len(fields) >= 100 {
			break
		}
		if len(v.Field) <= 256 && fieldPointer.MatchString(v.Field) && reasonKey.MatchString(v.Reason) {
			fields = append(fields, map[string]string{"field": v.Field, "reason": v.Reason})
		}
	}
	if len(fields) > 0 {
		out["field_errors"] = fields
	}
	conflicts := []map[string]any{}
	for _, v := range details.Conflicts {
		if len(conflicts) >= 100 {
			break
		}
		if validID(v.ResourceID) && reasonKey.MatchString(v.Reason) {
			row := map[string]any{"resource_id": v.ResourceID, "reason": v.Reason}
			if validRevision(v.CurrentRevision) {
				row["current_revision"] = v.CurrentRevision
			}
			if validID(v.SubjectID) && factKey.MatchString(v.FactKey) && v.ProposalIndex != nil && *v.ProposalIndex >= 0 && *v.ProposalIndex <= 99 && (v.CurrentEventID == nil || validID(*v.CurrentEventID)) {
				row["subject_id"] = v.SubjectID
				row["fact_key"] = v.FactKey
				row["proposal_index"] = *v.ProposalIndex
				row["current_event_id"] = v.CurrentEventID
			}
			conflicts = append(conflicts, row)
		}
	}
	if len(conflicts) > 0 {
		out["conflicts"] = conflicts
	}
	return out
}
