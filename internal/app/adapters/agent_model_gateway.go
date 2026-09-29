package adapters

import (
	"context"

	"InkLore/internal/agent/runtime/application/ports"
	llmapp "InkLore/internal/llm/application"
	"InkLore/internal/llm/domain"
)

type AgentModelGateway struct {
	llm *llmapp.GatewayService
}

func NewAgentModelGateway(llm *llmapp.GatewayService) *AgentModelGateway {
	return &AgentModelGateway{llm: llm}
}

func (g *AgentModelGateway) Generate(ctx context.Context, request ports.ModelRequest) (ports.ModelResult, error) {
	messages := make([]domain.Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		messages = append(messages, domain.Message{Role: message.Role, Content: message.Content})
	}
	result, err := g.llm.Generate(ctx, domain.GenerateRequest{
		Model:        request.Model,
		SystemPrompt: request.SystemPrompt,
		Messages:     messages,
		Temperature:  request.Temperature,
		RequestID:    request.RequestID,
	})
	if err != nil {
		return ports.ModelResult{}, err
	}
	return ports.ModelResult{Content: result.Content, Model: result.Model}, nil
}

func (g *AgentModelGateway) GenerateStream(ctx context.Context, request ports.ModelRequest, onChunk func(string) error) error {
	messages := make([]domain.Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		messages = append(messages, domain.Message{Role: message.Role, Content: message.Content})
	}
	return g.llm.GenerateStream(ctx, domain.GenerateRequest{
		Model:        request.Model,
		SystemPrompt: request.SystemPrompt,
		Messages:     messages,
		Temperature:  request.Temperature,
		RequestID:    request.RequestID,
	}, onChunk)
}
