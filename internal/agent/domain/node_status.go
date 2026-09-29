package domain

import (
	"errors"
	"fmt"
)

// AgentRunNodeStatus is the persisted lifecycle status of one Agent graph node.
type AgentRunNodeStatus string

const (
	AgentRunNodeStatusRunning         AgentRunNodeStatus = "running"
	AgentRunNodeStatusWaitingApproval AgentRunNodeStatus = "waiting_approval"
	AgentRunNodeStatusSuccess         AgentRunNodeStatus = "success"
	AgentRunNodeStatusFailed          AgentRunNodeStatus = "failed"
)

// AgentRunNodeAction narrows which caller intent may perform a status transition.
type AgentRunNodeAction string

const (
	AgentRunNodeActionWaitForApproval   AgentRunNodeAction = "wait_for_approval"
	AgentRunNodeActionResumeApproval    AgentRunNodeAction = "resume_approval"
	AgentRunNodeActionComplete          AgentRunNodeAction = "complete"
	AgentRunNodeActionFail              AgentRunNodeAction = "fail"
	AgentRunNodeActionTerminateApproval AgentRunNodeAction = "terminate_approval"
)

var (
	ErrAgentRunNodeInvalidArgument = errors.New("agent run node invalid argument")
	ErrAgentRunNodeStatusInvalid   = errors.New("agent run node status transition invalid")
	ErrAgentRunNodeNotFound        = errors.New("agent run node not found")
	ErrAgentRunNodeStatusConflict  = errors.New("agent run node status conflict")
)

func (s AgentRunNodeStatus) IsKnown() bool {
	switch s {
	case AgentRunNodeStatusRunning, AgentRunNodeStatusWaitingApproval, AgentRunNodeStatusSuccess, AgentRunNodeStatusFailed:
		return true
	default:
		return false
	}
}

func (a AgentRunNodeAction) IsKnown() bool {
	switch a {
	case AgentRunNodeActionWaitForApproval, AgentRunNodeActionResumeApproval, AgentRunNodeActionComplete, AgentRunNodeActionFail, AgentRunNodeActionTerminateApproval:
		return true
	default:
		return false
	}
}

// ValidateAgentRunNodeTransition enforces the complete action-aware node matrix.
// Terminal states intentionally have no outgoing transitions and same-state
// transitions are conflicts at persistence time rather than successful idempotency.
func ValidateAgentRunNodeTransition(from, to AgentRunNodeStatus, action AgentRunNodeAction) error {
	if !from.IsKnown() || !to.IsKnown() || !action.IsKnown() {
		return fmt.Errorf("%w: from=%q to=%q action=%q", ErrAgentRunNodeStatusInvalid, from, to, action)
	}

	valid := false
	switch action {
	case AgentRunNodeActionWaitForApproval:
		valid = from == AgentRunNodeStatusRunning && to == AgentRunNodeStatusWaitingApproval
	case AgentRunNodeActionResumeApproval:
		valid = from == AgentRunNodeStatusWaitingApproval && to == AgentRunNodeStatusRunning
	case AgentRunNodeActionComplete:
		valid = from == AgentRunNodeStatusRunning && to == AgentRunNodeStatusSuccess
	case AgentRunNodeActionFail:
		valid = from == AgentRunNodeStatusRunning && to == AgentRunNodeStatusFailed
	case AgentRunNodeActionTerminateApproval:
		valid = from == AgentRunNodeStatusWaitingApproval && (to == AgentRunNodeStatusSuccess || to == AgentRunNodeStatusFailed)
	}
	if !valid {
		return fmt.Errorf("%w: from=%q to=%q action=%q", ErrAgentRunNodeStatusInvalid, from, to, action)
	}
	return nil
}
