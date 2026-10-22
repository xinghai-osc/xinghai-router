package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const minimaxVideoPath = "/v2/video_generation"

type minimaxVideoRequest struct {
	Model   string `json:"model"`
	Content []struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Role     string          `json:"role"`
		ImageURL json.RawMessage `json:"image_url"`
		VideoURL json.RawMessage `json:"video_url"`
		AudioURL json.RawMessage `json:"audio_url"`
	} `json:"content"`
	Resolution    string          `json:"resolution"`
	Duration      int             `json:"duration"`
	Ratio         string          `json:"ratio"`
	Stream        bool            `json:"stream"`
	CallbackURL   string          `json:"callback_url"`
	Extra         json.RawMessage `json:"extra"`
	AIGCWatermark json.RawMessage `json:"aigc_watermark"`
}

func validateMiniMaxVideoRequest(body []byte) (minimaxVideoRequest, error) {
	var request minimaxVideoRequest
	if json.Unmarshal(body, &request) != nil {
		return request, fmt.Errorf("invalid MiniMax video request")
	}
	request.Model = strings.TrimSpace(request.Model)
	if request.CallbackURL != "" {
		return request, fmt.Errorf("callback_url is not supported; poll the gateway task ID for status updates")
	}
	if len(request.Extra) > 0 {
		var extra map[string]json.RawMessage
		if json.Unmarshal(request.Extra, &extra) != nil || extra == nil {
			return request, fmt.Errorf("extra must be a JSON object")
		}
	}
	if len(request.AIGCWatermark) > 0 {
		var watermark *bool
		if json.Unmarshal(request.AIGCWatermark, &watermark) != nil || watermark == nil {
			return request, fmt.Errorf("aigc_watermark must be a boolean")
		}
	}
	if !validModelName(request.Model) || len(request.Content) == 0 || len(request.Content) > 16 || request.Stream {
		return request, fmt.Errorf("model and content are required; video tasks do not support streaming")
	}
	if request.Duration < 4 || request.Duration > 15 || request.Model == "MiniMax-H3-Max" && request.Duration < 5 {
		return request, fmt.Errorf("duration must be 4-15 seconds (5-15 for MiniMax-H3-Max)")
	}
	if request.Resolution != "480P" && request.Resolution != "768P" && request.Resolution != "2K" {
		return request, fmt.Errorf("resolution must be 480P, 768P, or 2K")
	}
	if request.Model == "MiniMax-H3" && request.Resolution == "480P" || request.Model == "MiniMax-H3-Max" && request.Resolution == "2K" {
		return request, fmt.Errorf("resolution is not supported by this MiniMax model")
	}
	text, media := false, false
	frames, references := false, false
	for _, item := range request.Content {
		var value json.RawMessage
		switch item.Type {
		case "text":
			text = text || strings.TrimSpace(item.Text) != ""
			continue
		case "image_url":
			value = item.ImageURL
			if item.Role != "" && item.Role != "first_frame" && item.Role != "last_frame" && item.Role != "reference_image" {
				return request, fmt.Errorf("invalid image role")
			}
			frames = frames || item.Role != "reference_image"
		case "video_url":
			value = item.VideoURL
			if item.Role != "reference_video" {
				return request, fmt.Errorf("video role must be reference_video")
			}
		case "audio_url":
			value = item.AudioURL
			if item.Role != "reference_audio" {
				return request, fmt.Errorf("audio role must be reference_audio")
			}
		default:
			return request, fmt.Errorf("unsupported content type")
		}
		var urlValue struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(value, &urlValue) != nil || strings.TrimSpace(urlValue.URL) == "" {
			return request, fmt.Errorf("media content must contain a non-empty url")
		}
		media = true
		references = references || strings.HasPrefix(item.Role, "reference_")
	}
	if !text || frames && references {
		return request, fmt.Errorf("a non-empty text item is required; frame and reference inputs cannot be mixed")
	}
	validRatio := map[string]bool{"": true, "adaptive": true, "21:9": true, "16:9": true, "4:3": true, "1:1": true, "3:4": true, "9:16": true}
	if !validRatio[request.Ratio] || !media && (request.Ratio == "" || request.Ratio == "adaptive") {
		return request, fmt.Errorf("text-to-video requires an explicit supported ratio")
	}
	return request, nil
}

func (s *Service) minimaxVideoGenerations(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	body, err := s.readImageBody(w, r)
	if err != nil {
		s.rejectRequestBody(w, r, err, started)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, 400, "invalid_request", "Content-Type must be application/json")
		return
	}
	request, err := validateMiniMaxVideoRequest(body)
	if err != nil {
		s.logReject(r.Context(), request.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	key := r.Context().Value(contextKey{}).(keyContext)
	allowed, ctx := s.enforceContentPolicy(r.Context(), key, request.Model, minimaxVideoPath, body)
	if !allowed {
		s.logReject(ctx, request.Model, 400, "content_policy_violation", started)
		writeError(w, 400, "content_policy_violation", "request content violates the content policy")
		return
	}
	options := mediaGatewayOptions{kind: "video", path: minimaxVideoPath, contentType: "application/json", minimaxNative: true, facts: UsageFacts{VideoSeconds: float64(request.Duration), UsageSource: "accepted_request_seconds"}}
	ctx = context.WithValue(ctx, mediaGatewayOptionsKey{}, options)
	s.proxyChatCompletions(w, r.WithContext(ctx), body, request.Model, false, 0, nil, nil, nil, nil, nil, nil)
}

func miniMaxVideoBody(body []byte, contentType string, native bool, model string) ([]byte, error) {
	if native {
		body = rewriteJEVBody(body, model, model)
		_, err := validateMiniMaxVideoRequest(body)
		return body, err
	}
	fields := map[string]json.RawMessage{}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("invalid video content type")
	}
	var reference string
	if mediaType == "multipart/form-data" {
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("invalid video multipart body")
			}
			data, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				return nil, err
			}
			if part.FileName() != "" {
				if part.FormName() != "input_reference" {
					return nil, fmt.Errorf("unsupported video attachment")
				}
				contentType := http.DetectContentType(data)
				if !strings.HasPrefix(contentType, "image/") {
					return nil, fmt.Errorf("input_reference must be an image")
				}
				reference = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
			} else if part.FormName() == "extra" || part.FormName() == "aigc_watermark" {
				if !json.Valid(data) {
					return nil, fmt.Errorf("%s must contain valid JSON", part.FormName())
				}
				fields[part.FormName()] = json.RawMessage(data)
			} else {
				fields[part.FormName()], _ = json.Marshal(string(data))
			}
		}
	} else if json.Unmarshal(body, &fields) != nil || fields == nil {
		return nil, fmt.Errorf("invalid video JSON")
	}
	stringField := func(name string) string { var value string; _ = json.Unmarshal(fields[name], &value); return value }
	seconds := stringField("seconds")
	if seconds == "" {
		seconds = string(fields["seconds"])
	}
	duration := 4
	if seconds != "" {
		duration, err = strconv.Atoi(seconds)
		if err != nil {
			return nil, fmt.Errorf("invalid video seconds")
		}
	}
	resolution := stringField("resolution")
	if resolution == "" {
		resolution = "768P"
	}
	ratio := stringField("ratio")
	if ratio == "" {
		ratio = "16:9"
	}
	if size := stringField("size"); size != "" {
		switch size {
		case "1280x720", "1366x768":
			ratio = "16:9"
		case "720x1280", "768x1366":
			ratio = "9:16"
		case "2048x1152":
			ratio, resolution = "16:9", "2K"
		case "1152x2048":
			ratio, resolution = "9:16", "2K"
		default:
			return nil, fmt.Errorf("unsupported MiniMax video size; use resolution and ratio")
		}
	}
	content := []any{map[string]any{"type": "text", "text": stringField("prompt")}}
	if reference == "" {
		if raw, exists := fields["input_reference"]; exists {
			if json.Unmarshal(raw, &reference) != nil {
				var referenceObject map[string]json.RawMessage
				if json.Unmarshal(raw, &referenceObject) != nil || len(referenceObject) != 1 || json.Unmarshal(referenceObject["url"], &reference) != nil || strings.TrimSpace(reference) == "" {
					return nil, fmt.Errorf("input_reference must be an image URL string or an object containing a non-empty url")
				}
			} else if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				return nil, fmt.Errorf("input_reference must be an image URL string or an object containing a non-empty url")
			}
		}
	}
	if reference != "" {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": reference}, "role": "first_frame"})
		ratio = "adaptive"
	}
	payload := map[string]any{"model": model, "content": content, "duration": duration, "resolution": resolution, "ratio": ratio}
	for _, name := range []string{"extra", "aigc_watermark", "callback_url"} {
		if value, ok := fields[name]; ok {
			payload[name] = value
		}
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	_, err = validateMiniMaxVideoRequest(out)
	return out, err
}

func miniMaxCreatedVideo(body []byte) ([]byte, error) {
	var response struct {
		TaskID   string          `json:"task_id"`
		Error    json.RawMessage `json:"error"`
		BaseResp struct {
			StatusCode int `json:"status_code"`
		} `json:"base_resp"`
	}
	if len(body) > maxVideoTaskBody || json.Unmarshal(body, &response) != nil || !validVideoTaskID(response.TaskID) || response.BaseResp.StatusCode != 0 || len(response.Error) > 0 && string(response.Error) != "null" {
		return nil, fmt.Errorf("invalid MiniMax video creation response")
	}
	return json.Marshal(map[string]any{"id": response.TaskID, "object": "video", "status": "queued"})
}

func miniMaxVideoStatus(body []byte, id string) ([]byte, string, error) {
	var response struct {
		Task struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Status  string `json:"status"`
			Content struct {
				URL string `json:"url"`
			} `json:"content"`
			Duration  int             `json:"duration"`
			CreatedAt int64           `json:"created_at"`
			Error     json.RawMessage `json:"error"`
		} `json:"task"`
	}
	if len(body) > maxVideoTaskBody || json.Unmarshal(body, &response) != nil || response.Task.ID != id {
		return nil, "", fmt.Errorf("invalid MiniMax video task response")
	}
	status := map[string]string{"queued": "queued", "running": "in_progress", "succeeded": "completed", "failed": "failed", "cancelled": "failed"}[response.Task.Status]
	if status == "" {
		return nil, "", fmt.Errorf("invalid MiniMax video task status")
	}
	converted := map[string]any{"id": id, "object": "video", "status": status, "model": response.Task.Model, "seconds": strconv.Itoa(response.Task.Duration), "created_at": response.Task.CreatedAt}
	if response.Task.Content.URL != "" {
		converted["url"] = response.Task.Content.URL
	}
	if len(response.Task.Error) > 0 {
		converted["error"] = response.Task.Error
	}
	out, err := json.Marshal(converted)
	return out, response.Task.Content.URL, err
}

func (s *Service) getMiniMaxVideo(w http.ResponseWriter, r *http.Request) {
	s.proxyVideoTask(w, r, false)
}
func (s *Service) deleteMiniMaxVideo(w http.ResponseWriter, r *http.Request) {
	s.proxyVideoTask(w, r, false)
}
