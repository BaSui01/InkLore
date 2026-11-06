package adapters

import (
	"context"
	"encoding/json"
	"errors"

	agentdomain "InkLore/internal/agent/domain"
	llmdomain "InkLore/internal/llm/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
)

func creatorPolicyView(policy llmdomain.CreatorPolicy) (agentdomain.CreatorTaskPolicy, error) {
	schema, err := json.Marshal(map[string]any{"type": "object", "properties": map[string]any{
		"task_kind": map[string]any{"type": "string", "title_key": "agent.taskKind", "enum": policy.TaskKinds},
	}, "required": []string{"task_kind"}, "additionalProperties": false})
	if err != nil {
		return agentdomain.CreatorTaskPolicy{}, err
	}
	return agentdomain.CreatorTaskPolicy{PolicyID: policy.PolicyID, PolicyVersionID: policy.PolicyVersionID, TaskKey: policy.TaskKey, Title: policy.Title, Version: policy.Version, TaskKinds: policy.TaskKinds, InputSchema: schema, Budget: agentdomain.CreatorPolicyBudget{MaxOutputTokens: policy.MaxOutputTokens, TimeoutSeconds: policy.TimeoutSeconds}}, nil
}

func (g *AgentTaskModelGateway) ListCreatorTaskPolicies(ctx context.Context, p identity.Principal, taskKind, afterID string, limit int) ([]agentdomain.CreatorTaskPolicy, error) {
	if g == nil || g.catalog == nil {
		return nil, agentdomain.ErrInvalidConfiguration(nil)
	}
	policies, err := g.catalog.ListCreatorPolicies(ctx, p, llmdomain.CreatorPolicyQuery{Limit: limit, AfterID: afterID, TaskKind: taskKind, AllowedTaskKinds: agentdomain.CreatorTaskKinds()})
	if err != nil {
		return nil, err
	}
	result := make([]agentdomain.CreatorTaskPolicy, 0, len(policies))
	for _, policy := range policies {
		view, err := creatorPolicyView(policy)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

func (g *AgentTaskModelGateway) LockCreatorTaskPolicy(ctx context.Context, p identity.Principal, versionID, taskKind string) (agentdomain.CreatorTaskPolicy, error) {
	if g == nil || g.catalog == nil {
		return agentdomain.CreatorTaskPolicy{}, agentdomain.ErrInvalidConfiguration(nil)
	}
	policy, err := g.catalog.LockCreatorPolicy(ctx, p, versionID, taskKind)
	if err != nil {
		var app *pe.AppError
		if errors.As(err, &app) && app.Code == llmdomain.ErrNotFound.Error() {
			return agentdomain.CreatorTaskPolicy{}, pe.Contract(409, "agent.task_policy_version_changed", nil)
		}
		return agentdomain.CreatorTaskPolicy{}, err
	}
	return creatorPolicyView(policy)
}

// The LLM-owned port also makes a missing receipt an error; no partial totals.
func (g *AgentTaskModelGateway) SumReceiptCostMicros(ctx context.Context, tenantID string, receiptIDs []string) (int64, error) {
	if len(receiptIDs) == 0 {
		return 0, nil
	}
	if g == nil || g.catalog == nil {
		return 0, agentdomain.ErrInvalidConfiguration(nil)
	}
	reader, ok := any(g.catalog).(interface {
		SumReceiptCostMicros(context.Context, string, []string) (int64, error)
	})
	if !ok {
		return 0, agentdomain.ErrInvalidConfiguration(map[string]any{"reason": "usage receipt reader is unavailable"})
	}
	return reader.SumReceiptCostMicros(ctx, tenantID, receiptIDs)
}
