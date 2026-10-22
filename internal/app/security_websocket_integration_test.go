//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type securityFrameLimiter struct {
	*memoryLimiter
	unavailable atomic.Bool
}

func (l *securityFrameLimiter) allowNResult(key string, n int) (bool, error) {
	if l.unavailable.Load() {
		return false, errRedisUnavailable
	}
	return l.memoryLimiter.allowN(key, n), nil
}

func TestIntegrationWebSocketRateLimitsEveryGeneration(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		name := "exhausted"
		if unavailable {
			name = "backend-unavailable"
		}
		t.Run(name, func(t *testing.T) {
			s := rewardBanIntegrationService(t)
			limit := &securityFrameLimiter{memoryLimiter: newMemoryLimiter(1)}
			s.limiter = limit
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var calls atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"id\":\"chatcmpl-security\",\"model\":\"security-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n")
				io.WriteString(w, "data: {\"id\":\"chatcmpl-security\",\"model\":\"security-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n")
			}))
			defer upstream.Close()
			userID, _ := integrationUser(t, s.db, "ws-security@example.test", 100)
			secret := "sk-websocket-security-test"
			keyID := integrationKey(t, s.db, userID, secret)
			if _, err := s.db.Exec(ctx, `insert into channels(name,base_url,api_key,models,provider,user_id) values('security-model',$1,'fake-upstream','["security-model"]','openai',$2)`, upstream.URL, userID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'security-model',1,1)`); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(s.api(s.responsesWebSocket))
			defer server.Close()
			conn, _, err := websocket.Dial(ctx, server.URL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + secret}}})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			payload := []byte(`{"type":"response.create","model":"security-model","input":"hello"}`)
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				t.Fatal(err)
			}
			for {
				_, raw, err := conn.Read(ctx)
				if err != nil {
					t.Fatal(err)
				}
				var event struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(raw, &event); err != nil {
					t.Fatal(err)
				}
				if event.Type == "response.completed" {
					break
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("first generation calls=%d", calls.Load())
			}
			limit.unavailable.Store(unavailable)
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				t.Fatal(err)
			}
			_, _, err = conn.Read(ctx)
			want := websocket.StatusPolicyViolation
			if unavailable {
				want = websocket.StatusTryAgainLater
			}
			if websocket.CloseStatus(err) != want || calls.Load() != 1 {
				t.Fatalf("err=%v upstream=%d", err, calls.Load())
			}
			if !unavailable {
				allowed, err := allowRateLimit(s.limiter, keyID, 0)
				if err != nil || allowed {
					t.Fatalf("HTTP counter not shared: %v %v", allowed, err)
				}
			}
		})
	}
}
