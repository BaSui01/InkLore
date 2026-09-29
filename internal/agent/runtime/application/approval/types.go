package engineapproval

import (
	"context"
	"encoding/json"
	"time"

	enginedomain "InkLore/internal/agent/domain"
)

type ToolApprovalCreate struct {
	SessionID *int64
	AgentID   *int64
	RunID     *int64
	ToolName  string
	Args      json.RawMessage
	UserID    int64
}

type ToolApprovalGrant struct {
	ExpiresAt time.Time
	Token     string
	ToolName  string
	ArgsHash  string
	ID        int64
}

type ToolApprovalConsumption struct {
	SessionID *int64
	AgentID   *int64
	RunID     *int64
	ToolName  string
	ArgsHash  string
	Status    string
	ID        int64
}

type ToolApprovalService interface {
	CreateApproval(ctx context.Context, req ToolApprovalCreate) (*ToolApprovalGrant, error)
	ConsumeApproval(ctx context.Context, userID int64, toolName string, args json.RawMessage, token string) error
}

type ToolApprovalConsumptionService interface {
	ConsumeApprovalWithResult(ctx context.Context, userID int64, toolName string, args json.RawMessage, token string) (*ToolApprovalConsumption, error)
}

func ApprovalConsumptionData(consumption *ToolApprovalConsumption) map[string]interface{} {
	if consumption == nil {
		return nil
	}
	data := map[string]interface{}{
		"approval_id": consumption.ID,
		"tool":        consumption.ToolName,
		"args_hash":   consumption.ArgsHash,
		"status":      consumption.Status,
	}
	if consumption.AgentID != nil {
		data["agent_id"] = *consumption.AgentID
	}
	if consumption.SessionID != nil {
		data["session_id"] = *consumption.SessionID
	}
	if consumption.RunID != nil {
		data["run_id"] = *consumption.RunID
	}
	return data
}

func WithApprovedTool(ctx context.Context, toolName string) context.Context {
	return enginedomain.WithApprovedToolContext(ctx, toolName)
}

func WithApprovedToolContext(ctx context.Context, toolName string) context.Context {
	return enginedomain.WithApprovedToolContext(ctx, toolName)
}

func IsApprovedToolContext(ctx context.Context, toolName string) bool {
	return enginedomain.IsApprovedToolContext(ctx, toolName)
}
