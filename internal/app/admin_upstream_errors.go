package app

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var upstreamCredentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[^\s"'<>;,}]+`),
	regexp.MustCompile(`(?i)((?:api[-_ ]?key|authorization|access[-_ ]?token|secret)["']?\s*[:=]\s*["']?)[^\s"'<>;,}]+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+`),
}

func adminErrorDetail(detail string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		encoded, _ := json.Marshal(secret)
		for _, value := range []string{secret, string(encoded[1 : len(encoded)-1]), url.QueryEscape(secret), url.PathEscape(secret)} {
			detail = strings.ReplaceAll(detail, value, "[redacted]")
		}
	}
	for _, pattern := range upstreamCredentialPatterns {
		if pattern.NumSubexp() > 0 {
			detail = pattern.ReplaceAllString(detail, "${1}[redacted]")
		} else {
			detail = pattern.ReplaceAllString(detail, "[redacted]")
		}
	}
	return sanitizeErrorDetail(detail)
}

func upstreamErrorMessage(body []byte) string {
	var payload any
	if json.Unmarshal(body, &payload) == nil {
		if message := upstreamMessageValue(payload, 0); message != "" {
			return message
		}
	}
	return strings.TrimSpace(string(body))
}

func upstreamMessageValue(value any, depth int) string {
	if depth > 5 {
		return ""
	}
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		for _, key := range []string{"error", "message", "detail", "msg"} {
			if message := upstreamMessageValue(value[key], depth+1); message != "" {
				return message
			}
		}
	case []any:
		for _, item := range value {
			if message := upstreamMessageValue(item, depth+1); message != "" {
				return message
			}
		}
	}
	return ""
}

func adminUpstreamFailure(reason string, status int, body []byte, err error, apiKey string) string {
	if reason == "" {
		reason = "upstream_status_" + strconv.Itoa(status)
		if err != nil {
			reason = "upstream_unreachable"
		}
	}
	detail := upstreamErrorMessage(body)
	if err != nil {
		detail = err.Error()
	}
	if detail != "" {
		reason += ": " + detail
	}
	return adminErrorDetail(reason, apiKey)
}
