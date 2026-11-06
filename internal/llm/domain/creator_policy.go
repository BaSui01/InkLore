package domain

import (
	"context"
	"regexp"
)

var policyTaskKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func ValidPolicyTaskKinds(kinds []string) bool {
	if len(kinds) < 1 || len(kinds) > 16 {
		return false
	}
	seen := map[string]bool{}
	for _, kind := range kinds {
		if seen[kind] || !policyTaskKindPattern.MatchString(kind) {
			return false
		}
		seen[kind] = true
	}
	return true
}

// CreatorPolicy is an explicit public projection of an enabled policy head.
// Routing, provider credentials, prompts and internal model identifiers stay private.
type CreatorPolicy struct {
	PolicyID        string   `json:"policy_id"`
	PolicyVersionID string   `json:"policy_version_id"`
	TaskKey         string   `json:"task_key"`
	Title           string   `json:"title"`
	Version         int64    `json:"version"`
	TaskKinds       []string `json:"task_kinds"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	TimeoutSeconds  int      `json:"timeout_seconds"`
}

type CreatorPolicyQuery struct {
	Limit            int
	AfterID          string
	TaskKind         string
	AllowedTaskKinds []string
}

type CreatorPolicyRepository interface {
	ListCreatorPolicies(context.Context, string, CreatorPolicyQuery) ([]CreatorPolicy, error)
	LockCreatorPolicy(context.Context, string, string, string) (CreatorPolicy, error)
}
