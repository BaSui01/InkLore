package sqlscope

import "strings"

// OwnerScopeSQLPredicate builds a SQL predicate for an owner user-id column.
// Priority: UserID > TeamID > DepartmentIDs; UserID=-1 empty; all zero unrestricted (TRUE).
// Returns predicate + args suitable for embedding after AND/WHERE.
func OwnerScopeSQLPredicate(ownerColumn string, userID, teamID int64, departmentIDs []int64) (string, []any) {
	ownerColumn = strings.TrimSpace(ownerColumn)
	if ownerColumn == "" {
		ownerColumn = "user_id"
	}
	if !isSafeColumnReference(ownerColumn) {
		return "1 = 0", nil
	}
	if userID > 0 {
		return ownerColumn + " = ?", []any{userID}
	}
	if userID < 0 {
		return "1 = 0", nil
	}
	if teamID > 0 {
		return "EXISTS (SELECT 1 FROM users scope_u WHERE scope_u.id = " + ownerColumn + " AND scope_u.team_id = ?)", []any{teamID}
	}
	departmentIDs = UniquePositiveInt64(departmentIDs)
	if len(departmentIDs) > 0 {
		placeholders := make([]string, 0, len(departmentIDs))
		args := make([]any, 0, len(departmentIDs))
		for _, id := range departmentIDs {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		return "EXISTS (SELECT 1 FROM user_department_memberships scope_m WHERE scope_m.user_id = " +
			ownerColumn + " AND scope_m.status = 'active' AND scope_m.department_id IN (" +
			strings.Join(placeholders, ",") + "))", args
	}
	return "1 = 1", nil
}

// Placeholders returns "?, ?, ..." for SQL IN lists. Empty/non-positive count => "".
func Placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, count)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// UniquePositiveInt64 returns positive unique ints preserving first-seen order.
func UniquePositiveInt64(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isSafeColumnReference(value string) bool {
	if value == "" || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for _, part := range strings.Split(value, ".") {
		if part == "" {
			return false
		}
		for index, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (index > 0 && r >= '0' && r <= '9') {
				continue
			}
			return false
		}
	}
	return true
}
