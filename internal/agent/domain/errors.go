package domain

import (
	pe "InkLore/internal/platform/errors"
)

// Domain error constructors returning frozen contract errors.
func ErrInvalidConfiguration(details map[string]any) error {
	return pe.Contract(422, "agent.invalid_configuration", details)
}

func ErrInvalidDAG(details map[string]any) error {
	return pe.Contract(422, "agent.invalid_dag", details)
}

func ErrVersionConflict(details map[string]any) error {
	return pe.Contract(409, "agent.version_conflict", details)
}

func ErrResourceInUse(details map[string]any) error {
	return pe.Contract(409, "agent.resource_in_use", details)
}

func ErrNotFound() error {
	return pe.Contract(404, "platform.resource_not_found", nil)
}

func ErrValidationFailed(details map[string]any) error {
	return pe.Contract(400, "platform.validation_failed", details)
}

func ErrCursorInvalid() error {
	return pe.Contract(400, "platform.cursor_invalid", nil)
}

func ErrUnauthenticated() error {
	return pe.Contract(401, "auth.unauthenticated", nil)
}

func ErrForbidden() error {
	return pe.Contract(403, "auth.forbidden", nil)
}
