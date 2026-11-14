package domain

import (
	"context"
	"encoding/json"

	"InkLore/internal/platform/identity"
)

type CreatorTaskPolicy struct {
	PolicyID        string              `json:"policy_id"`
	PolicyVersionID string              `json:"policy_version_id"`
	TaskKey         string              `json:"task_key"`
	Title           string              `json:"title"`
	Version         int64               `json:"version"`
	TaskKinds       []string            `json:"task_kinds"`
	InputSchema     json.RawMessage     `json:"input_schema"`
	Budget          CreatorPolicyBudget `json:"budget"`
}
type CreatorPolicyBudget struct {
	MaxOutputTokens int `json:"max_output_tokens"`
	TimeoutSeconds  int `json:"timeout_seconds"`
}
type CreatorTaskPolicyPage struct {
	Items      []CreatorTaskPolicy `json:"items"`
	NextCursor *string             `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
}
type CreatorPolicyPort interface {
	ListCreatorTaskPolicies(context.Context, identity.Principal, string, string, int) ([]CreatorTaskPolicy, error)
	LockCreatorTaskPolicy(context.Context, identity.Principal, string, string) (CreatorTaskPolicy, error)
}

func CreatorTaskKinds() []string {
	return []string{string(TaskKindChapterDraft), string(TaskKindChapterRewrite), string(TaskKindOutlineSuggest), string(TaskKindWorldviewSuggest), string(TaskKindInspirationAnalyze), string(TaskKindComplianceAudit), string(TaskKindComplianceRewrite)}
}

func IsCreatorTaskKind(kind string) bool {
	for _, allowed := range CreatorTaskKinds() {
		if kind == allowed {
			return true
		}
	}
	return false
}
