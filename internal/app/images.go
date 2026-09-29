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
	"strconv"
	"strings"
	"time"
)

type imageGatewayOptions struct {
	path        string
	contentType string
	image       bool
	count       int64
	size        string
	quality     string
}

type imageGatewayOptionsKey struct{}

func withImageGatewayOptions(r *http.Request, options imageGatewayOptions) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), imageGatewayOptionsKey{}, options))
}

func imageGatewayOptionsFromContext(ctx context.Context) imageGatewayOptions {
	options, _ := ctx.Value(imageGatewayOptionsKey{}).(imageGatewayOptions)
	return options
}

func (s *Service) readImageBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	return readRequestBody(w, r, s.cfg.RequestBodyTimeout)
}

func (s *Service) imageGenerations(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	body, err := s.readImageBody(w, r)
	if err != nil {
		s.rejectRequestBody(w, r, err, started)
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
	options, err := imageBillingOptions(body, "application/json")
	if err != nil {
		s.logReject(policyCtx, request.Model, http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	options.path = "/v1/images/generations"
	r = withImageGatewayOptions(r.WithContext(policyCtx), options)
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
	body, err := s.readImageBody(w, r)
	if err != nil {
		s.rejectRequestBody(w, r, err, started)
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
	options, err := imageBillingOptions(body, contentType)
	if err != nil {
		s.logReject(policyCtx, model, http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	options.path = "/v1/images/edits"
	r = withImageGatewayOptions(r.WithContext(policyCtx), options)
	s.proxyChatCompletions(w, r, body, model, false, 0, nil, nil, nil, nil, nil, nil)
}

func imageBillingOptions(body []byte, contentType string) (imageGatewayOptions, error) {
	options := imageGatewayOptions{image: true, contentType: contentType, count: 1, size: "default", quality: "default"}
	fields := map[string]string{}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return options, errInvalid
	}
	if mediaType == "application/json" {
		var request struct {
			N       *int64 `json:"n"`
			Size    string `json:"size"`
			Quality string `json:"quality"`
			Stream  bool   `json:"stream"`
		}
		if json.Unmarshal(body, &request) != nil || request.Stream {
			return options, fmt.Errorf("image billing requires a non-streaming image request")
		}
		if request.N != nil {
			options.count = *request.N
		}
		fields["size"], fields["quality"] = request.Size, request.Quality
	} else if mediaType == "multipart/form-data" && params["boundary"] != "" {
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return options, errInvalid
			}
			name := part.FormName()
			if name == "n" || name == "size" || name == "quality" || name == "stream" {
				if _, exists := fields[name]; exists || part.FileName() != "" {
					part.Close()
					return options, fmt.Errorf("duplicate or invalid image billing field")
				}
				value, err := io.ReadAll(io.LimitReader(part, 129))
				if err != nil || len(value) > 128 {
					part.Close()
					return options, errInvalid
				}
				fields[name] = strings.TrimSpace(string(value))
			}
			part.Close()
		}
		if n, exists := fields["n"]; exists {
			options.count, err = strconv.ParseInt(n, 10, 64)
			if err != nil {
				return options, fmt.Errorf("n must be an integer between 1 and 100")
			}
		}
		if value := fields["stream"]; value != "" && value != "false" {
			return options, fmt.Errorf("image billing requires a non-streaming image request")
		}
	} else {
		return options, errInvalid
	}
	if options.count < 1 || options.count > 100 {
		return options, fmt.Errorf("n must be an integer between 1 and 100")
	}
	for name, target := range map[string]*string{"size": &options.size, "quality": &options.quality} {
		value := strings.TrimSpace(fields[name])
		if len(value) > 64 || strings.ContainsAny(value, "*\r\n\t") {
			return options, fmt.Errorf("invalid image %s", name)
		}
		if value != "" {
			*target = value
		}
	}
	return options, nil
}

func imageBillingOverrides(overrides channelRequestOverrides) bool {
	for _, field := range overrides.Delete {
		if field == "n" || field == "size" || field == "quality" || field == "stream" {
			return true
		}
	}
	for _, field := range []string{"n", "size", "quality", "stream"} {
		if _, exists := overrides.Set[field]; exists {
			return true
		}
	}
	return false
}

func (o imageGatewayOptions) reservationFacts() UsageFacts {
	return UsageFacts{ImageCount: o.count, ImageSize: o.size, ImageQuality: o.quality, UsageSource: "request_estimate"}
}

func imageResponseFacts(body []byte, options imageGatewayOptions, facts UsageFacts) (UsageFacts, error) {
	var response struct {
		Data []struct {
			URL    string `json:"url"`
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &response) != nil || len(response.Data) == 0 || int64(len(response.Data)) > options.count {
		return facts, fmt.Errorf("upstream image response has no valid output count or exceeds requested n")
	}
	for _, image := range response.Data {
		if strings.TrimSpace(image.URL) == "" && strings.TrimSpace(image.Base64) == "" {
			return facts, fmt.Errorf("upstream image response contains an empty image")
		}
	}
	facts.ImageCount = int64(len(response.Data))
	facts.ImageSize, facts.ImageQuality = options.size, options.quality
	if facts.UsageSource == "upstream" {
		facts.UsageSource = "upstream_and_response_count"
	} else {
		facts.UsageSource = "response_count"
	}
	return facts, nil
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
		data, readErr := io.ReadAll(part)
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
