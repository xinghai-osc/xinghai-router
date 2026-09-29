package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInstanceInputValidation(t *testing.T) {
	s := &Service{}
	for _, handler := range []http.HandlerFunc{s.createInstance, s.updateInstance} {
		for _, body := range []string{
			"", "{", "null", `{}`, `{"address":"node:8080"}`, `{"name":"node"}`,
			`{"name":" \t\n ","address":"node:8080"}`,
			`{"name":"node","address":" \t\n "}`,
			`{"name":42,"address":"node:8080"}`,
			`{"name":"node","address":42}`,
			`{"name":"node","address":"node:8080","metadata":[]}`,
			`{"name":"node","address":"node:8080","enabled":"true"}`,
			`{"name":"node","address":"node:8080","unknown":true}`,
			`{"name":"node","address":"node:8080"} {}`,
		} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("input=%q status=%d want=400 body=%s", body, w.Code, w.Body.String())
			}
		}
	}
}

func TestInstanceInputNormalizationAndLargeBody(t *testing.T) {
	body := `{"name":" \t节点一\n ","address":"  node.internal:8080  "}`
	w := httptest.NewRecorder()
	got, ok := decodeInstanceInput(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	if !ok || got.Name != "节点一" || got.Address != "node.internal:8080" || got.Metadata != nil || got.Enabled != nil {
		t.Fatalf("ok=%t input=%+v response=%s", ok, got, w.Body.String())
	}
	for _, length := range []int{2 << 20, (2 << 20) + 1, 4 << 20} {
		for _, unknownLength := range []bool{false, true} {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat(" ", length-len(body))+body))
			if unknownLength {
				r.ContentLength = -1
			}
			w := httptest.NewRecorder()
			large, ok := decodeInstanceInput(w, r)
			if !ok || w.Code != http.StatusOK || !reflect.DeepEqual(large, got) {
				t.Fatalf("length=%d unknown=%t ok=%t input=%+v status=%d", length, unknownLength, ok, large, w.Code)
			}
		}
	}
}

func clusterInstanceTestDatabase(t *testing.T) *Service {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("CLUSTER_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("CLUSTER_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	id, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "cluster_instances_test_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "create schema "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "drop schema "+quoted+" cascade"); err != nil {
			t.Error(err)
		}
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(ctx, `create table users (id bigint primary key)`); err != nil {
		t.Fatal(err)
	}
	sql, err := migrations.ReadFile("migrations/012_clusters.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	return &Service{db: db}
}

func TestClusterInstanceDatabaseCRUD(t *testing.T) {
	s := clusterInstanceTestDatabase(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/clusters/{id}/instances", s.createInstance)
	mux.HandleFunc("PUT /admin/clusters/{id}/instances/{instanceId}", s.updateInstance)
	mux.HandleFunc("GET /admin/clusters/{id}/instances", s.listInstances)
	mux.HandleFunc("DELETE /admin/clusters/{id}/instances/{instanceId}", s.deleteInstance)
	mux.HandleFunc("GET /admin/instances", s.listAllInstances)
	const clusterA = "11111111-1111-4111-8111-111111111111"
	const clusterB = "22222222-2222-4222-8222-222222222222"
	const missing = "33333333-3333-4333-8333-333333333333"
	ctx := context.Background()
	if _, err := s.db.Exec(ctx, `insert into clusters(id,name) values($1,'cluster A'),($2,'cluster B')`, clusterA, clusterB); err != nil {
		t.Fatal(err)
	}
	path := func(clusterID string) string { return "/admin/clusters/" + clusterID + "/instances" }
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	create := func(clusterID, body string) string {
		t.Helper()
		w := request(http.MethodPost, path(clusterID), body, http.StatusCreated)
		var response struct{ ID string }
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.ID == "" {
			t.Fatalf("invalid create response %s: %v", w.Body.String(), err)
		}
		return response.ID
	}
	assertStored := func(id, name, address string, metadata map[string]any, enabled bool) {
		t.Helper()
		var gotName, gotAddress string
		var gotMetadata map[string]any
		var gotEnabled bool
		if err := s.db.QueryRow(ctx, `select name,address,metadata,enabled from cluster_instances where id=$1`, id).Scan(&gotName, &gotAddress, &gotMetadata, &gotEnabled); err != nil {
			t.Fatal(err)
		}
		if gotName != name || gotAddress != address || !reflect.DeepEqual(gotMetadata, metadata) || gotEnabled != enabled {
			t.Fatalf("stored name=%q address=%q metadata=%v enabled=%t", gotName, gotAddress, gotMetadata, gotEnabled)
		}
	}
	list := func(url string) []map[string]any {
		t.Helper()
		w := request(http.MethodGet, url, "", http.StatusOK)
		var response struct{ Data []map[string]any }
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Data == nil {
			t.Fatal("list must return an array")
		}
		return response.Data
	}

	request(http.MethodPost, path(missing), `{"name":"node","address":"node:8080"}`, http.StatusNotFound)
	idA := create(clusterA, `{"name":"  node A  ","address":"  node-a:8080  "}`)
	assertStored(idA, "node A", "node-a:8080", map[string]any{}, true)
	request(http.MethodPost, path(clusterA), `{"name":" node A ","address":"duplicate:8080"}`, http.StatusConflict)
	idB := create(clusterB, `{"name":"node A","address":"node-b:8080","metadata":null,"enabled":false}`)
	assertStored(idB, "node A", "node-b:8080", map[string]any{}, false)
	idC := create(clusterA, `{"name":"node C","address":"node-c:8080","metadata":{"zone":"east"},"enabled":false}`)
	request(http.MethodPut, path(clusterA)+"/"+idC, `{"name":" updated C ","address":" updated-c:8080 "}`, http.StatusOK)
	assertStored(idC, "updated C", "updated-c:8080", map[string]any{"zone": "east"}, false)
	request(http.MethodPut, path(clusterA)+"/"+idC, `{"name":"updated C","address":"updated-c:8080","metadata":null,"enabled":null}`, http.StatusOK)
	assertStored(idC, "updated C", "updated-c:8080", map[string]any{"zone": "east"}, false)
	request(http.MethodPut, path(clusterA)+"/"+idC, `{"name":"updated C","address":"updated-c:8080","metadata":{},"enabled":true}`, http.StatusOK)
	assertStored(idC, "updated C", "updated-c:8080", map[string]any{}, true)
	request(http.MethodPut, path(clusterA)+"/"+idC, `{"name":"updated C","address":"updated-c:8080","metadata":{"zone":"west"},"enabled":false}`, http.StatusOK)
	request(http.MethodPut, path(clusterA)+"/"+idC, `{"name":" node A ","address":"duplicate:8080"}`, http.StatusConflict)
	request(http.MethodPut, path(clusterB)+"/"+idC, `{"name":"wrong cluster","address":"wrong:8080"}`, http.StatusNotFound)
	request(http.MethodDelete, path(clusterB)+"/"+idC, "", http.StatusNotFound)
	request(http.MethodPut, path(clusterA)+"/"+missing, `{"name":"missing","address":"missing:8080"}`, http.StatusNotFound)
	request(http.MethodDelete, path(clusterA)+"/"+missing, "", http.StatusNotFound)
	assertStored(idC, "updated C", "updated-c:8080", map[string]any{"zone": "west"}, false)

	rowsA := list(path(clusterA))
	if len(rowsA) != 2 {
		t.Fatalf("cluster A list=%v", rowsA)
	}
	for _, row := range rowsA {
		if row["id"] != idA && row["id"] != idC {
			t.Fatalf("cluster A exposed another cluster's instance: %v", row)
		}
	}
	if rowsB := list(path(clusterB)); len(rowsB) != 1 || rowsB[0]["id"] != idB {
		t.Fatalf("cluster B list=%v", rowsB)
	}
	all := list("/admin/instances")
	if len(all) != 3 {
		t.Fatalf("all instances=%v", all)
	}
	for _, row := range all {
		if len(row) != 7 || row["endpoint"] != row["address"] {
			t.Fatalf("all-instance response shape changed: %v", row)
		}
		for _, key := range []string{"id", "cluster_id", "name", "address", "endpoint", "enabled", "created_at"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("all-instance response missing %s: %v", key, row)
			}
		}
	}
	request(http.MethodDelete, path(clusterA)+"/"+idC, "", http.StatusNoContent)
	request(http.MethodDelete, path(clusterA)+"/"+idC, "", http.StatusNotFound)
	request(http.MethodDelete, path(clusterA)+"/"+idA, "", http.StatusNoContent)
	if rows := list(path(clusterA)); len(rows) != 0 {
		t.Fatalf("deleted instances still listed: %v", rows)
	}
	assertStored(idB, "node A", "node-b:8080", map[string]any{}, false)
	if _, err := s.db.Exec(ctx, `drop table cluster_instances`); err != nil {
		t.Fatal(err)
	}
	request(http.MethodPost, path(clusterA), `{"name":"node","address":"node:8080"}`, http.StatusInternalServerError)
	request(http.MethodPut, path(clusterB)+"/"+idB, `{"name":"node","address":"node:8080"}`, http.StatusInternalServerError)
	request(http.MethodDelete, path(clusterB)+"/"+idB, "", http.StatusInternalServerError)
	request(http.MethodGet, path(clusterB), "", http.StatusInternalServerError)
}
