package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/scope"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// PlatformUTCNowInputSchema and PlatformUTCNowOutputSchema are the exact
// governance schemas for the registered read-only platform clock capability.
// A configured key with incompatible schemas is never presented as runnable.
const (
	PlatformUTCNowInputSchema  = `{"type":"object","additionalProperties":false,"properties":{}}`
	PlatformUTCNowOutputSchema = `{"type":"object","additionalProperties":false,"properties":{"utc_now":{"type":"string","format":"date-time"}},"required":["utc_now"]}`
	platformUTCNowKey          = "platform_utc_now"
	maxToolPayload             = 32 * 1024
)

// ToolExecutor dispatches only explicitly registered capabilities. It never
// translates a configured MCP endpoint or arbitrary key into network or shell IO.
type ToolExecutor struct {
	repo     d.GovernanceRepository
	handlers map[string]builtinHandler
}

type builtinHandler struct {
	tool d.BuiltinTool
	run  func(context.Context) (map[string]any, error)
}

func NewToolExecutor(repo d.GovernanceRepository) *ToolExecutor {
	return &ToolExecutor{repo: repo, handlers: map[string]builtinHandler{
		platformUTCNowKey: {
			tool: d.BuiltinTool{
				ToolKey: platformUTCNowKey, Title: "UTC 时间", Description: "读取当前 UTC 时间。",
				ExecutionStatus: d.ToolExecutionNotBound, SideEffect: string(d.SideEffectReadOnly),
				InputSchema: json.RawMessage(PlatformUTCNowInputSchema), OutputSchema: json.RawMessage(PlatformUTCNowOutputSchema),
			},
			run: func(ctx context.Context) (map[string]any, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return map[string]any{"utc_now": time.Now().UTC().Format(time.RFC3339Nano)}, nil
			},
		},
	}}
}

// BuiltinTools projects installed handlers and scheduler control capabilities, never tenant
// tool heads. Entries default to not_bound until the application verifies the
// requesting tenant's current frozen version with ToolExecutionStatus.
func (e *ToolExecutor) BuiltinTools() []d.BuiltinTool {
	tools := make([]d.BuiltinTool, 0)
	if e == nil || e.repo == nil {
		return tools
	}
	for _, handler := range e.handlers {
		if handler.run != nil {
			tool := handler.tool
			tool.InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
			tool.OutputSchema = append(json.RawMessage(nil), tool.OutputSchema...)
			tools = append(tools, tool)
		}
	}
	tools = append(tools, d.BuiltinTool{ToolKey: d.ClarificationToolKey, Title: "向作者澄清", Description: "持久化澄清问题并等待本人回答；回答后恢复同一次运行。", ExecutionStatus: d.ToolExecutionNotBound, SideEffect: string(d.SideEffectReadOnly), InputSchema: json.RawMessage(d.ClarificationInputSchema), OutputSchema: json.RawMessage(d.ClarificationOutputSchema)})
	sort.Slice(tools, func(i, j int) bool { return tools[i].ToolKey < tools[j].ToolKey })
	return tools
}

// ToolExecutionStatus reflects an actual handler, an enabled tenant-scoped
// registration and the version's exact supported contract. An unbound callback
// at the application layer is always not_bound.
func (e *ToolExecutor) ToolExecutionStatus(ctx context.Context, tenantID string, view d.ToolView) string {
	if e == nil || e.repo == nil || ctx.Err() != nil || !scope.ValidID(tenantID) ||
		!scope.ValidID(view.CurrentVersionID) || !view.Enabled || view.ApprovalRequired || view.MCPServerID != nil ||
		view.RemoteToolName != nil || view.SideEffect != string(d.SideEffectReadOnly) ||
		view.TimeoutSeconds < 1 || view.TimeoutSeconds > 120 {
		return d.ToolExecutionNotBound
	}
	if view.ToolKey == d.ClarificationToolKey {
		if sameSchema(view.InputSchema, d.ClarificationInputSchema) && sameSchema(view.OutputSchema, d.ClarificationOutputSchema) {
			return d.ToolExecutionAvailable
		}
		return d.ToolExecutionNotBound
	}
	if handler, registered := e.handlers[view.ToolKey]; !registered || handler.run == nil {
		return d.ToolExecutionNotBound
	}
	if view.ToolKey != platformUTCNowKey || !sameSchema(view.InputSchema, PlatformUTCNowInputSchema) ||
		!sameSchema(view.OutputSchema, PlatformUTCNowOutputSchema) {
		return d.ToolExecutionNotBound
	}
	return d.ToolExecutionAvailable
}

func sameSchema(raw json.RawMessage, expected string) bool {
	if len(raw) == 0 || len(raw) > d.MaxSchemaBytes {
		return false
	}
	var got, want any
	return json.Unmarshal(raw, &got) == nil && json.Unmarshal([]byte(expected), &want) == nil && reflect.DeepEqual(got, want)
}

func validateToolDocument(schema json.RawMessage, payload []byte) error {
	if len(schema) == 0 || len(schema) > d.MaxSchemaBytes || len(payload) == 0 || len(payload) > maxToolPayload {
		return errors.New("tool schema or payload size invalid")
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	var definition any
	if err := json.Unmarshal(schema, &definition); err != nil {
		return err
	}
	const uri = "https://schemas.inklore.invalid/agent-execution.json"
	if err := compiler.AddResource(uri, definition); err != nil {
		return err
	}
	compiled, err := compiler.Compile(uri)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("tool payload contains trailing JSON")
	}
	return compiled.Validate(value)
}

// ExecuteTool resolves the exact frozen version, validates arguments and result,
// then executes the registered read-only handler under the version timeout.
func (e *ToolExecutor) ExecuteTool(ctx context.Context, input d.ToolCallInput) (*d.ToolCallOutput, error) {
	if e == nil || e.repo == nil || !scope.ValidID(input.TenantID) ||
		!scope.ValidID(input.ToolVersionID) || !scope.ValidID(input.RunID) ||
		!scope.ValidID(input.NodeID) || !scope.ValidID(input.ExecutionKey) {
		return nil, errors.New("agent tool executor: invalid identity or frozen version")
	}
	tool, version, err := e.repo.GetToolVersion(ctx, input.TenantID, input.ToolVersionID)
	if err != nil {
		return nil, err
	}
	if tool == nil || version == nil || tool.TenantID != input.TenantID || version.TenantID != input.TenantID ||
		version.ID != input.ToolVersionID || version.ToolID != tool.ID || tool.ToolKey != input.ToolKey ||
		tool.DeletedAt != nil || (input.SideEffect != "" && input.SideEffect != version.SideEffect) {
		return nil, errors.New("agent tool executor: tenant, key or version mismatch")
	}
	view := d.ToolView{ToolKey: tool.ToolKey, Enabled: tool.Enabled, CurrentVersionID: version.ID,
		InputSchema: version.InputSchema, OutputSchema: version.OutputSchema, SideEffect: version.SideEffect,
		TimeoutSeconds: version.TimeoutSeconds, ApprovalRequired: version.ApprovalRequired,
		MCPServerID: version.MCPServerID, RemoteToolName: version.RemoteToolName}
	if e.ToolExecutionStatus(ctx, input.TenantID, view) != d.ToolExecutionAvailable {
		return nil, errors.New("agent tool executor: no compatible registered handler")
	}
	if err := validateToolDocument(version.InputSchema, input.Arguments); err != nil {
		return nil, errors.New("agent tool executor: invalid frozen-schema arguments")
	}
	if input.ToolKey == d.ClarificationToolKey {
		return nil, errors.New("clarification must be persisted by the run scheduler")
	}
	handler := e.handlers[input.ToolKey].run
	limited, cancel := context.WithTimeout(ctx, time.Duration(version.TimeoutSeconds)*time.Second)
	defer cancel()
	result, err := handler(limited)
	if err != nil {
		return nil, err
	}
	if err := limited.Err(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err := validateToolDocument(version.OutputSchema, encoded); err != nil {
		return nil, errors.New("agent tool executor: handler output violates frozen schema")
	}
	digest := sha256.Sum256(encoded)
	return &d.ToolCallOutput{Success: true, ResultText: string(encoded), ResultHash: hex.EncodeToString(digest[:])}, nil
}
