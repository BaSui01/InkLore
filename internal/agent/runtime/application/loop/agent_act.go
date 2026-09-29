package engineloop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentdomain "InkLore/internal/agent/domain"
	ai "InkLore/internal/llm/domain"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/timeutil"
)

type actTurnResult struct {
	ToolMessages  []ai.ChatMessage
	ShouldSuspend bool
	ToolSpans     []agentdomain.TraceSpan
}

// executeActTurn 调度并执行本轮所有的 Tool Calls（Act + Observe 阶段），精准采集工具 Trace Spans
func (a *Agent) executeActTurn(
	ctx context.Context,
	req *RunRequest,
	turns int,
	toolCalls []ai.ToolCall,
	reqTools map[string]ToolEntry,
	capabilities *ToolCapabilityRegistry,
	permissionLevel string,
	approvalGate *ApprovalGate,
	loopGuard *LoopGuard,
	toolMW []ToolCallMiddleware,
	isAllowedTool func(string) bool,
	emit func(AgentEvent) error,
) (*actTurnResult, error) {
	outMessages := make([]ai.ChatMessage, 0, len(toolCalls))
	toolSpans := make([]agentdomain.TraceSpan, 0, len(toolCalls))
	shouldSuspend := false

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// 必须在任何预执行之前按原顺序计数一次；被拦截的调用不能进入 handler。
	// 保留权限检查先于 guard 的语义，按下标保存判定，避免重复调用 ID 混淆。
	guardMessages := make([]string, len(toolCalls))
	eligibleCalls := make([]ai.ToolCall, 0, len(toolCalls))
	for i, tc := range toolCalls {
		if isAllowedTool(tc.Function.Name) {
			if _, ok := a.getTool(tc.Function.Name, reqTools); ok && capabilities.CheckToolPermission(tc.Function.Name, req.Permissions) == nil {
				_, guardMessages[i] = loopGuard.CheckRepeatCall(tc.Function.Name, tc.Function.Arguments)
			}
		}
		if guardMessages[i] == "" {
			eligibleCalls = append(eligibleCalls, tc)
		}
	}
	preExecuted := a.preExecuteParallelTools(ctx, req, capabilities, permissionLevel, reqTools, isAllowedTool, toolMW, eligibleCalls, turns)

	for i, tc := range toolCalls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		toolStart := timeutil.Now().UTC()
		toolName := tc.Function.Name

		if !isAllowedTool(toolName) {
			errMsg := fmt.Sprintf("工具未启用: %s", toolName)
			logger.WarnContext(ctx, "agent tool is disabled", "turn", turns, "tool", toolName)
			if err := emit(ErrorStreamEvent(errMsg, ErrorCodePermissionDenied, ErrorReasonToolDisabled, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("错误: %s", errMsg),
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s", toolName),
				StartedAt:  toolStart,
				EndedAt:    timeutil.Now().UTC(),
				DurationMs: time.Since(toolStart).Milliseconds(),
				Status:     "error",
				Attributes: map[string]any{"error": errMsg, "tool": toolName},
			})
			continue
		}

		entry, ok := a.getTool(toolName, reqTools)
		if !ok {
			errMsg := fmt.Sprintf("未知工具: %s", toolName)
			logger.WarnContext(ctx, "agent tool is unknown", "turn", turns, "tool", toolName)
			if err := emit(ErrorStreamEvent(errMsg, ErrorCodeInvalidRequest, ErrorReasonToolUnknown, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("错误: %s", errMsg),
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s", toolName),
				StartedAt:  toolStart,
				EndedAt:    timeutil.Now().UTC(),
				DurationMs: time.Since(toolStart).Milliseconds(),
				Status:     "error",
				Attributes: map[string]any{"error": errMsg, "tool": toolName},
			})
			continue
		}

		if err := capabilities.CheckToolPermission(toolName, req.Permissions); err != nil {
			errMsg := fmt.Sprintf("工具权限不足: %s", toolName)
			logger.WarnContext(ctx, "agent tool permission denied", "turn", turns, "tool", toolName, "error", err)
			if err := emit(ErrorStreamEvent(errMsg, ErrorCodePermissionDenied, ErrorReasonToolPermissionDenied, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("错误: %s", errMsg),
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s", toolName),
				StartedAt:  toolStart,
				EndedAt:    timeutil.Now().UTC(),
				DurationMs: time.Since(toolStart).Milliseconds(),
				Status:     "error",
				Attributes: map[string]any{"error": errMsg, "tool": toolName},
			})
			continue
		}

		// Loop Guard: 重复调用相同参数检测
		if guardMsg := guardMessages[i]; guardMsg != "" {
			logger.WarnContext(ctx, "Loop Guard: 重复工具调用拦截", "turn", turns, "tool", toolName)
			if err := emit(AgentEvent{
				Type:    StandardEventToolResult,
				Turn:    turns,
				Content: guardMsg,
				Data: ToolResultData{
					Tool:       toolName,
					ToolCallID: tc.ID,
					Result:     guardMsg,
				},
			}); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    guardMsg,
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s (Loop Guarded)", toolName),
				StartedAt:  toolStart,
				EndedAt:    timeutil.Now().UTC(),
				DurationMs: time.Since(toolStart).Milliseconds(),
				Status:     "guarded",
				Attributes: map[string]any{"guard_message": guardMsg, "tool": toolName},
			})
			continue
		}

		rawArgs := json.RawMessage(tc.Function.Arguments)
		if err := ValidateToolArguments(entry.Definition.Function.Parameters, rawArgs); err != nil {
			errMsg := fmt.Sprintf("工具参数无效: %s: %v", toolName, err)
			logger.WarnContext(ctx, "agent tool arguments invalid", "turn", turns, "tool", toolName, "error", err)
			// 参数错误可由模型纠正，不能发送终止整个运行的 run_error 事件。
			contract := NewErrorContract(ErrorCodeInvalidRequest, errMsg, ErrorReasonToolInvalidArguments)
			if err := emit(AgentEvent{
				Type: StandardEventToolError, Turn: turns, Content: errMsg,
				Data: ToolResultData{Tool: toolName, ToolCallID: tc.ID, Error: errMsg, ErrorContract: &contract},
			}); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("[Self-Correction] 工具参数校验失败: %v。请根据工具参数 schema 修正入参格式后重新调用。", err),
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s (Self-Correction)", toolName),
				StartedAt:  toolStart,
				EndedAt:    timeutil.Now().UTC(),
				DurationMs: time.Since(toolStart).Milliseconds(),
				Status:     "invalid_arguments",
				Attributes: map[string]any{"validation_error": err.Error(), "tool": toolName},
			})
			continue
		}

		decision := approvalGate.Evaluate(ctx, ApprovalInput{
			UserID:          req.UserID,
			SessionID:       req.SessionID,
			AgentID:         req.AgentID,
			RunID:           req.RunID,
			PermissionLevel: permissionLevel,
			ToolName:        toolName,
			Args:            rawArgs,
			Tokens:          req.ApprovalTokens,
		})
		approved := decision.Approved
		consumption := decision.Consumption
		if decision.Required && !approved {
			approvalID := int64(0)
			expiresAt := ""
			if decision.Grant != nil {
				approvalID = decision.Grant.ID
				if !decision.Grant.ExpiresAt.IsZero() {
					expiresAt = timeutil.FormatDB(decision.Grant.ExpiresAt)
				}
			}
			resultMap := map[string]interface{}{
				"status":         "pending_approval",
				"error":          "permission_denied_need_approval",
				"error_contract": map[string]interface{}{"code": "tool_approval_required", "category": "approval_required", "message": "该操作需要审批"},
				"message":        "该操作需要您的批准。请在下方卡片中点击确认以授权执行。",
				"tool":           toolName,
				"arguments":      rawArgs,
			}
			if approvalID > 0 {
				resultMap["approval_id"] = approvalID
			}
			if expiresAt != "" {
				resultMap["expires_at"] = expiresAt
			}
			resultBytes, _ := json.Marshal(resultMap)
			result := string(resultBytes)
			logger.InfoContext(ctx, "工具 需要用户授权", "turn", turns, "function_name", toolName)
			if err := emit(AgentEvent{
				Type:    StandardEventToolApprovalRequired,
				Turn:    turns,
				Content: fmt.Sprintf("等待授权: %s", toolName),
				Data: ApprovalRequiredData{
					Tool:       toolName,
					ToolCallID: tc.ID,
					Status:     "pending_approval",
					Result:     result,
					ApprovalID: approvalID,
					ExpiresAt:  expiresAt,
					Arguments:  rawArgs,
				},
			}); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    result,
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s (Approval Required)", toolName),
				StartedAt:  toolStart,
				EndedAt:    timeutil.Now().UTC(),
				DurationMs: time.Since(toolStart).Milliseconds(),
				Status:     "pending_approval",
				Attributes: map[string]any{"approval_id": approvalID, "tool": toolName},
			})
			shouldSuspend = true
			continue
		}

		if consumption != nil {
			if err := emit(AgentEvent{
				Type:    StandardEventToolApprovalConsumed,
				Turn:    turns,
				Content: fmt.Sprintf("审批已消费: %s", toolName),
				Data: ToolStartData{
					Tool:       toolName,
					ToolCallID: tc.ID,
					Approval:   ApprovalConsumptionData(consumption),
				},
			}); err != nil {
				return nil, err
			}
		}

		logger.InfoContext(ctx, "执行工具", "turn", turns, "function_name", toolName, "arguments", tc.Function.Arguments)

		if err := emit(AgentEvent{
			Type:    StandardEventToolStart,
			Turn:    turns,
			Content: fmt.Sprintf("执行工具: %s", toolName),
			Data: ToolStartData{
				Tool:       toolName,
				ToolCallID: tc.ID,
				Arguments:  tc.Function.Arguments,
			},
		}); err != nil {
			return nil, err
		}

		var result string
		var err error
		isParallel := false
		if pre, ok := preExecuted[tc.ID]; ok {
			isParallel = true
			if pre.beforeErr != nil {
				if emitErr := emit(ErrorStreamEvent(pre.beforeErr.Error(), ErrorCodeInternalError, ErrorReasonToolMiddlewareFailed, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); emitErr != nil {
					return nil, emitErr
				}
				return nil, fmt.Errorf("agent tool middleware before %s failed: %w", toolName, pre.beforeErr)
			}
			result, err = pre.result, pre.err
		} else {
			toolCtx, toolCancel := context.WithTimeout(ctx, capabilities.ToolTimeout(toolName))
			toolCtx = WithRuntimeContext(toolCtx, NewRuntimeContext(req))
			if approved {
				toolCtx = WithApprovedTool(toolCtx, toolName)
			}
			toolCallCtx := &ToolCallContext{
				Turn:      turns,
				Request:   req,
				ToolCall:  tc,
				Tool:      entry,
				Arguments: rawArgs,
				Approved:  approved,
			}
			for _, middleware := range toolMW {
				if err := middleware.BeforeToolCall(toolCtx, toolCallCtx); err != nil {
					toolCancel()
					if emitErr := emit(ErrorStreamEvent(err.Error(), ErrorCodeInternalError, ErrorReasonToolMiddlewareFailed, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); emitErr != nil {
						return nil, emitErr
					}
					return nil, fmt.Errorf("agent tool middleware before %s failed: %w", toolName, err)
				}
			}
			if err = toolCtx.Err(); err == nil {
				result, err = entry.Handler(toolCtx, rawArgs)
			}
			if contextErr := toolCtx.Err(); contextErr != nil {
				result, err = "", contextErr
			}
			toolCancel()
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}
		toolEnd := timeutil.Now().UTC()
		toolDuration := toolEnd.Sub(toolStart).Milliseconds()

		if err != nil {
			toolResult := &ToolCallResult{
				Turn:       turns,
				Tool:       toolName,
				ToolCallID: tc.ID,
				Arguments:  rawArgs,
				Output:     &ToolOutput{Status: ToolOutputStatusError, Text: err.Error()},
				Error:      err.Error(),
			}
			for _, middleware := range toolMW {
				if mwErr := middleware.AfterToolCall(ctx, toolResult); mwErr != nil {
					if emitErr := emit(ErrorStreamEvent(mwErr.Error(), ErrorCodeInternalError, ErrorReasonToolMiddlewareFailed, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); emitErr != nil {
						return nil, emitErr
					}
					return nil, fmt.Errorf("agent tool middleware after %s failed: %w", toolName, mwErr)
				}
			}
			logger.ErrorContext(ctx, "工具 失败", "turn", turns, "function_name", toolName, "error", err)
			contract := NewErrorContract(ErrorCodeInternalError, err.Error(), ErrorReasonToolExecutionFailed)
			if err := emit(AgentEvent{
				Type:    StandardEventToolError,
				Turn:    turns,
				Content: fmt.Sprintf("工具 %s 执行失败", toolName),
				Data: ToolResultData{
					Tool:          toolName,
					ToolCallID:    tc.ID,
					Error:         err.Error(),
					ErrorContract: &contract,
					ToolResult:    toolResult,
				},
			}); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("[Self-Correction] 工具 %s 执行异常: %v。请根据报错信息修正参数并重试，或向用户说明情况。", toolName, err),
			})
			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s", toolName),
				StartedAt:  toolStart,
				EndedAt:    toolEnd,
				DurationMs: toolDuration,
				Status:     "error",
				Attributes: map[string]any{"error": err.Error(), "tool": toolName, "parallel": isParallel},
			})
		} else {
			normalized := NormalizeToolOutput(result)
			toolResult := &ToolCallResult{
				Turn:       turns,
				Tool:       toolName,
				ToolCallID: tc.ID,
				Arguments:  rawArgs,
				Output:     normalized,
				Raw:        result,
			}
			for _, middleware := range toolMW {
				if mwErr := middleware.AfterToolCall(ctx, toolResult); mwErr != nil {
					if emitErr := emit(ErrorStreamEvent(mwErr.Error(), ErrorCodeInternalError, ErrorReasonToolMiddlewareFailed, map[string]interface{}{"turn": turns, "tool": toolName, "tool_call_id": tc.ID})); emitErr != nil {
						return nil, emitErr
					}
					return nil, fmt.Errorf("agent tool middleware after %s failed: %w", toolName, mwErr)
				}
			}
			standardType := StandardEventToolResult
			spanStatus := "ok"
			if normalized.Status == ToolOutputStatusError {
				standardType = StandardEventToolError
				spanStatus = "error"
			}
			if err := emit(AgentEvent{
				Type:    standardType,
				Turn:    turns,
				Content: fmt.Sprintf("工具 %s 执行成功", toolName),
				Data: ToolResultData{
					Tool:       toolName,
					ToolCallID: tc.ID,
					Result:     result,
					ToolResult: toolResult,
					Output:     normalized,
				},
			}); err != nil {
				return nil, err
			}
			outMessages = append(outMessages, ai.ChatMessage{
				Role:       ai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    result,
			})

			toolSpans = append(toolSpans, agentdomain.TraceSpan{
				SpanID:     fmt.Sprintf("turn_%d_tool_%s", turns, tc.ID),
				Kind:       agentdomain.SpanKindTool,
				Name:       fmt.Sprintf("Tool: %s", toolName),
				StartedAt:  toolStart,
				EndedAt:    toolEnd,
				DurationMs: toolDuration,
				Status:     spanStatus,
				Attributes: map[string]any{"tool": toolName, "parallel": isParallel, "duration_ms": toolDuration},
			})

			if capability, ok := capabilities.Get(toolName); ok && capability.SuspendsRun {
				var parsed struct {
					Confirmed bool `json:"confirmed"`
				}
				if errJson := json.Unmarshal([]byte(result), &parsed); errJson == nil && !parsed.Confirmed {
					shouldSuspend = true
				}
			}
		}
	}

	return &actTurnResult{
		ToolMessages:  outMessages,
		ShouldSuspend: shouldSuspend,
		ToolSpans:     toolSpans,
	}, nil
}
