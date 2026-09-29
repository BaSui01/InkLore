package runtimeapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	agentdomain "InkLore/internal/agent/domain"
	engineloop "InkLore/internal/agent/runtime/application/loop"
	"InkLore/internal/agent/runtime/application/ports"
	enginepromptctx "InkLore/internal/agent/runtime/application/promptctx"
	toolapp "InkLore/internal/agent/tool/application"
	llmdomain "InkLore/internal/llm/domain"
	llmgateway "InkLore/internal/llm/domain/gateway"
	"InkLore/internal/platform/cache"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/timeutil"
)

// DefaultMaxTurns 是单次 Agent 运行的默认最大轮数（Think-Act 轮次上限）。
const DefaultMaxTurns = 8

var (
	// ErrEmptyUserInput 用户输入为空。
	ErrEmptyUserInput = errors.New("agent user input is required")
	// ErrModelGatewayUnavailable 模型执行网关未配置。
	ErrModelGatewayUnavailable = errors.New("agent model gateway is unavailable")
)

// Service 是智能体域的运行时入口。
//
// 职责边界：
//   - Run 执行真实的 TAO（Think-Act-Observe）循环，轨迹只由循环产生的事件映射而来；
//   - RunStream 提供请求级流式生成，供章节逐字推流使用。
//
// 本服务不产生任何解释性文案：Observation 的内容一律来自工具的真实返回值，
// 模型不可用时直接报错，绝不回退到模拟输出。
type Service struct {
	executor llmgateway.ModelExecutionGateway
	models   ports.ModelGateway
	tools    *toolapp.ToolService
	// recorder 为可选的运行审计记录器；未注入时不影响运行本身。
	recorder ports.RunRecorder
	// approvals 为可选的工具审批服务；未注入时高风险工具一律拒绝（fail-closed）。
	approvals engineloop.ToolApprovalService
	// prompts 负责 Prompt 段装配与稳定前缀缓存；未注入时退化为每次现算（不缓存）。
	prompts *promptAssembly
	// compaction 是上下文压缩的辅助模型绑定；未绑定时压缩退化为本地确定性交接文档（INK-050）。
	compaction enginepromptctx.AuxiliaryTaskBinding
	// activity 为可选的活动日志存储；未注入时不记录，运行本身不受影响（INK-037）。
	activity ports.ActivityLogStore
	// runs 为可选的运行列表读取端口；未注入时列表为空（前端「Agent 轨迹」视图，INK-041）。
	runs     ports.RunLister
	maxTurns int
}

// NewService 装配 Agent 运行时。
func NewService(models ports.ModelGateway, executor llmgateway.ModelExecutionGateway, tools *toolapp.ToolService, recorder ports.RunRecorder, approvals engineloop.ToolApprovalService, promptCache *cache.ServiceCache, compaction enginepromptctx.AuxiliaryTaskBinding, activity ports.ActivityLogStore) *Service {
	return &Service{
		models:     models,
		executor:   executor,
		tools:      tools,
		recorder:   recorder,
		approvals:  approvals,
		prompts:    newPromptAssembly(promptCache),
		compaction: compaction,
		activity:   activity,
		maxTurns:   DefaultMaxTurns,
	}
}

// Run 执行一次 TAO 循环并返回可审计的执行轨迹。
func (s *Service) Run(ctx context.Context, input agentdomain.RunInput) (agentdomain.RunResult, error) {
	return s.run(ctx, input, nil)
}

// RunEvents 与 Run 等价，但把循环产生的**标准化事件**实时回调出去（INK-109）。
//
// 为什么不是另写一条执行路径：Run 里除了循环本身还做了运行审计、Prompt 段装配、
// 轨迹累积与终态回填——复制一份必然漂移。这里只加一个转发出口，
// forward 为 nil 时行为与 Run 完全一致。
//
// forward 返回 error 会中止运行（与 loop 的 StreamCallback 契约一致），
// 因此调用方（SSE 写失败等）可以用它主动刹车。
func (s *Service) RunEvents(ctx context.Context, input agentdomain.RunInput, forward func(agentdomain.AgentEvent) error) (agentdomain.RunResult, error) {
	return s.run(ctx, input, forward)
}

func (s *Service) run(ctx context.Context, input agentdomain.RunInput, forward func(agentdomain.AgentEvent) error) (agentdomain.RunResult, error) {
	if strings.TrimSpace(input.UserInput) == "" {
		return agentdomain.RunResult{}, ErrEmptyUserInput
	}
	if s == nil || s.executor == nil {
		return agentdomain.RunResult{}, ErrModelGatewayUnavailable
	}

	resolved, err := resolveTools(s.tools, input.Definition.Tools)
	if err != nil {
		return agentdomain.RunResult{}, err
	}

	maxTurns := s.maxTurns
	if maxTurns <= 0 {
		maxTurns = DefaultMaxTurns
	}

	// 运行审计：先落一条 running 记录，运行结束后回填状态与事件流。
	// 记录失败只告警不阻断 —— 审计可用性不应以牺牲用户请求为代价。
	runID := int64(0)
	if s.recorder != nil {
		startedID, recordErr := s.recorder.StartRun(ctx, ports.RunRecord{
			AgentKey:  input.Definition.ID,
			RequestID: input.RequestID,
			UserInput: input.UserInput,
		})
		if recordErr != nil {
			logger.WarnContext(ctx, "创建 Agent 运行记录失败，本次运行不做审计留痕", "error", recordErr.Error())
		} else {
			runID = startedID
		}
	}

	traceSteps := make([]agentdomain.TraceStep, 0, 16)
	thinking := &strings.Builder{}
	agent := engineloop.New(s.executor, "")

	// Prompt 段装配（INK-014）：稳定前缀走缓存，用户输入作为易变段；
	// 段的最终合成与 Prefix-Cache 元数据由 loop 的 PromptSectionMiddleware 在模型调用前完成。
	sections, stableCacheHit := s.prompts.Build(ctx, promptAssemblyInput{
		AgentKey:     input.Definition.ID,
		Model:        input.Definition.Model,
		SystemPrompt: input.Definition.SystemPrompt,
		ToolIDs:      resolved.allowed,
		UserInput:    input.UserInput,
	}, toolGuidelines(resolved.allowed, s.describeTool))
	traceSteps = append(traceSteps, agentdomain.TraceStep{
		Type:      agentdomain.StepObservation,
		Content:   fmt.Sprintf("Prompt 装配完成：%d 段（稳定前缀缓存命中=%t）", len(sections), stableCacheHit),
		Timestamp: timeutil.Now(),
		Metadata:  promptAssemblyMetadata(sections, stableCacheHit, s.prompts.RedisEnabled()),
	})

	// 审批单关联到本次运行：运行时记录器返回的 runID 是唯一可追溯的锚点。
	var runIDRef *int64
	if runID > 0 {
		runIDRef = &runID
	}

	response, runErr := agent.Run(ctx, &engineloop.RunRequest{
		RequestID:       input.RequestID,
		Model:           input.Definition.Model,
		SystemPrompt:    input.Definition.SystemPrompt,
		Temperature:     input.Definition.Temperature,
		MaxTurns:        maxTurns,
		UserID:          input.UserID,
		ApprovalTokens:  input.ApprovalTokens,
		PermissionLevel: input.PermissionLevel,
		SessionID:       input.SessionID,
		AgentID:         input.AgentID,
		RunID:           runIDRef,
		Approvals:       s.approvals,
		Compaction:      s.compaction,
		Options:         map[string]interface{}{engineloop.PromptSectionsOptionKey: sections},
		Middlewares:     []engineloop.Middleware{engineloop.NewPromptSectionMiddleware()},
		Messages: []llmdomain.ChatMessage{
			{Role: llmdomain.ChatMessageRoleUser, Content: input.UserInput},
		},
		Tools:        resolved.entries,
		AllowedTools: resolved.allowed,
		Capabilities: resolved.caps,
		OnEvent: func(event engineloop.AgentEvent) error {
			// 先转发再累积：实时性优先，且转发失败（如 SSE 客户端断开）应尽快中止运行，
			// 而不是先把整段轨迹攒完再发现没人接了。
			if forward != nil {
				if forwardErr := forward(agentdomain.AgentEvent(event)); forwardErr != nil {
					return forwardErr
				}
			}
			switch event.Type {
			case agentdomain.StandardEventThinkingDelta:
				thinking.WriteString(event.Content)
			case agentdomain.StandardEventTurnDone:
				flushThinking(traceSteps, thinking)
			case agentdomain.StandardEventToolStart:
				traceSteps = append(traceSteps, agentdomain.TraceStep{
					Type:      agentdomain.StepToolCall,
					Content:   event.Content,
					Timestamp: timeutil.Now(),
					Metadata:  map[string]any{"turn": event.Turn},
				})
			case agentdomain.StandardEventToolResult:
				traceSteps = append(traceSteps, agentdomain.TraceStep{
					Type:      agentdomain.StepObservation,
					Content:   toolOutputContent(event),
					Timestamp: timeutil.Now(),
					Metadata:  map[string]any{"status": "success", "turn": event.Turn},
				})
			case agentdomain.StandardEventToolApprovalRequired:
				// 需要授权的工具不会被静默跳过：把「等待授权」作为一条观察记录进轨迹，
				// 使「循环因审批而中止」在运行结果里可追溯（INK-012）。
				metadata := map[string]any{"status": "pending_approval", "turn": event.Turn}
				if data, ok := event.Data.(engineloop.ApprovalRequiredData); ok {
					metadata["tool"] = data.Tool
					if data.ApprovalID > 0 {
						metadata["approval_id"] = data.ApprovalID
					}
				}
				traceSteps = append(traceSteps, agentdomain.TraceStep{
					Type:      agentdomain.StepObservation,
					Content:   event.Content,
					Timestamp: timeutil.Now(),
					Metadata:  metadata,
				})
			case agentdomain.StandardEventToolApprovalConsumed:
				// 审批被消费意味着工具即将真正执行：留一条可审计的观察记录。
				traceSteps = append(traceSteps, agentdomain.TraceStep{
					Type:      agentdomain.StepObservation,
					Content:   event.Content,
					Timestamp: timeutil.Now(),
					Metadata:  map[string]any{"status": "approval_consumed", "turn": event.Turn},
				})
			case agentdomain.StandardEventToolError:
				traceSteps = append(traceSteps, agentdomain.TraceStep{
					Type:      agentdomain.StepObservation,
					Content:   toolOutputContent(event),
					Timestamp: timeutil.Now(),
					Metadata:  map[string]any{"status": "error", "turn": event.Turn},
				})
			case agentdomain.StandardEventRunDone:
				flushThinking(traceSteps, thinking)
				traceSteps = append(traceSteps, agentdomain.TraceStep{
					Type:      agentdomain.StepEmit,
					Content:   event.Content,
					Timestamp: timeutil.Now(),
				})
			}
			return nil
		},
	})
	flushThinking(traceSteps, thinking)

	if runErr != nil {
		s.finishRun(ctx, runID, "failed", agentdomain.RunResult{TraceSteps: traceSteps}, runErr)
		return agentdomain.RunResult{TraceSteps: traceSteps}, normalizeGatewayError(runErr)
	}

	result := agentdomain.RunResult{TraceSteps: traceSteps, Model: input.Definition.Model}
	if response != nil {
		result.Content = response.Content
	}
	s.finishRun(ctx, runID, "succeeded", result, nil)
	return result, nil
}

// finishRun 回填运行记录的状态、输出快照与事件流（JSONB）。
//
// 同样采取"失败只告警"策略：审计写入异常不应把一次成功的生成变成失败。
func (s *Service) finishRun(ctx context.Context, runID int64, status string, result agentdomain.RunResult, runErr error) {
	if s == nil {
		return
	}
	errorText := ""
	if runErr != nil {
		errorText = runErr.Error()
	}

	if s.recorder != nil && runID > 0 {
		s.persistRun(ctx, runID, status, result, errorText)
	}

	// 活动日志：一句话摘要 + 逐步轨迹，供排查与轨迹列表使用（INK-037）。
	// 写失败只告警：活动日志是审计增强项，不能让一次成功的运行变成失败。
	if s.activity != nil && runID > 0 {
		summary := fmt.Sprintf("运行 %s：%d 步", status, len(result.TraceSteps))
		if errorText != "" {
			summary = fmt.Sprintf("运行 %s：%d 步，错误：%s", status, len(result.TraceSteps), errorText)
		}
		if logErr := s.activity.Log(ctx, ports.ActivityLogRecord{
			RunID:   runID,
			NodeID:  "run",
			Summary: summary,
			Content: renderActivityContent(result),
		}); logErr != nil {
			logger.WarnContext(ctx, "写入活动日志失败", "run_id", runID, "error", logErr.Error())
		}
	}

}

func (s *Service) persistRun(ctx context.Context, runID int64, status string, result agentdomain.RunResult, errorText string) {
	traceJSON, marshalErr := json.Marshal(result.TraceSteps)
	if marshalErr != nil {
		traceJSON = []byte("[]")
	}
	outputJSON, outputErr := json.Marshal(map[string]any{
		"content": result.Content,
		"model":   result.Model,
		"steps":   len(result.TraceSteps),
	})
	if outputErr != nil {
		outputJSON = []byte("{}")
	}
	if recordErr := s.recorder.FinishRun(ctx, runID, status, string(outputJSON), string(traceJSON), errorText); recordErr != nil {
		logger.WarnContext(ctx, "回填 Agent 运行记录失败", "run_id", runID, "error", recordErr.Error())
	}
}

// toolOutputContent 提取工具事件的真实输出。
//
// event.Content 只是「工具 X 执行成功」这类状态描述，不代表执行结果；
// 真实输出在事件载荷 ToolResultData.Result / Output.Text 中。
// 观察步骤必须使用真实输出，否则轨迹会退化成一句无信息量的状态文案。
func toolOutputContent(event engineloop.AgentEvent) string {
	if data, ok := event.Data.(engineloop.ToolResultData); ok {
		if result := strings.TrimSpace(data.Result); result != "" {
			return data.Result
		}
		if data.Output != nil {
			if text := strings.TrimSpace(data.Output.Text); text != "" {
				return text
			}
		}
		if message := strings.TrimSpace(data.Error); message != "" {
			return message
		}
	}
	return event.Content
}

// describeTool 把工具渲染为稳定前缀里的一行说明；工具服务缺失时返回 false。
func (s *Service) describeTool(id string) (string, bool) {
	if s == nil || s.tools == nil {
		return "", false
	}
	tool, ok := s.tools.Lookup(id)
	if !ok {
		return "", false
	}
	descriptor := tool.Descriptor()
	return fmt.Sprintf("- %s（%s）：%s", descriptor.ID, descriptor.Name, descriptor.Description), true
}

func flushThinking(steps []agentdomain.TraceStep, buffer *strings.Builder) []agentdomain.TraceStep {
	if buffer == nil || buffer.Len() == 0 {
		return steps
	}
	steps = append(steps, agentdomain.TraceStep{
		Type:      agentdomain.StepThink,
		Content:   buffer.String(),
		Timestamp: timeutil.Now(),
	})
	buffer.Reset()
	return steps
}

// RunStream 提供请求级流式生成（章节逐字推流）。
func (s *Service) RunStream(ctx context.Context, input agentdomain.RunInput, onChunk func(chunk string) error) error {
	if strings.TrimSpace(input.UserInput) == "" {
		return ErrEmptyUserInput
	}
	if s == nil || s.models == nil {
		return ErrModelGatewayUnavailable
	}
	return normalizeGatewayError(s.models.GenerateStream(ctx, ports.ModelRequest{
		Model:        input.Definition.Model,
		SystemPrompt: input.Definition.SystemPrompt,
		Temperature:  input.Definition.Temperature,
		RequestID:    input.RequestID,
		Messages:     []ports.ModelMessage{{Role: "user", Content: input.UserInput}},
	}, onChunk))
}

// normalizeGatewayError 把模型网关的领域错误翻译成对外可读的业务错误。
//
// 未配置 Provider 是部署期最常见的可自愈问题，必须给出明确的 code 与文案，
// 而不是让它退化成通用的 500「内部错误」。
func normalizeGatewayError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, llmdomain.ErrProviderNotConfigured) {
		return platformerr.Wrap(err, http.StatusServiceUnavailable,
			"llm_provider_not_configured", "llm.provider_not_configured")
	}
	return err
}

// renderActivityContent 把轨迹渲染为人类可读的逐行文本，便于前端直接展示。
func renderActivityContent(result agentdomain.RunResult) string {
	lines := make([]string, 0, len(result.TraceSteps))
	for _, step := range result.TraceSteps {
		content := strings.TrimSpace(step.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", step.Type, content))
	}
	return strings.Join(lines, "\n")
}

// SetRunLister 注入运行列表读取端口（组合根调用）。
//
// 采用 setter 而非构造参数：轨迹列表是只读可观测能力，缺失时前端退化为空列表，
// 不应影响 Agent 运行本身。
func (s *Service) SetRunLister(lister ports.RunLister) {
	if s == nil {
		return
	}
	s.runs = lister
}

// ListRuns 分页列出当前租户的最近运行；未装配时返回空列表与 0 总数，不报错。
func (s *Service) ListRuns(ctx context.Context, limit, offset int) ([]ports.RunSummary, int64, error) {
	if s == nil || s.runs == nil {
		return []ports.RunSummary{}, 0, nil
	}
	return s.runs.ListRuns(ctx, limit, offset)
}

// ListActivity 返回某次运行的活动日志；未装配时不返回错误而是空列表。
func (s *Service) ListActivity(ctx context.Context, runID int64) ([]ports.ActivityLog, error) {
	if s == nil || s.activity == nil {
		return nil, nil
	}
	return s.activity.ListByRunID(ctx, runID)
}

// ListNodes 返回某次运行的工作流节点明细；未装配时不返回错误而是空列表。
func (s *Service) ListNodes(ctx context.Context, runID int64) ([]ports.RunNodeSummary, error) {
	if s == nil || s.runs == nil {
		return []ports.RunNodeSummary{}, nil
	}
	return s.runs.ListNodes(ctx, runID)
}
