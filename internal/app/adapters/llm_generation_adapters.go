package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	compapp "InkLore/internal/compliance/application"
	compdomain "InkLore/internal/compliance/domain"
	inspapp "InkLore/internal/inspiration/application"
	inspdomain "InkLore/internal/inspiration/domain"
	llmapp "InkLore/internal/llm/application"
	llmdomain "InkLore/internal/llm/domain"

	"github.com/google/uuid"
)

// InspirationGeneration uses the configured model gateway and decodes the
// provider response into the inspiration domain's report contract.
type InspirationGeneration struct{ llm *llmapp.GatewayService }

func NewInspirationGeneration(llm *llmapp.GatewayService) *InspirationGeneration {
	return &InspirationGeneration{llm: llm}
}

func (g *InspirationGeneration) Generate(ctx context.Context, req inspapp.GenerationRequest) (inspapp.GenerationResponse, error) {
	if g == nil || g.llm == nil {
		return inspapp.GenerationResponse{}, errors.New("inspiration model gateway is not configured")
	}
	content := strings.Join(req.ContextSegments, "\n\n")
	if req.Brief != "" {
		content += "\n\n" + req.Brief
	}
	response, err := g.llm.GenerateChat(ctx, llmdomain.TextGenerationRequest{
		SystemPrompt: "Return only JSON matching the requested analysis structure. Do not invent evidence offsets.",
		Messages:     []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: content}},
		MaxTokens:    req.MaxOutputTokens,
		TaskKey:      "inspiration",
		RequestID:    req.ExecutionKey,
		ResponseFormat: llmdomain.NewJSONSchemaResponseFormat("inspiration_report", true, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary":       map[string]any{"type": "string"},
				"findings":      map[string]any{"type": "array"},
				"patterns":      map[string]any{"type": "array"},
				"style_metrics": map[string]any{"type": []string{"object", "null"}},
				"ideas":         map[string]any{"type": "array"},
			},
			"required":             []string{"summary", "findings", "patterns", "ideas"},
			"additionalProperties": false,
		}),
	})
	if err != nil {
		return inspapp.GenerationResponse{}, err
	}
	raw, err := llmdomain.ExtractJSON(response.Content)
	if err != nil {
		return inspapp.GenerationResponse{}, fmt.Errorf("decode inspiration response: %w", err)
	}
	var payload struct {
		Summary      string                   `json:"summary"`
		Findings     []inspdomain.Finding     `json:"findings"`
		Patterns     []inspdomain.Pattern     `json:"patterns"`
		StyleMetrics *inspdomain.StyleMetrics `json:"style_metrics"`
		Ideas        []inspdomain.Idea        `json:"ideas"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return inspapp.GenerationResponse{}, fmt.Errorf("decode inspiration report: %w", err)
	}
	return inspapp.GenerationResponse{
		Summary:      payload.Summary,
		Findings:     payload.Findings,
		Patterns:     payload.Patterns,
		StyleMetrics: payload.StyleMetrics,
		Ideas:        payload.Ideas,
		UsedTokens:   responseTokens(response.Usage),
		CallsUsed:    1,
	}, nil
}

// ComplianceGeneration keeps rule matching authoritative while allowing the
// configured provider to add model-backed review findings and rewrite patches.
type ComplianceGeneration struct{ llm *llmapp.GatewayService }

func NewComplianceGeneration(llm *llmapp.GatewayService) *ComplianceGeneration {
	return &ComplianceGeneration{llm: llm}
}

func (g *ComplianceGeneration) GenerateAudit(ctx context.Context, tenantID, reportID, novelID, chapterID, text string, budget compdomain.Budget) (compapp.AuditLLMResult, error) {
	if g == nil || g.llm == nil {
		return compapp.AuditLLMResult{}, errors.New("compliance model gateway is not configured")
	}
	maxTokens := budget.MaxOutputTokens
	if maxTokens < 1 {
		maxTokens = compdomain.DefaultMaxOutputTokens
	}
	response, err := g.llm.GenerateChat(ctx, llmdomain.TextGenerationRequest{
		SystemPrompt: "Return only JSON with summary and issues. Use character offsets into the supplied text.",
		Messages:     []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: text}},
		MaxTokens:    maxTokens,
		TaskKey:      "review",
		RequestID:    chapterID,
	})
	if err != nil {
		return compapp.AuditLLMResult{}, err
	}
	var payload struct {
		Summary string             `json:"summary"`
		Issues  []compdomain.Issue `json:"issues"`
	}
	if err := decodeModelJSON(response.Content, &payload); err != nil {
		return compapp.AuditLLMResult{}, err
	}
	for i := range payload.Issues {
		if payload.Issues[i].ID == "" {
			payload.Issues[i].ID = uuid.NewString()
		}
		payload.Issues[i].TenantID = tenantID
		payload.Issues[i].ReportID = reportID
		payload.Issues[i].Detector = compdomain.DetectorLLM
		if payload.Issues[i].Revision == 0 {
			payload.Issues[i].Revision = 1
		}
		if payload.Issues[i].State == "" {
			payload.Issues[i].State = compdomain.IssueStateOpen
		}
	}
	return compapp.AuditLLMResult{Summary: payload.Summary, Issues: payload.Issues, UsedTokens: responseTokens(response.Usage), CallsUsed: 1}, nil
}

func (g *ComplianceGeneration) GenerateRewrite(ctx context.Context, tenantID, novelID, chapterID, text string, issues []compdomain.Issue, round int, budget compdomain.Budget) (compapp.RewriteLLMResult, error) {
	if g == nil || g.llm == nil {
		return compapp.RewriteLLMResult{}, errors.New("compliance model gateway is not configured")
	}
	issuesJSON, err := json.Marshal(issues)
	if err != nil {
		return compapp.RewriteLLMResult{}, err
	}
	maxTokens := budget.MaxOutputTokens
	if maxTokens < 1 {
		maxTokens = compdomain.DefaultMaxOutputTokens
	}
	response, err := g.llm.GenerateChat(ctx, llmdomain.TextGenerationRequest{
		SystemPrompt: "Return only JSON with patches. Preserve every patch offset and old_text exactly.",
		Messages:     []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: text + "\n\nIssues:\n" + string(issuesJSON)}},
		MaxTokens:    maxTokens,
		TaskKey:      "review",
		RequestID:    chapterID,
	})
	if err != nil {
		return compapp.RewriteLLMResult{}, err
	}
	var payload struct {
		Patches []compdomain.Patch `json:"patches"`
	}
	if err := decodeModelJSON(response.Content, &payload); err != nil {
		return compapp.RewriteLLMResult{}, err
	}
	return compapp.RewriteLLMResult{Patches: payload.Patches, UsedTokens: responseTokens(response.Usage), CallsUsed: 1}, nil
}

func decodeModelJSON(content string, out any) error {
	raw, err := llmdomain.ExtractJSON(content)
	if err != nil {
		return fmt.Errorf("decode model JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("decode model payload: %w", err)
	}
	return nil
}

func responseTokens(usage *llmdomain.TokenUsage) int {
	if usage == nil {
		return 0
	}
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	return usage.PromptTokens + usage.CompletionTokens
}
