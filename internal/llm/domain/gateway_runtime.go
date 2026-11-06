package domain

import "context"

// GatewayPlan contains catalog metadata only. Secret material is resolved by
// the provider adapter after the selected reservation is marked dispatched.
type GatewayPlan struct {
	TimeoutSeconds  int
	MaxOutputTokens int
	Temperature     float64
	Candidates      []Selection
	KeyVersions     map[string]string
}

type GatewayInvocation struct {
	ID       string
	Response *TextGenerationResponse
	Receipt  *Usage
}

type GatewayDispatch struct {
	ReservationID string
	Lease         AdmissionLease
}

// GatewayRuntimeRepository owns only LLM catalog and invocation data. The
// caller supplies an operation key from its persisted command/node/turn.
type GatewayRuntimeRepository interface {
	Plan(context.Context, TextGenerationRequest) (GatewayPlan, error)
	BindInvocation(context.Context, TextGenerationRequest) (GatewayInvocation, error)
	SaveResult(context.Context, string, string, TextGenerationResponse) error
}
