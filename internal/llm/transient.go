package llm

import (
	"context"
	"errors"
	"strings"
)

func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http 429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "速率限制") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "client.timeout") ||
		errors.Is(err, context.DeadlineExceeded)
}
