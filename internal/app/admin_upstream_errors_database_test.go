package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminLogsPreserveUpstreamFailureDetails(t *testing.T) {
	s := channelCredentialDatabase(t)
	const detail = `{"error":{"message":"credit balance is too low","api_key":"sk-fake-log-secret"}}`
	if _, err := s.db.Exec(context.Background(), `insert into request_logs(id,request_id,model,status_code,duration_ms,error_code,error_detail) values(gen_random_uuid(),'admin-error-test','fake-model',429,10,'upstream_Too Many Requests',$1)`, detail); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{s.listLogs, s.listUsageLogs} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodGet, "/admin/usage-logs", nil))
		var result struct {
			Data []struct {
				Detail string `json:"error_detail"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || len(result.Data) != 1 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		got := result.Data[0].Detail
		if !strings.Contains(got, "credit balance is too low") || strings.Contains(got, "sk-fake-log-secret") || strings.Contains(got, noChannelAvailableDetail) {
			t.Fatalf("admin detail = %s", got)
		}
	}
}

func TestAdminChannelTestsPreserveUpstreamFailureDetails(t *testing.T) {
	s := channelCredentialDatabase(t)
	const secret = "fake-provider-key"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid credential fake-provider-key for this model"}}`))
	}))
	defer upstream.Close()
	s.httpClient = upstream.Client()
	channelID := channelCredentialInsert(t, s, "admin-error-test", secret)
	ctx := context.Background()
	if _, err := s.db.Exec(ctx, `update channels set base_url=$1,auto_disable=false where id=$2`, upstream.URL, channelID); err != nil {
		t.Fatal(err)
	}
	var keyID string
	if err := s.db.QueryRow(ctx, `insert into channel_api_keys(id,channel_id,name,key_encrypted) values(gen_random_uuid(),$1,'test',$2) returning id::text`, channelID, secret).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{s.testChannelHandler, s.testChannelKey} {
		w := channelCredentialRequest(t, handler, "", map[string]string{"id": channelID, "keyId": keyID}, 200)
		var result struct {
			Success    bool
			Reason     string
			StatusCode int `json:"status_code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Success || result.StatusCode != 401 || !strings.Contains(result.Reason, "invalid credential") || strings.Contains(result.Reason, secret) {
			t.Fatalf("body=%s", w.Body.String())
		}
		var lastError string
		if err := s.db.QueryRow(ctx, `select last_error from channel_api_keys where id=$1`, keyID).Scan(&lastError); err != nil {
			t.Fatal(err)
		}
		if lastError != result.Reason {
			t.Fatalf("stored reason=%s, response reason=%s", lastError, result.Reason)
		}
	}
}
