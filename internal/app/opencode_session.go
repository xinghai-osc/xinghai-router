package app

import (
	"context"
	"strings"
)

func opencodeSessionID(ctx context.Context, provided string) string {
	if value := strings.TrimSpace(provided); value != "" && len(value) <= 256 {
		return value
	}
	if value := strings.TrimSpace(requestID(ctx)); value != "" {
		return value
	}
	return randomIDString()
}
