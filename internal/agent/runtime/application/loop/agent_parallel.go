package engineloop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ai "InkLore/internal/llm/domain"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/safego"
)

type preExecutedTool struct {
	err       error
	beforeErr error
	result    string
}

// preExecuteParallelTools 并行快路径：当本轮全部工具调用都是可并行的只读工具
// （Parallelizable + readonly + ApprovalNever + 非 SuspendsRun）且权限级别非 always_ask 时，预先并发执行 handler。
// 中间件保持串行（不要求实现并发安全），仅网络/IO 密集的 handler 并发。
// 主循环按原顺序消费结果，事件与消息顺序保持确定性。不满足条件返回 nil（走串行路径）。
func (a *Agent) preExecuteParallelTools(
	ctx context.Context,
	req *RunRequest,
	capabilities *ToolCapabilityRegistry,
	permissionLevel string,
	reqTools map[string]ToolEntry,
	allowed func(string) bool,
	toolMW []ToolCallMiddleware,
	calls []ai.ToolCall,
	turns int,
) map[string]*preExecutedTool {
	if len(calls) < 2 || permissionLevel == permissionAlwaysAsk {
		return nil
	}
	type job struct {
		entry ToolEntry
		call  ai.ToolCall
	}
	jobs := make([]job, 0, len(calls))
	seenIDs := make(map[string]bool, len(calls))
	for _, tc := range calls {
		if tc.ID == "" || seenIDs[tc.ID] {
			return nil
		}
		seenIDs[tc.ID] = true
		capability, ok := capabilities.Get(tc.Function.Name)
		if !ok || !capability.Parallelizable || capability.SideEffectLevel != ToolSideEffectReadonly || capability.ApprovalPolicy != ApprovalNever || capability.SuspendsRun {
			return nil
		}
		if !allowed(tc.Function.Name) {
			return nil
		}
		entry, ok := a.getTool(tc.Function.Name, reqTools)
		if !ok {
			return nil
		}
		if capabilities.CheckToolPermission(tc.Function.Name, req.Permissions) != nil {
			return nil
		}
		if ValidateToolArguments(entry.Definition.Function.Parameters, json.RawMessage(tc.Function.Arguments)) != nil {
			return nil
		}
		jobs = append(jobs, job{call: tc, entry: entry})
	}
	// 批次截止时间也必须传给 handler；safego 的回调上下文会脱离取消链，不能使用它。
	batchCtx, batchCancel := context.WithTimeout(ctx, 30*time.Second)
	defer batchCancel()
	type pendingTool struct {
		id   string
		ctx  context.Context
		done <-chan preExecutedTool
	}
	pending := make([]pendingTool, 0, len(jobs))
	out := make(map[string]*preExecutedTool, len(jobs))
	for _, item := range jobs {
		toolCtx, toolCancel := context.WithTimeout(batchCtx, capabilities.ToolTimeout(item.call.Function.Name))
		defer toolCancel()
		toolCtx = WithRuntimeContext(toolCtx, NewRuntimeContext(req))
		toolCtx = WithApprovedTool(toolCtx, item.call.Function.Name)
		pre := &preExecutedTool{}
		out[item.call.ID] = pre
		if err := toolCtx.Err(); err != nil {
			pre.err = err
			continue
		}
		toolCallCtx := &ToolCallContext{
			Turn: turns, Request: req, ToolCall: item.call, Tool: item.entry,
			Arguments: json.RawMessage(item.call.Function.Arguments), Approved: true,
		}
		for _, middleware := range toolMW {
			if err := toolCtx.Err(); err != nil {
				pre.err = err
				break
			}
			if err := middleware.BeforeToolCall(toolCtx, toolCallCtx); err != nil {
				pre.beforeErr = err
				break
			}
		}
		if pre.beforeErr != nil || pre.err != nil {
			continue
		}
		// worker 只发布值，不持有 out 的指针。缓冲允许取消返回后迟到的结果自行退出，
		// panic 也通过相同通道发布，避免恢复回调晚于完成通知而产生伪成功。
		done := make(chan preExecutedTool, 1)
		pending = append(pending, pendingTool{id: item.call.ID, ctx: toolCtx, done: done})
		safego.GoWithRecover(toolCtx, "aiagent.parallel_tool_"+item.call.Function.Name, func(context.Context) {
			if err := toolCtx.Err(); err != nil {
				done <- preExecutedTool{err: err}
				return
			}
			result, err := item.entry.Handler(toolCtx, json.RawMessage(item.call.Function.Arguments))
			if contextErr := toolCtx.Err(); contextErr != nil {
				result, err = "", contextErr
			}
			done <- preExecutedTool{result: result, err: err}
		}, func(p any, stack string) {
			done <- preExecutedTool{err: fmt.Errorf("tool handler panic: %v", p)}
		})
	}
	for _, item := range pending {
		var result preExecutedTool
		select {
		case result = <-item.done:
		case <-item.ctx.Done():
			// 若结果已发布则保留它；否则未完成调用必须显式失败，不能返回零值成功。
			select {
			case result = <-item.done:
			default:
				result.err = item.ctx.Err()
			}
		}
		out[item.id] = &result
	}
	logger.InfoContext(ctx, "并行执行只读工具", "turn", turns, "job_count", len(jobs))
	return out
}
