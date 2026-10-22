package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const miniMaxVideoTestBody = `{"model":"MiniMax-H3","content":[{"type":"text","text":"A cat running"}],"duration":5,"resolution":"768P","ratio":"16:9"}`

func TestMiniMaxVideoChannelProbe(t *testing.T) {
	if !validChannelProvider("minimax") || resolveUpstreamFormat("minimax", "") != "openai" {
		t.Fatal("MiniMax provider is not configured")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer fake-key" {
			t.Errorf("unexpected health request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer server.Close()
	s := &Service{httpClient: server.Client()}
	for _, model := range []string{"MiniMax-H3", "MiniMax-H3-Max"} {
		status, _, _, err := s.testChannel(context.Background(), server.URL, "fake-key", "minimax", model, nil, 0)
		if err != nil || status != 200 {
			t.Fatalf("probe status=%d err=%v", status, err)
		}
	}
}

func TestMiniMaxVideoRequest(t *testing.T) {
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"text", miniMaxVideoTestBody, true},
		{"reference", `{"model":"MiniMax-H3","content":[{"type":"text","text":"animate"},{"type":"image_url","image_url":{"url":"https://example.test/a.png"},"role":"reference_image"},{"type":"video_url","video_url":{"url":"https://example.test/a.mp4"},"role":"reference_video"},{"type":"audio_url","audio_url":{"url":"https://example.test/a.mp3"},"role":"reference_audio"}],"duration":15,"resolution":"2K"}`, true},
		{"frames", `{"model":"MiniMax-H3","content":[{"type":"text","text":"animate"},{"type":"image_url","image_url":{"url":"https://example.test/a.png"},"role":"first_frame"},{"type":"image_url","image_url":{"url":"https://example.test/b.png"},"role":"last_frame"}],"duration":4,"resolution":"768P"}`, true},
		{"missing prompt", strings.Replace(miniMaxVideoTestBody, "A cat running", " ", 1), false},
		{"missing duration", strings.Replace(miniMaxVideoTestBody, `"duration":5,`, "", 1), false},
		{"large duration", strings.Replace(miniMaxVideoTestBody, `"duration":5`, `"duration":16`, 1), false},
		{"fractional duration", strings.Replace(miniMaxVideoTestBody, `"duration":5`, `"duration":5.5`, 1), false},
		{"text adaptive", strings.Replace(miniMaxVideoTestBody, "16:9", "adaptive", 1), false},
		{"invalid resolution", strings.Replace(miniMaxVideoTestBody, "768P", "1080P", 1), false},
		{"max 2K", strings.ReplaceAll(strings.ReplaceAll(miniMaxVideoTestBody, "MiniMax-H3", "MiniMax-H3-Max"), "768P", "2K"), false},
		{"stream", strings.Replace(miniMaxVideoTestBody, `"duration":5`, `"stream":true,"duration":5`, 1), false},
		{"invalid JSON", `null`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateMiniMaxVideoRequest([]byte(test.body))
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
}

func TestMiniMaxVideoBodyPreservesNativeFields(t *testing.T) {
	body := []byte(strings.Replace(miniMaxVideoTestBody, `"duration":5`, `"duration":5,"extra":{"seed":9007199254740993},"aigc_watermark":true,"promptCacheKey":"strip"`, 1))
	out, err := miniMaxVideoBody(body, "application/json", true, "MiniMax-H3-Max")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("9007199254740993")) || bytes.Contains(out, []byte("promptCacheKey")) || !bytes.Contains(out, []byte(`"model":"MiniMax-H3-Max"`)) || !bytes.Contains(out, []byte(`"aigc_watermark":true`)) {
		t.Fatalf("body=%s", out)
	}
	if extractPolicyText(out) != "A cat running" {
		t.Fatalf("policy text=%q", extractPolicyText(out))
	}
}

func TestMiniMaxVideoOpenAIConversion(t *testing.T) {
	body := []byte(`{"model":"public-video","prompt":"animate","seconds":"5","size":"720x1280","aigc_watermark":true}`)
	out, err := miniMaxVideoBody(body, "application/json", false, "MiniMax-H3")
	if err != nil {
		t.Fatal(err)
	}
	request, err := validateMiniMaxVideoRequest(out)
	if err != nil || request.Model != "MiniMax-H3" || request.Ratio != "9:16" || request.Duration != 5 || request.Content[0].Text != "animate" {
		t.Fatalf("request=%s err=%v", out, err)
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	_ = writer.WriteField("prompt", "animate")
	_ = writer.WriteField("seconds", "5")
	part, _ := writer.CreateFormFile("input_reference", "first.png")
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("a", 32)))
	_ = writer.Close()
	out, err = miniMaxVideoBody(buffer.Bytes(), writer.FormDataContentType(), false, "MiniMax-H3")
	if err != nil {
		t.Fatal(err)
	}
	request, err = validateMiniMaxVideoRequest(out)
	if err != nil || len(request.Content) != 2 || request.Ratio != "adaptive" || request.Content[1].Role != "first_frame" || !bytes.Contains(out, []byte("data:image/png;base64,")) {
		t.Fatalf("multipart=%s err=%v", out, err)
	}
}

func TestMiniMaxVideoExtensions(t *testing.T) {
	for _, extension := range []string{`,"callback_url":"https://example.test/callback"`, `,"extra":"bad"`, `,"extra":null`, `,"aigc_watermark":"true"`, `,"aigc_watermark":null`} {
		body := strings.TrimSuffix(miniMaxVideoTestBody, "}") + extension + "}"
		if _, err := validateMiniMaxVideoRequest([]byte(body)); err == nil {
			t.Fatalf("accepted extension %s", extension)
		}
	}
	for _, reference := range []string{`42`, `null`, `{}`, `{"file_id":"file-1"}`} {
		body := []byte(`{"model":"MiniMax-H3","prompt":"animate","seconds":5,"input_reference":` + reference + `}`)
		if _, err := miniMaxVideoBody(body, "application/json", false, "MiniMax-H3"); err == nil {
			t.Fatalf("silently ignored reference %s", reference)
		}
	}
	body := []byte(`{"model":"MiniMax-H3","prompt":"animate","seconds":5,"input_reference":{"url":"https://example.test/frame.png"}}`)
	out, err := miniMaxVideoBody(body, "application/json", false, "MiniMax-H3")
	if err != nil || !bytes.Contains(out, []byte("https://example.test/frame.png")) {
		t.Fatalf("reference body=%s err=%v", out, err)
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	_ = writer.WriteField("prompt", "animate")
	_ = writer.WriteField("seconds", "5")
	_ = writer.WriteField("extra", `{"prompt_expansion_mode":"balanced"}`)
	_ = writer.WriteField("aigc_watermark", "true")
	_ = writer.Close()
	out, err = miniMaxVideoBody(buffer.Bytes(), writer.FormDataContentType(), false, "MiniMax-H3-Max")
	if err != nil || !bytes.Contains(out, []byte(`"extra":{"prompt_expansion_mode":"balanced"}`)) || !bytes.Contains(out, []byte(`"aigc_watermark":true`)) {
		t.Fatalf("multipart extensions=%s err=%v", out, err)
	}
}

func TestMiniMaxVideoCreateResponse(t *testing.T) {
	for _, body := range []string{`{}`, `{"task_id":"../escape"}`, `{"task_id":42}`, `{"task_id":"task","error":{"message":"failed"}}`, `{"task_id":"task","base_resp":{"status_code":1008}}`} {
		if _, err := miniMaxCreatedVideo([]byte(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	out, err := miniMaxCreatedVideo([]byte(`{"task_id":"424010985738629"}`))
	if err != nil {
		t.Fatal(err)
	}
	id, status, err := parseVideoTask(out)
	if err != nil || id != "424010985738629" || status != "queued" {
		t.Fatalf("out=%s err=%v", out, err)
	}
}

func TestMiniMaxVideoTaskResponses(t *testing.T) {
	s := &Service{}
	for _, native := range []bool{true, false} {
		path := "/v1/videos/video_local"
		if native {
			path = "/v2/query/video_generation/video_local"
		}
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		body := `{"task":{"id":"upstream","model":"MiniMax-H3","status":"succeeded","duration":5,"content":{"url":"https://cdn.example.test/video.mp4"},"usage":{"total_seconds":5},"extra":9007199254740993}}`
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
		status, err := s.writeMiniMaxVideoTask(w, r, resp, "video_local", "upstream", "public-video", false)
		if err != nil || status != 200 || strings.Contains(w.Body.String(), `"upstream"`) || !strings.Contains(w.Body.String(), "video_local") || !strings.Contains(w.Body.String(), "public-video") {
			t.Fatalf("native=%v status=%d body=%s err=%v", native, status, w.Body.String(), err)
		}
		if native && (!strings.Contains(w.Body.String(), `"succeeded"`) || !strings.Contains(w.Body.String(), "9007199254740993")) {
			t.Fatalf("native fields lost: %s", w.Body.String())
		}
		if !native && !strings.Contains(w.Body.String(), `"completed"`) {
			t.Fatalf("status not converted: %s", w.Body.String())
		}
	}
	for _, state := range []string{"queued", "running", "succeeded", "failed", "cancelled"} {
		body, _ := json.Marshal(map[string]any{"task": map[string]string{"id": "upstream", "status": state}})
		if _, _, err := miniMaxVideoStatus(body, "upstream"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := miniMaxVideoStatus(body, "other"); err == nil {
			t.Fatal("accepted wrong task ID")
		}
	}
	r := httptest.NewRequest(http.MethodDelete, "/v2/video_generation/video_local", nil)
	w := httptest.NewRecorder()
	resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"task_id":"upstream","status":"cancelled","action":"cancelled"}`))}
	if status, err := s.writeMiniMaxVideoTask(w, r, resp, "video_local", "upstream", "public-video", false); err != nil || status != 200 || strings.Contains(w.Body.String(), "upstream") {
		t.Fatalf("delete=%s err=%v", w.Body.String(), err)
	}
}

func TestMiniMaxVideoDownloadOmitsCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-API-Key") != "" || r.Header.Get("Range") != "bytes=1-3" {
			t.Errorf("unsafe download headers=%v", r.Header)
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 1-3/4")
		w.WriteHeader(206)
		_, _ = w.Write([]byte{1, 2, 3})
	}))
	defer server.Close()
	s := &Service{streamClient: server.Client()}
	r := httptest.NewRequest(http.MethodGet, "/v1/videos/video_local/content", nil)
	r.Header.Set("Authorization", "Bearer fake-client-secret")
	r.Header.Set("Cookie", "_token=fake-login")
	r.Header.Set("Range", "bytes=1-3")
	w := httptest.NewRecorder()
	status, err := s.writeMiniMaxVideoContent(w, r, server.URL)
	if err != nil || status != 206 || !bytes.Equal(w.Body.Bytes(), []byte{1, 2, 3}) {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
