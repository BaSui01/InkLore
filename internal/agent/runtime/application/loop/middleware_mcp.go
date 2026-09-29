package engineloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const MCPQuotaPoliciesOptionKey = "mcp_quota_policies"

type MCPQuotaPolicy struct {
	Server      string `json:"server,omitempty"`
	Tool        string `json:"tool,omitempty"`
	WindowLabel string `json:"window_label,omitempty"`
	PerMinute   int    `json:"per_minute,omitempty"`
	PerRun      int    `json:"per_run,omitempty"`
}

type MCPQuotaMiddleware struct {
	perRun    map[string]int
	perMinute map[string][]time.Time
	now       func() time.Time
	mu        sync.Mutex
}

func NewMCPQuotaMiddleware() *MCPQuotaMiddleware {
	return &MCPQuotaMiddleware{
		perRun:    map[string]int{},
		perMinute: map[string][]time.Time{},
		now:       time.Now,
	}
}

func (m *MCPQuotaMiddleware) BeforeToolCall(ctx context.Context, req *ToolCallContext) error {
	if req == nil || req.Request == nil {
		return nil
	}
	policy, ok := policyForTool(req.Request.Options, req.ToolCall.Function.Name)
	if !ok {
		return nil
	}
	key := strings.TrimSpace(policy.Server) + ":" + strings.TrimSpace(req.ToolCall.Function.Name)
	m.mu.Lock()
	defer m.mu.Unlock()
	moment := m.now()
	if policy.PerRun > 0 {
		m.perRun[key]++
		if m.perRun[key] > policy.PerRun {
			return quotaError(req.ToolCall.Function.Name, "per_run", policy.PerRun)
		}
	}
	if policy.PerMinute > 0 {
		windowStart := moment.Add(-time.Minute)
		items := m.perMinute[key]
		kept := items[:0]
		for _, item := range items {
			if item.After(windowStart) {
				kept = append(kept, item)
			}
		}
		kept = append(kept, moment)
		m.perMinute[key] = kept
		if len(kept) > policy.PerMinute {
			return quotaError(req.ToolCall.Function.Name, "per_minute", policy.PerMinute)
		}
	}
	return nil
}

func (m *MCPQuotaMiddleware) AfterToolCall(ctx context.Context, result *ToolCallResult) error {
	return nil
}

func policyForTool(options map[string]interface{}, tool string) (MCPQuotaPolicy, bool) {
	if options == nil {
		return MCPQuotaPolicy{}, false
	}
	raw := options[MCPQuotaPoliciesOptionKey]
	policies := normalizeMCPQuotaPolicies(raw)
	tool = strings.TrimSpace(tool)
	for _, policy := range policies {
		if strings.TrimSpace(policy.Tool) == tool {
			return policy, true
		}
	}
	return MCPQuotaPolicy{}, false
}

func normalizeMCPQuotaPolicies(raw interface{}) []MCPQuotaPolicy {
	switch value := raw.(type) {
	case []MCPQuotaPolicy:
		return value
	case []map[string]interface{}:
		b, _ := json.Marshal(value)
		var out []MCPQuotaPolicy
		_ = json.Unmarshal(b, &out)
		return out
	case string:
		var out []MCPQuotaPolicy
		_ = json.Unmarshal([]byte(value), &out)
		return out
	default:
		b, _ := json.Marshal(value)
		var out []MCPQuotaPolicy
		_ = json.Unmarshal(b, &out)
		return out
	}
}

func quotaError(tool, scope string, limit int) error {
	return fmt.Errorf(`{"status":"error","error":"mcp_quota_exceeded","tool":%q,"scope":%q,"limit":%d}`, tool, scope, limit)
}
