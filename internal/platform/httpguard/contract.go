package httpguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/requestschema"
	"InkLore/internal/platform/strictjson"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UUIDv4(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.Version() == 4 && id.Variant() == uuid.RFC4122 && id.String() == value
}

const (
	maxJSONBytes = 2 << 20
	maxJSONDepth = 64
)

// RequireEmptyBody rejects payloads for operations whose contract has no body.
// Reading one byte bounds the work even for unknown-length/chunked requests.
func RequireEmptyBody(c *gin.Context) error {
	if c == nil || c.Request == nil {
		return decodeError(nil)
	}
	if c.Request.Body == nil {
		return nil
	}
	defer func() { _ = c.Request.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	if err != nil || len(raw) != 0 {
		return decodeError(err)
	}
	return nil
}

// DecodeJSON preserves the existing handler-facing API.
func DecodeJSON(c *gin.Context, target any) error {
	_, err := DecodeJSONBytes(c, target)
	return err
}

// DecodeJSONBytes reads and validates a JSON request body exactly once before
// decoding it into target. The returned bytes are the validated request body.
func DecodeJSONBytes(c *gin.Context, target any) ([]byte, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, decodeError(nil)
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return nil, decodeError(nil)
	}

	reader := http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBytes)
	defer func() { _ = reader.Close() }()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, decodeError(err)
	}
	value, err := strictjson.Decode(raw, maxJSONBytes, maxJSONDepth)
	if err != nil {
		return nil, decodeError(err)
	}
	if err := requestschema.Validate(c.Request.Method, c.FullPath(), value); err != nil {
		return nil, decodeError(err)
	}

	typed := json.NewDecoder(bytes.NewReader(raw))
	typed.UseNumber()
	if err := typed.Decode(target); err != nil {
		return nil, decodeError(err)
	}
	return raw, nil
}

func decodeError(err error) error {
	var size *http.MaxBytesError
	if errors.As(err, &size) || errors.Is(err, strictjson.ErrTooLarge) {
		return platformerr.Contract(413, "platform.request_too_large", nil)
	}
	return platformerr.Contract(400, "platform.validation_failed", nil)
}
func ExpectedRevision(value int64) error {
	if value < 1 || value > 9007199254740991 {
		return platformerr.Contract(400, "platform.validation_failed", map[string]any{"field_errors": []map[string]string{{"field": "/expected_revision", "reason": "required_positive_integer"}}})
	}
	return nil
}
func QueryRevision(c *gin.Context) (int64, error) {
	values, ok := c.Request.URL.Query()["expected_revision"]
	if !ok || len(values) != 1 {
		return 0, ExpectedRevision(0)
	}
	value, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return 0, ExpectedRevision(0)
	}
	return value, ExpectedRevision(value)
}
