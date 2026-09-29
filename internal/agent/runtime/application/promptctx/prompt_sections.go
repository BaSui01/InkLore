package enginepromptctx

import (
	"strings"

	agentdomain "InkLore/internal/agent/domain"
)

type PromptSection struct {
	Name            string `json:"name"`
	Content         string `json:"content"`
	Priority        int    `json:"priority"`
	Volatile        bool   `json:"volatile,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens,omitempty"`
}

type PromptSectionUsage struct {
	Name            string `json:"name"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Included        bool   `json:"included"`
	Truncated       bool   `json:"truncated,omitempty"`
}

type PromptBuildOptions struct {
	ModelName string
	MaxTokens int
}

type PromptBuildResult struct {
	Prompt string               `json:"prompt"`
	Usage  []PromptSectionUsage `json:"usage"`
}

func BuildPromptFromSections(sections []PromptSection, options PromptBuildOptions) PromptBuildResult {
	parts := make([]string, 0, len(sections))
	usage := make([]PromptSectionUsage, 0, len(sections))
	usedTokens := 0
	for _, section := range sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		tokens := section.EstimatedTokens
		if tokens <= 0 {
			tokens = agentdomain.EstimateTokenCountForModel(content, options.ModelName)
		}
		entry := PromptSectionUsage{Name: section.Name, EstimatedTokens: tokens, Included: true}
		if options.MaxTokens > 0 && usedTokens+tokens > options.MaxTokens {
			entry.Included = false
			entry.Truncated = true
			usage = append(usage, entry)
			continue
		}
		parts = append(parts, content)
		usedTokens += tokens
		usage = append(usage, entry)
	}
	return PromptBuildResult{
		Prompt: strings.Join(parts, "\n\n"),
		Usage:  usage,
	}
}

func Section(name, content string, priority int) PromptSection {
	return PromptSection{Name: name, Content: content, Priority: priority}
}
