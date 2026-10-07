package app

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNormalizeNodeInput(t *testing.T) {
	input := nodeInput{
		Name:          "  node-a ",
		SSHHost:       "10.0.0.12",
		SSHUser:       "deploy",
		SSHAuthMethod: "password",
		SSHPassword:   "fake-password",
		SSHHostKey:    "SHA256:fake-fingerprint",
	}
	if err := normalizeNodeInput(&input, true); err != nil {
		t.Fatal(err)
	}
	if input.Name != "node-a" || input.SSHPort != 22 || input.Address != "10.0.0.12:8080" || input.DeployPath != "/home/deploy/xinghai-router" || input.RouterImage != defaultNodeRouterImage {
		t.Fatalf("unexpected normalized input: %+v", input)
	}
	for name, invalid := range map[string]nodeInput{
		"missing fingerprint": inputWithNodeChanges(input, func(v *nodeInput) { v.SSHHostKey = "" }),
		"unsafe image":        inputWithNodeChanges(input, func(v *nodeInput) { v.RouterImage = "router image" }),
		"relative path":       inputWithNodeChanges(input, func(v *nodeInput) { v.DeployPath = "relative" }),
		"invalid host":        inputWithNodeChanges(input, func(v *nodeInput) { v.SSHHost = "host:with:colon" }),
	} {
		if err := normalizeNodeInput(&invalid, true); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func inputWithNodeChanges(input nodeInput, change func(*nodeInput)) nodeInput {
	copy := input
	change(&copy)
	return copy
}

func TestNodeComposeFileEscapesValues(t *testing.T) {
	s := &Service{cfg: Config{
		DatabaseURL:   "postgres://router:pa$$word@db:5432/router?sslmode=disable",
		RedisURL:      "redis://:redis'pass@redis:6379/0",
		EncryptionKey: "fake-encryption-key-with-enough-length",
	}}
	compose, err := s.nodeComposeFile(nodeRecord{RouterImage: defaultNodeRouterImage}, 18080)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ports:", "'18080:8080'", "pa$$$$word", "redis''pass"} {
		if !strings.Contains(compose, want) {
			t.Errorf("compose file missing escaped value %q: %s", want, compose)
		}
	}
}

func TestNodeHostKeyCallback(t *testing.T) {
	_, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	public, err := ssh.NewPublicKey(private.Public())
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := ssh.FingerprintSHA256(public)
	if err := nodeHostKeyCallback(fingerprint)("node:22", nil, public); err != nil {
		t.Fatalf("matching fingerprint rejected: %v", err)
	}
	if err := nodeHostKeyCallback("SHA256:wrong")("node:22", nil, public); err == nil {
		t.Fatal("mismatching fingerprint accepted")
	}
}

func TestNodeDeploymentErrorRedactsSecrets(t *testing.T) {
	message := nodeDeploymentError(errors.New("remote failed using fake-password and postgres://router:fake@db"), "fake-password", "postgres://router:fake@db")
	if strings.Contains(message, "fake-password") || strings.Contains(message, "postgres://router:fake@db") || !strings.Contains(message, "[redacted]") {
		t.Fatalf("secrets were not redacted: %q", message)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/home/deploy/xinghai-router"); got != "'/home/deploy/xinghai-router'" {
		t.Fatalf("unexpected shell quote: %q", got)
	}
	if got := shellQuote("a'b"); got != "'a'\\''b'" {
		t.Fatalf("unexpected escaped shell quote: %q", got)
	}
}
