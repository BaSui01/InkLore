package domain

import (
	pe "InkLore/internal/platform/errors"
)

func ErrRunCapacityExceeded(details map[string]any) error {
	return pe.Contract(429, "agent.run_capacity_exceeded", details)
}

func ErrSourceChanged(details map[string]any) error {
	return pe.Contract(409, "agent.source_changed", details)
}

func ErrContextBudgetExceeded(details map[string]any) error {
	return pe.Contract(422, "agent.context_budget_exceeded", details)
}

func ErrApprovalExpired(details map[string]any) error {
	return pe.Contract(409, "agent.approval_expired", details)
}

func ErrApprovalConsumed(details map[string]any) error {
	return pe.Contract(409, "agent.approval_consumed", details)
}

func ErrApprovalBindingMismatch(details map[string]any) error {
	return pe.Contract(409, "agent.approval_binding_mismatch", details)
}

func ErrApprovalSourceChanged(details map[string]any) error {
	return pe.Contract(409, "agent.approval_source_changed", details)
}

func ErrEventCursorExpired(details map[string]any) error {
	return pe.Contract(410, "agent.event_cursor_expired", details)
}

func ErrEventCursorInvalid(details map[string]any) error {
	return pe.Contract(400, "agent.event_cursor_invalid", details)
}

func ErrExecutionOutcomeUnknown(details map[string]any) error {
	return pe.Contract(409, "agent.execution_outcome_unknown", details)
}

func ErrLoopDetected(details map[string]any) error {
	return pe.Contract(422, "agent.loop_detected", details)
}
