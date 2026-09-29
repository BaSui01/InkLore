package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"InkLore/internal/agent/tool/domain"
)

// 生成侧工具族（INK-072）。
//
// 与内置的 7 个「分析器」的区别：这些工具**改文本**，而且只能由模型完成。
// 因此它们有一条硬约束——**没有生成能力时明确失败，绝不产出模板文本冒充结果**。
//
// 表驱动实现：六种工具的差别只有提示词、参数与标签，用一张表比六个几乎相同的
// struct 更好维护，也让「新增一种写作动作」变成加一行。

// generationSpec 描述一种写作动作。
type generationSpec struct {
	id           string
	name         string
	description  string
	category     string
	params       []domain.ToolParam
	actionLabel  string
	systemPrompt string
	temperature  float64
	// taskKey 是任务级模型策略（INK-066）里的任务标识：工具不写死模型，
	// 而是声明"我属于哪个创作任务"，由策略中心决定用哪个模型。
	taskKey string
	// buildPrompt 把入参拼成用户提示词；返回错误表示参数不合法。
	buildPrompt func(input map[string]any) (string, error)
}

type generationTool struct {
	spec      generationSpec
	generator domain.TextGenerator
}

func (t *generationTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          t.spec.id,
		Name:        t.spec.name,
		Description: t.spec.description,
		Category:    t.spec.category,
		Parameters:  t.spec.params,
	}
}

func (t *generationTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	// 参数校验放在生成之前：把「缺参数」与「模型失败」区分开，作者才知道该改提示还是该重试。
	prompt, err := t.spec.buildPrompt(input)
	if err != nil {
		return domain.ToolResult{
			ToolID:  t.spec.id,
			Success: false,
			Summary: err.Error(),
		}, nil
	}
	if t.generator == nil {
		return domain.ToolResult{
			ToolID:  t.spec.id,
			Success: false,
			Summary: "写作工具未装配模型网关，无法生成内容（不提供模板兜底）",
		}, nil
	}

	model, _ := input["model"].(string)
	response, genErr := t.generator.GenerateText(ctx, domain.TextRequest{
		Model:              strings.TrimSpace(model),
		SystemPrompt:       t.spec.systemPrompt,
		Prompt:             prompt,
		DefaultTemperature: t.spec.temperature,
		TaskKey:            t.spec.taskKey,
	})
	if genErr != nil {
		return domain.ToolResult{
			ToolID:  t.spec.id,
			Success: false,
			Summary: fmt.Sprintf("%s失败: %v", t.spec.actionLabel, genErr),
		}, nil
	}
	text := strings.TrimSpace(response.Content)
	if text == "" {
		return domain.ToolResult{
			ToolID:  t.spec.id,
			Success: false,
			Summary: t.spec.actionLabel + "失败: 模型返回空内容",
		}, nil
	}

	return domain.ToolResult{
		ToolID:  t.spec.id,
		Success: true,
		Summary: fmt.Sprintf("%s完成，共 %d 字", t.spec.actionLabel, len([]rune(text))),
		Details: map[string]any{
			"text":   text,
			"model":  response.Model,
			"action": t.spec.id,
		},
	}, nil
}

const (
	systemWritingBase = "你是一位中文长篇小说写作助手。只输出可以直接放进正文的文字，" +
		"不要解释、不要点评、不要复述要求、不要使用 Markdown 标题或代码块。"
	systemEditorBase = "你是一位中文长篇小说编辑。只输出修改后的正文文字，不要解释、不要列修改说明、不要使用 Markdown。"
)

// generationSpecs 是六种写作动作的定义。
var generationSpecs = []generationSpec{
	{
		id:           "continue_writing",
		taskKey:      "draft",
		name:         "智能续写",
		description:  "按已有正文与走向提示续写后续内容，保持人称、时态、文风与既有设定一致",
		category:     "writing",
		actionLabel:  "续写",
		systemPrompt: systemWritingBase + "续写必须紧接给定正文的最后一个字，不重复已有内容。",
		temperature:  0.85,
		params: []domain.ToolParam{
			{Name: "context", Type: "string", Description: "已有正文（续写起点）", Required: true},
			{Name: "direction", Type: "string", Description: "接下来的走向或要发生的事", Required: false},
			{Name: "word_count", Type: "number", Description: "期望字数，默认 500", Required: false},
			{Name: "model", Type: "string", Description: "可选：指定模型", Required: false},
		},
		buildPrompt: func(input map[string]any) (string, error) {
			context := strings.TrimSpace(stringValue(input["context"]))
			if context == "" {
				return "", fmt.Errorf("续写需要提供已有正文（context）")
			}
			wordCount := "500"
			if value := strings.TrimSpace(stringValue(input["word_count"])); value != "" {
				wordCount = value
			}
			direction := strings.TrimSpace(stringValue(input["direction"]))
			prompt := fmt.Sprintf("【已有正文】\n%s\n\n请紧接上文续写约 %s 字。", context, wordCount)
			if direction != "" {
				prompt += "\n【走向要求】\n" + direction
			}
			return prompt, nil
		},
	},
	{
		id:           "expand_selection",
		taskKey:      "draft",
		name:         "选区扩写",
		description:  "把一段过于简略的正文扩写成更丰满的场景，只增细节不改事实",
		category:     "writing",
		actionLabel:  "扩写",
		systemPrompt: systemWritingBase + "扩写只允许增加环境、动作、感官与心理细节；不得新增人物、事件或改变既有情节走向。",
		temperature:  0.8,
		params: []domain.ToolParam{
			{Name: "selection", Type: "string", Description: "待扩写的片段", Required: true},
			{Name: "context", Type: "string", Description: "所在章节的上下文", Required: false},
			{Name: "target_length", Type: "number", Description: "期望长度（字数）", Required: false},
			{Name: "model", Type: "string", Description: "可选：指定模型", Required: false},
		},
		buildPrompt: func(input map[string]any) (string, error) {
			selection := strings.TrimSpace(stringValue(input["selection"]))
			if selection == "" {
				return "", fmt.Errorf("扩写需要提供待扩写片段（selection）")
			}
			prompt := ""
			if context := strings.TrimSpace(stringValue(input["context"])); context != "" {
				prompt += "【上下文】\n" + context + "\n\n"
			}
			prompt += "【待扩写片段】\n" + selection + "\n\n请扩写上述片段"
			if length := strings.TrimSpace(stringValue(input["target_length"])); length != "" {
				prompt += fmt.Sprintf("，目标约 %s 字", length)
			}
			prompt += "。"
			return prompt, nil
		},
	},
	{
		id:           "polish",
		taskKey:      "deai",
		name:         "文风润色",
		description:  "只改表达不改内容：优化句式、节奏与用词，情节事实、人名与专有名词保持不变",
		category:     "writing",
		actionLabel:  "润色",
		systemPrompt: systemEditorBase + "润色是只改表达不改内容：情节、人物、时间线、专有名词必须完全保留，不得增删事件。",
		temperature:  0.5,
		params: []domain.ToolParam{
			{Name: "text", Type: "string", Description: "待润色的正文", Required: true},
			{Name: "style", Type: "string", Description: "期望文风（如克制悬疑、明快口语）", Required: false},
			{Name: "model", Type: "string", Description: "可选：指定模型", Required: false},
		},
		buildPrompt: func(input map[string]any) (string, error) {
			text := strings.TrimSpace(stringValue(input["text"]))
			if text == "" {
				return "", fmt.Errorf("润色需要提供正文（text）")
			}
			prompt := "请润色以下正文。"
			if style := strings.TrimSpace(stringValue(input["style"])); style != "" {
				prompt += "目标文风：" + style + "。"
			}
			return prompt + "\n\n" + text, nil
		},
	},
	{
		id:           "rewrite_with_style",
		taskKey:      "deai",
		name:         "按文风改写",
		description:  "按指定文风重写一段文字，允许调整句式与叙述视角，但情节事实保持不变",
		category:     "writing",
		actionLabel:  "改写",
		systemPrompt: systemEditorBase + "改写允许调整句式、节奏与叙述方式，但事件顺序与结果不得改变。",
		temperature:  0.7,
		params: []domain.ToolParam{
			{Name: "text", Type: "string", Description: "待改写的正文", Required: true},
			{Name: "style", Type: "string", Description: "目标文风（必填）", Required: true},
			{Name: "model", Type: "string", Description: "可选：指定模型", Required: false},
		},
		buildPrompt: func(input map[string]any) (string, error) {
			text := strings.TrimSpace(stringValue(input["text"]))
			if text == "" {
				return "", fmt.Errorf("改写需要提供正文（text）")
			}
			style := strings.TrimSpace(stringValue(input["style"]))
			if style == "" {
				return "", fmt.Errorf("改写需要提供目标文风（style）")
			}
			return fmt.Sprintf("请把以下正文改写为「%s」的文风：\n\n%s", style, text), nil
		},
	},
	{
		id:          "deai_rewrite",
		taskKey:     "deai",
		name:        "去 AI 味改写",
		description: "消除模板化表达：过度比喻、排比堆砌、解释性旁白、程式化情绪反应，保留原意",
		category:    "writing",
		actionLabel: "去 AI 味改写",
		systemPrompt: systemEditorBase + "你要消除的 AI 味典型特征：" +
			"1) 每段都用比喻或排比收尾；2) 直接解释人物情绪而不是通过动作呈现；" +
			"3) 滥用副词与四字词堆砌；4) 句长高度均匀；5) 用总结句代替场景。",
		temperature: 0.6,
		params: []domain.ToolParam{
			{Name: "text", Type: "string", Description: "待改写的正文", Required: true},
			{Name: "model", Type: "string", Description: "可选：指定模型", Required: false},
		},
		buildPrompt: func(input map[string]any) (string, error) {
			text := strings.TrimSpace(stringValue(input["text"]))
			if text == "" {
				return "", fmt.Errorf("去 AI 味改写需要提供正文（text）")
			}
			return "请改写以下正文，消除 AI 味，保持原意与信息量：\n\n" + text, nil
		},
	},
	{
		id:           "generate_names",
		taskKey:      "naming",
		name:         "命名生成器",
		description:  "按题材与限制生成人名、地名、门派或招式名，输出可直接使用的候选清单",
		category:     "writing",
		actionLabel:  "命名生成",
		systemPrompt: "你是中文小说命名助手。只输出候选名称清单，每行一个，不要解释、不要编号。",
		temperature:  0.95,
		params: []domain.ToolParam{
			{Name: "genre", Type: "string", Description: "题材（如都市异能、古典仙侠、赛博悬疑）", Required: true},
			{Name: "kind", Type: "string", Description: "命名对象（人名/地名/门派/招式），默认人名", Required: false},
			{Name: "constraint", Type: "string", Description: "额外限制（字数、姓氏、风格等）", Required: false},
			{Name: "count", Type: "number", Description: "候选数量，默认 10", Required: false},
			{Name: "model", Type: "string", Description: "可选：指定模型", Required: false},
		},
		buildPrompt: func(input map[string]any) (string, error) {
			genre := strings.TrimSpace(stringValue(input["genre"]))
			if genre == "" {
				return "", fmt.Errorf("命名需要提供题材（genre）")
			}
			kind := strings.TrimSpace(stringValue(input["kind"]))
			if kind == "" {
				kind = "人名"
			}
			count := strings.TrimSpace(stringValue(input["count"]))
			if count == "" {
				count = "10"
			}
			prompt := fmt.Sprintf("题材：%s\n命名对象：%s\n数量：%s", genre, kind, count)
			if constraint := strings.TrimSpace(stringValue(input["constraint"])); constraint != "" {
				prompt += "\n额外限制：" + constraint
			}
			return prompt, nil
		},
	},
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%v", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// NewGenerationTools 按表构建全部写作工具；generator 为 nil 时工具仍会注册，
// 但执行时明确失败而不是伪造文本。
func NewGenerationTools(generator domain.TextGenerator) []domain.Tool {
	built := make([]domain.Tool, 0, len(generationSpecs))
	for _, spec := range generationSpecs {
		built = append(built, &generationTool{spec: spec, generator: generator})
	}
	return built
}
