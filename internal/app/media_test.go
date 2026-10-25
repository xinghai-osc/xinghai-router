package app

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

func mediaTestMultipart(t *testing.T, fields [][2]string, fileField string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatal(err)
		}
	}
	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, "sample.wav")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(durationTestWAV(1, 8000, 1, 16, 20000)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func TestMediaSpeechUsesOnlyUpstreamTokens(t *testing.T) {
	_, _, options, err := parseMediaRequest([]byte(`{"model":" speech-model ","input":"你好🌊 A","voice":"alloy"}`), "Application/JSON", "speech")
	if err != nil || options.facts.AudioCharacters != 5 {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	pricing := billingTestPricing()
	pricing.dimensions.AudioPerMillionCharacters = billingTestRate(20)
	facts, err := options.responseFacts([]byte{0, 1, 255}, "audio/mpeg", pricing)
	if err != nil || facts.HasUsage() {
		t.Fatalf("binary speech response without token usage = %+v, %v", facts, err)
	}
	bill, err := calculateBill(facts, pricing, 1)
	if err != nil || bill.Status != "missing_usage" || bill.Amount != "0.00000000" {
		t.Fatalf("missing upstream usage bill=%+v err=%v", bill, err)
	}
	withTokens, err := options.responseFacts([]byte(`{"usage":{"prompt_tokens":2,"completion_tokens":3}}`), "audio/mpeg", pricing)
	if err != nil {
		t.Fatal(err)
	}
	bill, err = calculateBill(withTokens, pricing, 1)
	if err != nil || bill.Amount != "0.00001600" {
		t.Fatalf("token-priced speech bill=%+v err=%v", bill, err)
	}
	if _, err := options.responseFacts([]byte(`{"error":"failed"}`), "application/json", pricing); err == nil {
		t.Fatal("JSON speech response accepted as audio")
	}
}

func TestMediaValidation(t *testing.T) {
	for _, test := range []struct{ kind, body string }{
		{"speech", `{"model":"tts","input":"x","voice":"alloy","stream":true}`},
		{"speech", `{"model":"tts","input":"x","voice":"alloy","stream_format":"sse"}`},
		{"speech", `{"model":"tts","input":"","voice":"alloy"}`},
		{"speech", `{"model":"tts","input":"x"}`},
		{"video", `{"model":"video","prompt":"x","seconds":"NaN"}`},
		{"video", `{"model":"video","prompt":"x","seconds":1.5}`},
		{"video", `{"model":"video","prompt":"x","seconds":0}`},
		{"video", `{"model":"video","prompt":"x","seconds":3601}`},
		{"video", `{"model":"video","prompt":""}`},
	} {
		if _, _, _, err := parseMediaRequest([]byte(test.body), "application/json", test.kind); err == nil {
			t.Errorf("accepted %s", test.body)
		}
	}
	for _, fields := range [][][2]string{
		{{"model", "audio"}, {"model", "other"}},
		{{"model", "audio"}, {"max_audio_seconds", "-1"}},
		{{"model", "audio"}, {"max_audio_seconds", "NaN"}},
		{{"model", "audio"}, {"max_audio_seconds", "0.0000001"}},
		{{"model", "audio"}, {"max_audio_seconds", "1"}},
		{{"model", "audio"}, {"response_format", "srt"}},
		{{"model", "audio"}, {"stream", "true"}},
	} {
		body, ct := mediaTestMultipart(t, fields, "file")
		if _, _, _, err := parseMediaRequest(body, ct, "transcription"); err == nil {
			t.Errorf("accepted fields=%v", fields)
		}
	}
}

func TestMediaMultipartNormalizationAndVideoDefault(t *testing.T) {
	body, ct := mediaTestMultipart(t, [][2]string{{"model", "alias"}, {"max_audio_seconds", "60"}, {"response_format", "verbose_json"}}, "file")
	ct = strings.Replace(ct, "multipart/form-data", "Multipart/Form-Data", 1)
	_, policy, options, err := parseMediaRequest(body, ct, "transcription")
	if err != nil || options.maxAudioSeconds != 60 {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	if strings.Contains(string(policy), "sample.wav") {
		t.Fatal("file included in content policy")
	}
	rewritten, rewrittenCT := rewriteMultipartModel(options.body, options.contentType, "upstream-audio")
	_, params, err := mime.ParseMediaType(rewrittenCT)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(rewritten), params["boundary"])
	foundModel, foundFile := false, false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		value, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		switch part.FormName() {
		case "model":
			foundModel = string(value) == "upstream-audio"
		case "file":
			foundFile = bytes.Equal(value, durationTestWAV(1, 8000, 1, 16, 20000))
		case "max_audio_seconds":
			t.Fatal("router billing limit forwarded upstream")
		}
	}
	if !foundFile || !foundModel {
		t.Fatal("lost binary file or model mapping")
	}
	_, _, video, err := parseMediaRequest([]byte(`{"model":"video","prompt":"test"}`), "application/json", "video")
	if err != nil || video.facts.VideoSeconds != 4 || !bytes.Contains(video.body, []byte(`"seconds":"4"`)) {
		t.Fatalf("video=%+v err=%v", video, err)
	}
	videoFacts, err := video.responseFacts([]byte(`{"id":"task","status":"queued","seconds":"4"}`), "application/json", billingTestPricing())
	if err != nil || videoFacts.HasUsage() {
		t.Fatalf("video duration without upstream tokens = %+v, %v", videoFacts, err)
	}
}

func TestMediaTranscriptionReservationAndMetering(t *testing.T) {
	options := mediaGatewayOptions{kind: "transcription", maxAudioSeconds: 60, promptTokens: 5}
	pricing := billingTestPricing()
	reserve, err := options.reservationFacts(pricing)
	if err != nil || reserve.InputTokens != 6005 || reserve.AudioInputTokens != 0 || reserve.OutputTokens != 0 {
		t.Fatalf("reserve=%+v err=%v", reserve, err)
	}
	quote, err := calculateBill(reserve, pricing, 1)
	if err != nil || quote.Amount != "0.01201000" {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
	facts, err := options.responseFacts([]byte(`{"text":"hello","duration":2.5,"usage":{"input_tokens":10,"output_tokens":3,"input_token_details":{"audio_tokens":8,"text_tokens":2}}}`), "application/json", pricing)
	if err != nil || facts.AudioSeconds != 0 || facts.InputTokens != 2 || facts.AudioInputTokens != 8 || facts.OutputTokens != 3 {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
	bill, err := calculateBill(facts, pricing, 1)
	if err != nil || bill.Amount != "0.00003200" {
		t.Fatalf("actual token bill=%+v err=%v", bill, err)
	}
	if _, err := options.responseFacts([]byte(`{"text":"too long","duration":61}`), "application/json", pricing); err == nil {
		t.Fatal("over-limit duration accepted")
	}
	facts, err = options.responseFacts([]byte(`{"text":"hello","duration":2.5}`), "application/json", pricing)
	if err != nil || facts.HasUsage() {
		t.Fatalf("duration without upstream usage = %+v, %v", facts, err)
	}
}
