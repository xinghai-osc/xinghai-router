package app

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	maxAllowedModels    = 200
	maxAllowedModelName = 200
)

// allowedModelPattern accepts the characters a model name or wildcard pattern
// may contain. The '*' wildcard is matched separately by matchModelPattern.
var allowedModelPattern = regexp.MustCompile(`^[A-Za-z0-9._:@/*-]+$`)

// normalizeAllowedModels trims and de-duplicates the model restrictions of an
// API key. An empty result means the key may call every model its group
// reaches.
func normalizeAllowedModels(input []string) ([]string, error) {
	if len(input) == 0 {
		return []string{}, nil
	}
	models := make([]string, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, raw := range input {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if len(value) > maxAllowedModelName {
			return nil, fmt.Errorf("model name must be at most %d characters", maxAllowedModelName)
		}
		if !allowedModelPattern.MatchString(value) {
			return nil, fmt.Errorf("invalid model name %q", value)
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		models = append(models, value)
	}
	if len(models) > maxAllowedModels {
		return nil, fmt.Errorf("at most %d models are allowed", maxAllowedModels)
	}
	return models, nil
}

// modelAllowed reports whether a key restricted to the given models may call
// the requested model. An empty restriction list allows every model.
func modelAllowed(allowed []string, model string) bool {
	if len(allowed) == 0 {
		return true
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	for _, pattern := range allowed {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" {
			return true
		}
		if !strings.Contains(pattern, "*") {
			if strings.EqualFold(pattern, model) {
				return true
			}
			continue
		}
		if matchModelPattern(pattern, model) {
			return true
		}
	}
	return false
}

// matchModelPattern matches a model name against a pattern whose '*' stands for
// any sequence of characters.
func matchModelPattern(pattern, value string) bool {
	parts := strings.Split(strings.ToLower(pattern), "*")
	if len(parts) == 1 {
		return strings.EqualFold(pattern, value)
	}
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, parts[0]) {
		return false
	}
	rest := lower[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		index := strings.Index(rest, part)
		if index < 0 {
			return false
		}
		rest = rest[index+len(part):]
	}
	return strings.HasSuffix(rest, parts[len(parts)-1])
}
