package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type mediaGatewayOptions struct {
	path            string
	contentType     string
	kind            string
	minimaxNative   bool
	facts           UsageFacts
	body            []byte
	maxAudioSeconds float64
	promptTokens    int64
}

type mediaGatewayOptionsKey struct{}

func mediaOptionsFromContext(ctx context.Context) mediaGatewayOptions {
	options, _ := ctx.Value(mediaGatewayOptionsKey{}).(mediaGatewayOptions)
	return options
}

func (s *Service) audioSpeech(w http.ResponseWriter, r *http.Request) {
	s.mediaGeneration(w, r, "speech")
}

func (s *Service) audioTranscriptions(w http.ResponseWriter, r *http.Request) {
	s.mediaGeneration(w, r, "transcription")
}

func (s *Service) videoGenerations(w http.ResponseWriter, r *http.Request) {
	s.mediaGeneration(w, r, "video")
}

func (s *Service) mediaGeneration(w http.ResponseWriter, r *http.Request, kind string) {
	started := time.Now()
	body, err := s.readImageBody(w, r)
	if err != nil {
		s.rejectRequestBody(w, r, err, started)
		return
	}
	model, policyBody, options, err := parseMediaRequest(body, r.Header.Get("Content-Type"), kind)
	if err != nil {
		s.logReject(r.Context(), model, http.StatusBadRequest, "invalid_request", started)
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	body = options.body
	options.path = r.URL.Path
	key := r.Context().Value(contextKey{}).(keyContext)
	allowed, ctx := s.enforceContentPolicy(r.Context(), key, model, options.path, policyBody)
	if !allowed {
		s.logReject(ctx, model, http.StatusBadRequest, "content_policy_violation", started)
		writeError(w, http.StatusBadRequest, "content_policy_violation", "request content violates the content policy")
		return
	}
	ctx = context.WithValue(ctx, mediaGatewayOptionsKey{}, options)
	s.proxyChatCompletions(w, r.WithContext(ctx), body, model, false, 0, nil, nil, nil, nil, nil, nil)
}

func parseMediaRequest(body []byte, contentType, kind string) (string, []byte, mediaGatewayOptions, error) {
	options := mediaGatewayOptions{kind: kind, contentType: contentType}
	fail := func(message string) (string, []byte, mediaGatewayOptions, error) {
		return "", nil, options, fmt.Errorf("%s", message)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fail("Content-Type must be application/json or multipart/form-data")
	}
	options.contentType = mime.FormatMediaType(mediaType, params)
	fields := map[string]json.RawMessage{}
	hasFile := false
	var audioDuration float64
	switch mediaType {
	case "application/json":
		if kind == "transcription" || json.Unmarshal(body, &fields) != nil || fields == nil {
			return fail("a valid JSON object is required; audio transcription requires multipart/form-data")
		}
	case "multipart/form-data":
		if kind == "speech" || params["boundary"] == "" {
			return fail("speech requires application/json; multipart requests require a boundary")
		}
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		seen := map[string]bool{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fail("invalid multipart body")
			}
			name := part.FormName()
			if name == "" || seen[name] {
				part.Close()
				return fail("multipart fields must have unique names")
			}
			seen[name] = true
			if part.FileName() != "" {
				if kind == "transcription" && name != "file" || kind == "video" && name != "input_reference" {
					part.Close()
					return fail("unexpected multipart file field")
				}
				data, err := io.ReadAll(part)
				part.Close()
				if err != nil || len(data) == 0 {
					return fail("media file must not be empty")
				}
				if kind == "transcription" {
					audioDuration, err = mediaAudioDuration(data)
					if err != nil {
						return fail("audio must be a valid PCM/IEEE-float WAV or MP3 file")
					}
				}
				hasFile = true
				continue
			}
			value, err := io.ReadAll(io.LimitReader(part, (64<<10)+1))
			part.Close()
			if err != nil || len(value) > 64<<10 || !utf8.Valid(value) {
				return fail("invalid or oversized multipart field")
			}
			fields[name], _ = json.Marshal(string(value))
		}
	default:
		return fail("Content-Type must be application/json or multipart/form-data")
	}
	text := func(name string) string {
		var value string
		_ = json.Unmarshal(fields[name], &value)
		return value
	}
	model := strings.TrimSpace(text("model"))
	if !validModelName(model) {
		return fail("model must be 1-200 characters")
	}
	if raw, ok := fields["stream"]; ok && string(raw) != "false" && string(raw) != `"false"` {
		return fail("streaming media requests are not supported")
	}
	switch kind {
	case "speech":
		input := text("input")
		if strings.TrimSpace(input) == "" || !utf8.ValidString(input) || utf8.RuneCountInString(input) > 4096 || strings.TrimSpace(text("voice")) == "" {
			return fail("input (1-4096 characters) and voice are required")
		}
		if format := text("stream_format"); format != "" && format != "audio" {
			return fail("speech stream_format must be audio")
		}
		options.facts = UsageFacts{AudioCharacters: int64(utf8.RuneCountInString(input)), UsageSource: "request_characters"}
	case "transcription":
		options.maxAudioSeconds = 600
		if raw, exists := fields["max_audio_seconds"]; exists {
			value := text("max_audio_seconds")
			if value == "" {
				value = string(raw)
			}
			options.maxAudioSeconds, err = strconv.ParseFloat(value, 64)
			if err != nil || !validPositiveFinite(options.maxAudioSeconds) || options.maxAudioSeconds < 1 || options.maxAudioSeconds > 3600 {
				return fail("max_audio_seconds must be between 1 and 3600")
			}
		}
		if audioDuration > options.maxAudioSeconds {
			return fail("audio file duration exceeds max_audio_seconds")
		}
		options.promptTokens = int64(utf8.RuneCountInString(text("prompt")))
		if !hasFile {
			return fail("an audio file is required")
		}
		if format := text("response_format"); format != "" && format != "json" && format != "verbose_json" {
			return fail("audio response_format must be json or verbose_json for usage accounting")
		}
	case "video":
		if strings.TrimSpace(text("prompt")) == "" {
			return fail("prompt is required")
		}
		seconds := float64(4)
		if raw, ok := fields["seconds"]; ok {
			value := text("seconds")
			if value == "" {
				value = string(raw)
			}
			seconds, err = strconv.ParseFloat(value, 64)
			if err != nil || !validPositiveFinite(seconds) || seconds > 3600 || seconds != float64(int64(seconds)) {
				return fail("seconds must be an integer between 1 and 3600")
			}
		}
		options.facts = UsageFacts{VideoSeconds: seconds, UsageSource: "accepted_request_seconds"}
	default:
		return fail("unsupported media operation")
	}
	policyBody, _ := json.Marshal(fields)
	set := map[string]string{"model": model}
	if kind == "video" {
		set["seconds"] = strconv.FormatFloat(options.facts.VideoSeconds, 'f', 0, 64)
	}
	options.body, options.contentType, err = rewriteMediaFields(body, options.contentType, set, "max_audio_seconds")
	if err != nil {
		return fail("could not normalize media request")
	}
	return model, policyBody, options, nil
}

func (o mediaGatewayOptions) responseFacts(body []byte, contentType string, pricing pricingRule) (UsageFacts, error) {
	switch o.kind {
	case "speech":
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || !(strings.HasPrefix(mediaType, "audio/") || mediaType == "application/octet-stream") || len(body) == 0 {
			return UsageFacts{}, fmt.Errorf("upstream did not return audio")
		}
		return parseUsageFacts(body), nil
	case "video":
		return parseUsageFacts(body), nil
	case "transcription":
		var response struct {
			Text     *string  `json:"text"`
			Duration *float64 `json:"duration"`
		}
		if json.Unmarshal(body, &response) != nil || response.Text == nil {
			return UsageFacts{}, fmt.Errorf("upstream did not return a transcription object")
		}
		facts := parseUsageFacts(body)
		if facts.AudioSeconds == 0 && response.Duration != nil {
			if !validNonNegativeFinite(*response.Duration) {
				return UsageFacts{}, fmt.Errorf("invalid audio duration")
			}
			facts.AudioSeconds = *response.Duration
			facts.UsageSource = "upstream_duration"
		}
		if facts.AudioSeconds > o.maxAudioSeconds {
			return UsageFacts{}, fmt.Errorf("upstream audio duration exceeds max_audio_seconds")
		}
		facts.AudioSeconds = 0
		if facts.UsageSource == "upstream_duration" {
			facts.UsageSource = "missing"
		}
		return facts, nil
	}
	return UsageFacts{}, errInvalid
}

func (o mediaGatewayOptions) reservationFacts(pricing pricingRule) (UsageFacts, error) {
	switch o.kind {
	case "speech":
		tokens := int64(math.Ceil(float64(o.facts.AudioCharacters) / 4))
		return UsageFacts{InputTokens: tokens, UsageSource: "request_estimate"}, nil
	case "transcription":
		tokens := int64(math.Ceil(o.maxAudioSeconds * 100))
		return UsageFacts{InputTokens: o.promptTokens + tokens, UsageSource: "request_estimate"}, nil
	case "video":
		tokens := int64(math.Ceil(o.facts.VideoSeconds * 100))
		return UsageFacts{OutputTokens: tokens, UsageSource: "request_estimate"}, nil
	default:
		return UsageFacts{}, fmt.Errorf("unsupported media operation")
	}
}

func rewriteMediaFields(body []byte, contentType string, set map[string]string, remove string) ([]byte, string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", err
	}
	if mediaType == "application/json" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			return nil, "", err
		}
		delete(fields, remove)
		for name, value := range set {
			fields[name], _ = json.Marshal(value)
		}
		result, err := json.Marshal(fields)
		return result, "application/json", err
	}
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		_, replaced := set[part.FormName()]
		if replaced || part.FormName() == remove {
			part.Close()
			continue
		}
		dest, err := writer.CreatePart(part.Header)
		if err == nil {
			_, err = io.Copy(dest, part)
		}
		part.Close()
		if err != nil {
			return nil, "", err
		}
	}
	for name, value := range set {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return out.Bytes(), writer.FormDataContentType(), nil
}
