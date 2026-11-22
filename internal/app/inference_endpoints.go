package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type nativeInferencePathKey struct{}

func (s *Service) nativeInference(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	body, err := s.readGatewayBody(w, r)
	if err != nil {
		s.rejectRequestBody(w, r, err, started)
		return
	}
	var req gatewayRequestScan
	if err := json.Unmarshal(body, &req); err != nil || len(body) == 0 || strings.TrimSpace(string(body)) == "null" {
		s.logReject(r.Context(), "", 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "a JSON object is required")
		return
	}
	path := r.URL.Path
	if path == "/beta/completions" {
		path = "/v1/completions"
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" && path == "/v1/moderations" {
		req.Model = "omni-moderation-latest"
		body = rewriteJSONModel(body, req.Model)
	}
	if !validModelName(req.Model) || req.Stream && path != "/v1/completions" {
		s.logReject(r.Context(), req.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "a valid model and a supported response mode are required")
		return
	}
	key := r.Context().Value(contextKey{}).(keyContext)
	allowed, ctx := s.enforceContentPolicy(r.Context(), key, req.Model, path, body)
	if !allowed {
		s.logReject(ctx, req.Model, 400, "content_policy_violation", started)
		writeError(w, 400, "content_policy_violation", "request content violates the content policy")
		return
	}
	ctx = context.WithValue(ctx, nativeInferencePathKey{}, path)
	s.proxyChatCompletions(w, r.WithContext(ctx), body, req.Model, req.Stream, req.MaxTokens, nil, streamResponseDirect, nil, nil, nil, nil)
}
