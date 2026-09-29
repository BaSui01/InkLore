package engineloop

import (
	"context"
	"encoding/json"
	"strings"
)

// ApprovalGate 是工具审批的唯一决策与消费入口，TAO 循环与图运行时共用。
// 审批状态只存数据库（ToolApprovalService）；不存在消息令牌等退化路径。
type ApprovalGate struct {
	capabilities *ToolCapabilityRegistry
	approvals    ToolApprovalService
}

func NewApprovalGate(capabilities *ToolCapabilityRegistry, approvals ToolApprovalService) *ApprovalGate {
	if capabilities == nil {
		capabilities = DefaultToolCapabilities()
	}
	return &ApprovalGate{capabilities: capabilities, approvals: approvals}
}

type ApprovalInput struct {
	SessionID       *int64
	AgentID         *int64
	RunID           *int64
	PermissionLevel string
	ToolName        string
	Args            json.RawMessage
	Tokens          []string
	UserID          int64
}

type ApprovalDecision struct {
	Consumption *ToolApprovalConsumption
	Grant       *ToolApprovalGrant
	Required    bool
	Approved    bool
}

// Evaluate 判定 + 消费 + （需要时）创建待审批单，一次完成。
func (g *ApprovalGate) Evaluate(ctx context.Context, in ApprovalInput) ApprovalDecision {
	decision := ApprovalDecision{}
	if capability, ok := g.capabilities.Get(strings.TrimSpace(in.ToolName)); ok && capability.ExemptFromApproval {
		// 交互类工具（如 ask_user）自身就是向用户征询的手段，永不审批
		decision.Approved = true
		return decision
	}
	level := NormalizePermissionLevel(in.PermissionLevel, true)
	if !g.capabilities.ToolNeedsApproval(level, in.ToolName) {
		decision.Approved = true
		return decision
	}
	decision.Required = true
	if g.approvals == nil {
		// 无审批服务时一律不放行
		return decision
	}
	for _, token := range in.Tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if withResult, ok := g.approvals.(ToolApprovalConsumptionService); ok {
			if consumption, err := withResult.ConsumeApprovalWithResult(ctx, in.UserID, in.ToolName, in.Args, token); err == nil {
				decision.Approved = true
				decision.Consumption = consumption
				return decision
			}
			continue
		}
		if err := g.approvals.ConsumeApproval(ctx, in.UserID, in.ToolName, in.Args, token); err == nil {
			decision.Approved = true
			return decision
		}
	}
	if grant, err := g.approvals.CreateApproval(ctx, ToolApprovalCreate{
		UserID:    in.UserID,
		SessionID: in.SessionID,
		AgentID:   in.AgentID,
		RunID:     in.RunID,
		ToolName:  in.ToolName,
		Args:      in.Args,
	}); err == nil {
		decision.Grant = grant
	}
	return decision
}
