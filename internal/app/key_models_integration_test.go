//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegrationKeyModelRestriction(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "key-models@example.com", 10)
	const secret = "sk-xh-key-models"
	keyID := integrationKey(t, db, userID, secret)
	if _, err := db.Exec(context.Background(), `update api_keys set allowed_models=array['gpt-4*'] where id=$1`, keyID); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)

	key, err := s.loadAPIKeyContext(context.Background(), secret)
	if err != nil {
		t.Fatalf("load API key context: %v", err)
	}
	if len(key.allowedModels) != 1 || key.allowedModels[0] != "gpt-4*" {
		t.Fatalf("allowed models = %v", key.allowedModels)
	}

	handler := s.api(s.chatCompletions)
	call := func(model string) *httptest.ResponseRecorder {
		body := bytes.NewBufferString(`{"model":"` + model + `","messages":[{"role":"user","content":"hi"}]}`)
		r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
		r.Header.Set("Authorization", "Bearer "+secret)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	denied := call("claude-3-5-sonnet")
	if denied.Code != http.StatusForbidden || !strings.Contains(denied.Body.String(), "model_not_allowed") {
		t.Fatalf("disallowed model: status=%d body=%s", denied.Code, denied.Body.String())
	}
	allowed := call("gpt-4o")
	if allowed.Code == http.StatusForbidden || strings.Contains(allowed.Body.String(), "model_not_allowed") {
		t.Fatalf("allowed model was rejected: status=%d body=%s", allowed.Code, allowed.Body.String())
	}
}

func TestIntegrationAccountKeyModelEndpoints(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "key-models-account@example.com", 10)
	s := integrationService(t, db)

	var workspaceID string
	if err := db.QueryRow(context.Background(), `select id::text from workspaces where owner_id=$1 and is_personal`, userID).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	keyID := integrationKey(t, db, userID, "sk-xh-key-models-account")

	request := func(handler http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.SetPathValue("id", keyID)
		r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{userID: userID, workspaceID: workspaceID}))
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}

	created := request(s.setAccountKeyModels, http.MethodPut, "/account/keys/"+keyID+"/models", `{"allowed_models":["gpt-4o","claude-3*"]}`)
	if created.Code != http.StatusOK {
		t.Fatalf("set models: status=%d body=%s", created.Code, created.Body.String())
	}
	var stored []string
	if err := db.QueryRow(context.Background(), `select allowed_models from api_keys where id=$1`, keyID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0] != "gpt-4o" || stored[1] != "claude-3*" {
		t.Fatalf("stored models = %v", stored)
	}

	rejected := request(s.setAccountKeyModels, http.MethodPut, "/account/keys/"+keyID+"/models", `{"allowed_models":["bad model"]}`)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("invalid model name: status=%d body=%s", rejected.Code, rejected.Body.String())
	}

	kept := request(s.updateAccountKey, http.MethodPut, "/account/keys/"+keyID, `{"name":"renamed"}`)
	if kept.Code != http.StatusOK {
		t.Fatalf("update key: status=%d body=%s", kept.Code, kept.Body.String())
	}
	if err := db.QueryRow(context.Background(), `select allowed_models from api_keys where id=$1`, keyID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("models after an update without the field = %v", stored)
	}

	cleared := request(s.updateAccountKey, http.MethodPut, "/account/keys/"+keyID, `{"name":"renamed","allowed_models":[]}`)
	if cleared.Code != http.StatusOK {
		t.Fatalf("clear models: status=%d body=%s", cleared.Code, cleared.Body.String())
	}
	if err := db.QueryRow(context.Background(), `select allowed_models from api_keys where id=$1`, keyID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("models after clearing = %v", stored)
	}

	listed := request(s.accountKeys, http.MethodGet, "/account/keys", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list keys: status=%d body=%s", listed.Code, listed.Body.String())
	}
	var payload struct {
		Data []struct {
			ID            string   `json:"id"`
			AllowedModels []string `json:"allowed_models"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 1 || len(payload.Data[0].AllowedModels) != 0 {
		t.Fatalf("listed keys = %s", listed.Body.String())
	}
}
