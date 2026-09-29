package enginetooling

import (
	"encoding/base64"
	"fmt"
)

const (
	MaxBinaryBlockBytes  = 50 * 1024 * 1024
	MaxBinaryBlockCount  = 4
	MaxBinaryTotalBytes  = 50 * 1024 * 1024
	defaultBinaryMIME    = "application/octet-stream"
	defaultBinaryName    = "artifact"
	defaultImageMIMEType = "image/png"
)

type BinaryBlock struct {
	Type     string `json:"type"`
	Base64   string `json:"base64,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Name     string `json:"name,omitempty"`
}

func DecodeBinaryBlock(block BinaryBlock) ([]byte, error) {
	if block.Base64 == "" {
		return nil, fmt.Errorf("binary block missing base64")
	}
	data, err := base64.StdEncoding.DecodeString(block.Base64)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	if len(data) > MaxBinaryBlockBytes {
		return nil, fmt.Errorf("binary block exceeds %d bytes", MaxBinaryBlockBytes)
	}
	return data, nil
}

func decodeBinaryBlock(block BinaryBlock) ([]byte, error) {
	return DecodeBinaryBlock(block)
}

func NormalizeBinaryMetadata(block BinaryBlock, size int64) ArtifactBlock {
	mimeType := block.MIMEType
	if mimeType == "" {
		if block.Type == "image" {
			mimeType = defaultImageMIMEType
		} else {
			mimeType = defaultBinaryMIME
		}
	}
	name := block.Name
	if name == "" {
		name = defaultBinaryName
	}
	return ArtifactBlock{
		Type:     block.Type,
		Name:     name,
		MIMEType: mimeType,
		Size:     size,
	}
}

func normalizeBinaryMetadata(block BinaryBlock, size int64) ArtifactBlock {
	return NormalizeBinaryMetadata(block, size)
}
