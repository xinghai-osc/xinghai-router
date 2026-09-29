//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rewardBanIntegrationService(t *testing.T) *Service {
	t.Helper()
	root, dsn := integrationPool(t)
	ctx := context.Background()
	schema := fmt.Sprintf("reward_ban_%d", time.Now().UnixNano())
	name := pgx.Identifier{schema}.Sanitize()
	if _, err := root.Exec(ctx, "create schema "+name); err != nil {
		root.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := root.Exec(ctx, "drop schema "+name+" cascade"); err != nil {
			t.Error(err)
		}
		root.Close()
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 16
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update site_settings set reward_risk_settings='{"enabled":false,"auto_ban_enabled":false,"webrtc_enabled":false}' where id=true`); err != nil {
		t.Fatal(err)
	}
	return integrationService(t, pool)
}

func rewardBanIntegrationSession(t *testing.T, s *Service, userID string, recent bool) string {
	t.Helper()
	token, _, err := s.createSessionToken(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if recent {
		if _, err := s.db.Exec(context.Background(), `update user_sessions set reauthenticated_at=clock_timestamp() where token_hash=$1`, hashSecret(token)); err != nil {
			t.Fatal(err)
		}
	}
	return token
}

func rewardBanIntegrationBan(t *testing.T, s *Service, userID string) string {
	t.Helper()
	ctx := context.Background()
	tx, err := s.beginRewardRiskTx(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	observationID := randomIDString()
	if _, err := tx.Exec(ctx, `insert into risk_observations(id,user_id,action,source_id,name_snapshot,decision) select $1,id,'login',$3,name,'ban' from users where id=$2`, observationID, userID, observationID); err != nil {
		t.Fatal(err)
	}
	banned, err := s.autoBanRewardUserTx(ctx, tx, userID, observationID, 1, map[string]any{"reason": "integration-test"})
	if err != nil || !banned {
		t.Fatalf("banned=%v err=%v", banned, err)
	}
	var caseID string
	if err := tx.QueryRow(ctx, `select id::text from risk_bans where user_id=$1 and released_at is null`, userID).Scan(&caseID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return caseID
}

func rewardBanIntegrationRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	}
	r.Header.Set("X-Xinghai-Request", "1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestIntegrationRewardBanAccessAndUnblock(t *testing.T) {
	s := rewardBanIntegrationService(t)
	other := integrationService(t, s.db)
	ctx := context.Background()
	userID, _ := integrationUser(t, s.db, "banned-user@example.test", 100)
	adminID, _ := integrationUser(t, s.db, "ban-admin@example.test", 0)
	if _, err := s.db.Exec(ctx, `update users set role='admin' where id=$1`, adminID); err != nil {
		t.Fatal(err)
	}
	token := rewardBanIntegrationSession(t, s, userID, false)
	adminToken := rewardBanIntegrationSession(t, s, adminID, true)
	secret := "sk-xh-reward-ban-test-key"
	integrationKey(t, s.db, userID, secret)
	checkAccess := func(want int) {
		t.Helper()
		for _, service := range []*Service{s, other} {
			account := service.account(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			if w := rewardBanIntegrationRequest(account, "GET", "/account/me", "", token); w.Code != want {
				t.Fatalf("account status=%d want=%d body=%s", w.Code, want, w.Body.String())
			}
			api := service.api(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			r := httptest.NewRequest("GET", "/v1/models", nil)
			r.Header.Set("Authorization", "Bearer "+secret)
			w := httptest.NewRecorder()
			api.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatalf("API status=%d want=%d body=%s", w.Code, want, w.Body.String())
			}
		}
	}
	checkAccess(http.StatusNoContent)
	if _, err := s.db.Exec(ctx, `update users set enabled=false where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	checkAccess(http.StatusUnauthorized)
	if _, err := s.db.Exec(ctx, `update users set enabled=true where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	checkAccess(http.StatusNoContent)
	caseID := rewardBanIntegrationBan(t, s, userID)
	checkAccess(http.StatusUnauthorized)
	if _, _, err := s.createSessionToken(ctx, userID); err == nil {
		t.Fatal("disabled account minted a session")
	}
	unban := http.NewServeMux()
	unban.Handle("POST /admin/reward-risk/users/{id}/unban", s.permission("users.manage", s.requireRecentAuth(s.unbanRewardUser)))
	path := "/admin/reward-risk/users/" + userID + "/unban"
	body := fmt.Sprintf(`{"case_id":%q,"reason":"Reviewed evidence"}`, randomIDString())
	if w := rewardBanIntegrationRequest(unban, "POST", path, body, adminToken); w.Code != http.StatusConflict {
		t.Fatalf("stale case=%d %s", w.Code, w.Body.String())
	}
	checkAccess(http.StatusUnauthorized)
	body = fmt.Sprintf(`{"case_id":%q,"reason":"Reviewed evidence"}`, caseID)
	if w := rewardBanIntegrationRequest(unban, "POST", path, body, adminToken); w.Code != http.StatusOK {
		t.Fatalf("unblock=%d %s", w.Code, w.Body.String())
	}
	checkAccess(http.StatusUnauthorized)
	var enabled, reviewed, released bool
	var actor, reason string
	if err := s.db.QueryRow(ctx, `select u.enabled,u.risk_reviewed_through>'epoch'::timestamptz,b.released_at is not null,b.released_by::text,b.release_reason from users u join risk_bans b on b.user_id=u.id where b.id=$1`, caseID).Scan(&enabled, &reviewed, &released, &actor, &reason); err != nil || !enabled || !reviewed || !released || actor != adminID || reason != "Reviewed evidence" {
		t.Fatalf("release state enabled=%v reviewed=%v released=%v actor=%s reason=%s err=%v", enabled, reviewed, released, actor, reason, err)
	}
	var audits int
	if err := s.db.QueryRow(ctx, `select count(*) from audit_logs where action='reward_risk.unbanned' and entity_id=$1`, userID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("transactional unblock audit=%d err=%v", audits, err)
	}
	if w := rewardBanIntegrationRequest(unban, "POST", path, body, adminToken); w.Code != http.StatusConflict {
		t.Fatalf("replayed unblock=%d %s", w.Code, w.Body.String())
	}
	token = rewardBanIntegrationSession(t, s, userID, false)
	account := s.account(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if w := rewardBanIntegrationRequest(account, "GET", "/account/me", "", token); w.Code != http.StatusNoContent {
		t.Fatalf("fresh session=%d %s", w.Code, w.Body.String())
	}
	create := rewardBanIntegrationRequest(s.workspaceAccount(s.createAccountKey), "POST", "/account/keys", `{"name":"replacement"}`, token)
	if create.Code != http.StatusCreated {
		t.Fatalf("replacement key=%d %s", create.Code, create.Body.String())
	}
}

func TestIntegrationRewardBanGenericEnableResolvesCase(t *testing.T) {
	s := rewardBanIntegrationService(t)
	ctx := context.Background()
	adminID, _ := integrationUser(t, s.db, "enable-admin@example.test", 0)
	if _, err := s.db.Exec(ctx, `update users set role='admin' where id=$1`, adminID); err != nil {
		t.Fatal(err)
	}
	adminToken := rewardBanIntegrationSession(t, s, adminID, true)
	mux := http.NewServeMux()
	mux.Handle("PUT /admin/users/{id}", s.permission("users.manage", s.requireRecentAuth(s.updateUser)))
	mux.Handle("POST /admin/users/batch-update", s.permission("users.manage", s.requireRecentAuth(s.batchUpdateUsers)))
	for _, batch := range []bool{false, true} {
		userID, _ := integrationUser(t, s.db, fmt.Sprintf("enable-user-%v@example.test", batch), 0)
		caseID := rewardBanIntegrationBan(t, s, userID)
		method, path, body := "PUT", "/admin/users/"+userID, `{"enabled":true}`
		if batch {
			method, path, body = "POST", "/admin/users/batch-update", fmt.Sprintf(`{"user_ids":[%q],"enabled":true}`, userID)
		}
		if w := rewardBanIntegrationRequest(mux, method, path, body, adminToken); w.Code != http.StatusOK {
			t.Fatalf("batch=%v status=%d body=%s", batch, w.Code, w.Body.String())
		}
		var resolved bool
		if err := s.db.QueryRow(ctx, `select u.enabled and u.risk_reviewed_through>'epoch'::timestamptz and b.released_at is not null from users u join risk_bans b on b.user_id=u.id where b.id=$1`, caseID).Scan(&resolved); err != nil || !resolved {
			t.Fatalf("batch=%v resolved=%v err=%v", batch, resolved, err)
		}
	}
}

func TestIntegrationRewardBanKeyCreationWaitsForBan(t *testing.T) {
	s := rewardBanIntegrationService(t)
	for _, tc := range []struct {
		name     string
		managed  bool
		reenable bool
	}{
		{"self_disabled", false, false},
		{"admin_disabled", true, false},
		{"self_reenabled", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			managed := tc.managed
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			userID, _ := integrationUser(t, s.db, "queued-key-"+tc.name+"@example.test", 0)
			actorID := userID
			if managed {
				actorID, _ = integrationUser(t, s.db, "queued-key-admin@example.test", 0)
				if _, err := s.db.Exec(ctx, `update users set role='admin' where id=$1`, actorID); err != nil {
					t.Fatal(err)
				}
			}
			token := rewardBanIntegrationSession(t, s, actorID, false)
			blocker, err := s.beginRewardRiskTx(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback(ctx)
			admitted := make(chan struct{})
			result := make(chan *httptest.ResponseRecorder, 1)
			var handler http.Handler = s.workspaceAccount(func(w http.ResponseWriter, r *http.Request) {
				close(admitted)
				s.createAccountKey(w, r)
			})
			body := `{"name":"queued"}`
			if managed {
				handler = s.permission("keys.manage", func(w http.ResponseWriter, r *http.Request) {
					close(admitted)
					s.createKey(w, r)
				})
				body = fmt.Sprintf(`{"name":"queued","user_id":%q}`, userID)
			}
			go func() {
				result <- rewardBanIntegrationRequest(handler, "POST", "/account/keys", body, token)
			}()
			select {
			case <-admitted:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if err := disableUserAccessTx(ctx, blocker, userID); err != nil {
				t.Fatal(err)
			}
			if tc.reenable {
				if _, err := blocker.Exec(ctx, `update users set enabled=true where id=$1`, userID); err != nil {
					t.Fatal(err)
				}
			}
			if err := blocker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case w := <-result:
				want := http.StatusUnauthorized
				if managed {
					want = http.StatusForbidden
				}
				if w.Code != want {
					t.Fatalf("status=%d want=%d body=%s", w.Code, want, w.Body.String())
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var count int
			if err := s.db.QueryRow(ctx, `select count(*) from api_keys where user_id=$1`, userID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("post-ban key count=%d err=%v", count, err)
			}
		})
	}
}

func TestIntegrationRewardBanRollsBackWithTransaction(t *testing.T) {
	s := rewardBanIntegrationService(t)
	ctx := context.Background()
	userID, _ := integrationUser(t, s.db, "rollback-ban@example.test", 0)
	token := rewardBanIntegrationSession(t, s, userID, false)
	secret := "sk-xh-rollback-ban-test-key"
	integrationKey(t, s.db, userID, secret)
	tx, err := s.beginRewardRiskTx(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	observationID := randomIDString()
	if _, err := tx.Exec(ctx, `insert into risk_observations(id,user_id,action,source_id,name_snapshot,decision) values($1,$2,'login',$3,'rollback','ban')`, observationID, userID, observationID); err != nil {
		t.Fatal(err)
	}
	if banned, err := s.autoBanRewardUserTx(ctx, tx, userID, observationID, 1, nil); err != nil || !banned {
		t.Fatalf("banned=%v err=%v", banned, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.loadAPIKeyContext(ctx, secret); err != nil {
		t.Fatalf("rollback revoked API key: %v", err)
	}
	account := s.account(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if w := rewardBanIntegrationRequest(account, "GET", "/account/me", "", token); w.Code != http.StatusNoContent {
		t.Fatalf("rollback revoked session: %d %s", w.Code, w.Body.String())
	}
	var cases int
	if err := s.db.QueryRow(ctx, `select count(*) from risk_bans where user_id=$1`, userID).Scan(&cases); err != nil || cases != 0 {
		t.Fatalf("rollback retained case=%d err=%v", cases, err)
	}
}

func TestIntegrationRewardBanWebSocketReauthorizesNextFrame(t *testing.T) {
	s := rewardBanIntegrationService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl-risk\",\"model\":\"risk-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: {\"id\":\"chatcmpl-risk\",\"model\":\"risk-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	userID, _ := integrationUser(t, s.db, "websocket-ban@example.test", 100)
	secret := "sk-xh-websocket-ban-test-key"
	integrationKey(t, s.db, userID, secret)
	if _, err := s.db.Exec(ctx, `insert into channels(name,base_url,api_key,models,provider,user_id) values('risk-model',$1,'fake-upstream','["risk-model"]','openai',$2)`, upstream.URL, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'risk-model',1,1)`); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.api(s.responsesWebSocket))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, server.URL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + secret}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	payload := []byte(`{"type":"response.create","model":"risk-model","input":"hello"}`)
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
		t.Fatalf("first frame upstream calls=%d", calls.Load())
	}
	rewardBanIntegrationBan(t, s, userID)
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation || calls.Load() != 1 {
		t.Fatalf("post-ban websocket err=%v upstream calls=%d", err, calls.Load())
	}
}
