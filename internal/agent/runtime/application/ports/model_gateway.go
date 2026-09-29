package ports

import "context"

type ModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelRequest struct {
	Model        string         `json:"model"`
	SystemPrompt string         `json:"system_prompt"`
	Messages     []ModelMessage `json:"messages"`
	Temperature  float64        `json:"temperature"`
	RequestID    string         `json:"request_id"`
}

type ModelResult struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

type ModelGateway interface {
	Generate(context.Context, ModelRequest) (ModelResult, error)
	GenerateStream(context.Context, ModelRequest, func(chunk string) error) error
}
