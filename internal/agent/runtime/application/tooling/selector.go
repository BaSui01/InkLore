package enginetooling

import (
	"sort"
	"strings"
	"unicode"

	ai "InkLore/internal/llm/domain"
)

const (
	// DefaultDynamicToolThreshold 当工具总数超过此阈值时启动动态过滤，保护 LLM 上下文
	DefaultDynamicToolThreshold = 8
	// DefaultDynamicToolTopK 动态召回的最大候选工具数
	DefaultDynamicToolTopK = 6
)

var defaultCoreToolNames = map[string]struct{}{
	"ask_user":     {},
	"search_tools": {},
}

// ToolScore 工具评分项
type ToolScore struct {
	Tool  ToolEntry
	Score float64
	Name  string
}

// ToolSelector 动态工具检索选择器 (Tool RAG)
type ToolSelector struct {
	CoreTools map[string]struct{}
	Threshold int
	TopK      int
}

// NewToolSelector 创建默认动态工具选择器
func NewToolSelector() *ToolSelector {
	cores := make(map[string]struct{}, len(defaultCoreToolNames))
	for k, v := range defaultCoreToolNames {
		cores[k] = v
	}
	return &ToolSelector{
		CoreTools: cores,
		Threshold: DefaultDynamicToolThreshold,
		TopK:      DefaultDynamicToolTopK,
	}
}

// SelectTools 根据当前意图与上下文，从全量可用工具中自适应筛选最相关工具集
func (s *ToolSelector) SelectTools(tools []ToolEntry, userQuery string) []ToolEntry {
	if len(tools) == 0 {
		return tools
	}
	threshold := s.Threshold
	if threshold <= 0 {
		threshold = DefaultDynamicToolThreshold
	}
	// 工具总量未超过阈值，直接全量返回，零额外开销
	if len(tools) <= threshold {
		return tools
	}

	topK := s.TopK
	if topK <= 0 {
		topK = DefaultDynamicToolTopK
	}

	queryTokens := extractQueryTokens(userQuery)
	var coreSelected []ToolEntry
	var dynamicScores []ToolScore

	for _, t := range tools {
		name := strings.TrimSpace(t.Definition.Function.Name)
		if _, isCore := s.CoreTools[name]; isCore {
			coreSelected = append(coreSelected, t)
			continue
		}

		score := calculateToolScore(t.Definition, queryTokens)
		dynamicScores = append(dynamicScores, ToolScore{
			Tool:  t,
			Score: score,
			Name:  name,
		})
	}

	// 稳定降序排序：分数高的优先；分数相同时按名称字母序
	sort.SliceStable(dynamicScores, func(i, j int) bool {
		if dynamicScores[i].Score != dynamicScores[j].Score {
			return dynamicScores[i].Score > dynamicScores[j].Score
		}
		return dynamicScores[i].Name < dynamicScores[j].Name
	})

	result := make([]ToolEntry, 0, len(coreSelected)+topK)
	result = append(result, coreSelected...)

	for i := 0; i < len(dynamicScores) && i < topK; i++ {
		result = append(result, dynamicScores[i].Tool)
	}

	return result
}

// calculateToolScore 计算单个工具针对当前查询的关联度分数
func calculateToolScore(def ai.Tool, queryTokens []string) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	name := strings.ToLower(def.Function.Name)
	desc := strings.ToLower(def.Function.Description)

	score := 0.0
	for _, token := range queryTokens {
		tokenLower := strings.ToLower(token)
		if strings.Contains(name, tokenLower) {
			score += 4.0
		}
		if strings.Contains(desc, tokenLower) {
			score += 1.5
		}
	}
	return score
}

// extractQueryTokens 从用户输入中分词（提取中英文词素）
func extractQueryTokens(query string) []string {
	clean := strings.TrimSpace(query)
	if clean == "" {
		return nil
	}

	tokenSet := make(map[string]struct{})
	var currentWord strings.Builder

	runes := []rune(clean)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			if r < 128 {
				// ASCII 字母数字与连接符收集为词
				currentWord.WriteRune(r)
			} else {

				// 中文字符：提取单字与双字 n-gram
				tokenSet[string(r)] = struct{}{}
				if i+1 < len(runes) && runes[i+1] >= 128 && (unicode.IsLetter(runes[i+1]) || unicode.IsDigit(runes[i+1])) {
					tokenSet[string([]rune{r, runes[i+1]})] = struct{}{}
				}
				if currentWord.Len() > 0 {
					tokenSet[currentWord.String()] = struct{}{}
					currentWord.Reset()
				}
			}
		} else {
			if currentWord.Len() > 0 {
				tokenSet[currentWord.String()] = struct{}{}
				currentWord.Reset()
			}
		}
	}

	if currentWord.Len() > 0 {
		tokenSet[currentWord.String()] = struct{}{}
	}

	tokens := make([]string, 0, len(tokenSet))
	for t := range tokenSet {
		// 过滤过短的纯无意义字符
		if len(t) >= 2 || (len(t) > 0 && rune(t[0]) >= 128) {
			tokens = append(tokens, t)
		}
	}
	return tokens
}
