package enginepromptctx

import "strings"

const (
	PromptCacheOptionKey      = "prompt_cache"
	PromptCacheBreakpointsKey = "prompt_cache_breakpoints"
)

type PromptCacheBreakpoint struct {
	SectionName  string `json:"section_name"`
	CacheControl string `json:"cache_control"`
	Stable       bool   `json:"stable"`
}

type PromptCacheMetadata struct {
	Provider    string                  `json:"provider,omitempty"`
	Breakpoints []PromptCacheBreakpoint `json:"breakpoints,omitempty"`
	Enabled     bool                    `json:"enabled"`
}

func BuildPromptCacheMetadata(sections []PromptSection, provider string) PromptCacheMetadata {
	meta := PromptCacheMetadata{
		Enabled:  true,
		Provider: strings.TrimSpace(provider),
	}
	for _, section := range sections {
		name := strings.TrimSpace(section.Name)
		content := strings.TrimSpace(section.Content)
		if name == "" || content == "" || section.Volatile {
			continue
		}
		meta.Breakpoints = append(meta.Breakpoints, PromptCacheBreakpoint{
			SectionName:  name,
			CacheControl: "ephemeral",
			Stable:       true,
		})
	}
	return meta
}
