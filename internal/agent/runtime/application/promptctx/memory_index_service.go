package enginepromptctx

import (
	authdomain "InkLore/internal/platform/auth"
	"strings"
	"time"
)

type AgentMemoryIndexService struct {
	ttl time.Duration
}

func NewAgentMemoryIndexService() *AgentMemoryIndexService {
	return &AgentMemoryIndexService{ttl: 5 * time.Minute}
}

func (s *AgentMemoryIndexService) BuildSection(user *authdomain.User, summaries []string) PromptSection {
	items := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		summary = strings.TrimSpace(summary)
		if summary != "" {
			items = append(items, "- "+summary)
		}
	}
	if len(items) == 0 {
		return PromptSection{}
	}
	return PromptSection{
		Name:     "memory_index",
		Content:  "长期记忆索引：\n" + strings.Join(items, "\n"),
		Priority: 45,
	}
}

func (s *AgentMemoryIndexService) BuildPreferenceSection(prefText string) PromptSection {
	prefText = strings.TrimSpace(prefText)
	if prefText == "" {
		return PromptSection{}
	}
	return PromptSection{
		Name:     "user_preferences",
		Content:  prefText,
		Priority: 46,
	}
}
