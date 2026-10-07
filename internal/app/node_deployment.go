package app

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/ssh"
)

const (
	defaultNodeRouterImage = "ghcr.io/xinghai-osc/xinghai-router:latest"
	defaultNodeListenPort  = 8080
	nodeDeploymentTimeout  = 10 * time.Minute
)

type nodeInput struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	SSHHost       string `json:"ssh_host"`
	SSHPort       int    `json:"ssh_port"`
	SSHUser       string `json:"ssh_user"`
	SSHAuthMethod string `json:"ssh_auth_method"`
	SSHPassword   string `json:"ssh_password"`
	SSHPrivateKey string `json:"ssh_private_key"`
	SSHPassphrase string `json:"ssh_passphrase"`
	SSHHostKey    string `json:"ssh_host_key"`
	DeployPath    string `json:"deploy_path"`
	RouterImage   string `json:"router_image"`
	Deploy        *bool  `json:"deploy"`
}

type nodeRecord struct {
	ID             string
	ClusterID      string
	ClusterName    string
	Name           string
	Address        string
	Enabled        bool
	SSHHost        string
	SSHPort        int
	SSHUser        string
	SSHAuthMethod  string
	SSHSecret      string
	SSHPassphrase  string
	SSHHostKey     string
	DeployPath     string
	RouterImage    string
	DeployStatus   string
	DeployMessage  string
	DeployStarted  any
	DeployFinished any
	CreatedAt      any
}

func normalizeNodeInput(in *nodeInput, requireSecret bool) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.TrimSpace(in.Address)
	in.SSHHost = strings.TrimSpace(in.SSHHost)
	in.SSHUser = strings.TrimSpace(in.SSHUser)
	in.SSHAuthMethod = strings.ToLower(strings.TrimSpace(in.SSHAuthMethod))
	in.SSHPassword = strings.TrimSpace(in.SSHPassword)
	in.SSHPrivateKey = strings.TrimSpace(in.SSHPrivateKey)
	in.SSHPassphrase = strings.TrimSpace(in.SSHPassphrase)
	in.SSHHostKey = strings.TrimSpace(in.SSHHostKey)
	in.DeployPath = strings.TrimSpace(in.DeployPath)
	in.RouterImage = strings.TrimSpace(in.RouterImage)
	if in.SSHPort == 0 {
		in.SSHPort = 22
	}
	if in.SSHAuthMethod == "" {
		in.SSHAuthMethod = "password"
	}
	if in.RouterImage == "" {
		in.RouterImage = defaultNodeRouterImage
	}
	if in.Name == "" || len(in.Name) > 100 {
		return errors.New("name must be between 1 and 100 characters")
	}
	if in.SSHHost == "" || len(in.SSHHost) > 255 || strings.ContainsAny(in.SSHHost, "\t\r\n /\\") || (strings.Contains(in.SSHHost, ":") && net.ParseIP(in.SSHHost) == nil) {
		return errors.New("invalid SSH host")
	}
	if in.SSHPort < 1 || in.SSHPort > 65535 {
		return errors.New("SSH port must be between 1 and 65535")
	}
	if in.SSHUser == "" || len(in.SSHUser) > 100 || strings.ContainsAny(in.SSHUser, "\x00\t\r\n /\\:") {
		return errors.New("invalid SSH user")
	}
	if in.SSHAuthMethod != "password" && in.SSHAuthMethod != "private_key" {
		return errors.New("SSH authentication must be password or private_key")
	}
	if requireSecret || in.SSHPassword != "" || in.SSHPrivateKey != "" {
		if in.SSHAuthMethod == "password" {
			if in.SSHPassword == "" || len(in.SSHPassword) > 4096 {
				return errors.New("SSH password is required")
			}
			in.SSHPrivateKey = ""
		} else {
			if !strings.HasPrefix(in.SSHPrivateKey, "-----BEGIN ") || len(in.SSHPrivateKey) > 128*1024 {
				return errors.New("SSH private key is required")
			}
			in.SSHPassword = ""
		}
	}
	if !strings.HasPrefix(in.SSHHostKey, "SHA256:") || len(in.SSHHostKey) > 128 {
		return errors.New("SSH host key fingerprint must use SHA256 format")
	}
	if in.SSHPassphrase != "" && len(in.SSHPassphrase) > 4096 {
		return errors.New("SSH passphrase is too long")
	}
	if in.DeployPath == "" {
		in.DeployPath = "/home/" + in.SSHUser + "/xinghai-router"
	}
	if !strings.HasPrefix(in.DeployPath, "/") || len(in.DeployPath) > 500 || strings.ContainsAny(in.DeployPath, "\x00\t\r\n") || strings.Contains(in.DeployPath, "..") {
		return errors.New("deployment path must be an absolute safe path")
	}
	if len(in.RouterImage) > 500 || !safeNodeImage(in.RouterImage) {
		return errors.New("invalid router image")
	}
	if in.Address == "" {
		in.Address = net.JoinHostPort(in.SSHHost, strconv.Itoa(defaultNodeListenPort))
	}
	if len(in.Address) > 500 || strings.ContainsAny(in.Address, "\x00\t\r\n") {
		return errors.New("invalid node address")
	}
	return nil
}

func safeNodeImage(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:/@-", r) {
			continue
		}
		return false
	}
	return value != ""
}

func (s *Service) encryptNodeCredentials(in nodeInput) (secret, passphrase string, err error) {
	plain := in.SSHPassword
	if in.SSHAuthMethod == "private_key" {
		plain = in.SSHPrivateKey
	}
	secret, err = crypt(s.cfg.EncryptionKey, plain, false)
	if err != nil {
		return "", "", fmt.Errorf("encrypt SSH credential: %w", err)
	}
	if in.SSHPassphrase != "" {
		passphrase, err = crypt(s.cfg.EncryptionKey, in.SSHPassphrase, false)
		if err != nil {
			return "", "", fmt.Errorf("encrypt SSH passphrase: %w", err)
		}
	}
	return secret, passphrase, nil
}

func (s *Service) listNodes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select i.id,i.cluster_id,c.name,i.name,i.address,i.enabled,i.ssh_host,i.ssh_port,i.ssh_user,i.ssh_auth_method,i.ssh_host_key,i.deploy_path,i.router_image,i.deploy_status,i.deploy_message,i.deploy_started_at,i.deploy_finished_at,i.created_at from cluster_instances i join clusters c on c.id=i.cluster_id where i.ssh_host<>'' order by i.created_at desc`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var n nodeRecord
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ClusterName, &n.Name, &n.Address, &n.Enabled, &n.SSHHost, &n.SSHPort, &n.SSHUser, &n.SSHAuthMethod, &n.SSHHostKey, &n.DeployPath, &n.RouterImage, &n.DeployStatus, &n.DeployMessage, &n.DeployStarted, &n.DeployFinished, &n.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
			return
		}
		data = append(data, nodeResponse(n))
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func nodeResponse(n nodeRecord) map[string]any {
	return map[string]any{
		"id": n.ID, "cluster_id": n.ClusterID, "cluster_name": n.ClusterName, "name": n.Name,
		"address": n.Address, "endpoint": n.Address, "enabled": n.Enabled,
		"ssh_host": n.SSHHost, "ssh_port": n.SSHPort, "ssh_user": n.SSHUser,
		"ssh_auth_method": n.SSHAuthMethod, "ssh_host_key": n.SSHHostKey, "has_ssh_host_key": n.SSHHostKey != "",
		"deploy_path": n.DeployPath, "router_image": n.RouterImage,
		"deploy_status": n.DeployStatus, "deploy_message": n.DeployMessage,
		"deploy_started_at": n.DeployStarted, "deploy_finished_at": n.DeployFinished, "created_at": n.CreatedAt,
	}
}

func (s *Service) createNode(w http.ResponseWriter, r *http.Request) {
	var in nodeInput
	if decode(r, &in) != nil || normalizeNodeInput(&in, true) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid node or SSH configuration")
		return
	}
	secret, passphrase, err := s.encryptNodeCredentials(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not protect SSH credentials")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create node")
		return
	}
	deploy := in.Deploy == nil || *in.Deploy
	status := "idle"
	if deploy {
		status = "pending"
	}
	clusterID := r.PathValue("id")
	tag, err := s.db.Exec(r.Context(), `insert into cluster_instances(id,cluster_id,name,address,metadata,enabled,ssh_host,ssh_port,ssh_user,ssh_auth_method,ssh_secret,ssh_passphrase,ssh_host_key,deploy_path,router_image,deploy_status,deploy_message) select $1,id,$3,'{}',$4,true,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15 from clusters where id=$2`, id, clusterID, in.Name, in.Address, in.SSHHost, in.SSHPort, in.SSHUser, in.SSHAuthMethod, secret, passphrase, in.SSHHostKey, in.DeployPath, in.RouterImage, status, statusMessage(status))
	if err != nil {
		writeInstanceMutationError(w, err, "could not create node")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "cluster not found")
		return
	}
	if deploy {
		go s.processNodeDeployment(context.Background(), id)
	}
	s.audit(r, "node.created", "cluster_instance", id, map[string]any{"cluster_id": clusterID, "name": in.Name, "ssh_host": in.SSHHost, "ssh_user": in.SSHUser})
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": status})
}

func statusMessage(status string) string {
	if status == "pending" {
		return "deployment queued"
	}
	return ""
}

func (s *Service) updateNode(w http.ResponseWriter, r *http.Request) {
	var in nodeInput
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid node or SSH configuration")
		return
	}
	id := r.PathValue("id")
	var old nodeRecord
	if err := s.db.QueryRow(r.Context(), `select name,address,ssh_host,ssh_port,ssh_user,ssh_auth_method,ssh_secret,ssh_passphrase,ssh_host_key,deploy_path,router_image from cluster_instances where id=$1`, id).Scan(&old.Name, &old.Address, &old.SSHHost, &old.SSHPort, &old.SSHUser, &old.SSHAuthMethod, &old.SSHSecret, &old.SSHPassphrase, &old.SSHHostKey, &old.DeployPath, &old.RouterImage); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if in.Name == "" {
		in.Name = old.Name
	}
	if in.Address == "" {
		in.Address = old.Address
	}
	if in.SSHHost == "" {
		in.SSHHost = old.SSHHost
	}
	if in.SSHPort == 0 {
		in.SSHPort = old.SSHPort
	}
	if in.SSHUser == "" {
		in.SSHUser = old.SSHUser
	}
	if in.SSHAuthMethod == "" {
		in.SSHAuthMethod = old.SSHAuthMethod
	}
	if in.SSHHostKey == "" {
		in.SSHHostKey = old.SSHHostKey
	}
	if in.DeployPath == "" {
		in.DeployPath = old.DeployPath
	}
	if in.RouterImage == "" {
		in.RouterImage = old.RouterImage
	}
	newCredential := in.SSHPassword != "" || in.SSHPrivateKey != ""
	if in.SSHAuthMethod != old.SSHAuthMethod && !newCredential {
		writeError(w, http.StatusBadRequest, "invalid_request", "new SSH credentials are required")
		return
	}
	if err := normalizeNodeInput(&in, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid node or SSH configuration")
		return
	}
	secret, passphrase := old.SSHSecret, old.SSHPassphrase
	if newCredential {
		var err error
		secret, passphrase, err = s.encryptNodeCredentials(in)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not protect SSH credentials")
			return
		}
	} else if in.SSHPassphrase != "" {
		var err error
		passphrase, err = crypt(s.cfg.EncryptionKey, in.SSHPassphrase, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not protect SSH passphrase")
			return
		}
	}
	_, err := s.db.Exec(r.Context(), `update cluster_instances set name=$2,address=$3,ssh_host=$4,ssh_port=$5,ssh_user=$6,ssh_auth_method=$7,ssh_secret=$8,ssh_passphrase=$9,ssh_host_key=$10,deploy_path=$11,router_image=$12 where id=$1`, id, in.Name, in.Address, in.SSHHost, in.SSHPort, in.SSHUser, in.SSHAuthMethod, secret, passphrase, in.SSHHostKey, in.DeployPath, in.RouterImage)
	if err != nil {
		writeInstanceMutationError(w, err, "could not update node")
		return
	}
	if in.Deploy != nil && *in.Deploy {
		if err := s.queueNodeDeployment(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not queue deployment")
			return
		}
		go s.processNodeDeployment(context.Background(), id)
	}
	s.audit(r, "node.updated", "cluster_instance", id, map[string]any{"name": in.Name, "ssh_host": in.SSHHost, "ssh_user": in.SSHUser})
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Service) deleteNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag, err := s.db.Exec(r.Context(), `delete from cluster_instances where id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete node")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	s.audit(r, "node.deleted", "cluster_instance", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) deployNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.queueNodeDeployment(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "node not found or SSH is not configured")
			return
		}
		if errors.Is(err, errNodeDeploymentRunning) {
			writeError(w, http.StatusConflict, "deployment_running", "deployment is already running")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not queue deployment")
		return
	}
	s.audit(r, "node.deployment_queued", "cluster_instance", id, nil)
	go s.processNodeDeployment(context.Background(), id)
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "pending"})
}

var errNodeDeploymentRunning = errors.New("node deployment already running")

func (s *Service) queueNodeDeployment(ctx context.Context, id string) error {
	var status string
	err := s.db.QueryRow(ctx, `update cluster_instances set deploy_status='pending',deploy_message='deployment queued',deploy_started_at=null,deploy_finished_at=null where id=$1 and ssh_host<>'' and ssh_secret<>'' and deploy_status<>'running' returning deploy_status`, id).Scan(&status)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var current string
	if err := s.db.QueryRow(ctx, `select deploy_status from cluster_instances where id=$1 and ssh_host<>'' and ssh_secret<>''`, id).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return pgx.ErrNoRows
	} else if err != nil {
		return err
	} else if current == "running" {
		return errNodeDeploymentRunning
	}
	return pgx.ErrNoRows
}

func (s *Service) processNodeDeployment(parent context.Context, id string) {
	ctx, cancel := context.WithTimeout(parent, nodeDeploymentTimeout)
	defer cancel()
	var n nodeRecord
	if err := s.db.QueryRow(ctx, `select id,cluster_id,name,address,ssh_host,ssh_port,ssh_user,ssh_auth_method,ssh_secret,ssh_passphrase,ssh_host_key,deploy_path,router_image from cluster_instances where id=$1`, id).Scan(&n.ID, &n.ClusterID, &n.Name, &n.Address, &n.SSHHost, &n.SSHPort, &n.SSHUser, &n.SSHAuthMethod, &n.SSHSecret, &n.SSHPassphrase, &n.SSHHostKey, &n.DeployPath, &n.RouterImage); err != nil {
		return
	}
	var startedAt time.Time
	if err := s.db.QueryRow(ctx, `update cluster_instances set deploy_status='running',deploy_message='deployment started',deploy_started_at=now(),deploy_finished_at=null where id=$1 and deploy_status='pending' returning deploy_started_at`, id).Scan(&startedAt); err != nil {
		return
	}
	secret, err := crypt(s.cfg.EncryptionKey, n.SSHSecret, true)
	if err != nil {
		s.finishNodeDeployment(ctx, id, "failed", "could not decrypt SSH credentials")
		return
	}
	passphrase := ""
	if n.SSHPassphrase != "" {
		passphrase, err = crypt(s.cfg.EncryptionKey, n.SSHPassphrase, true)
		if err != nil {
			s.finishNodeDeployment(ctx, id, "failed", "could not decrypt SSH passphrase")
			return
		}
	}
	if err = s.deployNodeOverSSH(ctx, n, secret, passphrase); err != nil {
		s.finishNodeDeployment(ctx, id, "failed", nodeDeploymentError(err, secret, passphrase, s.cfg.DatabaseURL, s.cfg.RedisURL, s.cfg.EncryptionKey))
		return
	}
	s.finishNodeDeployment(ctx, id, "success", "deployment completed")
}

func (s *Service) finishNodeDeployment(_ context.Context, id, status, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.db.Exec(ctx, `update cluster_instances set deploy_status=$2,deploy_message=$3,deploy_finished_at=now(),last_seen_at=case when $2='success' then now() else last_seen_at end where id=$1`, id, status, message)
}

func nodeDeploymentError(err error, secrets ...string) string {
	message := strings.TrimSpace(err.Error())
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	if message == "" {
		return "deployment failed"
	}
	if len(message) > 800 {
		message = message[:800]
	}
	return message
}

func (s *Service) deployNodeOverSSH(ctx context.Context, n nodeRecord, credential, passphrase string) error {
	var auth ssh.AuthMethod
	if n.SSHAuthMethod == "password" {
		auth = ssh.Password(credential)
	} else {
		var signer ssh.Signer
		var err error
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(credential), []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(credential))
		}
		if err != nil {
			return fmt.Errorf("could not parse SSH private key: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	}
	config := &ssh.ClientConfig{User: n.SSHUser, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: nodeHostKeyCallback(n.SSHHostKey), Timeout: 20 * time.Second}
	address := net.JoinHostPort(n.SSHHost, strconv.Itoa(n.SSHPort))
	conn, err := (&net.Dialer{Timeout: 20 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	clientConn, channels, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SSH handshake failed: %w", err)
	}
	client := ssh.NewClient(clientConn, channels, requests)
	defer client.Close()
	port := nodeListenPort(n.Address)
	compose, err := s.nodeComposeFile(n, port)
	if err != nil {
		return err
	}
	command := "set -eu; mkdir -p " + shellQuote(n.DeployPath) + "; chmod 700 " + shellQuote(n.DeployPath) + "; cat > " + shellQuote(n.DeployPath+"/docker-compose.yml") + "; chmod 600 " + shellQuote(n.DeployPath+"/docker-compose.yml") + "; if docker compose version >/dev/null 2>&1; then docker compose -f " + shellQuote(n.DeployPath+"/docker-compose.yml") + " up -d; elif command -v docker-compose >/dev/null 2>&1; then docker-compose -f " + shellQuote(n.DeployPath+"/docker-compose.yml") + " up -d; else echo 'docker compose is required' >&2; exit 1; fi"
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("could not create SSH session: %w", err)
	}
	defer session.Close()
	var stdout, stderr limitedNodeBuffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	session.Stdin = strings.NewReader(compose)
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case err := <-done:
		if err != nil {
			output := strings.TrimSpace(stderr.String())
			if output == "" {
				output = strings.TrimSpace(stdout.String())
			}
			if output != "" {
				return fmt.Errorf("remote deployment failed: %s", output)
			}
			return fmt.Errorf("remote deployment failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		_ = client.Close()
		return ctx.Err()
	}
}

type limitedNodeBuffer struct{ bytes.Buffer }

func (b *limitedNodeBuffer) Write(p []byte) (int, error) {
	written := len(p)
	if b.Len() < 4096 {
		remaining := 4096 - b.Len()
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return written, nil
}

func (s *Service) nodeComposeFile(n nodeRecord, port int) (string, error) {
	values := []string{s.cfg.DatabaseURL, s.cfg.RedisURL, s.cfg.EncryptionKey, n.RouterImage}
	for _, value := range values {
		if strings.ContainsAny(value, "\x00\r\n") {
			return "", errors.New("deployment configuration contains unsupported characters")
		}
	}
	return fmt.Sprintf(`services:
  router:
    image: %s
    restart: unless-stopped
    environment:
      DATABASE_URL: %s
      REDIS_URL: %s
      DEPLOYMENT_MODE: 'cluster'
      REDIS_FAILURE_POLICY: 'deny'
      LOCAL_PROMPT_CACHE: 'false'
      ENCRYPTION_KEY: %s
      LISTEN_ADDR: ':8080'
    ports:
      - '%d:8080'
`, composeScalar(n.RouterImage), composeScalar(s.cfg.DatabaseURL), composeScalar(s.cfg.RedisURL), composeScalar(s.cfg.EncryptionKey), port), nil
}

func composeScalar(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, "$", "$$"), "'", "''") + "'"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func nodeHostKeyCallback(expected string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		actual := ssh.FingerprintSHA256(key)
		if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
			return fmt.Errorf("SSH host key fingerprint mismatch")
		}
		return nil
	}
}

func nodeListenPort(address string) int {
	if host, port, err := net.SplitHostPort(address); err == nil && host != "" {
		if value, err := strconv.Atoi(port); err == nil && value >= 1 && value <= 65535 {
			return value
		}
	}
	return defaultNodeListenPort
}
