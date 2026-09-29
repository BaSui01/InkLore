package enginetooling

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	agentdomain "InkLore/internal/agent/domain"
	ai "InkLore/internal/llm/domain"
)

// UnifiedToolRegistry 统一工具注册与调度中心
type UnifiedToolRegistry struct {
	mu           sync.RWMutex
	tools        map[string]agentdomain.Tool
	capabilities *ToolCapabilityRegistry
}

// NewUnifiedToolRegistry 创建统一工具注册表
func NewUnifiedToolRegistry(caps *ToolCapabilityRegistry) *UnifiedToolRegistry {
	if caps == nil {
		caps = DefaultToolCapabilities()
	}
	return &UnifiedToolRegistry{
		tools:        make(map[string]agentdomain.Tool),
		capabilities: caps,
	}
}

// Register 注册领域 Tool
func (r *UnifiedToolRegistry) Register(tool agentdomain.Tool) {
	if tool == nil || tool.Name() == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
	// 同步到能力注册表
	capability := tool.Capability()
	if capability.Name == "" {
		capability.Name = tool.Name()
	}
	r.capabilities.Register(ToolCapability{
		Name:                capability.Name,
		Type:                capability.Type,
		RiskLevel:           capability.RiskLevel,
		ApprovalPolicy:      capability.ApprovalPolicy,
		AuditLevel:          capability.AuditLevel,
		SideEffectLevel:     capability.SideEffectLevel,
		RequiredPermissions: capability.RequiredPermissions,
		RequiredActions:     capability.RequiredActions,
		Timeout:             capability.Timeout,
		Parallelizable:      capability.Parallelizable,
		SuspendsRun:         capability.SuspendsRun,
		ExemptFromApproval:  capability.ExemptFromApproval,
	})
}

// RegisterEntry 兼容注册旧版 ToolEntry
func (r *UnifiedToolRegistry) RegisterEntry(entry ToolEntry) {
	if entry.Definition.Function.Name == "" {
		return
	}
	r.Register(&toolEntryAdapter{
		entry:        entry,
		capabilities: r.capabilities,
	})
}

// Get 获取工具
func (r *UnifiedToolRegistry) Get(name string) (agentdomain.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// ListNames 获取所有已注册工具名称
func (r *UnifiedToolRegistry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// FilterAllowedTools 根据 Allowed 列表和过滤条件获取可用工具
func (r *UnifiedToolRegistry) FilterAllowedTools(allowed []string) []agentdomain.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hasFilter := len(allowed) > 0
	allowMap := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		if strings.TrimSpace(a) != "" {
			allowMap[strings.TrimSpace(a)] = struct{}{}
		}
	}

	res := make([]agentdomain.Tool, 0, len(r.tools))
	for name, tool := range r.tools {
		if hasFilter {
			if _, ok := allowMap[name]; !ok {
				continue
			}
		}
		res = append(res, tool)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Name() < res[j].Name()
	})
	return res
}

// BuildToolDefinitions 构建传递给模型的 Tool JSON Schema 列表
func (r *UnifiedToolRegistry) BuildToolDefinitions(allowed []string) []ai.Tool {
	tools := r.FilterAllowedTools(allowed)
	out := make([]ai.Tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, ai.Tool{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return out
}

// CollectToolGuidelines 自动汇总所有可用工具的 Prompt 行为指引
func (r *UnifiedToolRegistry) CollectToolGuidelines(allowed []string) []string {
	tools := r.FilterAllowedTools(allowed)
	guidelines := make([]string, 0, len(tools))
	for _, t := range tools {
		g := strings.TrimSpace(t.PromptGuidelines())
		if g != "" {
			guidelines = append(guidelines, fmt.Sprintf("### 工具 [%s] 指引:\n%s", t.Name(), g))
		}
	}
	return guidelines
}

// toolEntryAdapter 将 ToolEntry 适配为 agentdomain.Tool
type toolEntryAdapter struct {
	entry        ToolEntry
	capabilities *ToolCapabilityRegistry
}

func (a *toolEntryAdapter) Name() string {
	return a.entry.Definition.Function.Name
}

func (a *toolEntryAdapter) Description() string {
	return a.entry.Definition.Function.Description
}

func (a *toolEntryAdapter) Parameters() any {
	return a.entry.Definition.Function.Parameters
}

func (a *toolEntryAdapter) PromptGuidelines() string {
	return ""
}

func (a *toolEntryAdapter) Capability() agentdomain.ToolCapability {
	if a.capabilities != nil {
		if capability, ok := a.capabilities.Get(a.Name()); ok {
			return agentdomain.ToolCapability{
				Name:                capability.Name,
				Type:                capability.Type,
				RiskLevel:           capability.RiskLevel,
				ApprovalPolicy:      capability.ApprovalPolicy,
				AuditLevel:          capability.AuditLevel,
				SideEffectLevel:     capability.SideEffectLevel,
				RequiredPermissions: capability.RequiredPermissions,
				RequiredActions:     capability.RequiredActions,
				Timeout:             capability.Timeout,
				Parallelizable:      capability.Parallelizable,
				SuspendsRun:         capability.SuspendsRun,
				ExemptFromApproval:  capability.ExemptFromApproval,
			}
		}
	}
	return agentdomain.ToolCapability{
		Name:           a.Name(),
		Type:           "builtin",
		RiskLevel:      CapabilityRiskLow,
		ApprovalPolicy: ApprovalNever,
		Timeout:        60 * time.Second,
	}
}

func (a *toolEntryAdapter) Execute(ctx context.Context, rc *agentdomain.RuntimeContext, rawArgs json.RawMessage) (*agentdomain.ToolResult, error) {
	if a.entry.Handler == nil {
		return nil, fmt.Errorf("handler is nil for tool %s", a.Name())
	}
	rawStr, err := a.entry.Handler(ctx, rawArgs)
	if err != nil {
		return &agentdomain.ToolResult{
			ToolName: a.Name(),
			Content:  err.Error(),
			IsError:  true,
		}, err
	}
	return &agentdomain.ToolResult{
		ToolName: a.Name(),
		Content:  rawStr,
		Raw:      rawStr,
	}, nil
}
