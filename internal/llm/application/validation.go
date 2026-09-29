package application

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"InkLore/internal/llm/domain"
	"InkLore/internal/platform/strictjson"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// DecodeStrictJSON rejects duplicate properties before decoding, including nested
// objects and escaped spellings of the same property. No payload enters errors.
func DecodeStrictJSON(raw []byte, limit, depth int) (any, error) {
	value, err := strictjson.Decode(raw, limit, depth)
	if err != nil {
		return nil, domain.ErrStructured
	}
	return value, nil
}

type StrictOutput struct{ schema *jsonschema.Schema }
type denySchemaLoader struct{}

func (denySchemaLoader) Load(string) (any, error) { return nil, domain.ErrStructured }

// CompileOutput enforces the bounded Draft2020-12 profile. RE2 avoids regex
// backtracking; recursive references are rejected to bound validation work.
func CompileOutput(raw []byte) (*StrictOutput, error) {
	value, err := DecodeStrictJSON(raw, 32<<10, 16)
	if err != nil {
		return nil, domain.ErrStructured
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, domain.ErrStructured
	}
	if root["type"] != "object" && root["type"] != "array" {
		return nil, domain.ErrStructured
	}
	nodes := 0
	if err := checkOutputSchema(value, root, map[string]bool{}, 0, &nodes); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(denySchemaLoader{})
	c.UseRegexpEngine(func(pattern string) (jsonschema.Regexp, error) { return regexp.Compile(pattern) })
	if c.AddResource("urn:inklore:output", value) != nil {
		return nil, domain.ErrStructured
	}
	schema, err := c.Compile("urn:inklore:output")
	if err != nil {
		return nil, domain.ErrStructured
	}
	return &StrictOutput{schema}, nil
}
func (s *StrictOutput) Validate(raw []byte) error {
	if s == nil || s.schema == nil {
		return domain.ErrStructured
	}
	value, err := DecodeStrictJSON(raw, 2<<20, 64)
	if err != nil {
		return domain.ErrStructured
	}
	if s.schema.Validate(value) != nil {
		return domain.ErrStructured
	}
	return nil
}
func checkOutputSchema(value any, root map[string]any, active map[string]bool, depth int, nodes *int) error {
	*nodes++
	if depth > 16 || *nodes > 4096 {
		return domain.ErrStructured
	}
	if _, ok := value.(bool); ok {
		return nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return domain.ErrStructured
	}
	for key, val := range obj {
		switch key {
		case "$schema":
			if val != "https://json-schema.org/draft/2020-12/schema" {
				return domain.ErrStructured
			}
		case "$ref":
			ref, ok := val.(string)
			if !ok || !strings.HasPrefix(ref, "#/$defs/") || active[ref] {
				return domain.ErrStructured
			}
			var target any = root
			for _, part := range strings.Split(ref[2:], "/") {
				m, ok := target.(map[string]any)
				if !ok {
					return domain.ErrStructured
				}
				target = m[strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")]
			}
			active[ref] = true
			if err := checkOutputSchema(target, root, active, depth+1, nodes); err != nil {
				return err
			}
			delete(active, ref)
		case "properties", "patternProperties", "$defs", "dependentSchemas":
			children, ok := val.(map[string]any)
			if !ok {
				return domain.ErrStructured
			}
			for _, child := range children {
				if err := checkOutputSchema(child, root, active, depth+1, nodes); err != nil {
					return err
				}
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			children, ok := val.([]any)
			if !ok {
				return domain.ErrStructured
			}
			for _, child := range children {
				if err := checkOutputSchema(child, root, active, depth+1, nodes); err != nil {
					return err
				}
			}
		case "items", "contains", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "propertyNames", "not", "if", "then", "else":
			if err := checkOutputSchema(val, root, active, depth+1, nodes); err != nil {
				return err
			}
		case "type", "enum", "const", "required", "dependentRequired", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "pattern", "format", "minItems", "maxItems", "uniqueItems", "minContains", "maxContains", "minProperties", "maxProperties", "title", "description", "default", "examples", "readOnly", "writeOnly", "deprecated", "$comment":
		default:
			return domain.ErrStructured
		}
	}
	return nil
}

// 结构化生成（INK-065）。
//
// 这是「让模型稳定吐出一个结构」的单一入口：先试上游原生 json_schema 约束，
// 上游不支持时自动回退到 prompt 约束 + 解析器 + 有限重试，并把**实际用了哪条路径**
// 写在结果里。调用方（大纲、事实抽取、评分）不再需要自己拼 JSON 提示词与容错逻辑。
//
// 失败语义：所有尝试都解析不出合法 JSON 时返回错误，**不返回半成品**——
// 调用方拿到的一定是解析并形状校验通过的结果，或者是明确失败。

// 默认重试次数（不含首次）。
const defaultStructuredRetries = 2

// StructuredRequest 是一次结构化生成请求。
type StructuredRequest struct {
	Model        string
	SystemPrompt string
	Messages     []domain.ChatMessage
	// Schema 是 JSON Schema（map 形式）；同时用于上游约束与本地形状校验。
	Schema any
	// SchemaName 是 json_schema 约束的名字，上游要求必填。
	SchemaName  string
	Temperature float64
	MaxTokens   int
	RequestID   string
	// MaxRetries 是解析失败后的重试次数（不含首次）；0 表示用默认值，负数表示不重试。
	MaxRetries int
	// Mode 见 domain.StructuredMode*；空值按 auto 处理。
	Mode string
}

// StructuredResult 是一次结构化生成的结果。
type StructuredResult struct {
	// Content 是模型的原始输出，保留以便排障与审计。
	Content string
	// JSON 是通过解析与形状校验的 JSON 文本。
	JSON string
	// Value 是 JSON 反序列化后的对象，便于调用方直接取字段。
	Value    map[string]any
	Model    string
	Path     string
	Attempts int
	Usage    *domain.TokenUsage
	// FallbackReason 记录了从 json_schema 退回 prompt 约束的原因；未回退时为空。
	FallbackReason string
}

// GenerateStructured 执行一次结构化生成。
func (s *GatewayService) GenerateStructured(ctx context.Context, request StructuredRequest) (StructuredResult, error) {
	if s == nil {
		return StructuredResult{}, fmt.Errorf("模型网关未装配")
	}
	mode, err := domain.NormalizeStructuredMode(request.Mode)
	if err != nil {
		return StructuredResult{}, err
	}
	if strings.TrimSpace(request.Model) == "" {
		return StructuredResult{}, fmt.Errorf("结构化生成必须指定模型")
	}
	if request.Schema == nil {
		return StructuredResult{}, fmt.Errorf("结构化生成必须提供 JSON Schema")
	}
	retries := request.MaxRetries
	if retries == 0 {
		retries = defaultStructuredRetries
	}
	if retries < 0 {
		retries = 0
	}

	result := StructuredResult{}
	useSchema := mode != domain.StructuredModePrompt
	if mode == domain.StructuredModePrompt {
		result.Path = domain.StructuredPathPromptConstrained
	}

	messages := append([]domain.ChatMessage{}, request.Messages...)
	var lastErr error

	for attempt := 1; attempt <= retries+1; attempt++ {
		result.Attempts = attempt

		chatRequest := domain.TextGenerationRequest{
			Model:        request.Model,
			SystemPrompt: request.SystemPrompt,
			Messages:     messages,
			Temperature:  request.Temperature,
			MaxTokens:    request.MaxTokens,
			RequestID:    request.RequestID,
		}
		if useSchema {
			chatRequest.ResponseFormat = domain.NewJSONSchemaResponseFormat(request.SchemaName, true, request.Schema)
			result.Path = domain.StructuredPathJSONSchema
		} else {
			chatRequest.SystemPrompt = withJSONInstruction(request.SystemPrompt, request.Schema)
			result.Path = domain.StructuredPathPromptConstrained
		}

		response, callErr := s.GenerateChat(ctx, chatRequest)
		if callErr != nil {
			lastErr = callErr
			// 上游不支持 response_format 时**整体请求会失败**：auto 模式下改用 prompt 约束
			// 重来一次，并记录原因。这是"能力探测"而不是猜——失败是上游给的答案。
			if useSchema && mode == domain.StructuredModeAuto {
				useSchema = false
				result.FallbackReason = callErr.Error()
				continue
			}
			return StructuredResult{}, callErr
		}

		result.Model = response.Model
		result.Content = response.Content
		result.Usage = response.Usage

		jsonText, extractErr := domain.ExtractJSON(response.Content)
		if extractErr == nil {
			if shapeErr := domain.ValidateAgainstSchemaShape(request.Schema, jsonText); shapeErr != nil {
				extractErr = shapeErr
			}
		}
		if extractErr == nil {
			decoded := map[string]any{}
			if unmarshalErr := json.Unmarshal([]byte(jsonText), &decoded); unmarshalErr != nil {
				extractErr = fmt.Errorf("反序列化失败: %w", unmarshalErr)
			} else {
				result.JSON = jsonText
				result.Value = decoded
				return result, nil
			}
		}

		lastErr = extractErr
		// 把错误反馈给模型再试一次；这是"有限重试"，不是无限祈祷。
		messages = append(messages,
			domain.ChatMessage{Role: domain.ChatMessageRoleAssistant, Content: response.Content},
			domain.ChatMessage{Role: domain.ChatMessageRoleUser, Content: correctiveInstruction(extractErr)},
		)
	}
	return StructuredResult{}, fmt.Errorf("结构化生成在 %d 次尝试后仍未得到合法 JSON: %w", result.Attempts, lastErr)
}

// withJSONInstruction 给 prompt 约束路径追加 Schema 说明。
func withJSONInstruction(systemPrompt string, schema any) string {
	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		encoded = []byte(fmt.Sprintf("%v", schema))
	}
	instruction := "你必须只输出一个 JSON 对象，不要包含任何解释文字，也不要使用 Markdown 代码围栏。" +
		"输出必须符合以下 JSON Schema：\n" + string(encoded)
	if strings.TrimSpace(systemPrompt) == "" {
		return instruction
	}
	return systemPrompt + "\n\n" + instruction
}

// correctiveInstruction 把解析失败的原因转成一句可执行的纠正要求。
func correctiveInstruction(err error) string {
	return "上一次输出无法解析为合法 JSON（" + err.Error() + "）。" +
		"请只输出一个合法的 JSON 对象，不要包含解释文字，也不要使用 Markdown 代码围栏。"
}
