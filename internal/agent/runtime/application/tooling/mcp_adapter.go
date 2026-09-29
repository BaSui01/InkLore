package enginetooling

import (
	"context"
	"encoding/json"
	"fmt"

	agentdomain "InkLore/internal/agent/domain"
)

// MCPToolCaller MCP 工具调用客户端接口
type MCPToolCaller interface {
	CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error)
}

// MCPToolAdapter 将外部 MCP 服务的工具适配为标准 agentdomain.Tool
type MCPToolAdapter struct {
	serverName string
	toolName   string
	desc       string
	schema     any
	client     MCPToolCaller
	capability agentdomain.ToolCapability
}

// NewMCPToolAdapter 创建 MCP 工具适配器
func NewMCPToolAdapter(serverName, toolName, desc string, schema any, client MCPToolCaller) *MCPToolAdapter {
	return &MCPToolAdapter{
		serverName: serverName,
		toolName:   toolName,
		desc:       desc,
		schema:     schema,
		client:     client,
		capability: agentdomain.ToolCapability{
			Name:            toolName,
			Type:            "mcp",
			RiskLevel:       CapabilityRiskMedium,
			ApprovalPolicy:  ApprovalAskRisk,
			AuditLevel:      AuditFull,
			SideEffectLevel: ToolSideEffectExternal,
		},
	}
}

func (m *MCPToolAdapter) Name() string {
	return m.toolName
}

func (m *MCPToolAdapter) Description() string {
	if m.serverName != "" {
		return fmt.Sprintf("[%s] %s", m.serverName, m.desc)
	}
	return m.desc
}

func (m *MCPToolAdapter) Parameters() any {
	return m.schema
}

func (m *MCPToolAdapter) PromptGuidelines() string {
	return fmt.Sprintf("这是来自外部 MCP 服务 [%s] 的扩展工具。调用时请严格遵循输入参数 schema 格式。", m.serverName)
}

func (m *MCPToolAdapter) Capability() agentdomain.ToolCapability {
	return m.capability
}

func (m *MCPToolAdapter) Execute(ctx context.Context, rc *agentdomain.RuntimeContext, rawArgs json.RawMessage) (*agentdomain.ToolResult, error) {
	if m.client == nil {
		return nil, fmt.Errorf("mcp client is nil for server %s", m.serverName)
	}
	var arguments map[string]interface{}
	if len(rawArgs) > 0 {
		_ = json.Unmarshal(rawArgs, &arguments)
	}

	res, err := m.client.CallTool(ctx, m.toolName, arguments)
	if err != nil {
		return &agentdomain.ToolResult{
			ToolName: m.toolName,
			Content:  fmt.Sprintf("MCP 工具 [%s] 执行失败: %v", m.toolName, err),
			IsError:  true,
		}, err
	}

	var contentStr string
	if strRes, ok := res.(string); ok {
		contentStr = strRes
	} else {
		b, _ := json.Marshal(res)
		contentStr = string(b)
	}

	return &agentdomain.ToolResult{
		ToolName: m.toolName,
		Content:  contentStr,
		Raw:      res,
	}, nil
}
