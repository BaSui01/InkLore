package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

// --- Google Gemini generateContent Native Protocol ---

type geminiPart struct {
	Text         string            `json:"text,omitempty"`
	Thought      bool              `json:"thought,omitempty"`
	InlineData   *geminiInlineData `json:"inlineData,omitempty"`
	FunctionCall *geminiFunction   `json:"functionCall,omitempty"`
	FunctionResp *geminiFunction   `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiFunction struct {
	Name     string         `json:"name"`
	Args     map[string]any `json:"args,omitempty"`
	Response map[string]any `json:"response,omitempty"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *struct {
		Mode string `json:"mode,omitempty"`
	} `json:"functionCallingConfig,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *struct {
		Parts []geminiPart `json:"parts"`
	} `json:"systemInstruction,omitempty"`
	Contents []struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	} `json:"contents"`
	Tools []struct {
		FunctionDeclarations []struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters,omitempty"`
		} `json:"function_declarations"`
	} `json:"tools,omitempty"`
	ToolConfig       *geminiToolConfig     `json:"toolConfig,omitempty"`
	SafetySettings   []geminiSafetySetting `json:"safetySettings,omitempty"`
	GenerationConfig struct {
		Temperature      float64  `json:"temperature,omitempty"`
		TopP             float64  `json:"topP,omitempty"`
		PresencePenalty  float64  `json:"presencePenalty,omitempty"`
		FrequencyPenalty float64  `json:"frequencyPenalty,omitempty"`
		Seed             *int     `json:"seed,omitempty"`
		ResponseMimeType string   `json:"responseMimeType,omitempty"`
		ResponseSchema   any      `json:"responseSchema,omitempty"`
		MaxOutputTokens  int      `json:"maxOutputTokens,omitempty"`
		StopSequences    []string `json:"stopSequences,omitempty"`
		ThinkingConfig   *struct {
			IncludeThoughts bool   `json:"includeThoughts"`
			ThinkingBudget  int    `json:"thinkingBudget,omitempty"`
			ThinkingLevel   string `json:"thinkingLevel,omitempty"`
		} `json:"thinkingConfig,omitempty"`
	} `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Usage struct {
		Prompt             int `json:"promptTokenCount"`
		Output             int `json:"candidatesTokenCount"`
		Total              int `json:"totalTokenCount"`
		ThoughtsTokenCount int `json:"thoughtsTokenCount,omitempty"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func toGeminiToolConfig(choice any) *geminiToolConfig {
	if choice == nil {
		return nil
	}
	mode := "AUTO"
	if s, ok := choice.(string); ok {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "auto":
			mode = "AUTO"
		case "required", "any":
			mode = "ANY"
		case "none":
			mode = "NONE"
		default:
			return nil
		}
	}
	return &geminiToolConfig{
		FunctionCallingConfig: &struct {
			Mode string `json:"mode,omitempty"`
		}{Mode: mode},
	}
}

func toGeminiRequest(req nativeTextRequest, stream bool) geminiRequest {
	result := geminiRequest{}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		result.SystemInstruction = &struct {
			Parts []geminiPart `json:"parts"`
		}{Parts: []geminiPart{{Text: req.SystemPrompt}}}
	}

	for _, message := range req.Messages {
		role := "user"
		if message.Role == domain.ChatMessageRoleAssistant {
			role = "model"
		}
		parts := []geminiPart{}
		if message.Role == domain.ChatMessageRoleTool {
			name := message.Name
			if name == "" {
				name = "function_response"
			}
			parts = append(parts, geminiPart{FunctionResp: &geminiFunction{
				Name:     name,
				Response: map[string]any{"content": message.Content},
			}})
			if len(result.Contents) > 0 && result.Contents[len(result.Contents)-1].Role == "user" {
				result.Contents[len(result.Contents)-1].Parts = append(result.Contents[len(result.Contents)-1].Parts, parts...)
				continue
			}
		} else if strings.TrimSpace(message.Content) != "" {
			parts = append(parts, geminiPart{Text: message.Content})
		}

		for _, call := range message.ToolCalls {
			args := map[string]any{}
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
			parts = append(parts, geminiPart{FunctionCall: &geminiFunction{Name: call.Function.Name, Args: args}})
		}
		result.Contents = append(result.Contents, struct {
			Role  string       `json:"role"`
			Parts []geminiPart `json:"parts"`
		}{Role: role, Parts: parts})
	}

	for _, tool := range req.Tools {
		declaration := struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters,omitempty"`
		}{Name: tool.Function.Name, Description: tool.Function.Description, Parameters: tool.Function.Parameters}
		result.Tools = append(result.Tools, struct {
			FunctionDeclarations []struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				Parameters  any    `json:"parameters,omitempty"`
			} `json:"function_declarations"`
		}{FunctionDeclarations: []struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters,omitempty"`
		}{declaration}})
	}

	result.ToolConfig = toGeminiToolConfig(req.ToolChoice)
	result.GenerationConfig.Temperature = req.Temperature
	result.GenerationConfig.TopP = req.TopP
	result.GenerationConfig.PresencePenalty = req.PresencePenalty
	result.GenerationConfig.FrequencyPenalty = req.FrequencyPenalty
	result.GenerationConfig.Seed = req.Seed
	result.GenerationConfig.MaxOutputTokens = req.MaxTokens
	result.GenerationConfig.StopSequences = req.Stop
	if req.ResponseFormat != nil {
		if req.ResponseFormat.Type == domain.ResponseFormatJSONSchema && req.ResponseFormat.JSONSchema != nil {
			result.GenerationConfig.ResponseMimeType = "application/json"
			result.GenerationConfig.ResponseSchema = req.ResponseFormat.JSONSchema.Schema
		} else if req.ResponseFormat.Type == domain.ResponseFormatJSONObject {
			result.GenerationConfig.ResponseMimeType = "application/json"
		}
	}

	shouldThink := (req.ThinkingEnabled != nil && *req.ThinkingEnabled) ||
		strings.Contains(req.Model, "thinking") || strings.Contains(req.Model, "gemini-2.5") || strings.Contains(req.Model, "gemini-3")
	if shouldThink {
		budget := -1
		switch strings.ToLower(strings.TrimSpace(req.ReasoningEffort)) {
		case "minimal":
			budget = 1024
		case "low":
			budget = 2048
		case "medium":
			budget = 8192
		case "high", "max":
			budget = 16384
		}
		result.GenerationConfig.ThinkingConfig = &struct {
			IncludeThoughts bool   `json:"includeThoughts"`
			ThinkingBudget  int    `json:"thinkingBudget,omitempty"`
			ThinkingLevel   string `json:"thinkingLevel,omitempty"`
		}{
			IncludeThoughts: true,
			ThinkingBudget:  budget,
			ThinkingLevel:   req.ReasoningEffort,
		}
	}
	return result
}

func (r *ProviderRouter) geminiChat(ctx context.Context, target upstreamTarget, req nativeTextRequest) (domain.TextGenerationResponse, error) {
	body, err := json.Marshal(toGeminiRequest(req, false))
	if err != nil {
		return domain.TextGenerationResponse{}, err
	}
	var decoded geminiResponse
	base := strings.TrimRight(target.baseURL, "/")
	prefix := ""
	if !strings.HasSuffix(base, "/v1beta") && !strings.HasSuffix(base, "/v1") {
		prefix = "/v1beta"
	}
	path := prefix + "/models/" + url.PathEscape(req.Model) + ":generateContent?key=" + url.QueryEscape(target.apiKey)
	if _, err = r.doJSON(ctx, target, path, body, func(request *http.Request) {
		request.Header.Set("x-goog-api-key", target.apiKey)
	}, &decoded); err != nil {
		return domain.TextGenerationResponse{}, err
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return domain.TextGenerationResponse{}, platformerr.New(http.StatusInternalServerError, "upstream_error", decoded.Error.Message)
	}
	return geminiResult(decoded, req.Model), nil
}

func geminiResult(decoded geminiResponse, requestedModel string) domain.TextGenerationResponse {
	result := domain.TextGenerationResponse{
		Model: requestedModel,
		Usage: &domain.TokenUsage{
			PromptTokens:     decoded.Usage.Prompt,
			CompletionTokens: decoded.Usage.Output,
			TotalTokens:      decoded.Usage.Total,
			ReasoningTokens:  decoded.Usage.ThoughtsTokenCount,
			InputTokens:      decoded.Usage.Prompt,
			OutputTokens:     decoded.Usage.Output,
		},
	}
	if len(decoded.Candidates) == 0 {
		return result
	}
	result.FinishReason = strings.ToLower(decoded.Candidates[0].FinishReason)
	for index, part := range decoded.Candidates[0].Content.Parts {
		if part.Thought {
			result.Thinking += part.Text
		} else {
			result.Content += part.Text
		}
		if part.FunctionCall != nil {
			arguments, _ := json.Marshal(part.FunctionCall.Args)
			result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
				ID:   fmt.Sprintf("gemini_call_%d", index),
				Type: "function",
				Function: domain.ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: string(arguments),
				},
			})
		}
	}
	return result
}

func (r *ProviderRouter) geminiStream(ctx context.Context, target upstreamTarget, req nativeTextRequest, onEvent domain.StreamEventHandler) error {
	body, err := json.Marshal(toGeminiRequest(req, true))
	if err != nil {
		return err
	}
	base := strings.TrimRight(target.baseURL, "/")
	prefix := ""
	if !strings.HasSuffix(base, "/v1beta") && !strings.HasSuffix(base, "/v1") {
		prefix = "/v1beta"
	}
	path := prefix + "/models/" + url.PathEscape(req.Model) + ":streamGenerateContent?alt=sse&key=" + url.QueryEscape(target.apiKey)
	usage := domain.GenerationUsage{}
	toolIndex := 0

	err = r.nativeSSE(ctx, target, path, body, func(request *http.Request) {
		request.Header.Set("x-goog-api-key", target.apiKey)
	}, func(_ string, data []byte) error {
		var payload geminiResponse
		if json.Unmarshal(data, &payload) != nil {
			return nil
		}
		if payload.Error != nil && payload.Error.Message != "" {
			return platformerr.New(http.StatusInternalServerError, "upstream_stream_error", payload.Error.Message)
		}
		if len(payload.Candidates) > 0 {
			for _, part := range payload.Candidates[0].Content.Parts {
				if part.Thought && part.Text != "" {
					if err := onEvent(domain.StreamEvent{Type: domain.StreamEventThink, Delta: part.Text}); err != nil {
						return err
					}
				} else if part.Text != "" {
					if err := onEvent(domain.StreamEvent{Type: domain.StreamEventMessage, Delta: part.Text}); err != nil {
						return err
					}
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					toolIndex++
					_ = onEvent(domain.StreamEvent{
						Type:       domain.StreamEventToolCall,
						ToolCallID: fmt.Sprintf("gemini_call_%d", toolIndex),
						ToolName:   part.FunctionCall.Name,
						ToolArgs:   string(args),
					})
				}
			}
		}
		if payload.Usage.Prompt > 0 {
			usage.InputTokens = payload.Usage.Prompt
		}
		if payload.Usage.Output > 0 {
			usage.OutputTokens = payload.Usage.Output
		}
		return nil
	})
	if err == nil {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		err = onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
	}
	return err
}

// --- Google Gemini Interactions API Protocol (POST /v1beta/interactions) ---

type geminiInteractionsInputItem struct {
	Type      string `json:"type,omitempty"`
	Role      string `json:"role,omitempty"`
	Text      string `json:"text,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments any    `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type geminiInteractionsTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type geminiInteractionsRequest struct {
	Model                 string                        `json:"model"`
	Input                 []geminiInteractionsInputItem `json:"input"`
	SystemInstruction     string                        `json:"system_instruction,omitempty"`
	Tools                 []geminiInteractionsTool      `json:"tools,omitempty"`
	ResponseFormat        any                           `json:"response_format,omitempty"`
	ResponseMimeType      string                        `json:"response_mime_type,omitempty"`
	PreviousInteractionID string                        `json:"previous_interaction_id,omitempty"`
	Stream                bool                          `json:"stream,omitempty"`
}

type geminiInteractionsResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Outputs []struct {
		Type      string `json:"type"`
		Text      string `json:"text,omitempty"`
		CallID    string `json:"call_id,omitempty"`
		Name      string `json:"name,omitempty"`
		Arguments any    `json:"arguments,omitempty"`
	} `json:"outputs"`
	Steps []struct {
		Type    string `json:"type"`
		Content []struct {
			Type      string `json:"type"`
			Text      string `json:"text,omitempty"`
			CallID    string `json:"call_id,omitempty"`
			Name      string `json:"name,omitempty"`
			Arguments any    `json:"arguments,omitempty"`
		} `json:"content"`
	} `json:"steps"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code,omitempty"`
	} `json:"error,omitempty"`
}

func toGeminiInteractionsInput(req nativeTextRequest) []geminiInteractionsInputItem {
	result := make([]geminiInteractionsInputItem, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		if role == domain.ChatMessageRoleAssistant {
			role = "model"
		}
		if m.Role == domain.ChatMessageRoleTool {
			result = append(result, geminiInteractionsInputItem{
				Type:   "function_call_output",
				CallID: m.ToolCallID,
				Output: m.Content,
			})
			continue
		}
		if strings.TrimSpace(m.Content) != "" {
			result = append(result, geminiInteractionsInputItem{
				Type: "text",
				Role: role,
				Text: m.Content,
			})
		}
		for _, call := range m.ToolCalls {
			var args any
			if json.Unmarshal([]byte(call.Function.Arguments), &args) != nil {
				args = map[string]any{}
			}
			result = append(result, geminiInteractionsInputItem{
				Type:      "function_call",
				CallID:    call.ID,
				Name:      call.Function.Name,
				Arguments: args,
			})
		}
	}
	return result
}

func toGeminiInteractionsTools(tools []domain.Tool) []geminiInteractionsTool {
	if len(tools) == 0 {
		return nil
	}
	res := make([]geminiInteractionsTool, 0, len(tools))
	for _, t := range tools {
		res = append(res, geminiInteractionsTool{
			Type:        "function",
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	return res
}

func (r *ProviderRouter) geminiInteractionsChat(ctx context.Context, target upstreamTarget, req nativeTextRequest) (domain.TextGenerationResponse, error) {
	mimeType := ""
	var responseFormat any
	if req.ResponseFormat != nil {
		if req.ResponseFormat.Type == domain.ResponseFormatJSONSchema && req.ResponseFormat.JSONSchema != nil {
			mimeType = "application/json"
			responseFormat = req.ResponseFormat.JSONSchema.Schema
		} else if req.ResponseFormat.Type == domain.ResponseFormatJSONObject {
			mimeType = "application/json"
		}
	}

	body, err := json.Marshal(geminiInteractionsRequest{
		Model:             req.Model,
		Input:             toGeminiInteractionsInput(req),
		SystemInstruction: req.SystemPrompt,
		Tools:             toGeminiInteractionsTools(req.Tools),
		ResponseFormat:    responseFormat,
		ResponseMimeType:  mimeType,
		Stream:            false,
	})
	if err != nil {
		return domain.TextGenerationResponse{}, err
	}

	base := strings.TrimRight(target.baseURL, "/")
	prefix := ""
	if !strings.HasSuffix(base, "/v1beta") && !strings.HasSuffix(base, "/v1") {
		prefix = "/v1beta"
	}
	path := prefix + "/interactions?key=" + url.QueryEscape(target.apiKey)
	var decoded geminiInteractionsResponse
	if _, err = r.doJSON(ctx, target, path, body, func(request *http.Request) {
		request.Header.Set("x-goog-api-key", target.apiKey)
	}, &decoded); err != nil {
		return domain.TextGenerationResponse{}, err
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return domain.TextGenerationResponse{}, platformerr.New(http.StatusInternalServerError, "upstream_error", decoded.Error.Message)
	}

	result := domain.TextGenerationResponse{
		Model: decoded.Model,
		Usage: &domain.TokenUsage{
			PromptTokens:     decoded.Usage.PromptTokens,
			CompletionTokens: decoded.Usage.CompletionTokens,
			TotalTokens:      decoded.Usage.TotalTokens,
			InputTokens:      decoded.Usage.PromptTokens,
			OutputTokens:     decoded.Usage.CompletionTokens,
		},
	}
	if result.Model == "" {
		result.Model = req.Model
	}

	for _, out := range decoded.Outputs {
		switch out.Type {
		case "thought", "reasoning":
			result.Thinking += out.Text
		case "text":
			result.Content += out.Text
		case "function_call":
			argsBytes, _ := json.Marshal(out.Arguments)
			result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
				ID:   out.CallID,
				Type: "function",
				Function: domain.ToolCallFunction{
					Name:      out.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	if len(result.ToolCalls) == 0 && len(result.Content) == 0 {
		for _, step := range decoded.Steps {
			for _, c := range step.Content {
				switch c.Type {
				case "thought", "reasoning":
					result.Thinking += c.Text
				case "text":
					result.Content += c.Text
				case "function_call":
					argsBytes, _ := json.Marshal(c.Arguments)
					result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
						ID:   c.CallID,
						Type: "function",
						Function: domain.ToolCallFunction{
							Name:      c.Name,
							Arguments: string(argsBytes),
						},
					})
				}
			}
		}
	}

	return result, nil
}

func (r *ProviderRouter) geminiInteractionsStream(ctx context.Context, target upstreamTarget, req nativeTextRequest, onEvent domain.StreamEventHandler) error {
	mimeType := ""
	var responseFormat any
	if req.ResponseFormat != nil {
		if req.ResponseFormat.Type == domain.ResponseFormatJSONSchema && req.ResponseFormat.JSONSchema != nil {
			mimeType = "application/json"
			responseFormat = req.ResponseFormat.JSONSchema.Schema
		} else if req.ResponseFormat.Type == domain.ResponseFormatJSONObject {
			mimeType = "application/json"
		}
	}

	body, err := json.Marshal(geminiInteractionsRequest{
		Model:             req.Model,
		Input:             toGeminiInteractionsInput(req),
		SystemInstruction: req.SystemPrompt,
		Tools:             toGeminiInteractionsTools(req.Tools),
		ResponseFormat:    responseFormat,
		ResponseMimeType:  mimeType,
		Stream:            true,
	})
	if err != nil {
		return err
	}

	base := strings.TrimRight(target.baseURL, "/")
	prefix := ""
	if !strings.HasSuffix(base, "/v1beta") && !strings.HasSuffix(base, "/v1") {
		prefix = "/v1beta"
	}
	path := prefix + "/interactions?alt=sse&key=" + url.QueryEscape(target.apiKey)
	usage := domain.GenerationUsage{}
	doneSent := false
	toolCallArgs := make(map[string]*strings.Builder)

	err = r.nativeSSE(ctx, target, path, body, func(request *http.Request) {
		request.Header.Set("x-goog-api-key", target.apiKey)
	}, func(event string, data []byte) error {
		if event == "error" {
			var errPayload struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &errPayload)
			msg := errPayload.Error.Message
			if msg == "" {
				msg = string(data)
			}
			return platformerr.New(http.StatusInternalServerError, "upstream_stream_error", msg)
		}

		var payload struct {
			Delta struct {
				Type      string `json:"type"`
				Text      string `json:"text"`
				CallID    string `json:"call_id"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"delta"`
			Response geminiInteractionsResponse `json:"response"`
		}
		if json.Unmarshal(data, &payload) != nil {
			return nil
		}

		if payload.Delta.Type == "thought" || payload.Delta.Type == "reasoning" {
			if payload.Delta.Text != "" {
				_ = onEvent(domain.StreamEvent{Type: domain.StreamEventThink, Delta: payload.Delta.Text})
			}
		} else if payload.Delta.Type == "text" || (payload.Delta.Type == "" && payload.Delta.Text != "") {
			if payload.Delta.Text != "" {
				_ = onEvent(domain.StreamEvent{Type: domain.StreamEventMessage, Delta: payload.Delta.Text})
			}
		} else if payload.Delta.Type == "function_call" {
			if payload.Delta.CallID != "" {
				b, exists := toolCallArgs[payload.Delta.CallID]
				if !exists {
					b = &strings.Builder{}
					toolCallArgs[payload.Delta.CallID] = b
				}
				b.WriteString(payload.Delta.Arguments)
			}
		}

		if event == "step.delta" && payload.Delta.Text != "" {
			_ = onEvent(domain.StreamEvent{Type: domain.StreamEventMessage, Delta: payload.Delta.Text})
		}

		if event == "interaction.output_item.done" && payload.Delta.CallID != "" {
			args := ""
			if b, exists := toolCallArgs[payload.Delta.CallID]; exists {
				args = b.String()
			}
			_ = onEvent(domain.StreamEvent{
				Type:       domain.StreamEventToolCall,
				ToolCallID: payload.Delta.CallID,
				ToolName:   payload.Delta.Name,
				ToolArgs:   args,
			})
		}

		if event == "interaction.completed" {
			usage = domain.GenerationUsage{
				InputTokens:  payload.Response.Usage.PromptTokens,
				OutputTokens: payload.Response.Usage.CompletionTokens,
				TotalTokens:  payload.Response.Usage.TotalTokens,
			}
			doneSent = true
			return onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
		}
		return nil
	})

	if err == nil && !doneSent {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		err = onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
	}
	return err
}
