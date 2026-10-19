//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// upstreamRecorder captures the OpenCode session header of every upstream request.
type upstreamRecorder struct {
	mu       sync.Mutex
	sessions []string
}

func (r *upstreamRecorder) record(session string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = append(r.sessions, session)
}

func (r *upstreamRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.sessions...)
}

func (r *upstreamRecorder) last() string {
	sessions := r.all()
	if len(sessions) == 0 {
		return ""
	}
	return sessions[len(sessions)-1]
}

func affinityUpstream(recorder *upstreamRecorder) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		recorder.record(r.Header.Get("x-opencode-session"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-mock","model":"affinity-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)
	}))
}

// affinityGatewayCaller serves chat completions through the real gateway and
// reports which of the recorded upstreams answered.
type affinityGatewayCaller struct {
	t         *testing.T
	service   *Service
	secret    string
	recorders []*upstreamRecorder
}

func (c *affinityGatewayCaller) call(session, body string) int {
	c.t.Helper()
	before := make([]int, len(c.recorders))
	for i, recorder := range c.recorders {
		before[i] = len(recorder.all())
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+c.secret)
	request.Header.Set("Content-Type", "application/json")
	if session != "" {
		request.Header.Set("x-opencode-session", session)
	}
	recorder := httptest.NewRecorder()
	c.service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		c.t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for i, upstream := range c.recorders {
		if len(upstream.all()) > before[i] {
			return i
		}
	}
	c.t.Fatal("no upstream served the request")
	return -1
}

// TestIntegrationSessionAffinityPinsConversationsToOneChannel drives the gateway
// against two opencode_go upstreams and checks that every turn of a conversation
// reaches the upstream that served its first turn. Providers cache prompt
// prefixes per account, so switching channels between turns throws that cache
// away; the same must hold for clients that do not send x-opencode-session,
// whose conversation is identified by the head of its prompt.
func TestIntegrationSessionAffinityPinsConversationsToOneChannel(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "session-affinity@example.com", 100)
	secret := "sk-xh-session-affinity"
	integrationKey(t, db, userID, secret)

	recorders := []*upstreamRecorder{{}, {}}
	for i, recorder := range recorders {
		upstream := affinityUpstream(recorder)
		defer upstream.Close()
		if _, err := db.Exec(ctx, `insert into channels(name,base_url,api_key,models,provider,user_id,priority,weight) values($1,$2,'fake-upstream-key','["affinity-model"]','opencode_go',$3,100,1)`, "affinity-"+string(rune('a'+i)), upstream.URL, userID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'affinity-model',1,1)`); err != nil {
		t.Fatal(err)
	}
	caller := &affinityGatewayCaller{t: t, service: integrationService(t, db), secret: secret, recorders: recorders}

	t.Run("declared session", func(t *testing.T) {
		first := caller.call("ses-affinity-declared", `{"model":"affinity-model","messages":[{"role":"user","content":"first turn"}],"max_tokens":8}`)
		for _, body := range []string{
			`{"model":"affinity-model","messages":[{"role":"user","content":"first turn"},{"role":"assistant","content":"ok"},{"role":"user","content":"second turn"}],"max_tokens":8}`,
			`{"model":"affinity-model","messages":[{"role":"user","content":"first turn"},{"role":"assistant","content":"ok"},{"role":"user","content":"third turn"}],"max_tokens":8}`,
		} {
			if next := caller.call("ses-affinity-declared", body); next != first {
				t.Fatalf("conversation moved from channel %d to %d", first, next)
			}
		}
		if got := recorders[first].last(); got != "ses-affinity-declared" {
			t.Fatalf("upstream session = %q, want the client's session", got)
		}
	})

	t.Run("derived session", func(t *testing.T) {
		head := `{"role":"system","content":"you are a coding agent"},{"role":"user","content":"count the rows"}`
		first := caller.call("", `{"model":"affinity-model","messages":[`+head+`],"max_tokens":8}`)
		derived := recorders[first].last()
		if derived == "" {
			t.Fatal("upstream received no session header")
		}
		next := caller.call("", `{"model":"affinity-model","messages":[`+head+`,{"role":"assistant","content":"ok"},{"role":"user","content":"again"}],"max_tokens":8}`)
		if next != first {
			t.Fatalf("header-less conversation moved from channel %d to %d", first, next)
		}
		if got := recorders[next].last(); got != derived {
			t.Fatalf("derived session changed between turns: %q != %q", got, derived)
		}
		// A second conversation with the same system prompt must not inherit the
		// reference of the first one.
		other := caller.call("", `{"model":"affinity-model","messages":[{"role":"system","content":"you are a coding agent"},{"role":"user","content":"write a poem"}],"max_tokens":8}`)
		if got := recorders[other].last(); got == derived {
			t.Fatal("two conversations shared a derived session")
		}
	})
}

// TestIntegrationSessionAffinityFailsOverToTheRemainingChannel checks that a pin
// never strands a conversation: once the channel holding it stops being a
// candidate (disabled, deleted, out of the key's group) the next turn is served
// by another candidate.
func TestIntegrationSessionAffinityFailsOverToTheRemainingChannel(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "session-affinity-failover@example.com", 100)
	secret := "sk-xh-session-affinity-failover"
	integrationKey(t, db, userID, secret)

	recorders := []*upstreamRecorder{{}, {}}
	channelIDs := make([]string, len(recorders))
	for i, recorder := range recorders {
		upstream := affinityUpstream(recorder)
		defer upstream.Close()
		if err := db.QueryRow(ctx, `insert into channels(name,base_url,api_key,models,provider,user_id,priority,weight) values($1,$2,'fake-upstream-key','["affinity-model"]','opencode_go',$3,100,1) returning id::text`, "affinity-failover-"+string(rune('a'+i)), upstream.URL, userID).Scan(&channelIDs[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'affinity-model',1,1)`); err != nil {
		t.Fatal(err)
	}
	service := integrationService(t, db)
	caller := &affinityGatewayCaller{t: t, service: service, secret: secret, recorders: recorders}
	body := `{"model":"affinity-model","messages":[{"role":"user","content":"keep going"}],"max_tokens":8}`

	served := caller.call("ses-affinity-failover", body)
	if _, err := db.Exec(ctx, `update channels set enabled=false where id=$1`, channelIDs[served]); err != nil {
		t.Fatal(err)
	}
	service.channelCache.clear()
	if next := caller.call("ses-affinity-failover", body); next == served {
		t.Fatal("retired channel kept serving the session")
	}
}

// TestIntegrationSessionAffinitySurvivesRestart checks the disk snapshot: a
// conversation served before the gateway restarted is renewed onto the same
// channel afterwards instead of being reshuffled, which is what keeps the
// upstream prompt cache reachable across a redeploy.
func TestIntegrationSessionAffinitySurvivesRestart(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "session-affinity-restart@example.com", 100)
	secret := "sk-xh-session-affinity-restart"
	integrationKey(t, db, userID, secret)

	recorders := []*upstreamRecorder{{}, {}}
	for i, recorder := range recorders {
		upstream := affinityUpstream(recorder)
		defer upstream.Close()
		if _, err := db.Exec(ctx, `insert into channels(name,base_url,api_key,models,provider,user_id,priority,weight) values($1,$2,'fake-upstream-key','["affinity-model"]','opencode_go',$3,100,1)`, "affinity-restart-"+string(rune('a'+i)), upstream.URL, userID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'affinity-model',1,1)`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	body := `{"model":"affinity-model","messages":[{"role":"user","content":"survive the restart"}],"max_tokens":8}`

	first := integrationService(t, db)
	first.cfg.SessionAffinityDir = dir
	served := (&affinityGatewayCaller{t: t, service: first, secret: secret, recorders: recorders}).call("ses-affinity-restart", body)
	first.flushSessionAffinity()

	restarted := integrationService(t, db)
	restarted.cfg.SessionAffinityDir = dir
	restarted.loadSessionAffinity()
	if next := (&affinityGatewayCaller{t: t, service: restarted, secret: secret, recorders: recorders}).call("ses-affinity-restart", body); next != served {
		t.Fatalf("session moved from channel %d to %d across a restart", served, next)
	}
}

// TestIntegrationSessionAffinityAdminSurface exercises the operator surface end
// to end: the route is registered, the list reports the pinned conversation with
// the channel label the console renders, a destructive call requires a recent
// password check, and a delete drops the pin from both the cache and the
// snapshot.
func TestIntegrationSessionAffinityAdminSurface(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "session-affinity-admin@example.com", 100)
	if _, err := db.Exec(ctx, `update users set role='admin' where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	secret := "sk-xh-session-affinity-admin"
	integrationKey(t, db, userID, secret)

	recorder := &upstreamRecorder{}
	upstream := affinityUpstream(recorder)
	defer upstream.Close()
	if _, err := db.Exec(ctx, `insert into channels(name,base_url,api_key,models,provider,user_id,priority,weight) values('affinity-admin',$1,'fake-upstream-key','["affinity-model"]','opencode_go',$2,100,1)`, upstream.URL, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'affinity-model',1,1)`); err != nil {
		t.Fatal(err)
	}
	service := integrationService(t, db)
	service.cfg.SessionAffinityDir = filepath.Join(t.TempDir(), "sessions")
	const session = "ses-affinity-admin"
	caller := &affinityGatewayCaller{t: t, service: service, secret: secret, recorders: []*upstreamRecorder{recorder}}
	caller.call(session, `{"model":"affinity-model","messages":[{"role":"user","content":"hello"}],"max_tokens":8}`)

	// An unauthenticated call must be turned away by a route that exists, not
	// fall through to a 404, which is what a missing registration looks like.
	unauthenticated := httptest.NewRecorder()
	service.Handler().ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/admin/session-affinity", nil))
	if unauthenticated.Code == http.StatusNotFound {
		t.Fatal("the admin session affinity route is not registered")
	}

	loginAs := func(email string) *http.Cookie {
		t.Helper()
		login := httptest.NewRecorder()
		service.login(login, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"`+email+`","password":"password123"}`)))
		if login.Code != http.StatusOK {
			t.Fatalf("login %s status=%d body=%s", email, login.Code, login.Body.String())
		}
		for _, candidate := range login.Result().Cookies() {
			if candidate.Name == sessionCookieName {
				return candidate
			}
		}
		t.Fatalf("login %s did not create a console session", email)
		return nil
	}
	consoleRequest := func(cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		request := httptest.NewRequest(method, path, reader)
		if cookie != nil {
			request.AddCookie(cookie)
		}
		request.Header.Set("X-Xinghai-Request", "1")
		recorder := httptest.NewRecorder()
		service.Handler().ServeHTTP(recorder, request)
		return recorder
	}
	admin := loginAs("session-affinity-admin@example.com")
	// A signed-in user without system.manage may not inspect or drop pins.
	integrationUser(t, db, "session-affinity-viewer@example.com", 0)
	viewer := loginAs("session-affinity-viewer@example.com")
	if forbidden := consoleRequest(viewer, http.MethodGet, "/admin/session-affinity", ""); forbidden.Code != http.StatusForbidden || !strings.Contains(forbidden.Body.String(), "logs.read") {
		t.Fatalf("viewer list status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	if forbidden := consoleRequest(viewer, http.MethodDelete, "/admin/session-affinity/"+session, ""); forbidden.Code != http.StatusForbidden || !strings.Contains(forbidden.Body.String(), "system.manage") {
		t.Fatalf("viewer delete status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}

	list := consoleRequest(admin, http.MethodGet, "/admin/session-affinity?session="+session, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var page struct {
		Data []struct {
			Session     string    `json:"session"`
			UserID      int64     `json:"user_id"`
			Model       string    `json:"model"`
			ChannelID   int64     `json:"channel_id"`
			ChannelName string    `json:"channel_name"`
			ExpiresAt   time.Time `json:"expires_at"`
		} `json:"data"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Data) != 1 {
		t.Fatalf("list = %d rows total %d, want the pinned conversation", len(page.Data), page.Total)
	}
	row := page.Data[0]
	if row.Session != session || row.UserID <= 0 || strconv.FormatInt(row.UserID, 10) != userID || row.Model != "affinity-model" || row.ChannelName != "affinity-admin" || row.ChannelID == 0 || row.ExpiresAt.Before(time.Now()) {
		t.Fatalf("listed assignment = %#v", row)
	}

	// The console sends same-origin headers; a cross-site delete is refused.
	crossSite := httptest.NewRequest(http.MethodDelete, "/admin/session-affinity/"+session, nil)
	crossSite.AddCookie(admin)
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSiteRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(crossSiteRecorder, crossSite)
	if crossSiteRecorder.Code != http.StatusForbidden || !strings.Contains(crossSiteRecorder.Body.String(), "csrf_failed") {
		t.Fatalf("cross-site delete status=%d body=%s", crossSiteRecorder.Code, crossSiteRecorder.Body.String())
	}

	deleted := consoleRequest(admin, http.MethodDelete, "/admin/session-affinity/"+session, "")
	if deleted.Code != http.StatusOK || !strings.Contains(deleted.Body.String(), `"removed":1`) {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	if records := service.sessionAffinity.list(); len(records) != 0 {
		t.Fatalf("deleted assignment survived: %#v", records)
	}
	service.flushSessionAffinity()
	snapshot, err := os.ReadFile(filepath.Join(service.cfg.SessionAffinityDir, sessionAffinityFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(snapshot), session) {
		t.Fatal("the snapshot still holds the deleted assignment")
	}
	if missing := consoleRequest(admin, http.MethodDelete, "/admin/session-affinity/"+session, ""); missing.Code != http.StatusNotFound {
		t.Fatalf("second delete status=%d body=%s", missing.Code, missing.Body.String())
	}
}
