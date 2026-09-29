package domain

// 图片生成默认值是已发布配置的一部分；指针区分未配置与显式false/0，代码不得补业务默认值。
type AgentToolDefaults struct {
	GenerateImage *AgentImageToolDefaults `json:"generate_image,omitempty"`
}
type AgentImageToolDefaults struct {
	ModelID            *int64   `json:"model_id"`
	Mode               string   `json:"mode"`
	ExecutionAPI       string   `json:"execution_api"`
	BatchCount         *int     `json:"batch_count"`
	ImagesPerTask      *int     `json:"images_per_task"`
	ReferenceStrength  *float64 `json:"reference_strength"`
	EnableStream       *bool    `json:"enable_stream"`
	EnableThinking     *bool    `json:"enable_thinking"`
	EnableSearch       *bool    `json:"enable_search"`
	UseAttachedImages  *bool    `json:"use_attached_images"`
	AspectRatio        string   `json:"aspect_ratio"`
	ImageSize          string   `json:"image_size"`
	Quality            string   `json:"quality"`
	Style              string   `json:"style"`
	ResponseModalities []string `json:"response_modalities"`
	Width              *int     `json:"width,omitempty"`
	Height             *int     `json:"height,omitempty"`
	InputFidelity      string   `json:"input_fidelity,omitempty"`
	OutputFormat       string   `json:"output_format,omitempty"`
	Background         string   `json:"background,omitempty"`
	Moderation         string   `json:"moderation,omitempty"`
	OutputCompression  *int     `json:"output_compression,omitempty"`
	ResponsesProvider  string   `json:"responses_provider,omitempty"`
	ResponsesModel     string   `json:"responses_model,omitempty"`
	ResponsesAction    string   `json:"responses_action,omitempty"`
	PartialImages      *int     `json:"partial_images,omitempty"`
}
