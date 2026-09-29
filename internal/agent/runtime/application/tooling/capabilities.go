package enginetooling

import (
	authdomain "InkLore/internal/platform/auth"
	"errors"
	"os"
	"strings"
	"time"
)

var (
	ErrToolCapabilityNotRegistered = errors.New("tool capability not registered")
	ErrToolPermissionDenied        = errors.New("tool permission denied")
)

const (
	CapabilityRiskLow         = "low"
	CapabilityRiskMedium      = "medium"
	CapabilityRiskHigh        = "high"
	CapabilityRiskDestructive = "destructive"

	ApprovalNever   = "never"
	ApprovalAskRisk = "ask_risk"
	ApprovalAlways  = "always"

	AuditNone    = "none"
	AuditSummary = "summary"
	AuditFull    = "full"

	ToolSideEffectReadonly       = "readonly"
	ToolSideEffectCreatesTask    = "creates_task"
	ToolSideEffectWritesArtifact = "writes_artifact"
	ToolSideEffectMutatesState   = "mutates_state"
	ToolSideEffectExternal       = "external_side_effect"
)

type ToolCapability struct {
	SideEffectLevel     string        `json:"side_effect_level,omitempty"`
	Type                string        `json:"type"`
	IdempotencyKey      string        `json:"idempotency_key,omitempty"`
	Name                string        `json:"name"`
	RiskLevel           string        `json:"risk_level"`
	ApprovalPolicy      string        `json:"approval_policy"`
	AuditLevel          string        `json:"audit_level"`
	RequiredPermissions []string      `json:"required_permissions,omitempty"`
	RequiredActions     []string      `json:"required_actions,omitempty"`
	ResourceScopes      []string      `json:"resource_scopes,omitempty"`
	Timeout             time.Duration `json:"timeout,omitempty"`
	ExemptFromApproval  bool          `json:"exempt_from_approval,omitempty"`
	Parallelizable      bool          `json:"parallelizable,omitempty"`
	SuspendsRun         bool          `json:"suspends_run,omitempty"`
}

const defaultToolTimeout = 60 * time.Second

// ToolTimeout 返回工具的执行超时（未注册或未配置时为默认 60s）。
func (r *ToolCapabilityRegistry) ToolTimeout(toolName string) time.Duration {
	if r != nil {
		if capability, ok := r.Get(toolName); ok && capability.Timeout > 0 {
			return capability.Timeout
		}
	}
	return defaultToolTimeout
}

type ToolPermissionChecker interface {
	CanUseTool(toolName string, requiredPermissions []string) bool
}

type ToolCapabilityRegistry struct {
	items map[string]ToolCapability
}

func DefaultToolCapabilities() *ToolCapabilityRegistry {
	return NewToolCapabilityRegistry([]ToolCapability{
		{
			Name:                "ask_user",
			Type:                "tool",
			RequiredActions:     []string{"tool.execute"},
			RequiredPermissions: []string{"ai_content_platform.workspace:access"},
			RiskLevel:           CapabilityRiskLow,
			ApprovalPolicy:      ApprovalNever,
			AuditLevel:          AuditSummary,
			SuspendsRun:         true,
			ExemptFromApproval:  true,
			SideEffectLevel:     ToolSideEffectMutatesState,
		},
		{
			Parallelizable:      true,
			Name:                "web_search",
			Type:                "tool",
			RequiredActions:     []string{"tool.execute"},
			RequiredPermissions: []string{"ai_content_platform.workspace:access"},
			RiskLevel:           CapabilityRiskLow,
			ApprovalPolicy:      ApprovalNever,
			AuditLevel:          AuditSummary,
			SideEffectLevel:     ToolSideEffectReadonly,
		},
		{
			Parallelizable:      true,
			Name:                "web_fetch",
			Type:                "tool",
			RequiredActions:     []string{"tool.execute"},
			RequiredPermissions: []string{"ai_content_platform.workspace:access"},
			RiskLevel:           CapabilityRiskLow,
			ApprovalPolicy:      ApprovalNever,
			AuditLevel:          AuditSummary,
			SideEffectLevel:     ToolSideEffectReadonly,
		},
		{
			Parallelizable:      true,
			Name:                "analyze_images",
			Type:                "tool",
			RequiredActions:     []string{"media_asset.read"},
			RequiredPermissions: []string{authdomain.PermMediaAssetRead},
			RiskLevel:           CapabilityRiskMedium,
			ApprovalPolicy:      ApprovalNever,
			AuditLevel:          AuditSummary,
			SideEffectLevel:     ToolSideEffectReadonly,
		},
		{
			Name:                "generate_image",
			Type:                "tool",
			RequiredActions:     []string{"generation.execute", "generation_task.create"},
			RequiredPermissions: []string{authdomain.PermGenerationExecute, authdomain.PermGenerationTaskCreate},
			RiskLevel:           CapabilityRiskHigh,
			ApprovalPolicy:      ApprovalAskRisk,
			AuditLevel:          AuditFull,
			SideEffectLevel:     ToolSideEffectCreatesTask,
			ResourceScopes:      []string{"generation", "media_asset"},
		},
		{
			Name:                "generate_video",
			Type:                "tool",
			RequiredActions:     []string{"generation.execute", "generation_task.create"},
			RequiredPermissions: []string{authdomain.PermGenerationExecute, authdomain.PermGenerationTaskCreate},
			RiskLevel:           CapabilityRiskHigh,
			ApprovalPolicy:      ApprovalAskRisk,
			AuditLevel:          AuditFull,
			SideEffectLevel:     ToolSideEffectCreatesTask,
			ResourceScopes:      []string{"generation", "media_asset"},
		},
		{
			Parallelizable:      true,
			Name:                "get_media_task_status",
			Type:                "tool",
			RequiredActions:     []string{"generation_task.read"},
			RequiredPermissions: []string{authdomain.PermGenerationTaskRead},
			RiskLevel:           CapabilityRiskLow,
			ApprovalPolicy:      ApprovalNever,
			AuditLevel:          AuditSummary,
			SideEffectLevel:     ToolSideEffectReadonly,
			ResourceScopes:      []string{"generation", "media_asset"},
		},
		{
			Name:                "generate_document",
			Type:                "tool",
			RequiredActions:     []string{"generation.execute", "generation_task.create"},
			RequiredPermissions: []string{authdomain.PermGenerationExecute, authdomain.PermGenerationTaskCreate},
			RiskLevel:           CapabilityRiskHigh,
			ApprovalPolicy:      ApprovalAskRisk,
			AuditLevel:          AuditFull,
			SideEffectLevel:     ToolSideEffectWritesArtifact,
			ResourceScopes:      []string{"generation"},
		},
		{
			Name:                "create_skill",
			Type:                "tool",
			RequiredActions:     []string{"skill.write", "tool.execute"},
			RequiredPermissions: []string{authdomain.PermSkillCreate},
			RiskLevel:           CapabilityRiskHigh,
			ApprovalPolicy:      ApprovalAlways,
			AuditLevel:          AuditFull,
			SideEffectLevel:     ToolSideEffectMutatesState,
			ResourceScopes:      []string{"skill"},
		},
		{
			Name:                "create_agent",
			Type:                "tool",
			RequiredActions:     []string{"write", "tool.approve"},
			RequiredPermissions: []string{authdomain.PermAgentCreate},
			RiskLevel:           CapabilityRiskHigh,
			ApprovalPolicy:      ApprovalAlways,
			AuditLevel:          AuditFull,
			SideEffectLevel:     ToolSideEffectMutatesState,
			ResourceScopes:      []string{"agent"},
		},
		{
			Name:                "code_interpreter",
			Timeout:             180 * time.Second,
			Type:                "tool",
			RequiredActions:     []string{"tool.execute", "chat_session.update"},
			RequiredPermissions: []string{authdomain.PermWorkspaceAccess, authdomain.PermChatSessionUpdate},
			RiskLevel:           CapabilityRiskHigh,
			ApprovalPolicy:      ApprovalAskRisk,
			AuditLevel:          AuditFull,
			SideEffectLevel:     ToolSideEffectWritesArtifact,
			ResourceScopes:      []string{"sandbox", "chat_session"},
		},
	})
}

func NewToolCapabilityRegistry(items []ToolCapability) *ToolCapabilityRegistry {
	r := &ToolCapabilityRegistry{items: map[string]ToolCapability{}}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		item.Name = name
		if item.Type == "" {
			item.Type = "tool"
		}
		if item.RiskLevel == "" {
			item.RiskLevel = CapabilityRiskLow
		}
		if item.ApprovalPolicy == "" {
			item.ApprovalPolicy = ApprovalNever
		}
		if item.AuditLevel == "" {
			item.AuditLevel = AuditSummary
		}
		item.SideEffectLevel = NormalizeToolSideEffect(item.SideEffectLevel)
		item.IdempotencyKey = strings.TrimSpace(item.IdempotencyKey)
		item.ResourceScopes = normalizeCapabilityResourceScopes(item.ResourceScopes)
		r.items[name] = item
	}
	return r
}

func (r *ToolCapabilityRegistry) Register(c ToolCapability) {
	if r == nil {
		return
	}
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return
	}
	c.Name = name
	if c.Type == "" {
		c.Type = "tool"
	}
	if c.RiskLevel == "" {
		c.RiskLevel = CapabilityRiskLow
	}
	if c.ApprovalPolicy == "" {
		c.ApprovalPolicy = ApprovalNever
	}
	if c.AuditLevel == "" {
		c.AuditLevel = AuditSummary
	}
	c.SideEffectLevel = NormalizeToolSideEffect(c.SideEffectLevel)
	if r.items == nil {
		r.items = map[string]ToolCapability{}
	}
	r.items[name] = c
}

func NormalizeToolSideEffect(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case ToolSideEffectReadonly:
		return ToolSideEffectReadonly
	case ToolSideEffectCreatesTask:
		return ToolSideEffectCreatesTask
	case ToolSideEffectWritesArtifact:
		return ToolSideEffectWritesArtifact
	case ToolSideEffectExternal:
		return ToolSideEffectExternal
	case ToolSideEffectMutatesState, "":
		return ToolSideEffectMutatesState
	default:
		return ToolSideEffectMutatesState
	}
}

func normalizeCapabilityResourceScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	seen := map[string]bool{}
	for _, scope := range scopes {
		normalized := strings.ToLower(strings.TrimSpace(scope))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func (r *ToolCapabilityRegistry) Get(name string) (ToolCapability, bool) {
	if r == nil {
		return ToolCapability{}, false
	}
	item, ok := r.items[strings.TrimSpace(name)]
	return item, ok
}

func (r *ToolCapabilityRegistry) List() []ToolCapability {
	if r == nil {
		return nil
	}
	out := make([]ToolCapability, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out
}

func (r *ToolCapabilityRegistry) ToolNeedsApproval(permissionLevel string, toolName string) bool {
	capability, ok := r.Get(toolName)
	if !ok {
		return true
	}
	switch capability.ApprovalPolicy {
	case ApprovalNever:
		if permissionLevel == permissionAlwaysAsk {
			return true
		}
		return false
	case ApprovalAlways:
		return permissionLevel != permissionFullAccess
	case ApprovalAskRisk:
		if permissionLevel == permissionFullAccess {
			return false
		}
		return capability.RiskLevel == CapabilityRiskHigh || capability.RiskLevel == CapabilityRiskDestructive
	default:
		return true
	}
}

func (r *ToolCapabilityRegistry) CheckToolPermission(toolName string, checker ToolPermissionChecker) error {
	capability, ok := r.Get(toolName)
	if !ok {
		return ErrToolCapabilityNotRegistered
	}
	if StrictAgentToolsEnabled() && isHighRiskCapability(capability) && len(capability.RequiredPermissions) == 0 {
		return ErrToolPermissionDenied
	}
	if len(capability.RequiredPermissions) == 0 {
		return nil
	}
	if checker == nil {
		if StrictAgentToolsEnabled() {
			return ErrToolPermissionDenied
		}
		return nil
	}
	if !checker.CanUseTool(toolName, capability.RequiredPermissions) {
		return ErrToolPermissionDenied
	}
	return nil
}

func StrictAgentToolsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("STRICT_AGENT_TOOLS"))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isHighRiskCapability(capability ToolCapability) bool {
	switch capability.RiskLevel {
	case CapabilityRiskHigh, CapabilityRiskDestructive:
		return true
	default:
		return false
	}
}
