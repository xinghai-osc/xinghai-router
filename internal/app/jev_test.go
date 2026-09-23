package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateJEVRequestAcceptsOfficialQuestionShapes(t *testing.T) {
	body := []byte(`{
		"model":"jev-latest",
		"state":{"message":"I was charged twice"},
		"questions":{
			"refund":{"type":"noul","instructions":"Does the user request a refund?","criteria":{"true":"Refund requested","false":null}},
			"department":{"type":"choice","instructions":["Choose a department"],"criteria":{"billing":"Payments and refunds","technical":null}},
			"urgency":{"type":"score","instructions":null,"criteria":["Routine","Urgent","Emergency"]}
		}
	}`)

	request, err := validateJEVRequest(body)
	if err != nil {
		t.Fatalf("validateJEVRequest() error = %v", err)
	}
	if request.Model != "jev-latest" || len(request.Questions) != 3 {
		t.Fatalf("decoded request = %#v", request)
	}
}

func TestValidateJEVRequestAllowsOptionalInstructionsAndNoulCriteria(t *testing.T) {
	body := []byte(`{"model":"jev-latest","state":["one","two"],"questions":{"check":{"type":"noul"}}}`)
	if _, err := validateJEVRequest(body); err != nil {
		t.Fatalf("optional JEV fields should be accepted: %v", err)
	}
}

func TestValidateJEVRequestRejectsMalformedOrUnsupportedRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"model":`},
		{name: "non-object JSON", body: `[]`},
		{name: "missing model", body: `{"state":"text","questions":{"q":{"type":"noul"}}}`},
		{name: "null model", body: `{"model":null,"state":"text","questions":{"q":{"type":"noul"}}}`},
		{name: "null state", body: `{"model":"jev-latest","state":null,"questions":{"q":{"type":"noul"}}}`},
		{name: "empty score criteria", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"score","criteria":[]}}}`},
		{name: "null score criterion", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"score","criteria":[null]}}}`},
		{name: "string stream", body: `{"model":"jev-latest","state":"text","stream":"true","questions":{"q":{"type":"noul"}}}`},
		{name: "numeric stream", body: `{"model":"jev-latest","state":"text","stream":1,"questions":{"q":{"type":"noul"}}}`},
		{name: "null stream", body: `{"model":"jev-latest","state":"text","stream":null,"questions":{"q":{"type":"noul"}}}`},
		{name: "missing state", body: `{"model":"jev-latest","questions":{"q":{"type":"noul"}}}`},
		{name: "numeric state", body: `{"model":"jev-latest","state":1,"questions":{"q":{"type":"noul"}}}`},
		{name: "missing questions", body: `{"model":"jev-latest","state":"text"}`},
		{name: "unsupported question type", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"boolean"}}}`},
		{name: "choice criteria wrong shape", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"choice","criteria":[]}}}`},
		{name: "choice criterion wrong value", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"choice","criteria":{"yes":true}}}}`},
		{name: "score criteria wrong shape", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"score","criteria":{}}}}`},
		{name: "noul criteria wrong shape", body: `{"model":"jev-latest","state":"text","questions":{"q":{"type":"noul","criteria":[]}}}`},
		{name: "streaming", body: `{"model":"jev-latest","state":"text","stream":true,"questions":{"q":{"type":"noul"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := validateJEVRequest([]byte(tt.body)); err == nil {
				t.Fatal("validateJEVRequest() unexpectedly accepted request")
			}
		})
	}
}

func TestRewriteJEVBodyPreservesStructuredRequest(t *testing.T) {
	body := []byte(`{"model":"jev-latest","state":{"value":1},"questions":{"q":{"type":"noul"}},"promptCacheKey":"internal"}`)
	got := rewriteJEVBody(body, "jev-1.13.0", "jev-latest")
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("rewritten JEV body is invalid JSON: %v", err)
	}
	if payload["model"] != "jev-1.13.0" {
		t.Fatalf("model = %#v", payload["model"])
	}
	if _, ok := payload["promptCacheKey"]; ok {
		t.Fatal("router extension field was forwarded")
	}
	if state, ok := payload["state"].(map[string]any); !ok || state["value"] != float64(1) {
		t.Fatalf("state = %#v", payload["state"])
	}
}

func TestJEVHTTPContractEdges(t *testing.T) {
	for _, body := range []string{
		`{"model":"jev-latest","state":"","questions":{"q":{"type":"score","criteria":["only level"]}}}`,
		`{"model":"jev-latest","state":{"amount":1e400},"stream":false,"questions":{"q":{"type":"noul"}}}`,
	} {
		if _, err := validateJEVRequest([]byte(body)); err != nil {
			t.Errorf("valid HTTP request rejected: %s: %v", body, err)
		}
	}
	body := []byte(`{"model":"jev-latest","state":{"id":9007199254740993},"questions":{"q":{"type":"noul"}},"stream":false,"remove":true}`)
	got := applyJEVRequestOverrides(rewriteJEVBody(body, "jev-versioned", "jev-latest"), channelRequestOverrides{Delete: []string{"remove"}, Set: map[string]any{"extra": true, "stream": true}})
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["model"]) != `"jev-versioned"` || string(payload["state"]) != `{"id":9007199254740993}` || string(payload["extra"]) != "true" {
		t.Fatalf("rewritten request=%s", got)
	}
	for _, field := range []string{"remove", "stream"} {
		if _, ok := payload[field]; ok {
			t.Errorf("unexpected field %s in %s", field, got)
		}
	}
	clean := []byte(`{"model":"jev-latest","state":[],"questions":{"q":{"type":"noul"}}}`)
	if got := applyJEVRequestOverrides(rewriteJEVBody(clean, "", "jev-latest"), channelRequestOverrides{}); string(got) != string(clean) {
		t.Fatalf("unchanged request rewritten: %s", got)
	}
}

func TestJEVHandlerRejectsBeforeUpstream(t *testing.T) {
	for _, test := range []struct {
		name   string
		body   string
		status int
	}{
		{"oversized", strings.Repeat("x", maxJEVRequestBody+1), http.StatusRequestEntityTooLarge},
		{"invalid", `{}`, http.StatusUnprocessableEntity},
		{"unauthenticated", `{"model":"jev-latest","state":"ping","questions":{"q":{"type":"noul"}}}`, http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&Service{}).jevCompletions(rec, httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(test.body)))
			if rec.Code != test.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestJEVPolicyScansStructuredInputs(t *testing.T) {
	request, err := validateJEVRequest([]byte(`{"model":"jev-latest","state":{"arbitrary":[{"custom":"blocked-state"}]},"questions":{"q":{"type":"choice","instructions":{"custom":"blocked-instructions"},"criteria":{"blocked-choice":null,"other":{"custom":"blocked-criteria"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	body := jevPolicyBody(request)
	for _, term := range []string{"blocked-state", "blocked-instructions", "blocked-choice", "blocked-criteria"} {
		snapshot := contentPolicySnapshot{Settings: contentPolicySettings{Mode: "block"}, Rules: []contentPolicyRule{{ID: "test-rule", Term: term, Enabled: true, Action: "block"}}}
		if result := (&Service{}).evaluateContentPolicy(snapshot, body); result.Decision != "block" {
			t.Errorf("policy missed %s: %#v body=%s", term, result, body)
		}
	}
}

func TestUsageReadsJEVTokenFields(t *testing.T) {
	prompt, completion, total, cached := usage([]byte(`{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":307,"output_tokens":20}}`))
	if prompt != 307 || completion != 20 || total != 327 || cached != 0 {
		t.Fatalf("usage = %d/%d/%d/%d", prompt, completion, total, cached)
	}
}
