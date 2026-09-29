package engineloop

import (
	"context"
)

const PromptSectionsOptionKey = "prompt_sections"
const PromptSectionUsageOptionKey = "prompt_section_usage"

type PromptSectionMiddleware struct{}

func NewPromptSectionMiddleware() *PromptSectionMiddleware {
	return &PromptSectionMiddleware{}
}

func (m *PromptSectionMiddleware) BeforeModelCall(ctx context.Context, req *ModelCallContext) error {
	if req == nil || req.GenerationReq == nil || req.GenerationReq.Options == nil {
		return nil
	}
	raw := req.GenerationReq.Options[PromptSectionsOptionKey]
	sections, ok := raw.([]PromptSection)
	if !ok || len(sections) == 0 {
		return nil
	}
	result := BuildPromptFromSections(sections, PromptBuildOptions{ModelName: req.GenerationReq.Model})
	req.GenerationReq.SystemPrompt = result.Prompt
	req.GenerationReq.Options[PromptSectionUsageOptionKey] = result.Usage
	req.GenerationReq.Options[PromptCacheOptionKey] = BuildPromptCacheMetadata(sections, req.Provider)
	return nil
}

func (m *PromptSectionMiddleware) AfterModelCall(ctx context.Context, result *ModelCallResult) error {
	return nil
}
