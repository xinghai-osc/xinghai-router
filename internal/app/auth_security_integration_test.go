//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func authSecurityRequest(s *Service, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://router.example.test"+path, strings.NewReader(body))
	r.Header.Set("X-Xinghai-Request", "1")
	r.Header.Set("Origin", "https://router.example.test")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func authSecurityCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("missing session: status=%d body=%s", w.Code, w.Body.String())
	return nil
}

func TestIntegrationAuthPreHijackRecovery(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	ctx := context.Background()
	registered := authSecurityRequest(s, "POST", "/auth/register", `{"email":"victim@example.test","name":"attacker-name","password":"attacker-password"}`, nil)
	rewardRiskJSON(t, registered, http.StatusCreated)
	attackerCookie := authSecurityCookie(t, registered)
	var userID, oldHash string
	var verified bool
	if err := s.db.QueryRow(ctx, `select id,password_hash,email_verified_at is not null from users where email='victim@example.test'`).Scan(&userID, &oldHash, &verified); err != nil || verified {
		t.Fatalf("unverified registration verified=%v err=%v", verified, err)
	}
	keyID := integrationKey(t, s.db, userID, "sk-xh-attacker-prehijack")
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "1001", "victim@example.test", true, "victim", ""); !errors.Is(err, errOAuthEmailVerificationRequired) {
		t.Fatalf("unverified local account matched: %v", err)
	}
	if got := rewardRiskCount(t, s, `select count(*) from user_oauth_connections where user_id=$1`, userID); got != 0 {
		t.Fatalf("refused OAuth created %d bindings", got)
	}
	rewardRiskExec(t, s, `insert into user_oauth_connections(user_id,provider,provider_user_id) values($1,'github','666')`, userID)
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "666", "victim@example.test", true, "old-binding", ""); !errors.Is(err, errOAuthEmailVerificationRequired) {
		t.Fatalf("historical binding bypassed recovery: %v", err)
	}
	if _, _, err := s.createOAuthSessionToken(ctx, userID, "github", "666"); !errors.Is(err, errOAuthEmailVerificationRequired) {
		t.Fatalf("historical binding minted session: %v", err)
	}
	rewardRiskExec(t, s, `insert into email_verification_codes(email,code_hash,expires_at) values('victim@example.test',$1,now()+interval '10 minutes')`, hashEmailCode("victim@example.test", "123456"))
	s.cfg.SMTPHost, s.cfg.SMTPFrom = "unused.invalid", "unused@example.test"
	conflict := authSecurityRequest(s, "POST", "/auth/register", `{"email":"victim@example.test","name":"genuine-owner","password":"genuine-password","code":"123456"}`, nil)
	rewardRiskJSON(t, conflict, http.StatusConflict)
	s.cfg.SMTPHost, s.cfg.SMTPFrom = "", ""
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is not null`, userID); got != 0 {
		t.Fatal("registration code verified existing attacker account")
	}
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "1001", "victim@example.test", true, "victim", ""); !errors.Is(err, errOAuthEmailVerificationRequired) {
		t.Fatalf("consumed code enabled prehijack: %v", err)
	}
	token := "xh_reset_fake_recovery_token"
	rewardRiskExec(t, s, `insert into password_reset_tokens(email,token_hash,expires_at) values('victim@example.test',$1,now()+interval '30 minutes')`, hashSecret(token))
	body := fmt.Sprintf(`{"token":%q,"password":"genuine-password"}`, token)
	rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/password-reset/confirm", body, nil), http.StatusOK)
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is not null`, userID); got != 1 {
		t.Fatal("recovery did not persist verified email")
	}
	if got := rewardRiskCount(t, s, `select count(*) from user_sessions where user_id=$1`, userID); got != 0 {
		t.Fatalf("recovery retained %d sessions", got)
	}
	if got := rewardRiskCount(t, s, `select count(*) from api_keys where id=$1 and revoked_at is not null`, keyID); got != 1 {
		t.Fatal("recovery retained API key")
	}
	if got := rewardRiskCount(t, s, `select count(*) from user_oauth_connections where user_id=$1`, userID); got != 0 {
		t.Fatal("recovery retained malicious OAuth binding")
	}
	if _, _, err := s.createSessionToken(ctx, userID, oldHash); err == nil {
		t.Fatal("stale password created session after recovery")
	}
	if _, _, err := s.createOAuthSessionToken(ctx, userID, "github", "666"); err == nil {
		t.Fatal("stale OAuth callback created session after recovery")
	}
	rewardRiskJSON(t, authSecurityRequest(s, "GET", "/account/me", "", attackerCookie), http.StatusUnauthorized)
	apiRequest := httptest.NewRequest("GET", "/v1/models", nil)
	apiRequest.Header.Set("Authorization", "Bearer sk-xh-attacker-prehijack")
	apiResponse := httptest.NewRecorder()
	s.Handler().ServeHTTP(apiResponse, apiRequest)
	rewardRiskJSON(t, apiResponse, http.StatusUnauthorized)
	rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/login", `{"email":"victim@example.test","password":"attacker-password"}`, nil), http.StatusUnauthorized)
	rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/password-reset/confirm", body, nil), http.StatusBadRequest)
	rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/login", `{"email":"victim@example.test","password":"genuine-password"}`, nil), http.StatusOK)
	matched, err := s.findOrCreateOAuthUser(ctx, "github", "1001", "victim@example.test", true, "victim", "")
	if err != nil || matched != userID {
		t.Fatalf("genuine recovered OAuth failed user=%s err=%v", matched, err)
	}
	if _, _, err := s.createOAuthSessionToken(ctx, userID, "github", "1001"); err != nil {
		t.Fatalf("genuine OAuth session failed: %v", err)
	}
}

func TestIntegrationAuthVerificationSources(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	ctx := context.Background()
	s.cfg.SMTPHost, s.cfg.SMTPFrom = "unused.invalid", "unused@example.test"
	rewardRiskExec(t, s, `insert into email_verification_codes(email,code_hash,expires_at) values('verified@example.test',$1,now()+interval '10 minutes')`, hashEmailCode("verified@example.test", "654321"))
	rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/register", `{"email":"verified@example.test","name":"verified-local","password":"password123","code":"654321"}`, nil), http.StatusCreated)
	if got := rewardRiskCount(t, s, `select count(*) from users where email='verified@example.test' and email_verified_at is not null`); got != 1 {
		t.Fatal("successful code verification not persisted")
	}
	s.cfg.SMTPHost, s.cfg.SMTPFrom = "", ""
	for _, tc := range []struct {
		name, email string
		verified    bool
	}{
		{"proven-oauth", "proven@example.test", true},
		{"synthetic-oauth", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := s.findOrCreateOAuthUser(ctx, "github", tc.name, tc.email, tc.verified, tc.name, "")
			if err != nil {
				t.Fatal(err)
			}
			var verified bool
			if err := s.db.QueryRow(ctx, `select email_verified_at is not null from users where id=$1`, id).Scan(&verified); err != nil || verified != tc.verified {
				t.Fatalf("verified=%v want=%v err=%v", verified, tc.verified, err)
			}
		})
	}
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "unproven", "verified@example.test", false, "unproven", ""); !errors.Is(err, errOAuthEmailNotVerified) {
		t.Fatalf("unverified upstream email accepted: %v", err)
	}
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "proven-local", "verified@example.test", true, "verified-local", ""); err != nil {
		t.Fatalf("verified local match failed: %v", err)
	}
	id, err := s.findOrCreateOAuthUser(ctx, "github", "proven-oauth", "proven@example.test", true, "proven-oauth", "")
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, `update users set email_verified_at=null where id=$1`, id)
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "proven-oauth", "different@example.test", true, "proven-oauth", ""); err != nil {
		t.Fatal(err)
	}
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is not null`, id); got != 0 {
		t.Fatal("different upstream email verified historical local address")
	}
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "proven-oauth", "proven@example.test", true, "proven-oauth", ""); err != nil {
		t.Fatal(err)
	}
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is not null`, id); got != 1 {
		t.Fatal("matching proven upstream email was not persisted")
	}
}

func TestIntegrationAuthResetPasswordlessAndVerifiedAccounts(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	for _, verified := range []bool{false, true} {
		t.Run(fmt.Sprint(verified), func(t *testing.T) {
			email := fmt.Sprintf("reset-%v@example.test", verified)
			id, _ := integrationUser(t, s.db, email, 0)
			rewardRiskExec(t, s, `update users set password_hash=null,email_verified_at=case when $2 then now() else null end where id=$1`, id, verified)
			rewardRiskExec(t, s, `insert into user_oauth_connections(user_id,provider,provider_user_id) values($1,'github',$2)`, id, id)
			integrationKey(t, s.db, id, "sk-xh-reset-passwordless-"+id)
			token := "xh_reset_fake_" + id
			rewardRiskExec(t, s, `insert into password_reset_tokens(email,token_hash,expires_at) values($1,$2,now()+interval '30 minutes')`, email, hashSecret(token))
			body := fmt.Sprintf(`{"token":%q,"password":"new-password123"}`, token)
			rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/password-reset/confirm", body, nil), http.StatusOK)
			var passwordHash string
			if err := s.db.QueryRow(t.Context(), `select password_hash from users where id=$1 and email_verified_at is not null`, id).Scan(&passwordHash); err != nil || !passwordMatches(passwordHash, "new-password123") {
				t.Fatalf("passwordless recovery failed: %v", err)
			}
			want := 0
			if verified {
				want = 1
			}
			if got := rewardRiskCount(t, s, `select count(*) from user_oauth_connections where user_id=$1`, id); got != want {
				t.Fatalf("bindings=%d want=%d", got, want)
			}
			if got := rewardRiskCount(t, s, `select count(*) from api_keys where user_id=$1 and revoked_at is null`, id); got != 0 {
				t.Fatal("reset retained API key")
			}
		})
	}
}

func TestIntegrationAuthHistoricalVerificationMigration(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	id, _ := integrationUser(t, s.db, "historical@example.test", 0)
	rewardRiskExec(t, s, `update users set role='admin' where id=$1`, id)
	rewardRiskExec(t, s, `insert into email_verification_codes(email,code_hash,expires_at,consumed_at) values('historical@example.test','fake',now(),now())`)
	rewardRiskExec(t, s, `insert into user_oauth_connections(user_id,provider,provider_user_id) values($1,'github','historical')`, id)
	rewardRiskExec(t, s, `alter table users drop column email_verified_at`)
	raw, err := migrations.ReadFile("migrations/134_email_verification.sql")
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, string(raw))
	rewardRiskExec(t, s, string(raw))
	if got := rewardRiskCount(t, s, `select count(*) from users where email_verified_at is not null`); got != 0 {
		t.Fatal("migration inferred historical verification")
	}
	rewardRiskExec(t, s, `update users set email_verified_at=now() where id=$1`, id)
	rewardRiskExec(t, s, string(raw))
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is not null`, id); got != 1 {
		t.Fatal("idempotent migration erased new verification")
	}
}

func TestIntegrationAuthTrustedOrigins(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	ctx := context.Background()
	integrationUser(t, s.db, "reset-origin@example.test", 0)
	s.limiter.close()
	s.limiter = &sequenceLimiter{remaining: 1000}
	s.cfg.SMTPHost, s.cfg.SMTPFrom = "unused.invalid", "unused@example.test"
	rewardRiskExec(t, s, `insert into oauth_providers(id,client_id,client_secret_encrypted,enabled) values('github','fake-client',$1,true) on conflict(id) do update set client_id=excluded.client_id,client_secret_encrypted=excluded.client_secret_encrypted,enabled=true`, mustCrypt(t, "fake-secret"))
	for _, origin := range []string{"", "http://evil.example", "https://user:pass@router.example", "https://router.example/path", "https://router.example?next=evil"} {
		t.Run(origin, func(t *testing.T) {
			rewardRiskExec(t, s, `update site_settings set public_base_url=$1 where id=true`, origin)
			r := httptest.NewRequest("POST", "http://attacker.invalid/auth/password-reset/request", strings.NewReader(`{"email":"reset-origin@example.test"}`))
			r.Header.Set("X-Forwarded-Host", "attacker.invalid")
			r.Header.Set("X-Forwarded-Proto", "https")
			w := httptest.NewRecorder()
			s.requestPasswordReset(w, r)
			rewardRiskJSON(t, w, http.StatusServiceUnavailable)
			if got := rewardRiskCount(t, s, `select count(*) from password_reset_tokens where email='reset-origin@example.test'`); got != 0 {
				t.Fatal("invalid origin created reset token")
			}
			if _, err := s.resetLink(ctx, "fake-token"); err == nil {
				t.Fatal("invalid origin generated reset URL")
			}
			if _, err := s.callbackURI(ctx, "github"); err == nil {
				t.Fatal("invalid origin generated callback URL")
			}
			r = httptest.NewRequest("GET", "http://attacker.invalid/auth/oauth/github/authorize", nil)
			r.SetPathValue("provider", "github")
			w = httptest.NewRecorder()
			s.oauthAuthorize(w, r)
			rewardRiskJSON(t, w, http.StatusServiceUnavailable)
			if w.Header().Get("Location") != "" {
				t.Fatal("invalid origin redirected OAuth")
			}
			if len(w.Result().Cookies()) != 0 {
				t.Fatal("invalid origin issued OAuth state cookie")
			}
		})
	}
	rewardRiskExec(t, s, `update site_settings set public_base_url='https://router.example.test/' where id=true`)
	if link, err := s.resetLink(ctx, "fake&token"); err != nil || link != "https://router.example.test/auth/reset?token=fake%26token" {
		t.Fatalf("link=%s err=%v", link, err)
	}
	if link, err := s.callbackURI(ctx, "github"); err != nil || link != "https://router.example.test/api/auth/oauth/github/callback" {
		t.Fatalf("callback=%s err=%v", link, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.resetLink(cancelled, "fake-token"); err == nil {
		t.Fatal("query failure generated reset URL")
	}
	if _, err := s.callbackURI(cancelled, "github"); err == nil {
		t.Fatal("query failure generated callback URL")
	}
}

func TestIntegrationAuthResetExpiredAndConcurrent(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	id, _ := integrationUser(t, s.db, "reset-race@example.test", 0)
	rewardRiskExec(t, s, `insert into password_reset_tokens(email,token_hash,expires_at) values('reset-race@example.test',$1,$2)`, hashSecret("fake-expired"), time.Now().Add(-time.Minute))
	rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/password-reset/confirm", `{"token":"fake-expired","password":"new-password123"}`, nil), http.StatusBadRequest)
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is not null`, id); got != 0 {
		t.Fatal("expired token verified email")
	}
	rewardRiskExec(t, s, `update password_reset_tokens set consumed_at=now() where email='reset-race@example.test'`)
	rewardRiskExec(t, s, `insert into password_reset_tokens(email,token_hash,expires_at) values('reset-race@example.test',$1,now()+interval '30 minutes')`, hashSecret("fake-concurrent"))
	responses := rewardRiskParallel(t, 2, func() *httptest.ResponseRecorder {
		return authSecurityRequest(s, "POST", "/auth/password-reset/confirm", `{"token":"fake-concurrent","password":"new-password123"}`, nil)
	})
	ok, rejected := 0, 0
	for _, response := range responses {
		switch response.Code {
		case http.StatusOK:
			ok++
		case http.StatusBadRequest:
			rejected++
		default:
			t.Fatalf("unexpected reset: %d %s", response.Code, response.Body.String())
		}
	}
	if ok != 1 || rejected != 1 {
		t.Fatalf("concurrent token consumption successes=%d rejected=%d", ok, rejected)
	}
}

func TestIntegrationAuthOAuthCallbacks(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	rewardRiskExec(t, s, `update site_settings set public_base_url='https://router.example.test' where id=true`)
	rewardRiskExec(t, s, `insert into oauth_providers(id,client_id,client_secret_encrypted,enabled) values('github','fake-client',$1,true) on conflict(id) do update set client_id=excluded.client_id,client_secret_encrypted=excluded.client_secret_encrypted,enabled=true`, mustCrypt(t, "fake-secret"))
	integrationUser(t, s.db, "callback-existing@example.test", 0)
	for _, tc := range []struct {
		email string
		want  int
	}{
		{"callback-existing@example.test", http.StatusConflict},
		{"callback-new@example.test", http.StatusFound},
	} {
		t.Run(tc.email, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/login/oauth/access_token":
					if err := r.ParseForm(); err != nil || r.Form.Get("redirect_uri") != "https://router.example.test/api/auth/oauth/github/callback" {
						t.Errorf("untrusted callback URI: %v", r.Form)
					}
					writeJSON(w, 200, map[string]any{"access_token": "fake-token"})
				case "/user":
					writeJSON(w, 200, map[string]any{"id": 123, "login": "new-callback-user"})
				case "/user/emails":
					writeJSON(w, 200, []map[string]any{{"email": tc.email, "primary": true, "verified": true}})
				default:
					t.Errorf("unexpected upstream request %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			s.httpClient = testGithubService(t, server).httpClient
			nonce := fmt.Sprintf("github:%d:fake-nonce", time.Now().Unix())
			state := nonce + "." + oauthStateSignature(s.cfg.EncryptionKey, nonce)
			r := httptest.NewRequest("GET", "https://router.example.test/auth/oauth/github/callback?code=fake-code&state="+url.QueryEscape(state), nil)
			r.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: state})
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
			if tc.want == http.StatusConflict {
				if !strings.Contains(w.Body.String(), "email_verification_required") {
					t.Fatalf("missing recovery guidance: %s", w.Body.String())
				}
				for _, cookie := range w.Result().Cookies() {
					if cookie.Name == sessionCookieName && cookie.Value != "" {
						t.Fatal("conflict issued session")
					}
				}
			} else {
				authSecurityCookie(t, w)
				if w.Header().Get("Location") != "/console" {
					t.Fatalf("redirect=%q", w.Header().Get("Location"))
				}
				if got := rewardRiskCount(t, s, `select count(*) from users where email=$1 and email_verified_at is not null`, tc.email); got != 1 {
					t.Fatal("upstream proof not persisted")
				}
			}
		})
	}
}

func TestIntegrationAuthRecoveryRollsBackOnRevocationFailure(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	id, _ := integrationUser(t, s.db, "rollback@example.test", 0)
	integrationKey(t, s.db, id, "sk-xh-fake-rollback-key")
	if _, _, err := s.createSessionToken(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, `insert into user_oauth_connections(user_id,provider,provider_user_id) values($1,'github','rollback')`, id)
	rewardRiskExec(t, s, `insert into password_reset_tokens(email,token_hash,expires_at) values('rollback@example.test',$1,now()+interval '30 minutes')`, hashSecret("fake-rollback"))
	rewardRiskExec(t, s, `create function reject_test_key_revocation() returns trigger language plpgsql as $$ begin raise exception 'test revocation failure'; end $$; create trigger reject_test_key_revocation before update on api_keys for each row execute function reject_test_key_revocation()`)
	w := authSecurityRequest(s, "POST", "/auth/password-reset/confirm", `{"token":"fake-rollback","password":"new-password123"}`, nil)
	rewardRiskJSON(t, w, http.StatusInternalServerError)
	var passwordHash string
	if err := s.db.QueryRow(t.Context(), `select password_hash from users where id=$1 and email_verified_at is null`, id).Scan(&passwordHash); err != nil || !passwordMatches(passwordHash, "password123") {
		t.Fatalf("failed recovery changed password/verification: %v", err)
	}
	for _, query := range []string{
		`select count(*) from user_sessions where user_id=$1`,
		`select count(*) from api_keys where user_id=$1 and revoked_at is null`,
		`select count(*) from user_oauth_connections where user_id=$1`,
	} {
		if got := rewardRiskCount(t, s, query, id); got != 1 {
			t.Fatalf("rollback lost credentials: %s count=%d", query, got)
		}
	}
	if got := rewardRiskCount(t, s, `select count(*) from password_reset_tokens where email='rollback@example.test' and consumed_at is null`); got != 1 {
		t.Fatal("failed recovery consumed token")
	}
}
