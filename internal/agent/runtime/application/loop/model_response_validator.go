package engineloop

import (
	"errors"
	"net"
	"strings"
)

func IsRetryableModelError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	text := strings.ToLower(err.Error())
	retryableMarkers := []string{
		"429",
		"rate limit",
		"too many requests",
		"timeout",
		"deadline exceeded",
		"connection reset",
		"connection refused",
		"temporary",
		" 500",
		" 502",
		" 503",
		" 504",
		"status 500",
		"status 502",
		"status 503",
		"status 504",
		"empty response",
		"truncated",
	}
	for _, marker := range retryableMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func IsEmptyModelResponse(content string, toolCalls int) bool {
	return strings.TrimSpace(content) == "" && toolCalls == 0
}
