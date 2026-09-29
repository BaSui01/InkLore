package adapters

import (
	"context"
	"fmt"

	llmapp "InkLore/internal/llm/application"
	llmdomain "InkLore/internal/llm/domain"
	llmgateway "InkLore/internal/llm/domain/gateway"
)

// ModelExecutionGateway 把 LLM 域的对话补全能力适配为智能体 TAO 循环所需的执行网关。
//
// 该适配器是 agent 域与 llm 域之间的唯一桥接点（见 AGENTS.md 跨域调用红线）：
// agent 侧只依赖 llm 域暴露的 domain 契约，具体实现由组合根注入。
type ModelExecutionGateway struct {
	llm *llmapp.GatewayService
}

// NewModelExecutionGateway 构造适配器。
func NewModelExecutionGateway(llm *llmapp.GatewayService) *ModelExecutionGateway {
	return &ModelExecutionGateway{llm: llm}
}

// Execute 执行一次不带流式的对话补全，完整保留 tool_calls 与 tool_call_id。
func (g *ModelExecutionGateway) Execute(ctx context.Context, req llmgateway.ExecutionRequest) (*llmgateway.ExecutionResult, error) {
	if g == nil || g.llm == nil {
		return nil, fmt.Errorf("模型执行网关未配置")
	}
	if req.Text == nil {
		return nil, fmt.Errorf("文本执行请求缺少 text 负载")
	}
	response, err := g.llm.GenerateChat(ctx, toTextGenerationRequest(req))
	if err != nil {
		return nil, err
	}
	return &llmgateway.ExecutionResult{Content: response.Content}, nil
}

// ExecuteTextStream 执行一轮 TAO 思考（Think）。
//
// 关键契约：TAO 循环把原始的 *llmdomain.TextGenerationRequest 通过 Text.Raw 传入，
// 其中挂着 OnReasoningChunk / OnTextChunk / OnToolCalls / OnUsage 四个回调。
// 这是工具调用与用量回传的**唯一**通道 —— TextStreamRequest.OnChunk 只能传文本，
// 无法承载 tool_calls。因此本方法必须优先消费 Raw 回调，且不得同时调用
// TextStreamRequest.OnChunk，否则正文会被重复累加。
func (g *ModelExecutionGateway) ExecuteTextStream(ctx context.Context, req llmgateway.TextStreamRequest) error {
	if g == nil || g.llm == nil {
		return fmt.Errorf("模型执行网关未配置")
	}
	if req.Text == nil {
		return fmt.Errorf("文本流式请求缺少 text 负载")
	}

	if raw, ok := req.Text.Raw.(*llmdomain.TextGenerationRequest); ok && raw != nil {
		return g.executeRawTurn(ctx, req, *raw)
	}

	// 无 Raw 上下文（非 TAO 调用方）：退化为普通文本流。
	err := g.llm.GenerateStream(ctx, llmdomain.GenerateRequest{
		Model:        req.Model,
		SystemPrompt: req.Text.SystemPrompt,
		Messages:     toSimpleMessages(req.Text.Messages),
		Temperature:  req.Text.Temperature,
		RequestID:    req.AuditRef.RequestID,
	}, func(chunk string) error {
		if req.OnChunk == nil {
			return nil
		}
		return req.OnChunk(chunk, false)
	})
	if err != nil {
		return err
	}
	if req.OnChunk != nil {
		return req.OnChunk("", true)
	}
	return nil
}

func (g *ModelExecutionGateway) executeRawTurn(ctx context.Context, req llmgateway.TextStreamRequest, raw llmdomain.TextGenerationRequest) error {
	if raw.Model == "" {
		raw.Model = req.Model
	}
	// 审计关联：TAO 循环传入的 Raw 通常不带 RequestID，这里从审计引用补齐（INK-018）。
	if raw.RequestID == "" {
		raw.RequestID = req.AuditRef.RequestID
	}
	if raw.OnTextChunk == nil && req.OnChunk != nil {
		raw.OnTextChunk = func(chunk string) error {
			return req.OnChunk(chunk, false)
		}
	}
	return g.llm.GenerateChatStream(ctx, raw)
}

func toTextGenerationRequest(req llmgateway.ExecutionRequest) llmdomain.TextGenerationRequest {
	text := req.Text
	request := llmdomain.TextGenerationRequest{
		Model:        req.Model,
		RequestID:    req.AuditRef.RequestID,
		SystemPrompt: text.SystemPrompt,
		Temperature:  text.Temperature,
		TopP:         text.TopP,
		MaxTokens:    text.MaxTokens,
		ToolChoice:   text.ToolChoice,
		Options:      text.Options,
	}
	for _, message := range text.Messages {
		converted := llmdomain.ChatMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
		}
		for _, call := range message.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, llmdomain.ToolCall{
				ID:   call.ID,
				Type: call.Type,
				Function: llmdomain.ToolCallFunction{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
		}
		request.Messages = append(request.Messages, converted)
	}
	for _, tool := range text.Tools {
		request.Tools = append(request.Tools, llmdomain.Tool{
			Type: tool.Type,
			Function: llmdomain.ToolFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return request
}

func toSimpleMessages(messages []llmgateway.TextMessage) []llmdomain.Message {
	converted := make([]llmdomain.Message, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, llmdomain.Message{Role: message.Role, Content: message.Content})
	}
	return converted
}
