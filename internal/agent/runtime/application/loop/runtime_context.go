package engineloop

import "context"

type runtimeContextKey struct{}

type RuntimeContext struct {
	SessionID *int64 `json:"session_id,omitempty"`
	AgentID   *int64 `json:"agent_id,omitempty"`
	RunID     *int64 `json:"run_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	UserID    int64  `json:"user_id,omitempty"`
}

func NewRuntimeContext(req *RunRequest) RuntimeContext {
	if req == nil {
		return RuntimeContext{}
	}
	return RuntimeContext{
		RequestID: req.RequestID,
		UserID:    req.UserID,
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     req.RunID,
	}
}

func WithRuntimeContext(ctx context.Context, runtime RuntimeContext) context.Context {
	return context.WithValue(ctx, runtimeContextKey{}, runtime)
}

func RuntimeContextFromContext(ctx context.Context) (RuntimeContext, bool) {
	runtime, ok := ctx.Value(runtimeContextKey{}).(RuntimeContext)
	return runtime, ok
}
