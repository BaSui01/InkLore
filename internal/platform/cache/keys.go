package cache

import (
	"encoding/json"
	"fmt"
)

// SerializeValue 序列化缓存值
func SerializeValue(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to serialize cache value: %w", err)
	}
	return string(data), nil
}

// DeserializeValue 反序列化缓存值
func DeserializeValue(data string, target interface{}) error {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		return fmt.Errorf("failed to deserialize cache value: %w", err)
	}
	return nil
}
