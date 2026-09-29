package enginetooling

import (
	"encoding/json"
	"strings"
)

const defaultToolTextLimit = 8000

func NormalizeToolOutput(raw string) *ToolOutput {
	return NormalizeToolOutputWithLimit(raw, defaultToolTextLimit)
}

func NormalizeToolOutputWithLimit(raw string, limit int) *ToolOutput {
	if limit <= 0 {
		limit = defaultToolTextLimit
	}
	output := &ToolOutput{
		Status:   ToolOutputStatusOK,
		Metadata: map[string]any{},
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		output.Text = ""
		return output
	}
	if strings.HasPrefix(trimmed, "[MCP Tool Error]") {
		output.Status = ToolOutputStatusError
	}

	if blocks, ok := parseContentBlocks(trimmed); ok {
		output.Blocks = blocks.blocks
		output.Artifacts = blocks.artifacts
		output.Text = truncateText(blocks.text, limit, output.Metadata)
		if blocks.errorLike {
			output.Status = ToolOutputStatusError
		}
		if blocks.binarySkipped > 0 {
			output.Metadata["binary_skipped"] = blocks.binarySkipped
		}
		return output
	}

	var object map[string]any
	if json.Unmarshal([]byte(trimmed), &object) == nil {
		if isErrorObject(object) {
			output.Status = ToolOutputStatusError
		}
		output.Text = truncateText(trimmed, limit, output.Metadata)
		return output
	}

	output.Text = truncateText(raw, limit, output.Metadata)
	return output
}

type parsedBlocks struct {
	text          string
	blocks        []ContentBlock
	artifacts     []ArtifactBlock
	errorLike     bool
	binarySkipped int
}

func parseContentBlocks(raw string) (parsedBlocks, bool) {
	var blocks []map[string]any
	if json.Unmarshal([]byte(raw), &blocks) != nil {
		var wrapped struct {
			Content []map[string]any `json:"content"`
		}
		if json.Unmarshal([]byte(raw), &wrapped) != nil || len(wrapped.Content) == 0 {
			return parsedBlocks{}, false
		}
		blocks = wrapped.Content
	}
	if len(blocks) == 0 {
		return parsedBlocks{}, false
	}

	var result parsedBlocks
	var totalBytes int64
	for _, block := range blocks {
		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			text, _ := block["text"].(string)
			if result.text != "" && text != "" {
				result.text += "\n"
			}
			result.text += text
			result.blocks = append(result.blocks, ContentBlock{Type: "text", Text: text})
			if strings.HasPrefix(strings.TrimSpace(text), "[MCP Tool Error]") {
				result.errorLike = true
			}
		case "image", "file":
			if len(result.artifacts) >= MaxBinaryBlockCount {
				result.binarySkipped++
				continue
			}
			binary := BinaryBlock{
				Type:     blockType,
				Base64:   stringField(block, "base64"),
				MIMEType: stringField(block, "mime_type"),
				Name:     stringField(block, "name"),
			}
			data, err := decodeBinaryBlock(binary)
			if err != nil || totalBytes+int64(len(data)) > MaxBinaryTotalBytes {
				result.binarySkipped++
				continue
			}
			totalBytes += int64(len(data))
			artifact := normalizeBinaryMetadata(binary, int64(len(data)))
			artifact.Data = data
			result.artifacts = append(result.artifacts, artifact)
			result.blocks = append(result.blocks, ContentBlock{
				Type:     blockType,
				Name:     artifact.Name,
				MIMEType: artifact.MIMEType,
				Size:     artifact.Size,
			})
		default:
			return parsedBlocks{}, false
		}
	}
	return result, true
}

func stringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func isErrorObject(object map[string]any) bool {
	if status, _ := object["status"].(string); strings.EqualFold(status, "error") {
		return true
	}
	if _, ok := object["error"]; ok {
		return true
	}
	return false
}

func truncateText(value string, limit int, metadata map[string]any) string {
	if len(value) <= limit {
		return value
	}
	metadata["_truncated"] = true
	metadata["original_length"] = len(value)
	return value[:limit]
}
