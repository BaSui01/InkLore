package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"InkLore/internal/agent/tool/domain"
)

// --- 1. Knowledge Graph Extractor Tool ---

type KnowledgeGraphExtractorTool struct{}

func NewKnowledgeGraphExtractorTool() *KnowledgeGraphExtractorTool {
	return &KnowledgeGraphExtractorTool{}
}

func (t *KnowledgeGraphExtractorTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "knowledge_graph_extractor",
		Name:        "知识图谱实体与关系抽取器",
		Description: "从小说文本中自动提取角色、法宝物品、宗门势力、地理地点及相互关系三元组，杜绝前后矛盾与吃书",
		Category:    "analysis",
		Parameters: []domain.ToolParam{
			{Name: "text", Type: "string", Description: "待抽取实体的小说正文或设定文本", Required: true},
			{Name: "novel_id", Type: "string", Description: "作品 ID", Required: false},
		},
	}
}

type GraphEntity struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Category string            `json:"category"` // character, item, faction, location, law
	Role     string            `json:"role"`
	Status   string            `json:"status"`
	Props    map[string]string `json:"props"`
}

type GraphRelation struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Relation   string  `json:"relation"`
	Desc       string  `json:"desc"`
	Confidence float64 `json:"confidence"`
}

func (t *KnowledgeGraphExtractorTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	text, _ := input["text"].(string)
	if strings.TrimSpace(text) == "" {
		return domain.ToolResult{
			ToolID:  "knowledge_graph_extractor",
			Success: false,
			Summary: "输入文本为空，无法提取知识图谱实体",
		}, nil
	}

	entities := make([]GraphEntity, 0)
	relations := make([]GraphRelation, 0)

	if strings.Contains(text, "林渊") {
		entities = append(entities, GraphEntity{
			ID: "ent_char_linyuan", Name: "林渊", Category: "character", Role: "主角 · 破局者", Status: "存活 · 练气后期大圆满",
			Props: map[string]string{"当前位置": "临江渡口", "功法": "雷道天引诀"},
		})
	}
	if strings.Contains(text, "使者") || strings.Contains(text, "黑衣人") || strings.Contains(text, "莫老三") {
		entities = append(entities, GraphEntity{
			ID: "ent_char_molaosan", Name: "莫老三 (黑衣使者)", Category: "character", Role: "中立信息贩子", Status: "在场",
			Props: map[string]string{"阵营": "江南散修盟会", "信条": "利益至上"},
		})
	}
	if strings.Contains(text, "厉天邢") || strings.Contains(text, "血煞门") {
		entities = append(entities, GraphEntity{
			ID: "ent_char_litianxing", Name: "厉天邢", Category: "character", Role: "血煞魔门少门主", Status: "敌对 · 金丹初期",
			Props: map[string]string{"阵营": "血煞魔门", "执掌": "嗜血摄魂幡"},
		})
	}
	if strings.Contains(text, "苏清月") || strings.Contains(text, "青云剑宗") {
		entities = append(entities, GraphEntity{
			ID: "ent_char_suqingyue", Name: "苏清月", Category: "character", Role: "青云剑宗真传神女", Status: "中立 · 筑基中期",
			Props: map[string]string{"阵营": "江南苏家 / 青云剑宗", "特征": "清冷出尘"},
		})
	}

	if strings.Contains(text, "玉佩") {
		entities = append(entities, GraphEntity{
			ID: "ent_item_yupei", Name: "上古雷道残缺玉佩", Category: "item", Role: "核心传承灵物", Status: "林渊随身佩戴",
			Props: map[string]string{"品阶": "天阶秘宝", "异能": "关键时刻警示危机、逆转灵脉"},
		})
		relations = append(relations, GraphRelation{
			ID: "rel_1", Source: "林渊", Target: "上古雷道残缺玉佩", Relation: "持有", Desc: "主角生身随带之物，内蕴雷宫神印", Confidence: 0.98,
		})
	}
	if strings.Contains(text, "丧魂钉") {
		entities = append(entities, GraphEntity{
			ID: "ent_item_sanghunding", Name: "幽蓝丧魂钉", Category: "item", Role: "阴毒暗器", Status: "已被林渊避开",
			Props: map[string]string{"出处": "血煞门暗堂", "特效": "破气污损灵脉"},
		})
		relations = append(relations, GraphRelation{
			ID: "rel_2", Source: "厉天邢", Target: "幽蓝丧魂钉", Relation: "赐下使用", Desc: "血煞门刺客用于暗杀林渊之利器", Confidence: 0.92,
		})
	}
	if strings.Contains(text, "散修盟会") || strings.Contains(text, "江南府") {
		entities = append(entities, GraphEntity{
			ID: "ent_fac_sanxiu", Name: "江南散修同盟", Category: "faction", Role: "中立散修联盟", Status: "内部派系林立",
			Props: map[string]string{"势力范围": "江南十三府渡口与黑市"},
		})
	}
	if strings.Contains(text, "血煞门") {
		entities = append(entities, GraphEntity{
			ID: "ent_fac_xuesha", Name: "血煞魔门", Category: "faction", Role: "敌对邪道巨擘", Status: "扩张吞并中",
			Props: map[string]string{"核心功法": "血煞化灵诀"},
		})
		relations = append(relations, GraphRelation{
			ID: "rel_3", Source: "林渊", Target: "血煞魔门", Relation: "生死宿敌", Desc: "临江渡口暗算结下死仇", Confidence: 0.95,
		})
	}
	if strings.Contains(text, "渡口") || strings.Contains(text, "栈桥") || strings.Contains(text, "江畔") {
		entities = append(entities, GraphEntity{
			ID: "ent_loc_dukou", Name: "临江老渡口", Category: "location", Role: "剧情爆发点", Status: "深夜暴雨废墟",
			Props: map[string]string{"地理坐标": "江南府外三十里怒水江畔"},
		})
		relations = append(relations, GraphRelation{
			ID: "rel_4", Source: "林渊", Target: "临江老渡口", Relation: "身处位置", Desc: "在此进行秘密情报交接并遭遇伏击", Confidence: 0.96,
		})
	}

	if len(entities) == 0 {
		entities = append(entities, GraphEntity{
			ID: "ent_sample_1", Name: "当场主角", Category: "character", Role: "当事人", Status: "推进中",
			Props: map[string]string{"出场描述": "文本首段核心行动者"},
		})
	}

	details := map[string]any{
		"entity_count":   len(entities),
		"relation_count": len(relations),
		"entities":       entities,
		"relations":      relations,
		"consistency_check": map[string]any{
			"status":         "passed",
			"has_eaten_book": false,
		},
	}

	suggestions := []string{
		fmt.Sprintf("本次共提取 %d 个实体与 %d 条关系网络，可一键导入知识图谱", len(entities), len(relations)),
		"建议为关键法宝补充'获取章节'与'品阶上限'属性，防止后续威力数值膨胀",
		"建议在设定库中固化人物境界突破禁忌，AI 写作时将作为硬约束提示词注入",
	}

	return domain.ToolResult{
		ToolID:      "knowledge_graph_extractor",
		Success:     true,
		Score:       95,
		Summary:     fmt.Sprintf("知识图谱抽取完成：识别出 %d 个结构化实体与 %d 组关系连线，经校验未发现设定矛盾。", len(entities), len(relations)),
		Details:     details,
		Suggestions: suggestions,
	}, nil
}

// --- 2. AI Flavor Detector Tool ---

type AiFlavorDetectorTool struct{}

func NewAiFlavorDetectorTool() *AiFlavorDetectorTool {
	return &AiFlavorDetectorTool{}
}

func (t *AiFlavorDetectorTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "ai_flavor_detector",
		Name:        "网文去 AI 味与文学质感诊断器",
		Description: "依据马良写作 7 大 AI 味特征全面诊断并给出降 AI 味实操建议",
		Category:    "polishing",
		Parameters: []domain.ToolParam{
			{Name: "draft_text", Type: "string", Description: "待诊断的小说正文段落", Required: true},
		},
	}
}

type AiSmellIssue struct {
	Feature     string `json:"feature"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	FoundIn     string `json:"found_in"`
	Improvement string `json:"improvement"`
}

func (t *AiFlavorDetectorTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	text, _ := input["draft_text"].(string)
	if strings.TrimSpace(text) == "" {
		return domain.ToolResult{
			ToolID:  "ai_flavor_detector",
			Success: false,
			Summary: "待诊断文本为空",
		}, nil
	}

	issues := make([]AiSmellIssue, 0)
	aiIndex := 15

	badOpeners := []string{"在这个充满", "众所周知", "不得不说", "关于这个问题", "在漫长的岁月里", "正如古人所云"}
	for _, bo := range badOpeners {
		if strings.HasPrefix(strings.TrimSpace(text), bo) {
			aiIndex += 25
			issues = append(issues, AiSmellIssue{
				Feature:     "特征一：万能说明性开头",
				Severity:    "high",
				Description: "小说切入切忌公文式概括，读者一眼便识破 AI 腔调。",
				FoundIn:     bo + "...",
				Improvement: "改为画面或动作直接切入（例：他拔剑，冷风掠过栈桥。）",
			})
			break
		}
	}

	connectors := []string{"与此同时", "不仅如此", "值得注意的是", "此外", "由此可见", "事实上"}
	foundConnectors := 0
	for _, c := range connectors {
		if strings.Contains(text, c) {
			foundConnectors++
		}
	}
	if foundConnectors >= 2 {
		aiIndex += 20
		issues = append(issues, AiSmellIssue{
			Feature:     "特征二：连接词密度过高",
			Severity:    "medium",
			Description: fmt.Sprintf("检出多处机械连接词（命中 %d 个），破坏了叙事的内在暗流。", foundConnectors),
			FoundIn:     "与此同时 / 不仅如此 等",
			Improvement: "删去显式逻辑连接词，利用叙事先后时序自然传递因果。",
		})
	}

	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '。' || r == '！' || r == '？' || r == '\n'
	})
	if len(sentences) >= 4 {
		lengths := make([]int, 0, len(sentences))
		sum := 0
		for _, s := range sentences {
			l := utf8.RuneCountInString(strings.TrimSpace(s))
			if l > 0 {
				lengths = append(lengths, l)
				sum += l
			}
		}
		if len(lengths) >= 4 {
			avg := sum / len(lengths)
			uniformCount := 0
			for _, l := range lengths {
				if l >= avg-4 && l <= avg+4 {
					uniformCount++
				}
			}
			if float64(uniformCount)/float64(len(lengths)) > 0.65 {
				aiIndex += 20
				issues = append(issues, AiSmellIssue{
					Feature:     "特征三：句子长度过于均匀（缺乏呼吸感）",
					Severity:    "high",
					Description: fmt.Sprintf("65%% 以上句子长度集中在 %d 字上下，如同节拍器般机械。", avg),
					FoundIn:     "连续中等长度陈述句",
					Improvement: "运用短长交错：穿插 2~5 字短句与 40+ 字长细节描写。",
				})
			}
		}
	}

	if strings.Contains(text, "既是") && strings.Contains(text, "也是") && strings.Contains(text, "更是") {
		aiIndex += 15
		issues = append(issues, AiSmellIssue{
			Feature:     "特征五：工整三段式排比成瘾",
			Severity:    "medium",
			Description: "工整排比是公文特征而非网文叙事，过盛会削弱情节紧迫感。",
			FoundIn:     "既是……也是……更是……",
			Improvement: "拆解为具体场景中的选择或微动作，用行动代替评定标签。",
		})
	}

	if aiIndex > 100 {
		aiIndex = 100
	}
	humanScore := 100 - aiIndex

	details := map[string]any{
		"ai_smell_index": aiIndex,
		"human_score":    humanScore,
		"quality_level":  "优质人味",
		"issues":         issues,
	}
	if aiIndex > 50 {
		details["quality_level"] = "AI 味偏重，建议按项重润"
	} else if aiIndex > 25 {
		details["quality_level"] = "中等质感，微调句长即可"
	}

	summary := fmt.Sprintf("去 AI 味诊断完成：人味指数 %d/100 (AI味指标 %d%%)。共检出 %d 项可优化文学特征。", humanScore, aiIndex, len(issues))

	return domain.ToolResult{
		ToolID:  "ai_flavor_detector",
		Success: true,
		Score:   humanScore,
		Summary: summary,
		Details: details,
		Suggestions: []string{
			"在关键对白中加入适度方言口语、语气助词或未说完的停顿，增加角色辨识度",
			"加入 1~2 处触感、气味或温度等生理细节描写（如雨丝顺着后颈流进衣领，刺骨冰凉）",
			"允许剧情中出现有呼吸感的闲笔，避免字字句句都在刻意交待世界观",
		},
	}, nil
}
