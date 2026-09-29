package gateway

import (
	"context"
	"errors"
	"net"
	"strings"
)

type ExecutionErrorType string

const (
	ErrorTypeProviderAuthFailed     ExecutionErrorType = "provider_auth_failed"
	ErrorTypeProviderRateLimited    ExecutionErrorType = "provider_rate_limited"
	ErrorTypeProviderInvalidRequest ExecutionErrorType = "provider_invalid_request"
	ErrorTypeSchemaValidationFailed ExecutionErrorType = "schema_validation_failed"
	ErrorTypeRetryable              ExecutionErrorType = "retryable"
	ErrorTypeUnknown                ExecutionErrorType = "unknown"
)

// httpStatusCarrier 是"携带上游 HTTP 状态码"的最小契约。
//
// 刻意不依赖具体包类型：llm/ports 与 llm/infrastructure 都能满足它，
// 避免错误分类逻辑反向依赖某个实现包。
type httpStatusCarrier interface {
	ProviderHTTPStatus() int
}

// ClassifyExecutionError 把上游错误归类为可调度的失败类型。
//
// 该分类是密钥级熔断的唯一输入：routing 只在 ProviderDown / Transport 上累加
// 熔断计数。历史上这里是恒返回 Unknown 的桩函数，导致"连续失败触发熔断"
// 永远不成立（见 INK-005 验证结论）。因此这里必须做真实分类。
func ClassifyExecutionError(err error) ExecutionErrorType {
	if err == nil {
		return ""
	}

	var carrier httpStatusCarrier
	if errors.As(err, &carrier) {
		switch status := carrier.ProviderHTTPStatus(); {
		case status == 401 || status == 403:
			return ErrorTypeProviderAuthFailed
		case status == 429:
			return ErrorTypeProviderRateLimited
		case status >= 500:
			return ErrorTypeRetryable
		case status >= 400:
			return ErrorTypeProviderInvalidRequest
		}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrorTypeRetryable
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return ErrorTypeRetryable
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "connection refused"),
		strings.Contains(message, "connection reset"),
		strings.Contains(message, "no such host"),
		strings.Contains(message, "timeout"),
		strings.Contains(message, "eof"):
		return ErrorTypeRetryable
	}
	return ErrorTypeUnknown
}

type Capability string

const (
	CapabilityText Capability = "text"
)

type TextRequest struct {
	Prompt       string
	SystemPrompt string
	Messages     []TextMessage
	Tools        []TextTool
	ToolChoice   interface{}
	Model        string
	MaxTokens    int
	Temperature  float64
	TopP         float64
	Options      map[string]interface{}
	Raw          interface{}
}

type AuditRef struct {
	RequestID  string
	ActorID    int64
	Resource   string
	ResourceID int64
	RunID      int64
	Metadata   map[string]string
}

type TextToolFunction struct {
	Name        string
	Description string
	Parameters  any
}

type TextTool struct {
	Type     string
	Function TextToolFunction
}

type TextToolCallFunction struct {
	Name      string
	Arguments string
}

type TextToolCall struct {
	ID               string
	Type             string
	Function         TextToolCallFunction
	ProviderMetadata map[string]string
}

type TextMessage struct {
	Role       string
	Content    string
	ToolCalls  []TextToolCall
	ToolCallID string
}

type ExecutionRequest struct {
	Capability Capability
	Action     string
	Model      string
	Provider   string
	Text       *TextRequest
	AuditRef   AuditRef
}

type ExecutionResult struct {
	Content string
}

type TextStreamRequest struct {
	Provider string
	Model    string
	Text     *TextRequest
	AuditRef AuditRef
	OnChunk  func(chunk string, done bool) error
}

type ModelExecutionGateway interface {
	Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)
	ExecuteTextStream(ctx context.Context, req TextStreamRequest) error
}

type defaultGateway struct {
	underlying ModelExecutionGateway
}

func (g *defaultGateway) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	if g.underlying != nil {
		return g.underlying.Execute(ctx, req)
	}
	return &ExecutionResult{}, nil
}

func (g *defaultGateway) ExecuteTextStream(ctx context.Context, req TextStreamRequest) error {
	if g.underlying != nil {
		return g.underlying.ExecuteTextStream(ctx, req)
	}
	return nil
}

func New(underlying ModelExecutionGateway) ModelExecutionGateway {
	if underlying != nil {
		return underlying
	}
	return &defaultGateway{}
}
