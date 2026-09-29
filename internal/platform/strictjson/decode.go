// Package strictjson validates and decodes JSON without HTTP dependencies.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

var (
	// ErrInvalid reports malformed JSON without including untrusted input.
	ErrInvalid = errors.New("strictjson: invalid JSON")
	// ErrTooLarge reports input that exceeds the caller-provided byte limit.
	ErrTooLarge = errors.New("strictjson: input exceeds size limit")
	// ErrDepthExceeded reports JSON nested beyond the caller-provided limit.
	ErrDepthExceeded = errors.New("strictjson: nesting exceeds depth limit")
)

// Decode validates raw JSON, preserving JSON numbers as json.Number. It rejects
// invalid UTF-8, duplicate object members, trailing input, and excess nesting.
func Decode(raw []byte, limit, depth int) (any, error) {
	if limit < 0 || len(raw) > limit {
		return nil, ErrTooLarge
	}
	if depth < 1 || !utf8.Valid(raw) {
		return nil, ErrInvalid
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 0, depth)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder, currentDepth, maxDepth int) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, ErrInvalid
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	if currentDepth >= maxDepth {
		return nil, ErrDepthExceeded
	}

	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return nil, ErrInvalid
			}
			key, ok := token.(string)
			if !ok {
				return nil, ErrInvalid
			}
			if _, exists := object[key]; exists {
				return nil, ErrInvalid
			}
			value, err := decodeValue(decoder, currentDepth+1, maxDepth)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
			return nil, ErrInvalid
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeValue(decoder, currentDepth+1, maxDepth)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
			return nil, ErrInvalid
		}
		return array, nil
	default:
		return nil, ErrInvalid
	}
}
