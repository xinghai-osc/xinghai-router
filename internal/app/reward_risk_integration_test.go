//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rewardRiskIntegrationService(t *testing.T, settings rewardRiskSettings) *Service {
	t.Helper()
	root, dsn := integrationPool(t)
	ctx := context.Background()
	schema := fmt.Sprintf("reward_entry_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := root.Exec(ctx, "create schema "+quoted); err != nil {
		root.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := root.Exec(ctx, "drop schema "+quoted+" cascade"); err != nil {
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
	if err = migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, pool)
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, `update site_settings set reward_risk_settings=$1,checkin_base_reward=1.25,checkin_streak_bonus=0.25,checkin_max_bonus_days=7,invitations_enabled=true,inviter_reward=2,invitee_reward=1 where id=true`, raw)
	return s
}

func rewardRiskExec(t *testing.T, s *Service, query string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func rewardRiskCount(t *testing.T, s *Service, query string, args ...any) int {
	t.Helper()
	var count int
	if err := s.db.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func rewardRiskAssertBalance(t *testing.T, s *Service, userID, want string) {
	t.Helper()
	var balance string
	var matches bool
	if err := s.db.QueryRow(context.Background(), `select balance::text,balance=$2::numeric from user_wallets where user_id=$1`, userID, want).Scan(&balance, &matches); err != nil || !matches {
		t.Fatalf("balance=%s want=%s err=%v", balance, want, err)
	}
}

func rewardRiskRequest(handler http.HandlerFunc, userID, body, ip string, cookie *http.Cookie, pathValues map[string]string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.RemoteAddr = ip + ":12345"
	if userID != "" {
		ctx = context.WithValue(ctx, accountContextKey{}, accountContext{userID: userID})
	}
	r = r.WithContext(ctx)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for key, value := range pathValues {
		r.SetPathValue(key, value)
	}
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func rewardRiskJSON(t *testing.T, w *httptest.ResponseRecorder, wantStatus int) map[string]any {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("status=%d want=%d body=%s", w.Code, wantStatus, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	return body
}

func rewardRiskParallel(t *testing.T, count int, call func() *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	t.Helper()
	results := make(chan *httptest.ResponseRecorder, count)
	for range count {
		go func() { results <- call() }()
	}
	responses := make([]*httptest.ResponseRecorder, 0, count)
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for range count {
		select {
		case response := <-results:
			responses = append(responses, response)
		case <-timer.C:
			t.Fatal("concurrent reward requests timed out")
		}
	}
	return responses
}

func rewardRiskCookie(t *testing.T, s *Service, ip string) (*http.Cookie, rewardRiskSignals) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	signals, err := s.rewardSignals(w, r)
	if err != nil {
		t.Fatal(err)
	}
	cookie, err := r.Cookie(rewardBrowserCookie)
	if err != nil || signals.HTTPIPHash == "" || signals.BrowserHash == "" {
		t.Fatalf("missing trusted signals or browser cookie: %+v err=%v", signals, err)
	}
	return cookie, signals
}

func rewardRiskClaimID(t *testing.T, s *Service, userID string) string {
	t.Helper()
	var id string
	if err := s.db.QueryRow(context.Background(), `select id::text from reward_claims where source='checkin' and user_id=$1`, userID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func rewardRiskAdmin(t *testing.T, s *Service) string {
	t.Helper()
	id, _ := integrationUser(t, s.db, "reviewer@example.test", 0)
	rewardRiskExec(t, s, `update users set role='admin' where id=$1`, id)
	return id
}

func TestIntegrationRewardRiskCheckinThresholdReviewAndWithdrawal(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	adminID := rewardRiskAdmin(t, s)
	ids := make([]string, 0, 5)
	for _, name := range []string{"falcon", "orchid", "marble", "tundra", "zircon"} {
		id, _ := integrationUser(t, s.db, name+"@example.test", 0)
		ids = append(ids, id)
	}
	for _, id := range ids[:3] {
		body := rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, id, `{}`, "8.8.8.8", nil, nil), http.StatusOK)
		if body["reward_status"] != "credited" || body["reward"] != 1.25 {
			t.Fatalf("clean check-in: %#v", body)
		}
	}
	responses := rewardRiskParallel(t, 8, func() *httptest.ResponseRecorder {
		return rewardRiskRequest(s.accountCheckin, ids[3], `{}`, "8.8.8.8", nil, nil)
	})
	fresh := 0
	var date string
	for _, response := range responses {
		body := rewardRiskJSON(t, response, http.StatusOK)
		if body["reward_status"] != "pending" || body["reward"] != float64(0) || body["scheduled_reward"] != 1.25 {
			t.Fatalf("held check-in: %#v", body)
		}
		if body["already_checked_in"] == false {
			fresh++
		}
		date = body["checkin_date"].(string)
	}
	if fresh != 1 || rewardRiskCount(t, s, `select count(*) from user_checkins`) != 4 || rewardRiskCount(t, s, `select count(*) from risk_observations where action='checkin'`) != 4 || rewardRiskCount(t, s, `select count(*) from wallet_ledger where kind='checkin'`) != 3 {
		t.Fatalf("duplicate check-in created extra rows or rewards; fresh=%d", fresh)
	}
	rewardRiskAssertBalance(t, s, ids[3], "0")
	claimID := rewardRiskClaimID(t, s, ids[3])
	responses = rewardRiskParallel(t, 8, func() *httptest.ResponseRecorder {
		return rewardRiskRequest(s.approveRewardRiskClaim, adminID, `{"reason":"Verified legitimate account"}`, "1.1.1.1", nil, map[string]string{"id": claimID})
	})
	for _, response := range responses {
		rewardRiskJSON(t, response, http.StatusOK)
	}
	if rewardRiskCount(t, s, `select count(*) from wallet_ledger where reward_claim_id=$1`, claimID) != 1 {
		t.Fatal("concurrent approval did not produce exactly one credit")
	}
	rewardRiskAssertBalance(t, s, ids[3], "1.25")
	rewardRiskExec(t, s, `update user_wallets set reserved=1 where user_id=$1`, ids[3])
	path := map[string]string{"user_id": ids[3], "date": date}
	rewardRiskJSON(t, rewardRiskRequest(s.withdrawAdminCheckin, adminID, `{}`, "1.1.1.1", nil, path), http.StatusBadRequest)
	rewardRiskAssertBalance(t, s, ids[3], "1.25")
	if rewardRiskCount(t, s, `select count(*) from reward_claims where id=$1 and status='credited'`, claimID) != 1 {
		t.Fatal("failed reserved-balance withdrawal changed the claim")
	}
	rewardRiskExec(t, s, `update user_wallets set reserved=0 where user_id=$1`, ids[3])
	responses = rewardRiskParallel(t, 8, func() *httptest.ResponseRecorder {
		return rewardRiskRequest(s.withdrawAdminCheckin, adminID, `{}`, "1.1.1.1", nil, path)
	})
	debitTotal := float64(0)
	for _, response := range responses {
		debitTotal += rewardRiskJSON(t, response, http.StatusOK)["reward"].(float64)
	}
	if debitTotal != 1.25 || rewardRiskCount(t, s, `select count(*) from wallet_ledger where user_id=$1 and kind='adjustment'`, ids[3]) != 1 {
		t.Fatalf("withdrawal debited multiple times: total=%v", debitTotal)
	}
	rewardRiskAssertBalance(t, s, ids[3], "0")
	if rewardRiskCount(t, s, `select count(*) from user_checkins where user_id=$1 and reward_status='withdrawn'`, ids[3]) != 1 || rewardRiskCount(t, s, `select count(*) from reward_claims where id=$1 and status='withdrawn'`, claimID) != 1 {
		t.Fatal("withdrawal did not retain both claim and check-in history")
	}
	body := rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, ids[3], `{}`, "8.8.8.8", nil, nil), http.StatusOK)
	if body["already_checked_in"] != true || body["reward_status"] != "withdrawn" || body["reward"] != float64(0) {
		t.Fatalf("withdrawn date was reclaimed: %#v", body)
	}
	body = rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, ids[4], `{}`, "8.8.8.8", nil, nil), http.StatusOK)
	if body["reward_status"] != "pending" {
		t.Fatalf("expected fifth check-in to be held: %#v", body)
	}
	rejectedID := rewardRiskClaimID(t, s, ids[4])
	rewardRiskJSON(t, rewardRiskRequest(s.rejectRewardRiskClaim, adminID, `{"reason":"Duplicate reward abuse"}`, "1.1.1.1", nil, map[string]string{"id": rejectedID}), http.StatusOK)
	rewardRiskJSON(t, rewardRiskRequest(s.approveRewardRiskClaim, adminID, `{}`, "1.1.1.1", nil, map[string]string{"id": rejectedID}), http.StatusConflict)
	body = rewardRiskJSON(t, rewardRiskRequest(s.withdrawAdminCheckin, adminID, `{}`, "1.1.1.1", nil, map[string]string{"user_id": ids[4], "date": date}), http.StatusOK)
	if body["reward"] != float64(0) || rewardRiskCount(t, s, `select count(*) from wallet_ledger where user_id=$1`, ids[4]) != 0 {
		t.Fatal("withdrawing a rejected claim debited the wallet")
	}
}

func TestIntegrationRewardRiskDailyBrowserAndIPReset(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	cookie, _ := rewardRiskCookie(t, s, "8.8.8.8")
	ids := make([]string, 0, 3)
	for _, name := range []string{"falcon", "orchid", "marble"} {
		id, _ := integrationUser(t, s.db, name+"@example.test", 0)
		ids = append(ids, id)
	}
	body := rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, ids[0], `{}`, "8.8.8.8", cookie, nil), http.StatusOK)
	if body["reward_status"] != "credited" {
		t.Fatalf("first browser check-in: %#v", body)
	}
	body = rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, ids[1], `{}`, "1.1.1.1", cookie, nil), http.StatusOK)
	if body["reward_status"] != "pending" {
		t.Fatalf("shared browser escaped daily limit across IPs: %#v", body)
	}
	rewardRiskExec(t, s, `update risk_observations set created_at=((clock_timestamp() at time zone 'UTC')::date-1)::timestamp at time zone 'UTC'`)
	body = rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, ids[2], `{}`, "8.8.8.8", cookie, nil), http.StatusOK)
	if body["reward_status"] != "credited" {
		t.Fatalf("previous UTC day still consumed today's quota: %#v", body)
	}
}

func TestIntegrationRewardRiskZeroLegacyAndWithdrawnStreak(t *testing.T) {
	settings := defaultRewardRiskSettings()
	settings.Enabled = false
	s := rewardRiskIntegrationService(t, settings)
	id, _ := integrationUser(t, s.db, "legacy@example.test", 2)
	adminID := rewardRiskAdmin(t, s)
	var yesterday string
	if err := s.db.QueryRow(context.Background(), `select ((clock_timestamp() at time zone 'UTC')::date-1)::text`).Scan(&yesterday); err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, `insert into user_checkins(user_id,checkin_date,streak,reward) values($1,$2,99,2)`, id, yesterday)
	body := rewardRiskJSON(t, rewardRiskRequest(s.withdrawAdminCheckin, adminID, `{}`, "1.1.1.1", nil, map[string]string{"user_id": id, "date": yesterday}), http.StatusOK)
	if body["reward"] != float64(2) {
		t.Fatalf("legacy withdrawal=%#v", body)
	}
	body = rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, id, `{}`, "8.8.8.8", nil, nil), http.StatusOK)
	if body["streak"] != float64(1) || body["reward"] != 1.25 {
		t.Fatalf("withdrawn previous day inflated streak: %#v", body)
	}
	zeroID, _ := integrationUser(t, s.db, "zero@example.test", 0)
	rewardRiskExec(t, s, `update site_settings set checkin_base_reward=0,checkin_streak_bonus=0 where id=true`)
	body = rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, zeroID, `{}`, "8.8.4.4", nil, nil), http.StatusOK)
	if body["reward_status"] != "credited" || body["reward"] != float64(0) || rewardRiskCount(t, s, `select count(*) from reward_claims where user_id=$1 and amount=0`, zeroID) != 1 || rewardRiskCount(t, s, `select count(*) from wallet_ledger where user_id=$1`, zeroID) != 0 {
		t.Fatalf("zero reward created an invalid claim or ledger: %#v", body)
	}
}

func TestIntegrationRewardRiskInvitationRegistrationBanAndLogin(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	inviterID, _ := integrationUser(t, s.db, "sponsor@example.test", 0)
	rewardRiskExec(t, s, `insert into invitation_codes(user_id,code) values($1,'RISKINVITE')`, inviterID)
	cookie, _ := rewardRiskCookie(t, s, "8.8.8.8")
	var bannedID string
	for i := 1; i <= 5; i++ {
		email := fmt.Sprintf("cohort%03d@example.test", i)
		payload := fmt.Sprintf(`{"email":%q,"name":%q,"password":"password123","invitation_code":"RISKINVITE"}`, email, fmt.Sprintf("cohort%03d", i))
		want := http.StatusCreated
		if i == 5 {
			want = http.StatusForbidden
		}
		response := rewardRiskRequest(s.register, "", payload, "8.8.8.8", cookie, nil)
		body := rewardRiskJSON(t, response, want)
		var userID string
		var enabled bool
		if err := s.db.QueryRow(context.Background(), `select id::text,enabled from users where email=$1`, email).Scan(&userID, &enabled); err != nil {
			t.Fatal(err)
		}
		status := "credited"
		if i >= 3 {
			status = "pending"
		}
		if rewardRiskCount(t, s, `select count(*) from reward_claims where origin_user_id=$1 and source='invitation' and status=$2`, userID, status) != 2 {
			t.Fatalf("signup %d did not share one decision for both recipients: %#v", i, body)
		}
		if i == 5 {
			bannedID = userID
			if enabled || rewardRiskCount(t, s, `select count(*) from user_sessions where user_id=$1`, userID) != 0 || rewardRiskCount(t, s, `select count(*) from risk_bans where user_id=$1 and released_at is null`, userID) != 1 {
				t.Fatalf("ban was not committed without a session: enabled=%v", enabled)
			}
			for _, c := range response.Result().Cookies() {
				if c.Name == sessionCookieName && c.Value != "" {
					t.Fatal("banned registration issued a session cookie")
				}
			}
			rewardRiskAssertBalance(t, s, userID, "0")
		}
	}
	if rewardRiskCount(t, s, `select count(*) from invitations`) != 5 || rewardRiskCount(t, s, `select count(*) from reward_claims`) != 10 || rewardRiskCount(t, s, `select count(*) from wallet_ledger where kind='invitation'`) != 4 {
		t.Fatal("invitation cohort has incorrect relationship, claim, or ledger counts")
	}
	rewardRiskAssertBalance(t, s, inviterID, "4")
	adminID := rewardRiskAdmin(t, s)
	var blockedClaimID string
	if err := s.db.QueryRow(context.Background(), `select id::text from reward_claims where origin_user_id=$1 and user_id=$2`, bannedID, inviterID).Scan(&blockedClaimID); err != nil {
		t.Fatal(err)
	}
	rewardRiskJSON(t, rewardRiskRequest(s.approveRewardRiskClaim, adminID, `{"reason":"Check origin restriction"}`, "1.1.1.1", nil, map[string]string{"id": blockedClaimID}), http.StatusConflict)
	list := rewardRiskJSON(t, rewardRiskRequest(s.accountInvitations, inviterID, ``, "1.1.1.1", nil, nil), http.StatusOK)
	credited, pending := 0, 0
	for _, raw := range list["data"].([]any) {
		entry := raw.(map[string]any)
		if entry["inviter_reward_status"] != entry["invitee_reward_status"] {
			t.Fatalf("invitation recipients disagree: %#v", entry)
		}
		switch entry["inviter_reward_status"] {
		case "credited":
			credited++
		case "pending":
			pending++
		}
	}
	if credited != 2 || pending != 3 {
		t.Fatalf("invitation listing statuses credited=%d pending=%d", credited, pending)
	}
	rewardRiskJSON(t, rewardRiskRequest(s.login, "", `{"email":"cohort001@example.test","password":"password123"}`, "8.8.8.8", cookie, nil), http.StatusOK)
	if rewardRiskCount(t, s, `select count(*) from risk_observations where action='login' and decision='allow'`) != 1 || rewardRiskCount(t, s, `select count(*) from risk_bans`) != 1 {
		t.Fatal("successful login either was not observed or triggered a new ban")
	}
	rewardRiskJSON(t, rewardRiskRequest(s.register, "", `{"email":"independent@example.test","name":"independent","password":"password123"}`, "1.1.1.1", nil, nil), http.StatusCreated)
	if rewardRiskCount(t, s, `select count(*) from risk_observations o join users u on u.id=o.user_id where u.email='independent@example.test' and o.action='register'`) != 1 {
		t.Fatal("registration without an invitation skipped risk observation")
	}
	rewardRiskExec(t, s, `update users set enabled=false where id=$1`, inviterID)
	rewardRiskJSON(t, rewardRiskRequest(s.register, "", `{"email":"invalidinvite@example.test","name":"invalidinvite","password":"password123","invitation_code":"RISKINVITE"}`, "8.8.4.4", nil, nil), http.StatusBadRequest)
	if rewardRiskCount(t, s, `select count(*) from users where email='invalidinvite@example.test'`) != 0 {
		t.Fatal("disabled inviter registration was not rolled back")
	}
}

func rewardRiskContextID(t *testing.T, s *Service, userID, purpose string, cookie *http.Cookie) string {
	t.Helper()
	body := fmt.Sprintf(`{"purpose":%q,"status":"completed","addresses":["8.8.4.4"]}`, purpose)
	response := rewardRiskJSON(t, rewardRiskRequest(s.createRewardRiskContext, userID, body, "8.8.8.8", cookie, nil), http.StatusCreated)
	return response["id"].(string)
}

func TestIntegrationRewardRiskContextBindingAndReplay(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	ctx := context.Background()
	userID, _ := integrationUser(t, s.db, "falcon@example.test", 0)
	otherID, _ := integrationUser(t, s.db, "orchid@example.test", 0)
	cookie, signals := rewardRiskCookie(t, s, "8.8.8.8")
	_, otherSignals := rewardRiskCookie(t, s, "8.8.8.8")
	id := rewardRiskContextID(t, s, userID, "checkin", cookie)
	for _, tc := range []struct {
		name, purpose, userID string
		signals               rewardRiskSignals
	}{
		{"wrong browser", "checkin", userID, otherSignals},
		{"wrong user", "checkin", otherID, signals},
		{"wrong purpose", "register", userID, signals},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := s.beginRewardRiskTx(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := s.consumeRewardContextTx(ctx, tx, id, tc.purpose, tc.signals, tc.userID); !errors.Is(err, errRiskContext) {
				t.Fatalf("context mismatch err=%v", err)
			}
		})
	}
	tx, err := s.beginRewardRiskTx(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	consumed, err := s.consumeRewardContextTx(ctx, tx, id, "checkin", signals, userID)
	if err != nil || len(consumed.RTCHashes) != 1 || consumed.RTCStatus != "completed" || consumed.HTTPIPHash != signals.HTTPIPHash {
		t.Fatalf("context signals=%+v err=%v", consumed, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = s.beginRewardRiskTx(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.consumeRewardContextTx(ctx, tx, id, "checkin", signals, userID); !errors.Is(err, errRiskContext) {
		t.Fatalf("consumed context replay err=%v", err)
	}
	tx.Rollback(ctx)
	expiredID := rewardRiskContextID(t, s, "", "register", cookie)
	rewardRiskExec(t, s, `update risk_contexts set expires_at=clock_timestamp()-interval '1 second' where id=$1`, expiredID)
	body := fmt.Sprintf(`{"email":"expired@example.test","name":"expired","password":"password123","risk_context_id":%q}`, expiredID)
	rewardRiskJSON(t, rewardRiskRequest(s.register, "", body, "8.8.8.8", cookie, nil), http.StatusBadRequest)
	if rewardRiskCount(t, s, `select count(*) from users where email='expired@example.test'`) != 0 {
		t.Fatal("expired registration context left an account behind")
	}
	validID := rewardRiskContextID(t, s, userID, "checkin", cookie)
	body = fmt.Sprintf(`{"risk_context_id":%q}`, validID)
	rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, userID, body, "8.8.8.8", cookie, nil), http.StatusOK)
	if rewardRiskCount(t, s, `select count(*) from risk_contexts where id=$1 and consumed_at is not null`, validID) != 1 || rewardRiskCount(t, s, `select count(*) from risk_observations where user_id=$1 and rtc_status='completed' and cardinality(rtc_hashes)=1`, userID) != 1 {
		t.Fatal("check-in did not consume and record its bound browser context")
	}
}

func TestIntegrationRewardRiskOAuthContextAndRestrictedIdentity(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	ctx := context.Background()
	cookie, signals := rewardRiskCookie(t, s, "8.8.8.8")
	id := rewardRiskContextID(t, s, "", "oauth", cookie)
	nonce := "github:integration:bound-context"
	if err := s.bindOAuthRiskContext(ctx, id, "github", nonce, signals); err != nil {
		t.Fatal(err)
	}
	if err := s.bindOAuthRiskContext(ctx, id, "github", "different-nonce", signals); !errors.Is(err, errRiskContext) {
		t.Fatalf("OAuth context was rebound: %v", err)
	}
	for _, tc := range []struct {
		name, provider string
		signals        rewardRiskSignals
	}{
		{"wrong provider", "other", signals},
		{"wrong browser", "github", rewardRiskSignals{HTTPIPHash: signals.HTTPIPHash, BrowserHash: "different-browser"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.findOrCreateOAuthUser(ctx, "github", "101", "oauth@example.test", true, "oauth-account", "", &oauthRewardRiskInput{Signals: tc.signals, Provider: tc.provider, StateNonce: nonce})
			if !errors.Is(err, errRiskContext) || rewardRiskCount(t, s, `select count(*) from users where email='oauth@example.test'`) != 0 {
				t.Fatalf("invalid OAuth context did not roll back identity: %v", err)
			}
		})
	}
	input := &oauthRewardRiskInput{Signals: signals, Provider: "github", StateNonce: nonce}
	userID, err := s.findOrCreateOAuthUser(ctx, "github", "101", "oauth@example.test", true, "oauth-account", "", input)
	if err != nil {
		t.Fatal(err)
	}
	if rewardRiskCount(t, s, `select count(*) from risk_contexts where id=$1 and consumed_at is not null`, id) != 1 || rewardRiskCount(t, s, `select count(*) from risk_observations where user_id=$1 and action='register' and rtc_status='completed' and cardinality(rtc_hashes)=1`, userID) != 1 || rewardRiskCount(t, s, `select count(*) from reward_claims`) != 0 {
		t.Fatal("OAuth account creation did not record its bound registration context")
	}
	if _, err = s.findOrCreateOAuthUser(ctx, "github", "101", "oauth@example.test", true, "oauth-account", "", input); !errors.Is(err, errRiskContext) {
		t.Fatalf("OAuth context replay err=%v", err)
	}
	input.StateNonce = "github:integration:fresh-login"
	loggedInID, err := s.findOrCreateOAuthUser(ctx, "github", "101", "oauth@example.test", true, "oauth-account", "", input)
	if err != nil || loggedInID != userID || rewardRiskCount(t, s, `select count(*) from risk_observations where user_id=$1 and action='login'`, userID) != 1 {
		t.Fatalf("existing OAuth user login id=%s err=%v", loggedInID, err)
	}
	rewardRiskExec(t, s, `update users set enabled=false where id=$1`, userID)
	for _, tc := range []struct {
		name, providerID, email string
	}{
		{"connected identity", "101", "new-email@example.test"},
		{"matching verified email", "202", "oauth@example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.findOrCreateOAuthUser(ctx, "github", tc.providerID, tc.email, true, "another-name", "", input); !errors.Is(err, errRiskAccountRestricted) {
				t.Fatalf("disabled identity was not restricted: %v", err)
			}
		})
	}
	if rewardRiskCount(t, s, `select count(*) from user_oauth_connections`) != 1 || rewardRiskCount(t, s, `select count(*) from users where email='new-email@example.test'`) != 0 {
		t.Fatal("restricted OAuth identity created a replacement connection or account")
	}
	syntheticID, _ := integrationUser(t, s.db, "github-303@oauth.local", 0)
	rewardRiskExec(t, s, `update users set enabled=false where id=$1`, syntheticID)
	if _, err = s.findOrCreateOAuthUser(ctx, "github", "303", "", false, "synthetic-account", "", input); !errors.Is(err, errRiskAccountRestricted) {
		t.Fatalf("disabled synthetic identity was not restricted: %v", err)
	}
	rewardRiskExec(t, s, `update site_settings set registration_email_whitelist_enabled=true,registration_email_whitelist=array['allowed.test'] where id=true`)
	if _, err = s.findOrCreateOAuthUser(ctx, "github", "404", "blocked@denied.test", true, "blocked-oauth", "", input); !errors.Is(err, errOAuthEmailNotAllowed) {
		t.Fatalf("OAuth registration skipped the email whitelist: %v", err)
	}
}

func TestIntegrationRewardRiskConcurrentClaimDecisions(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	adminID := rewardRiskAdmin(t, s)
	firstID, _ := integrationUser(t, s.db, "falcon@example.test", 0)
	secondID, _ := integrationUser(t, s.db, "orchid@example.test", 0)
	cookie, _ := rewardRiskCookie(t, s, "8.8.8.8")
	rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, firstID, `{}`, "8.8.8.8", cookie, nil), http.StatusOK)
	body := rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, secondID, `{}`, "1.1.1.1", cookie, nil), http.StatusOK)
	if body["reward_status"] != "pending" {
		t.Fatalf("expected pending claim before competing decisions: %#v", body)
	}
	claimID := rewardRiskClaimID(t, s, secondID)
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	for _, handler := range []http.HandlerFunc{s.approveRewardRiskClaim, s.rejectRewardRiskClaim} {
		go func(handler http.HandlerFunc) {
			<-start
			results <- rewardRiskRequest(handler, adminID, `{"reason":"Concurrent review"}`, "1.0.0.1", nil, map[string]string{"id": claimID})
		}(handler)
	}
	close(start)
	ok, conflict := 0, 0
	for range 2 {
		response := <-results
		switch response.Code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("unexpected concurrent review: %d %s", response.Code, response.Body.String())
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("competing decisions ok=%d conflict=%d", ok, conflict)
	}
	var claimStatus, checkinStatus string
	if err := s.db.QueryRow(context.Background(), `select c.status,u.reward_status from reward_claims c join user_checkins u on u.reward_source_id::text=c.source_id where c.id=$1`, claimID).Scan(&claimStatus, &checkinStatus); err != nil || claimStatus != checkinStatus {
		t.Fatalf("claim/check-in statuses disagree: %s/%s err=%v", claimStatus, checkinStatus, err)
	}
	wantBalance, wantLedger := "0", 0
	if claimStatus == "credited" {
		wantBalance, wantLedger = "1.25", 1
	}
	rewardRiskAssertBalance(t, s, secondID, wantBalance)
	if rewardRiskCount(t, s, `select count(*) from wallet_ledger where reward_claim_id=$1`, claimID) != wantLedger {
		t.Fatal("competing review decisions produced incorrect ledger credit")
	}
}

func TestIntegrationRewardRiskCheckinInviterCohortBan(t *testing.T) {
	settings := defaultRewardRiskSettings()
	settings.CheckinsPerIP = 100
	s := rewardRiskIntegrationService(t, settings)
	inviterID, _ := integrationUser(t, s.db, "sponsor@example.test", 0)
	for i := 1; i <= 5; i++ {
		id, _ := integrationUser(t, s.db, fmt.Sprintf("cohort%03d@example.test", i), 0)
		rewardRiskExec(t, s, `insert into invitations(id,inviter_id,invitee_id,code,inviter_reward,invitee_reward) values(gen_random_uuid(),$1,$2,'RISKINVITE',0,0)`, inviterID, id)
		want := http.StatusOK
		if i == 5 {
			want = http.StatusForbidden
		}
		rewardRiskJSON(t, rewardRiskRequest(s.accountCheckin, id, `{}`, "8.8.8.8", nil, nil), want)
		if i == 5 {
			if rewardRiskCount(t, s, `select count(*) from users where id=$1 and not enabled`, id) != 1 || rewardRiskCount(t, s, `select count(*) from user_checkins where user_id=$1 and reward_status='pending'`, id) != 1 || rewardRiskCount(t, s, `select count(*) from reward_claims where user_id=$1 and status='pending'`, id) != 1 {
				t.Fatal("cohort check-in ban did not retain disabled account and pending reward")
			}
			rewardRiskAssertBalance(t, s, id, "0")
		}
	}
	if rewardRiskCount(t, s, `select count(*) from risk_observations where action='checkin' and inviter_id=$1`, inviterID) != 5 || rewardRiskCount(t, s, `select count(*) from risk_bans`) != 1 {
		t.Fatal("check-in burst omitted the inviter cohort")
	}
}

func TestIntegrationRewardRiskOAuthAuthorizeCallback(t *testing.T) {
	s := rewardRiskIntegrationService(t, defaultRewardRiskSettings())
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			writeJSON(w, http.StatusOK, map[string]any{"access_token": "fake-integration-provider-token"})
		case "/user":
			writeJSON(w, http.StatusOK, map[string]any{"id": 12345, "login": "oauth-callback", "name": "OAuth Callback"})
		case "/user/emails":
			writeJSON(w, http.StatusOK, []map[string]any{{"email": "callback@example.test", "primary": true, "verified": true}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	s.httpClient = &http.Client{Transport: githubAPITransport{base: provider.Listener.Addr().String()}}
	rewardRiskExec(t, s, `insert into oauth_providers(id,client_id,client_secret_encrypted,enabled) values('github','fake-client-id',$1,true) on conflict(id) do update set client_id=excluded.client_id,client_secret_encrypted=excluded.client_secret_encrypted,enabled=true`, mustCrypt(t, "fake-client-secret"))
	cookie, signals := rewardRiskCookie(t, s, "8.8.8.8")
	callback := func(contextID string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/auth/oauth/github/authorize?risk_context_id="+url.QueryEscape(contextID), nil)
		r.RemoteAddr = "8.8.8.8:12345"
		r.SetPathValue("provider", "github")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.oauthAuthorize(w, r)
		if w.Code != http.StatusFound {
			t.Fatalf("authorize status=%d body=%s", w.Code, w.Body.String())
		}
		location, err := url.Parse(w.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		state := location.Query().Get("state")
		dot := strings.LastIndexByte(state, '.')
		if dot < 0 {
			t.Fatalf("missing signed state in redirect: %s", location)
		}
		if rewardRiskCount(t, s, `select count(*) from risk_contexts where id=$1 and oauth_nonce_hash=$2 and oauth_provider='github' and browser_hash=$3`, contextID, s.riskHash("oauth", state[:dot]), signals.BrowserHash) != 1 {
			t.Fatal("OAuth authorization did not bind its risk context to the browser and state")
		}
		var stateCookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == oauthStateCookie {
				stateCookie = c
			}
		}
		if stateCookie == nil {
			t.Fatal("OAuth authorization did not set the state cookie")
		}
		r = httptest.NewRequest(http.MethodGet, "/auth/oauth/github/callback?code=fake-provider-code&state="+url.QueryEscape(state), nil)
		r.RemoteAddr = "8.8.8.8:12345"
		r.SetPathValue("provider", "github")
		r.AddCookie(cookie)
		r.AddCookie(stateCookie)
		w = httptest.NewRecorder()
		s.oauthCallback(w, r)
		return w
	}
	id := rewardRiskContextID(t, s, "", "oauth", cookie)
	response := callback(id)
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/console" {
		t.Fatalf("OAuth callback status=%d body=%s", response.Code, response.Body.String())
	}
	if rewardRiskCount(t, s, `select count(*) from risk_contexts where id=$1 and consumed_at is not null`, id) != 1 || rewardRiskCount(t, s, `select count(*) from user_sessions`) != 1 || rewardRiskCount(t, s, `select count(*) from risk_observations where action='register' and rtc_status='completed'`) != 1 {
		t.Fatal("OAuth callback did not consume risk context and create the account session")
	}
	rewardRiskExec(t, s, `update users set enabled=false where email='callback@example.test'`)
	rewardRiskExec(t, s, `delete from user_sessions`)
	id = rewardRiskContextID(t, s, "", "oauth", cookie)
	response = callback(id)
	rewardRiskJSON(t, response, http.StatusForbidden)
	if rewardRiskCount(t, s, `select count(*) from user_sessions`) != 0 || rewardRiskCount(t, s, `select count(*) from user_oauth_connections`) != 1 {
		t.Fatal("restricted OAuth callback created a session or replacement identity")
	}
	for _, c := range response.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			t.Fatal("restricted OAuth callback issued a session cookie")
		}
	}
}
