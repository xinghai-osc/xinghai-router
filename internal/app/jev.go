package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type jevQuestion struct {
	Type         string          `json:"type"`
	Instructions json.RawMessage `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria"`
}

type jevRequest struct {
	Model     string                 `json:"model"`
	State     json.RawMessage        `json:"state"`
	Questions map[string]jevQuestion `json:"questions"`
}

func validateJEVRequest(body []byte) (jevRequest, error) {
	var request jevRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return request, fmt.Errorf("invalid JEV request")
	}
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		return request, fmt.Errorf("request body must be a JSON object")
	}
	request.Model = strings.TrimSpace(request.Model)
	if !validModelName(request.Model) {
		return request, fmt.Errorf("model must be 1-200 characters")
	}
	if !validJEVValue(request.State) {
		return request, fmt.Errorf("state must be a string, object, or array")
	}
	if len(request.Questions) == 0 {
		return request, fmt.Errorf("questions are required")
	}
	for name, question := range request.Questions {
		if strings.TrimSpace(name) == "" || len(name) > 200 {
			return request, fmt.Errorf("question ids must be 1-200 characters")
		}
		if question.Type != "choice" && question.Type != "score" && question.Type != "noul" {
			return request, fmt.Errorf("question %q has an unsupported type", name)
		}
		if !validJEVOptionalValue(question.Instructions) {
			return request, fmt.Errorf("question %q instructions must be a string, object, array, or null", name)
		}
		switch question.Type {
		case "choice":
			var criteria map[string]json.RawMessage
			if json.Unmarshal(question.Criteria, &criteria) != nil || len(criteria) < 1 {
				return request, fmt.Errorf("choice question %q needs at least one criterion", name)
			}
			for criterion, description := range criteria {
				if strings.TrimSpace(criterion) == "" || !validJEVOptionalValue(description) {
					return request, fmt.Errorf("choice question %q has invalid criterion %q", name, criterion)
				}
			}
		case "score":
			var criteria []json.RawMessage
			if json.Unmarshal(question.Criteria, &criteria) != nil || len(criteria) < 1 {
				return request, fmt.Errorf("score question %q needs at least one criterion", name)
			}
			for _, description := range criteria {
				if !validJEVValue(description) {
					return request, fmt.Errorf("score question %q has an invalid criterion", name)
				}
			}
		case "noul":
			criteriaRaw := bytes.TrimSpace(question.Criteria)
			if len(criteriaRaw) > 0 && !bytes.Equal(criteriaRaw, []byte("null")) {
				var criteria map[string]json.RawMessage
				if json.Unmarshal(question.Criteria, &criteria) != nil {
					return request, fmt.Errorf("noul question %q criteria must be an object", name)
				}
				for criterion, description := range criteria {
					if criterion != "true" && criterion != "false" || !validJEVOptionalValue(description) {
						return request, fmt.Errorf("noul question %q has invalid criteria", name)
					}
				}
			}
		}
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil || top == nil {
		return request, fmt.Errorf("invalid JEV request")
	}
	if raw, ok := top["stream"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("false")) {
		return request, fmt.Errorf("JEV streaming is not supported; omit stream or set it to false")
	}
	return request, nil
}

func rewriteJEVBody(body []byte, upstreamModel, requestedModel string) []byte {
	var payload map[string]json.RawMessage
	if json.Unmarshal(body, &payload) != nil || payload == nil {
		return body
	}
	changed := false
	if _, ok := payload["promptCacheKey"]; ok {
		delete(payload, "promptCacheKey")
		changed = true
	}
	desiredModel := requestedModel
	if upstreamModel != "" && upstreamModel != requestedModel {
		desiredModel = upstreamModel
	}
	if desiredModel != "" {
		currentModel := ""
		if raw, ok := payload["model"]; ok {
			_ = json.Unmarshal(raw, &currentModel)
		}
		if currentModel != desiredModel {
			encodedModel, err := json.Marshal(desiredModel)
			if err == nil {
				payload["model"] = encodedModel
				changed = true
			}
		}
	}
	if !changed {
		return body
	}
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

func applyJEVRequestOverrides(body []byte, ov channelRequestOverrides) []byte {
	var payload map[string]json.RawMessage
	if json.Unmarshal(body, &payload) != nil || payload == nil {
		return body
	}
	changed := false
	for _, field := range ov.Delete {
		field = strings.TrimSpace(field)
		if _, exists := payload[field]; exists {
			delete(payload, field)
			changed = true
		}
	}
	for field, value := range ov.Set {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return body
		}
		payload[field] = encoded
		changed = true
	}
	if _, exists := payload["stream"]; exists {
		delete(payload, "stream")
		changed = true
	}
	if !changed {
		return body
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return encoded
}

func validJEVValue(raw json.RawMessage) bool {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	return validJEVOptionalValue(raw)
}

func validJEVOptionalValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return true
	}
	if !json.Valid(trimmed) {
		return false
	}
	return trimmed[0] == '"' || trimmed[0] == '[' || trimmed[0] == '{'
}

func appendJEVPolicyText(builder *strings.Builder, raw json.RawMessage) {
	if builder.Len() >= maxPolicyScanBytes || len(raw) == 0 {
		return
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) == nil {
		appendJEVPolicyValue(builder, value)
	}
}

func appendJEVPolicyValue(builder *strings.Builder, value any) {
	if builder.Len() >= maxPolicyScanBytes {
		return
	}
	switch value := value.(type) {
	case string:
		appendPolicyText(builder, value, "text")
	case []any:
		for _, item := range value {
			appendJEVPolicyValue(builder, item)
		}
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendPolicyText(builder, key, "text")
			appendJEVPolicyValue(builder, value[key])
		}
	}
}

func jevPolicyBody(request jevRequest) []byte {
	var text strings.Builder
	appendJEVPolicyText(&text, request.State)
	names := make([]string, 0, len(request.Questions))
	for name := range request.Questions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		question := request.Questions[name]
		appendJEVPolicyText(&text, question.Instructions)
		appendJEVPolicyText(&text, question.Criteria)
	}
	body, _ := json.Marshal(map[string]string{"text": text.String()})
	return body
}

func (s *Service) jevCompletions(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	body, err := s.readGatewayBody(w, r)
	if err != nil {
		s.rejectRequestBody(w, r, err, started)
		return
	}
	request, err := validateJEVRequest(body)
	if err != nil {
		s.logReject(r.Context(), request.Model, http.StatusUnprocessableEntity, "invalid_request", started)
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	key, ok := r.Context().Value(contextKey{}).(keyContext)
	if !ok {
		s.logReject(r.Context(), request.Model, http.StatusUnauthorized, "invalid_request", started)
		writeError(w, http.StatusUnauthorized, "invalid_request", "API key required")
		return
	}
	policyCtx := markContentPolicyEvaluated(withContentPolicyRequest(r.Context(), body, "/v1/systemone"))
	snapshot := s.contentPolicy(policyCtx)
	result := s.evaluateContentPolicy(snapshot, jevPolicyBody(request))
	s.recordContentAudit(policyCtx, key, request.Model, "/v1/systemone", len(body), result, snapshot.Settings)
	if result.Decision == "block" {
		s.logReject(policyCtx, request.Model, http.StatusBadRequest, "content_policy_violation", started)
		writeError(w, http.StatusBadRequest, "content_policy_violation", "request content violates the content policy")
		return
	}
	policyCtx = context.WithValue(policyCtx, upstreamFormatOnlyKey{}, "jev")
	s.proxyChatCompletions(w, r.WithContext(policyCtx), body, request.Model, false, 0, nil, nil, nil, nil, nil, nil)
}
