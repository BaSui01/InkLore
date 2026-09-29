package enginetooling

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateToolArguments 按工具定义的 JSON Schema（Parameters 字段）做轻量校验：
// 顶层 required 必填 + properties 基本类型匹配。LLM 生成的坏参数在进入 handler 前拦截。
func ValidateToolArguments(schema interface{}, raw json.RawMessage) error {
	spec, ok := schema.(map[string]interface{})
	if !ok || len(spec) == 0 {
		return nil // 无 schema 不校验
	}
	if t, _ := spec["type"].(string); t != "" && t != "object" {
		return nil // 仅校验 object 形态的顶层 schema
	}
	var args map[string]interface{}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		trimmed = "{}"
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return fmt.Errorf("参数不是合法 JSON 对象: %w", err)
	}
	if required, ok := spec["required"].([]interface{}); ok {
		for _, item := range required {
			name, _ := item.(string)
			if name == "" {
				continue
			}
			value, exists := args[name]
			if !exists || value == nil {
				return fmt.Errorf("缺少必填参数: %s", name)
			}
			if str, isStr := value.(string); isStr && strings.TrimSpace(str) == "" {
				return fmt.Errorf("必填参数不能为空: %s", name)
			}
		}
	}
	properties, _ := spec["properties"].(map[string]interface{})
	for name, value := range args {
		propSpec, ok := properties[name].(map[string]interface{})
		if !ok {
			continue // 未声明的参数放过（与 OpenAI additionalProperties 默认一致）
		}
		expected, _ := propSpec["type"].(string)
		if expected == "" || value == nil {
			continue
		}
		if err := checkJSONType(name, expected, value); err != nil {
			return err
		}
	}
	return nil
}

func checkJSONType(name, expected string, value interface{}) error {
	ok := false
	switch expected {
	case "string":
		_, ok = value.(string)
	case "integer":
		num, isNum := value.(float64)
		ok = isNum && num == float64(int64(num))
	case "number":
		_, ok = value.(float64)
	case "boolean":
		_, ok = value.(bool)
	case "array":
		_, ok = value.([]interface{})
	case "object":
		_, ok = value.(map[string]interface{})
	default:
		return nil // 未知类型声明不校验
	}
	if !ok {
		return fmt.Errorf("参数 %s 类型应为 %s", name, expected)
	}
	return nil
}
