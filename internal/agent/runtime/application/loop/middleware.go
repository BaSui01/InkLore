package engineloop

import (
	"context"
	"encoding/json"

	ai "InkLore/internal/llm/domain"
)

// Middleware is a marker type for optional Agent runtime extensions.
// A value may implement one or more of the specific middleware interfaces below.
type Middleware interface{}

type ModelCallContext struct {
	Request       *RunRequest
	GenerationReq *ai.TextGenerationRequest
	Provider      string
	Messages      []ai.ChatMessage
	Tools         []ai.Tool
	Turn          int
}

type ModelCallResult struct {
	Error    error
	Request  *RunRequest
	Response *ai.TextGenerationResponse
	Usage    *ai.TokenUsage
	Turn     int
}

type ToolCallContext struct {
	Tool      ToolEntry
	Request   *RunRequest
	ToolCall  ai.ToolCall
	Arguments json.RawMessage
	Turn      int
	Approved  bool
}

type ToolCallResult struct {
	Output     *ToolOutput     `json:"output,omitempty"`
	Tool       string          `json:"tool"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Error      string          `json:"error,omitempty"`
	Raw        string          `json:"raw,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	Turn       int             `json:"turn"`
}

type ModelCallMiddleware interface {
	BeforeModelCall(ctx context.Context, req *ModelCallContext) error
	AfterModelCall(ctx context.Context, result *ModelCallResult) error
}

type ToolCallMiddleware interface {
	BeforeToolCall(ctx context.Context, req *ToolCallContext) error
	AfterToolCall(ctx context.Context, result *ToolCallResult) error
}

type StreamEventMiddleware interface {
	OnEvent(ctx context.Context, event *AgentEvent) (*AgentEvent, error)
}

func modelMiddlewares(middlewares []Middleware) []ModelCallMiddleware {
	result := make([]ModelCallMiddleware, 0, len(middlewares))
	for _, middleware := range middlewares {
		if typed, ok := middleware.(ModelCallMiddleware); ok && typed != nil {
			result = append(result, typed)
		}
	}
	return result
}

func toolMiddlewares(middlewares []Middleware) []ToolCallMiddleware {
	result := make([]ToolCallMiddleware, 0, len(middlewares))
	for _, middleware := range middlewares {
		if typed, ok := middleware.(ToolCallMiddleware); ok && typed != nil {
			result = append(result, typed)
		}
	}
	return result
}

func eventMiddlewares(middlewares []Middleware) []StreamEventMiddleware {
	result := make([]StreamEventMiddleware, 0, len(middlewares))
	for _, middleware := range middlewares {
		if typed, ok := middleware.(StreamEventMiddleware); ok && typed != nil {
			result = append(result, typed)
		}
	}
	return result
}
