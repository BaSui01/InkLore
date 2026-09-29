package domain

import (
	"context"
	"strings"

	authdomain "InkLore/internal/platform/auth"
)

type runtimeContextKey struct{}

// RuntimeContext Agent 运行期执行与隔离上下文。
// 携带会话、项目、租户、权限和审批等关键隔离边界信息。
type RuntimeContext struct {
	SessionID       *int64               `json:"session_id,omitempty"`
	AgentID         *int64               `json:"agent_id,omitempty"`
	RunID           *int64               `json:"run_id,omitempty"`
	ProjectID       *int64               `json:"project_id,omitempty"`
	WorkspaceID     *int64               `json:"workspace_id,omitempty"`
	UserID          int64                `json:"user_id,omitempty"`
	RequestID       string               `json:"request_id,omitempty"`
	PermissionLevel string               `json:"permission_level,omitempty"`
	Principal       authdomain.Principal `json:"principal,omitempty"`
	AllowedTools    []string             `json:"allowed_tools,omitempty"`
	ApprovalTokens  []string             `json:"approval_tokens,omitempty"`
	Metadata        map[string]any       `json:"metadata,omitempty"`
}

func (r *RuntimeContext) GetSessionID() int64 {
	if r != nil && r.SessionID != nil {
		return *r.SessionID
	}
	return 0
}

func (r *RuntimeContext) GetProjectID() int64 {
	if r != nil && r.ProjectID != nil {
		return *r.ProjectID
	}
	return 0
}

func (r *RuntimeContext) GetUserID() int64 {
	if r != nil {
		if r.UserID > 0 {
			return r.UserID
		}
		if r.Principal.UserID > 0 {
			return r.Principal.UserID
		}
	}
	return 0
}

// WithRuntimeContext 将 RuntimeContext 存入 Go context
func WithRuntimeContext(ctx context.Context, rc RuntimeContext) context.Context {
	return context.WithValue(ctx, runtimeContextKey{}, rc)
}

// RuntimeContextFromContext 从 Go context 中提取 RuntimeContext
func RuntimeContextFromContext(ctx context.Context) (RuntimeContext, bool) {
	rc, ok := ctx.Value(runtimeContextKey{}).(RuntimeContext)
	return rc, ok
}

type approvedToolContextKey struct{}

func WithApprovedToolContext(ctx context.Context, toolName string) context.Context {
	toolName = strings.TrimSpace(toolName)
	if ctx == nil {
		ctx = context.Background()
	}
	if toolName == "" {
		return ctx
	}
	approved, _ := ctx.Value(approvedToolContextKey{}).(map[string]bool)
	next := map[string]bool{}
	for name, ok := range approved {
		next[name] = ok
	}
	next[toolName] = true
	return context.WithValue(ctx, approvedToolContextKey{}, next)
}

func IsApprovedToolContext(ctx context.Context, toolName string) bool {
	if ctx == nil {
		return false
	}
	approved, _ := ctx.Value(approvedToolContextKey{}).(map[string]bool)
	return approved[strings.TrimSpace(toolName)]
}
