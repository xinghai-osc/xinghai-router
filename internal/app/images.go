package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

const maxImageRequestBody = 50 << 20

type imageGatewayOptions struct {
	path        string
	contentType string
	image       bool
}

type imageGatewayOptionsKey struct{}

func withImageGatewayOptions(r *http.Request, options imageGatewayOptions) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), imageGatewayOptionsKey{}, options))
}

func imageGatewayOptionsFromContext(ctx context.Context) imageGatewayOptions {
	options, _ := ctx.Value(imageGatewayOptionsKey{}).(imageGatewayOptions)
	return options
}

func readImageBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxImageRequestBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxImageRequestBody {
		return nil, fmt.Errorf("request body exceeds %d bytes", maxImageRequestBody)
	}
	return body, nil
}

func (s *Service) imageGenerations(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	body, err := readImageBody(r)
	if err != nil {
		s.logReject(r.Context(), "", http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is too large or could not be read")
		return
	}
	var request struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &request) != nil {
		s.logReject(r.Context(), "", http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", "model is required")
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	if !validModelName(request.Model) {
		s.logReject(r.Context(), request.Model, http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", "model must be 1-200 characters")
		return
	}
	key := r.Context().Value(contextKey{}).(keyContext)
	allowed, policyCtx := s.enforceContentPolicy(r.Context(), key, request.Model, "/v1/images/generations", body)
	if !allowed {
		s.logReject(policyCtx, request.Model, http.StatusBadRequest, "content_policy_violation", started)
		writeError(w, http.StatusBadRequest, "content_policy_violation", "request content violates the content policy")
		return
	}
	r = withImageGatewayOptions(r.WithContext(policyCtx), imageGatewayOptions{path: "/v1/images/generations", contentType: "application/json", image: true})
	s.proxyChatCompletions(w, r, body, request.Model, false, 0, nil, nil, nil, nil, nil, nil)
}

func (s *Service) imageEdits(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	contentType := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		s.logReject(r.Context(), "", http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", "Content-Type must be multipart/form-data")
		return
	}
	body, err := readImageBody(r)
	if err != nil {
		s.logReject(r.Context(), "", http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is too large or could not be read")
		return
	}
	model, hasFile, err := imageEditFields(body, params["boundary"])
	if err != nil || !hasFile || !validModelName(model) {
		s.logReject(r.Context(), model, http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", "model and image file are required")
		return
	}
	key := r.Context().Value(contextKey{}).(keyContext)
	policyBody, _ := json.Marshal(map[string]string{"model": model})
	allowed, policyCtx := s.enforceContentPolicy(r.Context(), key, model, "/v1/images/edits", policyBody)
	if !allowed {
		s.logReject(policyCtx, model, http.StatusBadRequest, "content_policy_violation", started)
		writeError(w, http.StatusBadRequest, "content_policy_violation", "request content violates the content policy")
		return
	}
	r = withImageGatewayOptions(r.WithContext(policyCtx), imageGatewayOptions{path: "/v1/images/edits", contentType: contentType, image: true})
	s.proxyChatCompletions(w, r, body, model, false, 0, nil, nil, nil, nil, nil, nil)
}

func imageEditFields(body []byte, boundary string) (model string, hasFile bool, err error) {
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, readErr := reader.NextPart()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", false, readErr
		}
		data, readErr := io.ReadAll(io.LimitReader(part, maxImageRequestBody+1))
		part.Close()
		if readErr != nil {
			return "", false, readErr
		}
		if part.FileName() != "" || strings.HasPrefix(strings.ToLower(part.Header.Get("Content-Type")), "image/") {
			hasFile = true
		}
		if part.FormName() == "model" {
			model = strings.TrimSpace(string(data))
		}
	}
	return model, hasFile, nil
}

func rewriteMultipartModel(body []byte, contentType, model string) ([]byte, string) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return body, contentType
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	for {
		part, readErr := reader.NextPart()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return body, contentType
		}
		data, readErr := io.ReadAll(part)
		part.Close()
		if readErr != nil {
			return body, contentType
		}
		header := make(textproto.MIMEHeader, len(part.Header))
		for key, values := range part.Header {
			header[key] = append([]string(nil), values...)
		}
		if part.FormName() == "model" && part.FileName() == "" {
			data = []byte(model)
		}
		created, createErr := writer.CreatePart(header)
		if createErr != nil {
			return body, contentType
		}
		if _, writeErr := created.Write(data); writeErr != nil {
			return body, contentType
		}
	}
	if err := writer.Close(); err != nil {
		return body, contentType
	}
	return out.Bytes(), writer.FormDataContentType()
}
