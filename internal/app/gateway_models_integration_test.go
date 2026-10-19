//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntegrationModelsWithMissingMetadata(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "models-metadata@example.com", 0)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `insert into channels(name,base_url,api_key,models,user_id) values('models-metadata','https://example.com','fake-key','["a-without-metadata","b-with-consent","c-without-consent"]',$1)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into model_catalog_metadata(model,requires_data_transfer_consent) values('b-with-consent',true),('c-without-consent',false)`); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, keyContext{userID: userID}))
	rec := httptest.NewRecorder()
	s.models(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("models status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Object string `json:"object"`
		Data   []struct {
			ID              string `json:"id"`
			RequiresConsent *bool  `json:"requires_data_transfer_consent"`
		} `json:"data"`
		Models map[string]json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Object != "list" || len(result.Data) != 3 || len(result.Models) != 3 {
		t.Fatalf("unexpected model list: %s", rec.Body.String())
	}
	for i, id := range []string{"a-without-metadata", "b-with-consent", "c-without-consent"} {
		item := result.Data[i]
		if item.ID != id || item.RequiresConsent == nil || *item.RequiresConsent != (id == "b-with-consent") || result.Models[id] == nil {
			t.Fatalf("unexpected model %s: %s", id, rec.Body.String())
		}
	}
}
