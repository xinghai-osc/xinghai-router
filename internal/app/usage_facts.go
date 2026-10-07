package app

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
)

const maxUsageFact = int64(1_000_000_000)

type UsageFacts struct {
	InputTokens        int64   `json:"input_tokens"`
	OutputTokens       int64   `json:"output_tokens"`
	CacheReadTokens    int64   `json:"cache_read_tokens"`
	CacheWriteTokens   int64   `json:"cache_write_tokens"`
	CacheWrite5mTokens int64   `json:"cache_write_5m_tokens"`
	CacheWrite1hTokens int64   `json:"cache_write_1h_tokens"`
	ImageCount         int64   `json:"image_count"`
	AudioInputTokens   int64   `json:"audio_input_tokens"`
	AudioOutputTokens  int64   `json:"audio_output_tokens"`
	ImageInputTokens   int64   `json:"image_input_tokens"`
	ImageOutputTokens  int64   `json:"image_output_tokens"`
	ToolCalls          int64   `json:"tool_calls"`
	AudioSeconds       float64 `json:"audio_seconds"`
	VideoSeconds       float64 `json:"video_seconds"`
	UsageSource        string  `json:"usage_source"`
	PricingVersion     string  `json:"pricing_version"`
	ImageSize          string  `json:"image_size"`
	ImageQuality       string  `json:"image_quality"`
	present            bool
	promptPresent      bool
	outputPresent      bool
	inclusiveInput     bool
	inputTotal         int64
	outputTotal        int64
	inputTotalPresent  bool
	outputTotalPresent bool
}

func (f UsageFacts) LegacyTokens() (prompt, cached, completion int) {
	f = normalizeUsageFacts(f)
	p := f.InputTokens + f.CacheReadTokens + f.CacheWriteTokens + f.AudioInputTokens + f.ImageInputTokens
	c := f.OutputTokens + f.AudioOutputTokens + f.ImageOutputTokens
	p = min(p, int64(math.MaxInt32))
	c = min(c, int64(math.MaxInt32)-p)
	return int(p), int(min(f.CacheReadTokens, p)), int(c)
}

func (f UsageFacts) HasUsage() bool {
	return f.present || f.UsageSource != "" && f.UsageSource != "missing" ||
		f.InputTokens > 0 || f.OutputTokens > 0 || f.CacheReadTokens > 0 || f.CacheWriteTokens > 0 ||
		f.CacheWrite5mTokens > 0 || f.CacheWrite1hTokens > 0 || f.ImageCount > 0 ||
		f.AudioInputTokens > 0 || f.AudioOutputTokens > 0 || f.ImageInputTokens > 0 || f.ImageOutputTokens > 0 ||
		f.ToolCalls > 0 || f.AudioSeconds > 0 || f.VideoSeconds > 0
}

func usageFactsFromLegacy(prompt, cached, completion int, source string) UsageFacts {
	p := clampUsageInt(int64(prompt))
	c := min(clampUsageInt(int64(cached)), p)
	if source == "" {
		source = "missing"
	}
	return UsageFacts{
		InputTokens: p - c, CacheReadTokens: c, OutputTokens: clampUsageInt(int64(completion)),
		UsageSource: source, present: source != "missing", promptPresent: true, outputPresent: true,
	}
}

func clampUsageInt(n int64) int64 {
	return min(max(n, 0), maxUsageFact)
}

func clampUsageFloat(n float64) float64 {
	if math.IsNaN(n) || n < 0 {
		return 0
	}
	return min(n, float64(maxUsageFact))
}

func normalizeUsageFacts(f UsageFacts) UsageFacts {
	for _, n := range []*int64{&f.InputTokens, &f.OutputTokens, &f.CacheReadTokens, &f.CacheWriteTokens,
		&f.CacheWrite5mTokens, &f.CacheWrite1hTokens, &f.ImageCount, &f.AudioInputTokens,
		&f.AudioOutputTokens, &f.ImageInputTokens, &f.ImageOutputTokens, &f.ToolCalls} {
		*n = clampUsageInt(*n)
	}
	f.CacheWriteTokens = max(f.CacheWriteTokens, f.CacheWrite5mTokens+f.CacheWrite1hTokens)
	f.AudioSeconds = clampUsageFloat(f.AudioSeconds)
	f.VideoSeconds = clampUsageFloat(f.VideoSeconds)
	return f
}

func mergeUsageFacts(dst *UsageFacts, next UsageFacts) {
	if !next.HasUsage() {
		return
	}
	next = normalizeUsageFacts(next)
	dst.InputTokens = max(dst.InputTokens, next.InputTokens)
	dst.OutputTokens = max(dst.OutputTokens, next.OutputTokens)
	dst.CacheReadTokens = max(dst.CacheReadTokens, next.CacheReadTokens)
	dst.CacheWriteTokens = max(dst.CacheWriteTokens, next.CacheWriteTokens)
	dst.CacheWrite5mTokens = max(dst.CacheWrite5mTokens, next.CacheWrite5mTokens)
	dst.CacheWrite1hTokens = max(dst.CacheWrite1hTokens, next.CacheWrite1hTokens)
	dst.ImageCount = max(dst.ImageCount, next.ImageCount)
	dst.AudioInputTokens = max(dst.AudioInputTokens, next.AudioInputTokens)
	dst.AudioOutputTokens = max(dst.AudioOutputTokens, next.AudioOutputTokens)
	dst.ImageInputTokens = max(dst.ImageInputTokens, next.ImageInputTokens)
	dst.ImageOutputTokens = max(dst.ImageOutputTokens, next.ImageOutputTokens)
	dst.ToolCalls = max(dst.ToolCalls, next.ToolCalls)
	dst.AudioSeconds = max(dst.AudioSeconds, next.AudioSeconds)
	dst.VideoSeconds = max(dst.VideoSeconds, next.VideoSeconds)
	dst.present = dst.present || next.present
	dst.promptPresent = dst.promptPresent || next.promptPresent
	dst.outputPresent = dst.outputPresent || next.outputPresent
	if next.inputTotalPresent {
		dst.inputTotal = max(dst.inputTotal, next.inputTotal)
		dst.inputTotalPresent = true
		dst.inclusiveInput = next.inclusiveInput
	}
	if next.outputTotalPresent {
		dst.outputTotal = max(dst.outputTotal, next.outputTotal)
		dst.outputTotalPresent = true
	}
	for _, pair := range [][2]*string{{&dst.UsageSource, &next.UsageSource}, {&dst.PricingVersion, &next.PricingVersion}, {&dst.ImageSize, &next.ImageSize}, {&dst.ImageQuality, &next.ImageQuality}} {
		if *pair[1] != "" {
			*pair[0] = *pair[1]
		}
	}
	*dst = normalizeUsageFacts(*dst)
	reconcileUsageTotals(dst)
}

func reconcileUsageTotals(f *UsageFacts) {
	if f.inputTotalPresent {
		remaining := f.inputTotal
		if f.inclusiveInput {
			f.CacheReadTokens = min(f.CacheReadTokens, remaining)
			remaining -= f.CacheReadTokens
			f.CacheWriteTokens = min(f.CacheWriteTokens, remaining)
			remaining -= f.CacheWriteTokens
			f.CacheWrite5mTokens = min(f.CacheWrite5mTokens, f.CacheWriteTokens)
			f.CacheWrite1hTokens = min(f.CacheWrite1hTokens, f.CacheWriteTokens-f.CacheWrite5mTokens)
		}
		f.AudioInputTokens = min(f.AudioInputTokens, remaining)
		remaining -= f.AudioInputTokens
		f.ImageInputTokens = min(f.ImageInputTokens, remaining)
		f.InputTokens = remaining - f.ImageInputTokens
	}
	if f.outputTotalPresent {
		f.AudioOutputTokens = min(f.AudioOutputTokens, f.outputTotal)
		f.ImageOutputTokens = min(f.ImageOutputTokens, f.outputTotal-f.AudioOutputTokens)
		f.OutputTokens = f.outputTotal - f.AudioOutputTokens - f.ImageOutputTokens
	}
}

type usageObject map[string]json.RawMessage

func (u usageObject) object(keys ...string) usageObject {
	for _, key := range keys {
		raw := u[key]
		if len(raw) == 0 {
			continue
		}
		var child usageObject
		if json.Unmarshal(raw, &child) == nil && child != nil {
			return child
		}
	}
	return nil
}

func (u usageObject) number(keys ...string) (float64, bool) {
	for _, key := range keys {
		raw := bytes.TrimSpace(u[key])
		if len(raw) == 0 || raw[0] != '-' && (raw[0] < '0' || raw[0] > '9') || !json.Valid(raw) {
			continue
		}
		var n json.Number
		if json.Unmarshal(raw, &n) != nil {
			continue
		}
		v, err := n.Float64()
		if err != nil && !math.IsInf(v, 0) {
			continue
		}
		return clampUsageFloat(v), true
	}
	return 0, false
}

func (u usageObject) count(keys ...string) (int64, bool) {
	n, ok := u.number(keys...)
	return int64(n), ok
}

func (u usageObject) text(keys ...string) string {
	for _, key := range keys {
		raw := u[key]
		if len(raw) == 0 {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	return ""
}

func parseUsageFacts(body []byte) UsageFacts {
	return parseUsageFactsForFormat(body, "")
}

func parseUsageFactsForFormat(body []byte, format string) UsageFacts {
	var root usageObject
	if json.Unmarshal(body, &root) != nil {
		facts := UsageFacts{UsageSource: "missing"}
		for _, line := range bytes.Split(body, []byte{'\n'}) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			var event usageObject
			if json.Unmarshal(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))), &event) == nil {
				mergeUsageFacts(&facts, parseUsageFactsObject(event, format))
			}
		}
		return facts
	}
	return parseUsageFactsObject(root, format)
}

func parseUsageFactsObject(root usageObject, format string) UsageFacts {
	f := UsageFacts{UsageSource: "missing"}
	if root == nil {
		return f
	}
	u := root.object("usage", "usageMetadata", "usage_metadata", "totalUsage", "total_usage")
	kind := root.text("type")
	if nested := root.object("response"); nested != nil {
		if usage := nested.object("usage", "usageMetadata", "usage_metadata"); usage != nil {
			u = usage
		}
	}
	if kind == "message_start" {
		u = root.object("message").object("usage")
	}
	if u == nil {
		return f
	}
	anthropic := strings.EqualFold(format, "anthropic")
	if format == "" {
		for _, key := range []string{"cache_creation", "cacheCreation", "cache_creation_input_tokens", "cacheCreationInputTokens", "cache_read_input_tokens", "cacheReadInputTokens"} {
			if _, exists := u[key]; exists {
				anthropic = true
			}
		}
	}
	_, hasPrompt := u.count("prompt_tokens", "promptTokens")
	_, hasChoices := root["choices"]
	if hasPrompt || hasChoices || root.text("object") == "response" || strings.HasPrefix(kind, "response.") || root.object("response", "usageMetadata", "usage_metadata", "totalUsage", "total_usage") != nil {
		anthropic = false
	}
	if kind == "message" || kind == "message_start" || kind == "message_delta" {
		anthropic = true
	}
	readCount := func(dst *int64, obj usageObject, keys ...string) bool {
		value, exists := obj.count(keys...)
		if exists {
			*dst = max(*dst, value)
			f.present = true
			switch dst {
			case &f.InputTokens, &f.CacheReadTokens, &f.CacheWriteTokens, &f.CacheWrite5mTokens, &f.CacheWrite1hTokens, &f.AudioInputTokens, &f.ImageInputTokens:
				f.promptPresent = true
			case &f.OutputTokens, &f.AudioOutputTokens, &f.ImageOutputTokens:
				f.outputPresent = true
			}
		}
		return exists
	}
	readSeconds := func(dst *float64, keys ...string) {
		if value, exists := u.number(keys...); exists {
			*dst = value
			f.present = true
		}
	}
	f.inputTotalPresent = readCount(&f.inputTotal, u, "prompt_tokens", "promptTokens", "input_tokens", "inputTokens", "promptTokenCount", "prompt_token_count")
	f.outputTotalPresent = readCount(&f.outputTotal, u, "completion_tokens", "completionTokens", "output_tokens", "outputTokens", "candidatesTokenCount", "candidates_token_count")
	f.promptPresent = f.inputTotalPresent
	f.outputPresent = f.outputTotalPresent
	f.inclusiveInput = !anthropic
	inputDetails := []usageObject{u.object("prompt_tokens_details", "promptTokensDetails"), u.object("input_tokens_details", "inputTokensDetails", "inputTokenDetails")}
	outputDetails := []usageObject{u.object("completion_tokens_details", "completionTokensDetails"), u.object("output_tokens_details", "outputTokensDetails", "outputTokenDetails")}
	readCount(&f.CacheReadTokens, u, "cache_read_input_tokens", "cacheReadInputTokens", "cache_read_tokens", "cachedContentTokenCount", "cached_content_token_count")
	readCount(&f.CacheWriteTokens, u, "cache_creation_input_tokens", "cacheCreationInputTokens", "cache_write_tokens", "cacheWriteTokens")
	readCount(&f.CacheWrite5mTokens, u, "cache_write_5m_tokens", "cache_creation_5m_input_tokens")
	readCount(&f.CacheWrite1hTokens, u, "cache_write_1h_tokens", "cache_creation_1h_input_tokens")
	creation := u.object("cache_creation", "cacheCreation")
	readCount(&f.CacheWrite5mTokens, creation, "ephemeral_5m_input_tokens", "ephemeral5mInputTokens")
	readCount(&f.CacheWrite1hTokens, creation, "ephemeral_1h_input_tokens", "ephemeral1hInputTokens")
	readCount(&f.AudioInputTokens, u, "audio_input_tokens", "audioInputTokens")
	readCount(&f.AudioOutputTokens, u, "audio_output_tokens", "audioOutputTokens")
	readCount(&f.ImageInputTokens, u, "image_input_tokens", "imageInputTokens")
	readCount(&f.ImageOutputTokens, u, "image_output_tokens", "imageOutputTokens")
	for _, details := range inputDetails {
		readCount(&f.CacheReadTokens, details, "cached_tokens", "cachedTokens", "cache_read_tokens", "cacheReadTokens")
		readCount(&f.CacheWriteTokens, details, "cache_write_tokens", "cacheWriteTokens")
		readCount(&f.AudioInputTokens, details, "audio_tokens", "audioTokens")
		readCount(&f.ImageInputTokens, details, "image_tokens", "imageTokens")
		if !f.inputTotalPresent {
			readCount(&f.InputTokens, details, "text_tokens", "textTokens", "noCacheTokens")
		}
	}
	for _, details := range outputDetails {
		readCount(&f.AudioOutputTokens, details, "audio_tokens", "audioTokens")
		readCount(&f.ImageOutputTokens, details, "image_tokens", "imageTokens")
		if !f.outputTotalPresent {
			readCount(&f.OutputTokens, details, "text_tokens", "textTokens")
		}
	}
	for _, group := range []struct {
		keys  []string
		audio *int64
		image *int64
	}{
		{[]string{"promptTokensDetails", "prompt_tokens_details"}, &f.AudioInputTokens, &f.ImageInputTokens},
		{[]string{"candidatesTokensDetails", "candidates_tokens_details"}, &f.AudioOutputTokens, &f.ImageOutputTokens},
	} {
		for _, key := range group.keys {
			raw := u[key]
			if len(raw) == 0 {
				continue
			}
			var modalities []usageObject
			if json.Unmarshal(raw, &modalities) != nil {
				continue
			}
			for _, modality := range modalities {
				switch strings.ToLower(modality.text("modality")) {
				case "audio":
					readCount(group.audio, modality, "tokenCount", "token_count")
				case "image":
					readCount(group.image, modality, "tokenCount", "token_count")
				}
			}
		}
	}
	readCount(&f.ImageCount, u, "image_count", "imageCount", "images")
	readCount(&f.ToolCalls, u, "tool_calls", "toolCalls", "server_tool_calls", "serverToolCalls")
	serverTools := u.object("server_tool_use", "serverToolUse")
	var serverCalls int64
	for _, key := range []string{"web_search_requests", "web_fetch_requests", "code_execution_requests", "file_search_requests", "computer_use_requests"} {
		var calls int64
		if readCount(&calls, serverTools, key) {
			serverCalls += calls
		}
	}
	f.ToolCalls = max(f.ToolCalls, clampUsageInt(serverCalls))
	readSeconds(&f.AudioSeconds, "audio_seconds", "audioSeconds", "audio_duration_seconds")
	readSeconds(&f.VideoSeconds, "video_seconds", "videoSeconds", "video_duration_seconds")
	if u.text("type") == "duration" {
		if seconds, ok := u.number("seconds"); ok {
			f.AudioSeconds = max(f.AudioSeconds, seconds)
			f.present = true
		}
	}
	if total, ok := u.count("total_tokens", "totalTokens", "totalTokenCount", "total_token_count"); ok && f.inputTotalPresent && !f.outputTotalPresent {
		input := f.inputTotal
		if anthropic {
			input += f.CacheReadTokens + max(f.CacheWriteTokens, f.CacheWrite5mTokens+f.CacheWrite1hTokens)
		}
		f.outputTotal = max(total-input, 0)
		f.outputTotalPresent = true
		f.outputPresent = true
	}
	f.PricingVersion = u.text("pricing_version", "pricingVersion")
	f.ImageSize = u.text("image_size", "imageSize", "size")
	f.ImageQuality = u.text("image_quality", "imageQuality", "quality")
	if f.ImageSize == "" {
		f.ImageSize = root.text("size")
	}
	if f.ImageQuality == "" {
		f.ImageQuality = root.text("quality")
	}
	if f.present {
		f.UsageSource = "upstream"
	}
	f = normalizeUsageFacts(f)
	reconcileUsageTotals(&f)
	return f
}

func usageFactsOpenAIUsage(f UsageFacts) map[string]any {
	if !f.HasUsage() {
		return nil
	}
	prompt, cached, completion := f.LegacyTokens()
	return map[string]any{
		"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion,
		"prompt_tokens_details":     map[string]any{"cached_tokens": cached, "cache_write_tokens": f.CacheWriteTokens, "audio_tokens": f.AudioInputTokens, "image_tokens": f.ImageInputTokens},
		"completion_tokens_details": map[string]any{"audio_tokens": f.AudioOutputTokens, "image_tokens": f.ImageOutputTokens},
		"cache_write_5m_tokens":     f.CacheWrite5mTokens, "cache_write_1h_tokens": f.CacheWrite1hTokens,
		"image_count": f.ImageCount, "audio_seconds": f.AudioSeconds, "video_seconds": f.VideoSeconds, "tool_calls": f.ToolCalls,
		"pricing_version": f.PricingVersion, "image_size": f.ImageSize, "image_quality": f.ImageQuality,
	}
}

func usageFactsAnthropicUsage(f UsageFacts) map[string]any {
	if !f.HasUsage() {
		return nil
	}
	return map[string]any{
		"input_tokens":            f.InputTokens + f.AudioInputTokens + f.ImageInputTokens,
		"output_tokens":           f.OutputTokens + f.AudioOutputTokens + f.ImageOutputTokens,
		"cache_read_input_tokens": f.CacheReadTokens, "cache_creation_input_tokens": f.CacheWriteTokens,
		"cache_creation":     map[string]int64{"ephemeral_5m_input_tokens": f.CacheWrite5mTokens, "ephemeral_1h_input_tokens": f.CacheWrite1hTokens},
		"audio_input_tokens": f.AudioInputTokens, "audio_output_tokens": f.AudioOutputTokens,
		"image_input_tokens": f.ImageInputTokens, "image_output_tokens": f.ImageOutputTokens,
		"image_count": f.ImageCount, "audio_seconds": f.AudioSeconds, "video_seconds": f.VideoSeconds, "tool_calls": f.ToolCalls,
		"pricing_version": f.PricingVersion, "image_size": f.ImageSize, "image_quality": f.ImageQuality,
	}
}
