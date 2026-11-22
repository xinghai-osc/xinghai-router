package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
)

type nodeDeploymentTarget struct {
	SSHHost     string `json:"ssh_host"`
	SSHPort     int    `json:"ssh_port"`
	SSHUser     string `json:"ssh_user"`
	SSHHostKey  string `json:"ssh_host_key"`
	RouterImage string `json:"router_image"`
}

var nodeImageDigest = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]*@sha256:[0-9a-f]{64}$`)
var nodeHostnameLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
var errNodeTargetDenied = errors.New("node deployment target is not approved by the operator")

func parseNodeDeploymentTargets(raw string) (map[nodeDeploymentTarget]struct{}, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var entries []nodeDeploymentTarget
	if err := decoder.Decode(&entries); err != nil || entries == nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("expected a JSON array of node deployment targets")
	}
	targets := make(map[nodeDeploymentTarget]struct{}, len(entries))
	for i, target := range entries {
		if !target.valid() {
			return nil, fmt.Errorf("entry %d requires an exact SSH host, port, user, SHA256 host fingerprint and image pinned by sha256 digest", i+1)
		}
		targets[target] = struct{}{}
	}
	return targets, nil
}

func (t nodeDeploymentTarget) valid() bool {
	if len(t.SSHHost) > 253 || t.SSHPort < 1 || t.SSHPort > 65535 || t.SSHUser == "" || len(t.SSHUser) > 100 || strings.ContainsAny(t.SSHUser, "\x00\t\r\n /\\:*") || len(t.RouterImage) > 500 || !nodeImageDigest.MatchString(t.RouterImage) {
		return false
	}
	if net.ParseIP(t.SSHHost) == nil {
		for _, label := range strings.Split(t.SSHHost, ".") {
			if !nodeHostnameLabel.MatchString(label) {
				return false
			}
		}
	}
	raw, ok := strings.CutPrefix(t.SSHHostKey, "SHA256:")
	digest, err := base64.RawStdEncoding.Strict().DecodeString(raw)
	return ok && err == nil && len(digest) == sha256.Size && base64.RawStdEncoding.EncodeToString(digest) == raw
}

func (in nodeInput) deploymentTarget() nodeDeploymentTarget {
	return nodeDeploymentTarget{in.SSHHost, in.SSHPort, in.SSHUser, in.SSHHostKey, in.RouterImage}
}

func (n nodeRecord) deploymentTarget() nodeDeploymentTarget {
	return nodeDeploymentTarget{n.SSHHost, n.SSHPort, n.SSHUser, n.SSHHostKey, n.RouterImage}
}

func (c Config) approvesNodeTarget(target nodeDeploymentTarget) bool {
	_, ok := c.nodeDeploymentTargets[target]
	return ok
}

func (n nodeRecord) needsNewSSHCredential(in nodeInput) bool {
	return n.SSHHost != in.SSHHost || n.SSHPort != in.SSHPort || n.SSHUser != in.SSHUser || n.SSHHostKey != in.SSHHostKey || n.SSHAuthMethod != in.SSHAuthMethod
}
