//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegrationWorkspaceResourceIsolation(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	owner, _ := integrationUser(t, db, "workspace-owner@example.test", 100)
	member, _ := integrationUser(t, db, "workspace-member@example.test", 100)
	outsider, _ := integrationUser(t, db, "workspace-outsider@example.test", 100)
	if _, err := db.Exec(ctx, `update users set role='admin' where id=$1`, outsider); err != nil {
		t.Fatal(err)
	}
	sessions := map[string]string{}
	for _, id := range []string{owner, member, outsider} {
		token, _, err := s.createSessionToken(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		sessions[id] = token
	}
	if _, err := db.Exec(ctx, `update user_sessions set reauthenticated_at=now()`); err != nil {
		t.Fatal(err)
	}
	call := func(user, workspace, method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessions[user]})
		r.Header.Set("X-Xinghai-Request", "1")
		if workspace != "" {
			r.Header.Set("X-Workspace-ID", workspace)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	object := func(w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var data map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	listCount := func(w *httptest.ResponseRecorder, count int) {
		t.Helper()
		data := object(w)["data"].([]any)
		if len(data) != count {
			t.Fatalf("count=%d want=%d body=%s", len(data), count, w.Body.String())
		}
	}
	personal, _, err := s.resolveWorkspace(ctx, owner, "")
	if err != nil || personal == "" {
		t.Fatalf("personal=%q err=%v", personal, err)
	}
	team := object(call(owner, "", "POST", "/account/workspaces", `{"name":"Team one","slug":"team-one"}`, 201))["id"].(string)
	personalKey := object(call(owner, personal, "POST", "/account/keys", `{"name":"personal"}`, 201))
	teamKey := object(call(owner, team, "POST", "/account/keys", `{"name":"team"}`, 201))
	keyID := teamKey["id"].(string)
	listCount(call(owner, "", "GET", "/account/keys", "", 200), 1)
	listCount(call(owner, team, "GET", "/account/keys", "", 200), 1)
	call(outsider, team, "GET", "/account/keys", "", 403)
	call(outsider, team, "GET", "/account/usage", "", 403)
	call(owner, "invalid", "GET", "/account/keys", "", 400)
	for _, action := range []struct{ method, suffix, body string }{
		{"GET", "/secret", ""}, {"GET", "/quota", ""},
		{"POST", "/quota", `{"window":"day","max_requests":10}`},
		{"DELETE", "/quota?window=day", ""},
		{"PUT", "", `{"name":"changed"}`},
		{"PUT", "/group", `{"group_id":""}`},
		{"POST", "/revoke", ""},
	} {
		call(owner, personal, action.method, "/account/keys/"+keyID+action.suffix, action.body, 404)
	}
	call(owner, team, "GET", "/account/keys/"+keyID+"/secret", "", 200)
	call(owner, team, "POST", "/account/keys/"+keyID+"/quota", `{"window":"day","max_requests":10}`, 200)
	call(owner, "", "POST", "/account/workspaces/"+team+"/members", `{"email":"workspace-member@example.test","role":"member"}`, 201)
	listCount(call(member, team, "GET", "/account/keys", "", 200), 0)
	call(member, team, "GET", "/account/keys/"+keyID+"/secret", "", 404)
	memberKey := object(call(member, team, "POST", "/account/keys", `{"name":"member key"}`, 201))
	for i, key := range []map[string]any{personalKey, teamKey, memberKey} {
		uid := owner
		if i == 2 {
			uid = member
		}
		request := fmt.Sprintf("workspace-call-%d", i)
		if _, err := db.Exec(ctx, `insert into request_logs(id,request_id,user_id,api_key_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms) values(gen_random_uuid(),$1,$2,$3,'workspace-model',200,10,5,15,100)`, request, uid, key["id"]); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,completion_tokens,cost) values(gen_random_uuid(),$1,$2,$3,'workspace-model',10,5,1)`, request, uid, key["id"]); err != nil {
			t.Fatal(err)
		}
	}
	listCount(call(owner, personal, "GET", "/account/usage", "", 200), 1)
	listCount(call(owner, team, "GET", "/account/usage", "", 200), 2)
	listCount(call(member, team, "GET", "/account/usage", "", 200), 2)
	if got := object(call(member, team, "GET", "/account/usage/summary", "", 200))["requests"].(float64); got != 2 {
		t.Fatalf("team summary requests=%v", got)
	}
	daily := object(call(member, team, "GET", "/account/usage/daily", "", 200))["data"].([]any)
	if len(daily) != 1 || daily[0].(map[string]any)["requests"].(float64) != 2 {
		t.Fatalf("team daily=%v", daily)
	}
	apiCall := func(secret, path string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+secret)
		r.Header.Set("X-Workspace-ID", personal)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("API %s status=%d want=%d body=%s", path, w.Code, status, w.Body.String())
		}
		return w
	}
	listCount(apiCall(teamKey["key"].(string), "/me/keys", 200), 1)
	listCount(apiCall(teamKey["key"].(string), "/me/usage", 200), 1)
	apiCall(memberKey["key"].(string), "/v1/models", 200)
	call(owner, "", "DELETE", "/account/workspaces/"+team+"/members/"+member, "", 204)
	call(member, team, "GET", "/account/usage", "", 403)
	apiCall(memberKey["key"].(string), "/v1/models", 401)
	call(owner, "", "POST", "/account/workspaces/"+team+"/members", `{"email":"workspace-member@example.test","role":"member"}`, 201)
	apiCall(memberKey["key"].(string), "/v1/models", 401)
	call(owner, "", "DELETE", "/account/workspaces/"+team, "", 204)
	call(owner, team, "GET", "/account/usage", "", 403)
	apiCall(teamKey["key"].(string), "/v1/models", 401)
	apiCall(personalKey["key"].(string), "/v1/models", 200)
	var history int
	if err := db.QueryRow(ctx, `select count(*) from request_logs where workspace_id=$1`, team).Scan(&history); err != nil || history != 2 {
		t.Fatalf("archived history count=%d err=%v", history, err)
	}
}
