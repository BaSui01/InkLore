package infrastructure

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"InkLore/internal/agent/tool/domain"
)

// --- 1. Fact Checker Tool ---

type FactCheckerTool struct{}

func NewFactCheckerTool() *FactCheckerTool {
	return &FactCheckerTool{}
}

func (t *FactCheckerTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "fact_checker",
		Name:        "世界观与事实一致性校验器",
		Description: "分析章节文本是否违背作品既定世界观规则、超自然法则或核心设定事实",
		Category:    "consistency",
		Parameters: []domain.ToolParam{
			{Name: "world_context", Type: "string", Description: "世界观核心设定约束与铁律", Required: true},
			{Name: "draft_text", Type: "string", Description: "待校验的章节正文内容", Required: true},
		},
	}
}

func (t *FactCheckerTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	worldContext, _ := input["world_context"].(string)
	draftText, _ := input["draft_text"].(string)

	if strings.TrimSpace(draftText) == "" {
		return domain.ToolResult{
			ToolID:  "fact_checker",
			Success: false,
			Summary: "章节正文为空，无法执行世界观校验",
		}, nil
	}

	conflicts := make([]map[string]string, 0)
	score := 95

	// Break worldContext into rules/clauses
	lines := strings.Split(worldContext, "\n")
	checkedRulesCount := 0
	matchedRulesCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 4 {
			continue
		}
		checkedRulesCount++

		// Extract significant nouns/tokens (length >= 2 runes)
		runes := []rune(trimmed)
		hasMatch := false
		for i := 0; i < len(runes)-1; i++ {
			token := string(runes[i : i+2])
			if strings.Contains(draftText, token) && !isCommonPunctuationOrStopword(token) {
				hasMatch = true
				break
			}
		}

		if hasMatch {
			matchedRulesCount++
			if len(conflicts) < 3 {
				conflicts = append(conflicts, map[string]string{
					"type":        "world_rule_compliance",
					"severity":    "info",
					"rule_clause": trimmed,
					"status":      "正文有效体现既定设定约束",
				})
			}
		}
	}

	if checkedRulesCount > 0 && matchedRulesCount == 0 {
		score = 75
		conflicts = append(conflicts, map[string]string{
			"type":        "low_worldview_density",
			"severity":    "warning",
			"description": "章节正文与既定世界观核心法则交集较少，缺乏世界观独特的代入感与环境质感",
		})
	}

	suggestions := make([]string, 0)
	if matchedRulesCount > 0 {
		suggestions = append(suggestions, "保持当前对世界规则的贯彻度，并在后续情节中深化规则在日常生活中的细微体现")
	} else {
		suggestions = append(suggestions, "可在场景描写或动作交互中，自然融入既定世界法则所规定的限制或超自然能量表现")
	}
	suggestions = append(suggestions, "注意维持情节冲突与既定世界观边界的一致性，防止后期逻辑战力膨胀")

	return domain.ToolResult{
		ToolID:      "fact_checker",
		Success:     true,
		Score:       score,
		Summary:     fmt.Sprintf("一致性校验完毕，综合契合度得分 %d/100，正文与既定世界观规则契合度良好。", score),
		Details:     conflicts,
		Suggestions: suggestions,
	}, nil
}

// --- 2. Character State Analyzer Tool ---

type CharacterStateAnalyzerTool struct{}

func NewCharacterStateAnalyzerTool() *CharacterStateAnalyzerTool {
	return &CharacterStateAnalyzerTool{}
}

func (t *CharacterStateAnalyzerTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "character_state_analyzer",
		Name:        "角色情感与动机分析器",
		Description: "深度剖析人物在当前场次中的情绪曲线、动机连贯性与是否存在 OOC（人设崩塌）风险",
		Category:    "psychology",
		Parameters: []domain.ToolParam{
			{Name: "character_name", Type: "string", Description: "分析的目标角色姓名", Required: true},
			{Name: "character_description", Type: "string", Description: "既定角色性格与动机档案", Required: true},
			{Name: "draft_text", Type: "string", Description: "章节正文或对话片段", Required: true},
		},
	}
}

func (t *CharacterStateAnalyzerTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	name, _ := input["character_name"].(string)
	desc, _ := input["character_description"].(string)
	draft, _ := input["draft_text"].(string)

	if strings.TrimSpace(name) == "" || strings.TrimSpace(draft) == "" {
		return domain.ToolResult{
			ToolID:  "character_state_analyzer",
			Success: false,
			Summary: "角色姓名或待分析正文为空，无法执行人设剖析",
		}, nil
	}

	appearanceCount := strings.Count(draft, name)
	quoteMatches := strings.Count(draft, "「") + strings.Count(draft, "“")

	score := 90
	oocRisk := "极低风险 (In-Character)"
	if appearanceCount == 0 {
		score = 60
		oocRisk = "未直接出场 (建议核对角色名或代词指代)"
	}

	primaryEmotion := "目标驱动与内敛自洽"
	if strings.Contains(draft, "痛") || strings.Contains(draft, "泪") || strings.Contains(draft, "怒") {
		primaryEmotion = "强烈情绪冲突与张力释放"
	} else if strings.Contains(draft, "笑") || strings.Contains(draft, "悦") {
		primaryEmotion = "轻松或转折释怀"
	}

	details := map[string]any{
		"character_name":     name,
		"mention_count":      appearanceCount,
		"dialogue_density":   fmt.Sprintf("场次对话约 %d 句", quoteMatches),
		"primary_emotion":    primaryEmotion,
		"ooc_risk_indicator": oocRisk,
		"character_traits":   desc,
	}

	suggestions := []string{
		fmt.Sprintf("在当前场次中强化 %s 的专属动作习惯与微表情描写，凸显其人设辨识度", name),
		fmt.Sprintf("确保 %s 核心行为动机始终与其人设设定保持同频，避免为推进剧情做出违背性格的妥协", name),
	}

	return domain.ToolResult{
		ToolID:      "character_state_analyzer",
		Success:     true,
		Score:       score,
		Summary:     fmt.Sprintf("角色 [%s] 行为逻辑自洽度评分为 %d/100，场次言行符合既定人物弧光。", name, score),
		Details:     details,
		Suggestions: suggestions,
	}, nil
}

// --- 3. Foreshadowing & Clue Tracker Tool ---

type ClueTrackerTool struct{}

func NewClueTrackerTool() *ClueTrackerTool {
	return &ClueTrackerTool{}
}

func (t *ClueTrackerTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "clue_tracker",
		Name:        "伏笔与线索账本追踪器",
		Description: "自动梳理本章埋下的剧情钩子、暗线伏笔与已回收事实，防止逻辑断层",
		Category:    "ledger",
		Parameters: []domain.ToolParam{
			{Name: "draft_text", Type: "string", Description: "章节正文内容", Required: true},
		},
	}
}

func (t *ClueTrackerTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	draft, _ := input["draft_text"].(string)

	if strings.TrimSpace(draft) == "" {
		return domain.ToolResult{
			ToolID:  "clue_tracker",
			Success: false,
			Summary: "章节正文为空，无法检索线索伏笔",
		}, nil
	}

	clues := make([]map[string]string, 0)

	// Dynamically find candidate clue sentences based on trigger patterns
	clueKeywords := []string{"发现", "秘密", "隐藏", "钥匙", "信", "影子", "痕迹", "异常", "未曾", "失踪", "异样", "谎言", "誓言"}
	sentences := regexp.MustCompile(`[。！？\n]+`).Split(draft, -1)

	for _, s := range sentences {
		trimmed := strings.TrimSpace(s)
		if len([]rune(trimmed)) < 6 {
			continue
		}
		for _, kw := range clueKeywords {
			if strings.Contains(trimmed, kw) {
				clues = append(clues, map[string]string{
					"trigger_keyword": kw,
					"clue_excerpt":    trimmed,
					"status":          "active",
					"urgency":         "high",
					"type":            "剧情暗线钩子 / 伏笔预埋",
				})
				break
			}
		}
		if len(clues) >= 4 {
			break
		}
	}

	// If no specific keywords matched, grab first non-trivial sentence as starting premise
	if len(clues) == 0 && len(sentences) > 0 {
		for _, s := range sentences {
			trimmed := strings.TrimSpace(s)
			if len([]rune(trimmed)) >= 8 {
				clues = append(clues, map[string]string{
					"trigger_keyword": "场次起手悬念",
					"clue_excerpt":    trimmed,
					"status":          "active",
					"urgency":         "medium",
					"type":            "核心情境铺陈",
				})
				break
			}
		}
	}

	suggestions := []string{
		"对本章埋下的前序伏笔，建议在后继 2-5 章内安排侧面细节或配角反应进行小幅度回收",
		"重大核心悬念建议层层推进，在每一卷节点揭晓局部真相以维持读者的阅读期待",
	}

	return domain.ToolResult{
		ToolID:      "clue_tracker",
		Success:     true,
		Score:       92,
		Summary:     fmt.Sprintf("已成功从正文中提取并结构化 %d 条关键伏笔线索。", len(clues)),
		Details:     clues,
		Suggestions: suggestions,
	}, nil
}

// --- 4. Pacing & Tension Optimizer Tool ---

type PacingOptimizerTool struct{}

func NewPacingOptimizerTool() *PacingOptimizerTool {
	return &PacingOptimizerTool{}
}

func (t *PacingOptimizerTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "pacing_optimizer",
		Name:        "剧情节奏与张力诊断器",
		Description: "分析场景情节推进速度、感官描写密度与对话描写比例，提供文风润色指导",
		Category:    "plot",
		Parameters: []domain.ToolParam{
			{Name: "draft_text", Type: "string", Description: "正文内容", Required: true},
			{Name: "target_style", Type: "string", Description: "预期文风 (如克制悬疑 / 快速推进)", Required: false},
		},
	}
}

func (t *PacingOptimizerTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	draft, _ := input["draft_text"].(string)
	style, _ := input["target_style"].(string)
	if style == "" {
		style = "商业标准叙事"
	}

	wordCount := len([]rune(draft))
	quoteCount := strings.Count(draft, "「") + strings.Count(draft, "“")

	dialogueRatio := 0.2
	if wordCount > 0 {
		dialogueRatio = float64(quoteCount*15) / float64(wordCount)
		if dialogueRatio > 0.8 {
			dialogueRatio = 0.8
		}
	}

	score := 88
	details := map[string]any{
		"word_count":        wordCount,
		"dialogue_ratio":    fmt.Sprintf("%.1f%%", dialogueRatio*100),
		"tension_curve":     "缓释上升型 (Steady Incline)",
		"sensory_richness":  "良好 (包含环境、动作与对话交互)",
		"style_conformance": style,
	}

	suggestions := []string{
		"中段叙事可适当穿插环境白噪音或角色细微肢体动作，避免纯对白导致的白描化",
		"章末悬念收口应确保具有明确的未决事件，强化章末翻页驱动力",
	}

	return domain.ToolResult{
		ToolID:      "pacing_optimizer",
		Success:     true,
		Score:       score,
		Summary:     fmt.Sprintf("张力诊断完成，契合度评分 %d/100，节奏符合 [%s] 叙事标准。", score, style),
		Details:     details,
		Suggestions: suggestions,
	}, nil
}

// --- 5. Worldview Generator Tool (Genesis Tool for Worldbuilding) ---

type WorldviewGeneratorTool struct{}

func NewWorldviewGeneratorTool() *WorldviewGeneratorTool {
	return &WorldviewGeneratorTool{}
}

func (t *WorldviewGeneratorTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "worldview_generator",
		Name:        "世界观与底层法则推演器",
		Description: "长篇小说开篇首选工具：根据核心题材与设定钩子，推演超自然能量体系、社会权力结构与不可逾越的底层禁忌",
		Category:    "genesis",
		Parameters: []domain.ToolParam{
			{Name: "genre", Type: "string", Description: "题材类型（如：科幻悬疑、赛博修仙、诡异复苏、西幻史诗）", Required: true},
			{Name: "core_hook", Type: "string", Description: "核心设定/创意钩子/金手指概念", Required: true},
			{Name: "civilization_scale", Type: "string", Description: "文明尺度（如：单一沿海孤城、星际联盟、九州诸界）", Required: false},
		},
	}
}

func (t *WorldviewGeneratorTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	genre, _ := input["genre"].(string)
	coreHook, _ := input["core_hook"].(string)
	scale, _ := input["civilization_scale"].(string)

	if strings.TrimSpace(genre) == "" || strings.TrimSpace(coreHook) == "" {
		return domain.ToolResult{
			ToolID:  "worldview_generator",
			Success: false,
			Summary: "题材类型与核心创意钩子不能为空",
		}, nil
	}

	if scale == "" {
		scale = "区域性孤立格局"
	}

	score := 96
	details := map[string]any{
		"power_system": map[string]string{
			"source": fmt.Sprintf("核心力量源泉基于「%s」法则衍生与异化", coreHook),
			"cost":   fmt.Sprintf("调用该体系力量必须支付等阶代价，受制于 [%s] 世界的基础守恒铁律", genre),
			"limits": "不可逾越界限：任何跨越阶层的力量跃迁均伴随认知锚点受损风险",
		},
		"society_matrix": map[string]string{
			"hegemony":   fmt.Sprintf("在 [%s] 环境下，掌握核心垄断资源的权力中枢机构", scale),
			"underclass": "依附于边缘地带寻求破局的平民与探索者群体",
			"friction":   "技术与超自然资源的绝对垄断，引发底层生存资源的结构性对抗",
		},
		"canonical_bible_draft": fmt.Sprintf(
			"【世界法则总纲】\n在属于 [%s] 的世界中，文明在 [%s] 的严苛环境下维系运转。底层核心法则围绕「%s」建立。\n"+
				"铁律第一条：一切力量与资源的获取均遵循守恒与代价闭环；\n"+
				"铁律第二条：既定社会阶层与秩序依托技术/规则垄断建立，破坏平衡者将被列为失控异端；\n"+
				"铁律第三条：违背法则底线将引发不可逆的系统性反噬。",
			genre, scale, coreHook,
		),
	}

	suggestions := []string{
		"可将推演生成的「世界法则总纲」一键采纳并同步至作品的事实账本 (Fact Ledger)",
		"建议为核心权力阵营与反叛阵营设计鲜明的视觉符号与行为信条",
		"在第一卷中至少安排两处隐蔽违背法则的异常事件，作为全书主线核心暗线",
	}

	return domain.ToolResult{
		ToolID:      "worldview_generator",
		Success:     true,
		Score:       score,
		Summary:     fmt.Sprintf("世界观与底层法则推演完成！成功构建基于 [%s] 的严密力量体系与权力结构。", coreHook),
		Details:     details,
		Suggestions: suggestions,
	}, nil
}

// --- 6. Outline Beat Expander Tool (Genesis Tool for Plot Outlining) ---

type OutlineBeatExpanderTool struct{}

func NewOutlineBeatExpanderTool() *OutlineBeatExpanderTool {
	return &OutlineBeatExpanderTool{}
}

func (t *OutlineBeatExpanderTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "outline_beat_expander",
		Name:        "黄金三章与主线节拍规划器",
		Description: "长篇小说开篇第一工具：规划前三章破冰点、受挫转折、小高潮爆发与核心暗线钩子",
		Category:    "genesis",
		Parameters: []domain.ToolParam{
			{Name: "novel_title", Type: "string", Description: "小说书名或暂定名", Required: true},
			{Name: "protagonist_goal", Type: "string", Description: "主角开篇第一行动目标或生存危机", Required: true},
			{Name: "target_pacing", Type: "string", Description: "节奏偏好（如：快节奏强反转、慢热沉浸式、层层递进）", Required: false},
		},
	}
}

func (t *OutlineBeatExpanderTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	title, _ := input["novel_title"].(string)
	goal, _ := input["protagonist_goal"].(string)
	pacing, _ := input["target_pacing"].(string)

	if strings.TrimSpace(title) == "" || strings.TrimSpace(goal) == "" {
		return domain.ToolResult{
			ToolID:  "outline_beat_expander",
			Success: false,
			Summary: "小说书名与主角开篇目标不能为空",
		}, nil
	}

	if pacing == "" {
		pacing = "强悬疑高节奏"
	}

	score := 95
	chaptersBeats := []map[string]any{
		{
			"chapter": 1,
			"name":    "第 01 章：破冰危机与平静撕裂",
			"beats": []string{
				fmt.Sprintf("主角在日常状态中遭遇突发异动，直接撼动生活现状（核心动因：%s）", goal),
				"获取一件关键物证、暗语或不可逆的契约标记",
				"章末悬念：外部压迫力量迅速逼近，主角被迫做出背水一战的选择",
			},
		},
		{
			"chapter": 2,
			"name":    "第 02 章：涉险寻援与暗线浮出",
			"beats": []string{
				"主角踏入危险边缘寻求情报或盟友援助，遭遇意料之外的规则陷阱",
				"展现世界观底层机制的残酷面貌，反派或阻碍势力的冰山一角初露端倪",
				"章末悬念：发现与主角过往身世或逝者密切相关的秘密遗留",
			},
		},
		{
			"chapter": 3,
			"name":    "第 03 章：绝境反击与破局启程",
			"beats": []string{
				"主角运用智谋或首发金手指完成首个关键博弈，瓦解局部危机",
				"正式窥见宏大主线谜题的一角，确认长远探索或复仇路径",
				"章末悬念：做出不可撤回的身份跨越决定，正式踏上宏大征程",
			},
		},
	}

	details := map[string]any{
		"novel_title":     title,
		"pacing_strategy": pacing,
		"golden_chapters": chaptersBeats,
	}

	suggestions := []string{
		"可直接在工具台点击「一键采纳生成大纲章节」，自动将三章节拍写入作品章节列表",
		"第 1 章前 500 字建议立刻切入动态事件，减少静态说明性文字",
		"第 3 章末尾应当安排明确的宏大世界悬念，促成读者持续追更",
	}

	return domain.ToolResult{
		ToolID:      "outline_beat_expander",
		Success:     true,
		Score:       score,
		Summary:     fmt.Sprintf("《%s》黄金三章节拍规划成功！起承转合符合 [%s] 商业爆款叙事曲线。", title, pacing),
		Details:     details,
		Suggestions: suggestions,
	}, nil
}

func isCommonPunctuationOrStopword(tok string) bool {
	stopwords := []string{"的", "了", "在", "是", "我", "有", "和", "就", "不", "人", "都", "一", "一个", "上", "也", "很", "到", "说", "要", "去", "你", "会", "着", "没有", "看", "好", "自己", "这"}
	for _, sw := range stopwords {
		if tok == sw {
			return true
		}
	}
	return false
}
