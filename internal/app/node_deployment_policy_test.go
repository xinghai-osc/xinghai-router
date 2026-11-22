package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const nodeTestFingerprint = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const nodeTestImage = "example.test/router@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestNodeTargetOperatorBindings(t *testing.T) {
	a := nodeDeploymentTarget{"node-a.test", 22, "deploy", nodeTestFingerprint, nodeTestImage}
	b := nodeDeploymentTarget{"node-b.test", 2022, "router", "SHA256:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBA", strings.Replace(nodeTestImage, "example.test/router", "example.test/other", 1)}
	data, err := json.Marshal([]nodeDeploymentTarget{a, b})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "postgres://local-test/router")
	t.Setenv("ENCRYPTION_KEY", "local-node-test-configuration-key")
	t.Setenv("NODE_DEPLOYMENT_TARGETS", string(data))
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.approvesNodeTarget(a) || !cfg.approvesNodeTarget(b) {
		t.Fatal("intact approved bindings were rejected")
	}
	for _, mutate := range []func(*nodeDeploymentTarget){
		func(v *nodeDeploymentTarget) { v.SSHHost = b.SSHHost }, func(v *nodeDeploymentTarget) { v.SSHPort = b.SSHPort }, func(v *nodeDeploymentTarget) { v.SSHUser = b.SSHUser }, func(v *nodeDeploymentTarget) { v.SSHHostKey = b.SSHHostKey }, func(v *nodeDeploymentTarget) { v.RouterImage = b.RouterImage },
	} {
		mixed := a
		mutate(&mixed)
		if cfg.approvesNodeTarget(mixed) {
			t.Fatalf("mixed binding accepted: %+v", mixed)
		}
	}
	for _, raw := range []string{"", "[]"} {
		t.Setenv("NODE_DEPLOYMENT_TARGETS", raw)
		cfg, err := LoadConfig()
		if err != nil || cfg.approvesNodeTarget(a) {
			t.Fatalf("empty policy: %v", err)
		}
	}
	for _, raw := range []string{"null", "[{}]", string(data) + " []", strings.Replace(string(data), "ssh_user", "username", 1), strings.Replace(string(data), a.SSHHost, "*.test", 1), strings.Replace(string(data), nodeTestImage, "example.test/router:latest", 1), strings.Replace(string(data), nodeTestFingerprint, "SHA256:short", 1), strings.Replace(string(data), `"ssh_port":22`, `"ssh_port":0`, 1)} {
		t.Setenv("NODE_DEPLOYMENT_TARGETS", raw)
		if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "NODE_DEPLOYMENT_TARGETS") {
			t.Fatalf("invalid config accepted or misclassified: %v", err)
		}
	}
}

func TestNodeDeploymentDeniedBeforeSSH(t *testing.T) {
	s := &Service{}
	record := nodeRecord{SSHHost: "node-a.test", SSHPort: 22, SSHUser: "deploy", SSHAuthMethod: "private_key", SSHHostKey: nodeTestFingerprint, RouterImage: nodeTestImage}
	if err := s.deployNodeOverSSH(context.Background(), record, "invalid-test-key", ""); !errors.Is(err, errNodeTargetDenied) {
		t.Fatalf("unapproved destination reached SSH setup: %v", err)
	}
	w := httptest.NewRecorder()
	s.createNode(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"node-a","ssh_host":"node-a.test","ssh_user":"deploy","ssh_password":"fake","ssh_host_key":"`+nodeTestFingerprint+`","router_image":"`+nodeTestImage+`"}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("default create/deploy status: %d", w.Code)
	}
}

func TestNodeDeploymentDatabaseBinding(t *testing.T) {
	s := clusterInstanceTestDatabase(t)
	ctx := context.Background()
	s.cfg.EncryptionKey = "isolated-node-test-configuration-key"
	migration, err := migrations.ReadFile("migrations/105_node_deployment.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	clusterID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `insert into clusters(id,name) values($1,'test')`, clusterID); err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, method, id, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		handler(w, r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{})))
		if w.Code != want {
			t.Fatalf("status=%d want=%d body=%s", w.Code, want, w.Body.String())
		}
		return w
	}
	w := call(s.createNode, "POST", clusterID, `{"name":"node-a","deploy":false,"address":"node-a.test:8080","ssh_host":"node-a.test","ssh_user":"deploy","ssh_password":"old-test-password","ssh_host_key":"`+nodeTestFingerprint+`","router_image":"`+nodeTestImage+`"}`, http.StatusAccepted)
	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.ID == "" {
		t.Fatalf("create response: %s", w.Body.String())
	}
	id := response.ID
	var address, metadata string
	if err := s.db.QueryRow(ctx, `select address,metadata::text from cluster_instances where id=$1`, id).Scan(&address, &metadata); err != nil || address != "node-a.test:8080" || metadata != "{}" {
		t.Fatalf("address=%q metadata=%q err=%v", address, metadata, err)
	}
	call(s.deployNode, "POST", id, "", http.StatusForbidden)
	call(s.updateNode, "PUT", id, `{"deploy":true,"name":"not-saved"}`, http.StatusForbidden)
	for _, body := range []string{`{"ssh_host":"node-b.test"}`, `{"ssh_port":2022}`, `{"ssh_user":"other"}`, `{"ssh_host_key":"SHA256:new"}`, `{"ssh_auth_method":"private_key"}`, `{"ssh_host":"node-b.test","ssh_password":"  "}`, `{"ssh_host":"node-b.test","ssh_private_key":"wrong-type"}`} {
		call(s.updateNode, "PUT", id, body, http.StatusBadRequest)
	}
	call(s.updateNode, "PUT", id, `{"ssh_host":"node-b.test","ssh_password":"new-test-password"}`, http.StatusOK)
	var host, encrypted, name string
	if err := s.db.QueryRow(ctx, `select ssh_host,ssh_secret,name from cluster_instances where id=$1`, id).Scan(&host, &encrypted, &name); err != nil {
		t.Fatal(err)
	}
	if plain, err := crypt(s.cfg.EncryptionKey, encrypted, true); err != nil || plain != "new-test-password" || host != "node-b.test" || name != "node-a" {
		t.Fatal("updated node does not match fresh credentials/approved fields")
	}
	binding := nodeDeploymentTarget{host, 22, "deploy", nodeTestFingerprint, nodeTestImage}
	s.cfg.nodeDeploymentTargets = map[nodeDeploymentTarget]struct{}{binding: {}}
	if err := s.queueNodeDeployment(ctx, id); err != nil {
		t.Fatal(err)
	}
	call(s.updateNode, "PUT", id, `{"ssh_host":"node-c.test","ssh_password":"fresh"}`, http.StatusConflict)
	if _, err := s.db.Exec(ctx, `update cluster_instances set router_image='example.test/router:latest' where id=$1`, id); err != nil {
		t.Fatal(err)
	}
	s.processNodeDeployment(ctx, id)
	var status, message string
	if err := s.db.QueryRow(ctx, `select deploy_status,deploy_message from cluster_instances where id=$1`, id).Scan(&status, &message); err != nil || status != "failed" || message != errNodeTargetDenied.Error() {
		t.Fatalf("worker snapshot was not rechecked: %s %s %v", status, message, err)
	}
}
