package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMiniMaxBalanceDatabaseCredentialsAndRefresh(t *testing.T) {
	s := channelCredentialDatabase(t)
	ctx := context.Background()
	channelID := channelCredentialInsert(t, s, "minimax-credits", "fake-model-key")
	if _, err := s.db.Exec(ctx, `update channels set provider='minimax' where id=$1`, channelID); err != nil {
		t.Fatal(err)
	}
	var keyID string
	if err := s.db.QueryRow(ctx, `insert into channel_api_keys(channel_id,key_encrypted) values($1,'encrypted:v1:invalid-model-key') returning id::text`, channelID).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{"id": channelID}
	check := func(handler http.HandlerFunc, body string, status int, configured bool, groupID string) {
		t.Helper()
		w := channelCredentialRequest(t, handler, body, paths, status)
		if strings.Contains(w.Body.String(), "fake-login") || strings.Contains(w.Body.String(), "cookie") || strings.Contains(w.Body.String(), "encrypted") {
			t.Fatal("credential response exposed sensitive fields")
		}
		if status == http.StatusOK {
			var got minimaxBalanceConfiguration
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Configured != configured || got.GroupID != groupID {
				t.Fatalf("configuration=%+v err=%v", got, err)
			}
		}
	}
	check(s.getMiniMaxBalanceCredentials, "", http.StatusOK, false, "")
	channelCredentialRequest(t, s.updateMiniMaxBalanceCredentials, `{"cookie":"","group_id":"123"}`, paths, http.StatusBadRequest)
	check(s.updateMiniMaxBalanceCredentials, `{"cookie":"_token=fake-login-cookie","group_id":"123"}`, http.StatusOK, true, "123")
	var encrypted string
	if err := s.db.QueryRow(ctx, `select cookie_encrypted from minimax_balance_credentials where channel_id=$1`, channelID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "fake-login") {
		t.Fatal("login cookie was stored as plaintext")
	}
	decrypted, err := crypt(s.cfg.EncryptionKey, encrypted, true)
	if err != nil || decrypted != "_token=fake-login-cookie" {
		t.Fatal("stored login cookie cannot be decrypted")
	}
	check(s.getMiniMaxBalanceCredentials, "", http.StatusOK, true, "123")
	check(s.updateMiniMaxBalanceCredentials, `{"cookie":"","group_id":"456"}`, http.StatusOK, true, "456")
	var unchanged string
	if err := s.db.QueryRow(ctx, `select cookie_encrypted from minimax_balance_credentials where channel_id=$1`, channelID).Scan(&unchanged); err != nil || unchanged != encrypted {
		t.Fatal("empty cookie failed to preserve encrypted credential")
	}
	calls := 0
	s.httpClient = &http.Client{Transport: miniMaxBalanceTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != minimaxBalanceURL || r.Header.Get("Cookie") != "_token=fake-login-cookie" || r.Header.Get("X-Group-Id") != "456" || r.Header.Get("Authorization") != "" {
			t.Error("refresh did not use independent login credentials")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"total_credits":0,"used_credits":0,"remaining_credits":0,"api_key":"fake-secret-model-key","base_resp":{"status_code":0}}`)), Request: r}, nil
	})}
	got, err := s.refreshChannelBalance(ctx, channelID, nil)
	if err != nil || got.Balance == nil || *got.Balance != 0 || got.Currency != "credits" || calls != 1 {
		t.Fatalf("refresh=%+v err=%v calls=%d", got, err, calls)
	}
	var currency string
	var balance float64
	if err := s.db.QueryRow(ctx, `select balance,currency from channel_balances where channel_id=$1 and key_id=$2`, channelID, keyID).Scan(&balance, &currency); err != nil || balance != 0 || currency != "credits" {
		t.Fatalf("cached balance=%v currency=%s err=%v", balance, currency, err)
	}
	check(s.updateMiniMaxBalanceCredentials, `{"group_id":"456"}`, http.StatusOK, true, "456")
	var count int
	if err := s.db.QueryRow(ctx, `select count(*) from channel_balances where channel_id=$1`, channelID).Scan(&count); err != nil || count != 0 {
		t.Fatal("credential change did not clear cached balance")
	}
	if _, err := s.refreshChannelBalance(ctx, channelID, &keyID); err != nil {
		t.Fatal(err)
	}
	check(s.deleteMiniMaxBalanceCredentials, "", http.StatusOK, false, "")
	check(s.getMiniMaxBalanceCredentials, "", http.StatusOK, false, "")
	if err := s.db.QueryRow(ctx, `select count(*) from channel_balances where channel_id=$1`, channelID).Scan(&count); err != nil || count != 0 {
		t.Fatal("credential deletion did not clear cached balance")
	}
	if _, err := s.refreshChannelBalance(ctx, channelID, nil); err == nil || calls != 2 {
		t.Fatal("missing credentials reached an upstream")
	}
	check(s.deleteMiniMaxBalanceCredentials, "", http.StatusOK, false, "")
}

func TestMiniMaxBalanceDatabaseRejectsWrongProviderAndMissingChannel(t *testing.T) {
	s := channelCredentialDatabase(t)
	channelID := channelCredentialInsert(t, s, "other-provider", "fake-key")
	for _, handler := range []http.HandlerFunc{s.getMiniMaxBalanceCredentials, s.updateMiniMaxBalanceCredentials, s.deleteMiniMaxBalanceCredentials} {
		channelCredentialRequest(t, handler, `{"cookie":"_token=fake-login","group_id":"123"}`, map[string]string{"id": channelID}, http.StatusBadRequest)
		channelCredentialRequest(t, handler, `{"cookie":"_token=fake-login","group_id":"123"}`, map[string]string{"id": "9223372036854775807"}, http.StatusNotFound)
	}
}
