package runtimeapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	engineloop "InkLore/internal/agent/runtime/application/loop"
	enginetooling "InkLore/internal/agent/runtime/application/tooling"
	toolapp "InkLore/internal/agent/tool/application"
	tooldomain "InkLore/internal/agent/tool/domain"
	llmdomain "InkLore/internal/llm/domain"
)

// ErrToolNotRegistered 表示 Agent 定义里声明了尚未注册的工具。
var ErrToolNotRegistered = errors.New("agent tool is not registered")

// resolvedTools 是一次运行中真正可执行的工具集合。
type resolvedTools struct {
	caps    *enginetooling.ToolCapabilityRegistry
	entries []engineloop.ToolEntry
	allowed []string
}

// resolveTools 把 Agent 定义中的工具 ID 解析为 TAO 循环可调用的工具项。
//
// 只有真实注册在 ToolService 中的工具才会被装配：声明了但无法执行的工具必须
// 直接报错，而不是被静默忽略或在轨迹里伪造一次"调用成功"。
func resolveTools(service *toolapp.ToolService, toolIDs []string) (resolvedTools, error) {
	resolved := resolvedTools{
		entries: make([]engineloop.ToolEntry, 0, len(toolIDs)),
		allowed: make([]string, 0, len(toolIDs)),
	}
	if len(toolIDs) == 0 {
		return resolved, nil
	}
	if service == nil {
		return resolved, fmt.Errorf("%w: 工具服务未装配", ErrToolNotRegistered)
	}

	seen := make(map[string]bool, len(toolIDs))
	for _, raw := range toolIDs {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		tool, ok := service.Lookup(id)
		if !ok {
			return resolved, fmt.Errorf("%w: %s", ErrToolNotRegistered, id)
		}
		seen[id] = true
		resolved.entries = append(resolved.entries, buildToolEntry(service, tool))
		resolved.allowed = append(resolved.allowed, id)
	}
	resolved.caps = buildCapabilityRegistry(resolved.allowed)
	return resolved, nil
}

// buildToolEntry 把领域工具绑定为循环可调用的「定义 + 执行器」，输出直接来自工具返回值。
func buildToolEntry(service *toolapp.ToolService, tool tooldomain.Tool) engineloop.ToolEntry {
	descriptor := tool.Descriptor()
	toolID := descriptor.ID

	return engineloop.ToolEntry{
		Definition: llmdomain.Tool{
			Type: "function",
			Function: llmdomain.ToolFunction{
				Name:        toolID,
				Description: descriptor.Description,
				Parameters:  toolParameterSchema(descriptor.Parameters),
			},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			input := map[string]any{}
			if trimmed := bytes.TrimSpace(args); len(trimmed) > 0 {
				if err := json.Unmarshal(trimmed, &input); err != nil {
					return "", fmt.Errorf("工具 %s 参数解析失败: %w", toolID, err)
				}
			}
			executed, err := service.ExecuteTool(ctx, toolID, input)
			if err != nil {
				return "", err
			}
			payload, marshalErr := json.Marshal(executed)
			if marshalErr != nil {
				return executed.Summary, nil
			}
			return string(payload), nil
		},
	}
}

func toolParameterSchema(params []tooldomain.ToolParam) map[string]any {
	properties := make(map[string]any, len(params))
	required := make([]string, 0, len(params))
	for _, param := range params {
		entry := map[string]any{
			"type":        normalizeJSONSchemaType(param.Type),
			"description": param.Description,
		}
		if param.Default != nil {
			entry["default"] = param.Default
		}
		properties[param.Name] = entry
		if param.Required {
			required = append(required, param.Name)
		}
	}
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func normalizeJSONSchemaType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "string", "number", "integer", "boolean", "array", "object":
		return normalized
	default:
		return "string"
	}
}

// buildCapabilityRegistry 为创作工具声明能力与审批策略。
//
// 必须显式注册：TAO 循环在派发工具前会调用 CheckToolPermission，
// 未注册的工具会直接返回 ErrToolCapabilityNotRegistered 而被拒绝执行，
// 表现为「模型调用了工具，但什么都没发生」。
func buildCapabilityRegistry(toolIDs []string) *enginetooling.ToolCapabilityRegistry {
	items := enginetooling.DefaultToolCapabilities().List()
	for _, id := range toolIDs {
		items = append(items, enginetooling.ToolCapability{
			Name:            id,
			Type:            "builtin",
			RiskLevel:       enginetooling.CapabilityRiskLow,
			ApprovalPolicy:  enginetooling.ApprovalNever,
			AuditLevel:      enginetooling.AuditSummary,
			SideEffectLevel: sideEffectLevelFor(id),
			Parallelizable:  true,
			// 这里刻意**不设** ExemptFromApproval：该标记是给 ask_user 这类「本身就是
			// 向用户征询的手段」用的，它会在权限级别判定之前直接放行，使
			// permission_level=always_ask 完全失效（INK-012 探针实测）。
			// 创作工具的审批语义由 ApprovalNever + always_ask 的组合表达：
			// 常态不打断，用户显式要求「每次都问」时仍会拦下。
		})
	}
	return enginetooling.NewToolCapabilityRegistry(items)
}

// sideEffectLevelFor：web_search 会出网，其余创作工具是本地确定性计算。
func sideEffectLevelFor(toolID string) string {
	if toolID == "web_search" {
		return enginetooling.ToolSideEffectExternal
	}
	return enginetooling.ToolSideEffectReadonly
}
