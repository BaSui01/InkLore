package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	apperrors "InkLore/internal/platform/errors"

	"github.com/google/uuid"
)

var tracePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func invalid() error       { return apperrors.Contract(400, "platform.validation_failed", nil) }
func invalidStored() error { return errors.New("audit: stored event violates safe schema") }
func validID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.Version() == 4 && id.Variant() == uuid.RFC4122 && id.String() == s
}
func optionalID(s *string) bool { return s == nil || validID(*s) }
func validAction(s string) bool {
	switch s {
	case ActionLogin, ActionRegister, ActionPasswordReset, ActionRefresh, ActionLogout, ActionWSTicket, ActionRefreshReused, ActionCreateMember, ActionUpdateMember, ActionResetPassword, ActionRevokeSessions, ActionBootstrap, ActionPasswordChanged, ActionTenantSwitched, ActionMailSettingsUpdated, ActionMailTestSent:
		return true
	}
	return domainAction(s)
}
func validAudience(s string) bool { return s == "admin" || s == "creator" || s == "system" }
func validOutcome(s string) bool  { return s == "succeeded" || s == "denied" || s == "failed" }

// RolesValue serializes a known role set as a canonical comma-separated string.
// The wire schema permits strings, not JSON arrays, for before/after values.
func RolesValue(roles []string) (string, error) {
	if len(roles) == 0 || len(roles) > 3 {
		return "", invalid()
	}
	copyRoles := append([]string(nil), roles...)
	sort.Strings(copyRoles)
	for i, role := range copyRoles {
		if role != "creator" && role != "operator" && role != "auditor" {
			return "", invalid()
		}
		if i > 0 && role == copyRoles[i-1] {
			return "", invalid()
		}
	}
	return strings.Join(copyRoles, ","), nil
}

func safeValue(field string, value *string) bool {
	// Check field even for null values: no hidden password/body/key field labels.
	switch field {
	case "user_id", "member_id", "session_id":
		return value == nil || validID(*value)
	case "roles":
		if value == nil {
			return true
		}
		canonical, err := RolesValue(strings.Split(*value, ","))
		return err == nil && canonical == *value
	case "status":
		return value == nil || *value == "active" || *value == "disabled"
	case "revision", "revoked_count":
		if value == nil {
			return true
		}
		n, err := strconv.ParseInt(*value, 10, 64)
		return err == nil && n >= 0 && (field != "revision" || n > 0) && strconv.FormatInt(n, 10) == *value
	}
	return false
}

func safeChanges(changes []SafeChange) bool {
	if len(changes) > 100 {
		return false
	}
	seen := make(map[string]bool, len(changes))
	for _, change := range changes {
		if seen[change.Field] || !safeValue(change.Field, change.Before) || !safeValue(change.Field, change.After) {
			return false
		}
		seen[change.Field] = true
	}
	return true
}

func validateRecord(r Record) bool {
	if r.Action == ActionPasswordChanged {
		for _, change := range r.SafeChanges {
			if change.Field != "session_id" {
				return false
			}
		}
	}
	return validID(r.TenantID) && optionalID(r.ActorID) && optionalID(r.ResourceID) &&
		validAudience(r.Audience) && validAction(r.Action) && validOutcome(r.Outcome) &&
		validID(r.RequestID) && tracePattern.MatchString(r.TraceID) && safeChanges(r.SafeChanges)
}

func decodeChanges(raw []byte) ([]SafeChange, error) {
	compact := new(bytes.Buffer)
	if json.Compact(compact, raw) != nil {
		return nil, invalidStored()
	}
	// The frozen DDL default is an empty object; normalize only that exact shape.
	if compact.String() == "{}" {
		return []SafeChange{}, nil
	}
	if compact.Len() == 0 || compact.Bytes()[0] != '[' {
		return nil, invalidStored()
	}
	var changes []SafeChange
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&changes) != nil {
		return nil, invalidStored()
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || !safeChanges(changes) {
		return nil, invalidStored()
	}
	// Enforce all three required fields, including explicit null before/after.
	var fields []map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, invalidStored()
	}
	for _, f := range fields {
		if len(f) != 3 || f["field"] == nil || f["before"] == nil || f["after"] == nil {
			return nil, invalidStored()
		}
	}
	if changes == nil {
		changes = []SafeChange{}
	}
	return changes, nil
}
