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
type InspirationGeneration struct {
	llm     *llmapp.GatewayService
	catalog *llmapp.Catalog
}

func NewInspirationGeneration(llm *llmapp.GatewayService, catalog *llmapp.Catalog) *InspirationGeneration {
	return &InspirationGeneration{llm: llm, catalog: catalog}
}

func (g *InspirationGeneration) Generate(ctx context.Context, req inspapp.GenerationRequest) (inspapp.GenerationResponse, error) {
	if g == nil || g.llm == nil || g.catalog == nil {
		return inspapp.GenerationResponse{}, errors.New("inspiration model gateway is not configured")
	}
	content := strings.Join(req.ContextSegments, "\n\n")
	if req.Brief != "" {
		content += "\n\n" + req.Brief
	}
	policy, err := g.catalog.CurrentPolicySnapshot(ctx, req.TenantID, "inspiration")
	if err != nil {
		return inspapp.GenerationResponse{}, err
	}
	response, err := g.llm.GenerateChat(ctx, llmdomain.TextGenerationRequest{
		TenantID:        req.TenantID,
		PolicyVersionID: policy.VersionID,
		ModelID:         policy.ModelID,
		Model:           policy.ModelName,
		Temperature:     policy.Temperature,
		SystemPrompt:    "Return only JSON matching the requested analysis structure. Do not invent evidence offsets.",
		Messages:        []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: content}},
		MaxTokens:       capModelOutputTokens(req.MaxOutputTokens, policy.MaxOutputTokens),
		TaskKey:         "inspiration",
		RequestID:       req.ExecutionKey,
		ExecutionKey:    "inspiration/" + req.ExecutionKey,
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
type ComplianceGeneration struct {
	llm     *llmapp.GatewayService
	catalog *llmapp.Catalog
}

func NewComplianceGeneration(llm *llmapp.GatewayService, catalog *llmapp.Catalog) *ComplianceGeneration {
	return &ComplianceGeneration{llm: llm, catalog: catalog}
}

func (g *ComplianceGeneration) GenerateAudit(ctx context.Context, tenantID, reportID, novelID, chapterID, text string, budget compdomain.Budget) (compapp.AuditLLMResult, error) {
	if g == nil || g.llm == nil || g.catalog == nil {
		return compapp.AuditLLMResult{}, errors.New("compliance model gateway is not configured")
	}
	maxTokens := budget.MaxOutputTokens
	if maxTokens < 1 {
		maxTokens = compdomain.DefaultMaxOutputTokens
	}
	policy, err := g.catalog.CurrentPolicySnapshot(ctx, tenantID, "review")
	if err != nil {
		return compapp.AuditLLMResult{}, err
	}
	response, err := g.llm.GenerateChat(ctx, llmdomain.TextGenerationRequest{
		TenantID:        tenantID,
		PolicyVersionID: policy.VersionID,
		ModelID:         policy.ModelID,
		Model:           policy.ModelName,
		Temperature:     policy.Temperature,
		SystemPrompt:    "Return only JSON with summary and issues. Use character offsets into the supplied text.",
		Messages:        []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: text}},
		MaxTokens:       capModelOutputTokens(maxTokens, policy.MaxOutputTokens),
		TaskKey:         "review",
		RequestID:       chapterID,
		ExecutionKey:    "compliance/audit/" + reportID,
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

func (g *ComplianceGeneration) GenerateRewrite(ctx context.Context, tenantID, rewriteID, novelID, chapterID, text string, issues []compdomain.Issue, round int, budget compdomain.Budget) (compapp.RewriteLLMResult, error) {
	if g == nil || g.llm == nil || g.catalog == nil {
		return compapp.RewriteLLMResult{}, errors.New("compliance model gateway is not configured")
	}
	if rewriteID == "" || len(issues) == 0 || issues[0].ReportID == "" {
		return compapp.RewriteLLMResult{}, errors.New("persisted rewrite and review report are required for model execution")
	}
	issuesJSON, err := json.Marshal(issues)
	if err != nil {
		return compapp.RewriteLLMResult{}, err
	}
	maxTokens := budget.MaxOutputTokens
	if maxTokens < 1 {
		maxTokens = compdomain.DefaultMaxOutputTokens
	}
	policy, err := g.catalog.CurrentPolicySnapshot(ctx, tenantID, "review")
	if err != nil {
		return compapp.RewriteLLMResult{}, err
	}
	response, err := g.llm.GenerateChat(ctx, llmdomain.TextGenerationRequest{
		TenantID:        tenantID,
		PolicyVersionID: policy.VersionID,
		ModelID:         policy.ModelID,
		Model:           policy.ModelName,
		Temperature:     policy.Temperature,
		SystemPrompt:    "Return only JSON with patches. Preserve every patch offset and old_text exactly.",
		Messages:        []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: text + "\n\nIssues:\n" + string(issuesJSON)}},
		MaxTokens:       capModelOutputTokens(maxTokens, policy.MaxOutputTokens),
		TaskKey:         "review",
		RequestID:       chapterID,
		ExecutionKey:    fmt.Sprintf("compliance/rewrite/%s/%d", rewriteID, round),
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

// capModelOutputTokens never lets a domain budget override the tenant policy limit.
func capModelOutputTokens(requested, limit int) int {
	if requested > 0 && requested < limit {
		return requested
	}
	return limit
}
