package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func (c *AgentImageToolDefaults) UnmarshalJSON(data []byte) error {
	type plain AgentImageToolDefaults
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(data, &fields); e != nil {
		return e
	}
	t := reflect.TypeOf(plain{})
	for i := 0; i < t.NumField(); i++ {
		tag := strings.Split(t.Field(i).Tag.Get("json"), ",")
		if len(tag) > 1 && tag[1] == "omitempty" {
			continue
		}
		raw, ok := fields[tag[0]]
		if !ok || (tag[0] != "model_id" && bytes.Equal(bytes.TrimSpace(raw), []byte("null"))) {
			return fmt.Errorf("tool_defaults.generate_image.%s必填", tag[0])
		}
	}
	var value plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&value); e != nil {
		return e
	}
	*c = AgentImageToolDefaults(value)
	return c.Validate()
}
func (c AgentImageToolDefaults) Validate() error {
	if c.ModelID != nil && *c.ModelID <= 0 {
		return fmt.Errorf("图片默认model_id无效")
	}
	if c.BatchCount == nil || c.ImagesPerTask == nil || c.ReferenceStrength == nil || c.EnableStream == nil || c.EnableThinking == nil || c.EnableSearch == nil || c.UseAttachedImages == nil {
		return fmt.Errorf("图片默认数字和布尔值必须显式配置")
	}
	if *c.BatchCount < 1 || *c.ImagesPerTask < 1 || !finiteRange(*c.ReferenceStrength, 0, 1) {
		return fmt.Errorf("图片批量数量或参考强度无效")
	}
	switch c.Mode {
	case "text_to_image", "image_to_image", "reference_to_image", "image_edit":
	default:
		return fmt.Errorf("图片生成mode无效")
	}
	switch c.ExecutionAPI {
	case "workflow", "responses", "responses_image_tool", "openai_responses_image_tool":
	default:
		return fmt.Errorf("图片执行API无效")
	}
	if strings.TrimSpace(c.AspectRatio) == "" || strings.TrimSpace(c.ImageSize) == "" || strings.TrimSpace(c.Quality) == "" {
		return fmt.Errorf("图片比例、尺寸和质量必须显式配置")
	}
	if (c.Width == nil) != (c.Height == nil) || c.Width != nil && (*c.Width <= 0 || *c.Height <= 0) {
		return fmt.Errorf("图片像素宽高必须成对正数")
	}
	if c.OutputCompression != nil && (*c.OutputCompression < 0 || *c.OutputCompression > 100) {
		return fmt.Errorf("图片压缩必须在0-100之间")
	}
	if c.PartialImages != nil && (*c.PartialImages < 0 || *c.PartialImages > 3) {
		return fmt.Errorf("partial_images必须在0-3之间")
	}
	if e := uniqueNames(c.ResponseModalities); e != nil {
		return e
	}
	hasImage := false
	for _, m := range c.ResponseModalities {
		hasImage = hasImage || m == "IMAGE"
	}
	if !hasImage {
		return fmt.Errorf("图片输出模态必须包含IMAGE")
	}
	if c.ExecutionAPI != "workflow" {
		if strings.TrimSpace(c.ResponsesProvider) == "" || strings.TrimSpace(c.ResponsesModel) == "" || c.PartialImages == nil {
			return fmt.Errorf("Responses图片默认配置缺少路由或partial_images")
		}
		switch c.ResponsesAction {
		case "generate", "edit", "auto":
		default:
			return fmt.Errorf("Responses图片action无效")
		}
		if *c.BatchCount != 1 || *c.ImagesPerTask != 1 || *c.ReferenceStrength != 0 || *c.EnableSearch || *c.EnableThinking || c.Style != "" || c.InputFidelity != "" || c.OutputCompression != nil || len(c.ResponseModalities) != 1 {
			return fmt.Errorf("Responses图片默认配置包含该执行端口不支持的参数")
		}
	}
	return nil
}
func (c AgentToolDefaults) Validate(generateImageEnabled bool) error {
	if c.GenerateImage == nil {
		// 生图默认参数不再强制绑定；允许由 Agent 结合提示词自主决定可选参数
		return nil
	}
	return c.GenerateImage.Validate()
}
