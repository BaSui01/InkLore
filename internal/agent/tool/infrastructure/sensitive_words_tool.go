package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"InkLore/internal/agent/tool/domain"
)

// SensitiveWordsAuditorTool 中文网文合规与敏感词审计器
type SensitiveWordsAuditorTool struct{}

func NewSensitiveWordsAuditorTool() *SensitiveWordsAuditorTool {
	return &SensitiveWordsAuditorTool{}
}

func (t *SensitiveWordsAuditorTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "sensitive_words_auditor",
		Name:        "中文网文合规与敏感词审计器",
		Description: "对标番茄、起点、晋江主流网文平台合规红线，对章节正文进行违规敏感词全面扫描与替换建议",
		Category:    "compliance",
		Parameters: []domain.ToolParam{
			{Name: "text", Type: "string", Description: "待审查的章节正文文本", Required: true},
		},
	}
}

type SensitiveHit struct {
	Word        string `json:"word"`
	Category    string `json:"category"`
	Level       string `json:"level"`
	Context     string `json:"context"`
	Replacement string `json:"replacement"`
}

func (t *SensitiveWordsAuditorTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	text, _ := input["text"].(string)
	if strings.TrimSpace(text) == "" {
		return domain.ToolResult{
			ToolID:  "sensitive_words_auditor",
			Success: false,
			Summary: "待审查文本为空",
		}, nil
	}

	riskDict := []struct {
		word        string
		category    string
		level       string
		replacement string
	}{
		{word: "虐杀", category: "extreme_violence", level: "high", replacement: "击溃 / 斩退"},
		{word: "分尸", category: "extreme_violence", level: "high", replacement: "重创 / 碎裂"},
		{word: "开膛", category: "extreme_violence", level: "high", replacement: "重伤"},
		{word: "血肉横飞", category: "extreme_violence", level: "medium", replacement: "气劲崩碎 / 气血翻腾"},
		{word: "血祭苍生", category: "taboo", level: "medium", replacement: "聚纳天地煞气"},
		{word: "剥皮炼尸", category: "extreme_violence", level: "high", replacement: "炼化魔煞灵器"},
		{word: "淫笑", category: "vulgar", level: "medium", replacement: "狞笑 / 冷笑"},
		{word: "亵渎", category: "vulgar", level: "low", replacement: "冒犯 / 僭越"},
		{word: "奸邪", category: "vulgar", level: "low", replacement: "狡诈 / 阴鸷"},
	}

	hits := make([]SensitiveHit, 0)
	penalty := 0

	for _, item := range riskDict {
		idx := strings.Index(text, item.word)
		if idx != -1 {
			start := idx - 15
			if start < 0 {
				start = 0
			}
			end := idx + len(item.word) + 15
			if end > len(text) {
				end = len(text)
			}
			snippet := text[start:end]

			if item.level == "high" {
				penalty += 35
			} else if item.level == "medium" {
				penalty += 15
			} else {
				penalty += 5
			}

			hits = append(hits, SensitiveHit{
				Word:        item.word,
				Category:    item.category,
				Level:       item.level,
				Context:     "..." + strings.TrimSpace(snippet) + "...",
				Replacement: item.replacement,
			})
		}
	}

	score := 100 - penalty
	if score < 0 {
		score = 0
	}

	status := "passed"
	var summary string
	if len(hits) == 0 {
		summary = "合规审查完全通过：正文符合各大网文平台主流签约与上架内容安全规范，无违规违禁词。"
	} else if score >= 80 {
		status = "warning"
		summary = fmt.Sprintf("合规预警（得分 %d/100）：检出 %d 处轻中度敏感用词，建议根据推荐词替换后发布。", score, len(hits))
	} else {
		status = "rejected"
		summary = fmt.Sprintf("合规拦截（得分 %d/100）：检出 %d 处高危红线用词，存在平台封书下架风险，必须修订！", score, len(hits))
	}

	suggestions := make([]string, 0)
	for _, h := range hits {
		suggestions = append(suggestions, fmt.Sprintf("建议将短句中【%s】替换为【%s】（%s）", h.Word, h.Replacement, h.Context))
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "正文文风健康合规，叙事张力适度，可以直接排期连载发布。")
	}

	details := map[string]any{
		"compliance_score": score,
		"audit_status":     status,
		"hit_count":        len(hits),
		"hits":             hits,
		"total_words":      utf8.RuneCountInString(text),
	}

	return domain.ToolResult{
		ToolID:      "sensitive_words_auditor",
		Success:     true,
		Score:       score,
		Summary:     summary,
		Details:     details,
		Suggestions: suggestions,
	}, nil
}
