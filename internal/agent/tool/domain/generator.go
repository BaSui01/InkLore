package domain

import "context"

// TextGenerator 是「生成侧工具」依赖的文本生成端口（INK-072）。
//
// 为什么需要它：内置的 7 个工具都是**本地确定性计算**（一致性校验、节奏诊断……），
// 不依赖模型；而续写、扩写、润色、改写、去 AI 味、命名这些动作**只能由模型完成**。
// 没有这一层就会出现两种坏结果：要么工具无法实现，要么工具自己拼模板文本冒充 AI 改写
// （ADR-0005 禁止的伪造）。这里把「谁来生成」交给组合根适配，工具只负责提示词与校验。
//
// 端口定义在工具域内、由 internal/app/adapters 适配到模型基座域：
// 工具域不感知供应商、Key、路由与限流。
type TextGenerator interface {
	GenerateText(ctx context.Context, request TextRequest) (TextResponse, error)
}

// TextRequest 是一次生成请求。
type TextRequest struct {
	// Model 为空表示使用装配时的默认写作模型。
	Model        string
	SystemPrompt string
	Prompt       string
	// Temperature 是**显式**温度（调用方明确要求的值）。0 表示未指定。
	Temperature float64
	// DefaultTemperature 是工具自带的建议温度：只有在既没有显式 Temperature、
	// 也没有任务级策略温度时才使用（INK-066）。这样"运营在控制台配的温度"
	// 永远压得过"代码里写死的建议值"，而两者都缺席时工具仍有合理默认。
	DefaultTemperature float64
	RequestID          string
	// TaskKey 声明本次生成属于哪个创作任务（INK-066），供任务级模型策略解析。
	TaskKey string
}

// TextResponse 是一次生成结果。
type TextResponse struct {
	Content string
	Model   string
}
