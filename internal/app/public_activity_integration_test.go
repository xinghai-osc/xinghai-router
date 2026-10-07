//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntegrationPublicActivity(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()

	fetch := func() *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		s.publicActivity(w, httptest.NewRequest(http.MethodGet, "/public/activity", nil))
		return w
	}

	w := fetch()
	var empty struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || string(empty.Data) != "[]" {
		t.Fatalf("empty activity: status=%d body=%s", w.Code, w.Body.String())
	}

	userID, _ := integrationUser(t, db, "public-activity@example.com", 0)
	created := time.Date(2026, 9, 30, 20, 0, 0, 123456000, time.FixedZone("UTC+8", 8*60*60))
	if _, err := db.Exec(ctx, `insert into request_logs(id,request_id,model,status_code,duration_ms,first_token_ms,created_at) values
		(gen_random_uuid(),'activity-success','success-model',200,1250,25,$1),
		(gen_random_uuid(),'activity-failure','failure-model',502,50,null,$2)`, created, created.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into usage_records(id,request_id,user_id,model,prompt_tokens,completion_tokens,cost) values(gen_random_uuid(),'activity-success',$1,'success-model',12,34,0)`, userID); err != nil {
		t.Fatal(err)
	}

	w = fetch()
	if w.Code != http.StatusOK {
		t.Fatalf("activity: status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Data []struct {
			Model            string `json:"model"`
			StatusCode       int    `json:"status_code"`
			DurationMs       int    `json:"duration_ms"`
			FirstTokenMs     *int   `json:"first_token_ms"`
			PromptTokens     int    `json:"prompt_tokens"`
			CompletionTokens int    `json:"completion_tokens"`
			TotalTokens      int    `json:"total_tokens"`
			CreatedAt        string `json:"created_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("activity count=%d, want 2: %s", len(response.Data), w.Body.String())
	}
	failed, successful := response.Data[0], response.Data[1]
	if failed.Model != "failure-model" || failed.StatusCode != 502 || failed.DurationMs != 50 || failed.FirstTokenMs != nil || failed.PromptTokens != 0 || failed.CompletionTokens != 0 || failed.TotalTokens != 0 {
		t.Fatalf("activity without usage=%+v", failed)
	}
	if successful.Model != "success-model" || successful.StatusCode != 200 || successful.DurationMs != 1250 || successful.FirstTokenMs == nil || *successful.FirstTokenMs != 25 || successful.PromptTokens != 12 || successful.CompletionTokens != 34 || successful.TotalTokens != 46 {
		t.Fatalf("activity with usage=%+v", successful)
	}
	for i, item := range response.Data {
		var fields map[string]json.RawMessage
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"model", "status_code", "duration_ms", "first_token_ms", "prompt_tokens", "completion_tokens", "total_tokens", "created_at"} {
			if _, ok := fields[key]; !ok {
				t.Errorf("activity item %d is missing %q", i, key)
			}
		}
		if len(fields) != 8 {
			t.Errorf("activity item %d exposes unexpected fields: %v", i, fields)
		}
	}
	for i, want := range []time.Time{created.Add(time.Second), created} {
		got, err := time.Parse(time.RFC3339Nano, response.Data[i].CreatedAt)
		if err != nil || !got.Equal(want) {
			t.Errorf("activity timestamp=%q, want %s: %v", response.Data[i].CreatedAt, want, err)
		}
	}
}
