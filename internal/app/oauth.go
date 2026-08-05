package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type oauthProviderConfig struct {
	ClientID     string
	ClientSecret string
	Enabled      bool
}

func (s *Service) loadOAuthProvider(ctx context.Context, provider string) (*oauthProviderConfig, error) {
	var id, secretEnc string
	var enabled bool
	err := s.db.QueryRow(ctx, `select client_id,client_secret_encrypted,enabled from oauth_providers where id=$1`, provider).Scan(&id, &secretEnc, &enabled)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, fmt.Errorf("provider disabled")
	}
	secret, err := crypt(s.cfg.EncryptionKey, secretEnc, true)
	if err != nil {
		return nil, err
	}
	return &oauthProviderConfig{ClientID: id, ClientSecret: secret, Enabled: true}, nil
}

func oauthStateSignature(encryptionKey, data string) string {
	mac := hmac.New(sha256.New, []byte(encryptionKey))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func callbackURI(r *http.Request, provider string) string {
	scheme := "https"
	if r.TLS == nil && (strings.HasPrefix(r.Host, "localhost:") || r.Host == "localhost" || strings.HasPrefix(r.Host, "127.0.0.1")) {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/api/auth/oauth/%s/callback", scheme, r.Host, provider)
}

func (s *Service) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	cfg, err := s.loadOAuthProvider(r.Context(), provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_provider", "OAuth provider is not configured or disabled")
		return
	}
	stateNonce := fmt.Sprintf("%s:%d", provider, time.Now().Unix())
	sig := oauthStateSignature(s.cfg.EncryptionKey, stateNonce)
	cb := callbackURI(r, provider)
	var authURL string
	switch provider {
	case "github":
		authURL = fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s.%s&scope=read:user,user:email",
			url.QueryEscape(cfg.ClientID), url.QueryEscape(cb), url.QueryEscape(stateNonce), url.QueryEscape(sig))
	default:
		writeError(w, http.StatusBadRequest, "unsupported_provider", "unsupported OAuth provider")
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Service) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateRaw := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || stateRaw == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "missing code or state")
		return
	}
	dot := strings.LastIndexByte(stateRaw, '.')
	if dot < 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid state")
		return
	}
	stateNonce := stateRaw[:dot]
	givenSig := stateRaw[dot+1:]
	expectedSig := oauthStateSignature(s.cfg.EncryptionKey, stateNonce)
	if !hmac.Equal([]byte(givenSig), []byte(expectedSig)) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid state signature")
		return
	}
	parts := strings.SplitN(stateNonce, ":", 2)
	if len(parts) < 1 || parts[0] != provider {
		writeError(w, http.StatusBadRequest, "invalid_request", "state provider mismatch")
		return
	}
	cfg, err := s.loadOAuthProvider(r.Context(), provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_provider", "OAuth provider is not configured")
		return
	}
	cb := callbackURI(r, provider)
	var accessToken, userEmail, userName, userAvatar, providerUserID string
	switch provider {
	case "github":
		accessToken, err = s.githubExchangeToken(r.Context(), cfg, code, cb)
		if err != nil {
			writeError(w, http.StatusBadGateway, "oauth_error", fmt.Sprintf("token exchange failed: %v", err))
			return
		}
		providerUserID, userEmail, userName, userAvatar, err = s.githubFetchUser(r.Context(), accessToken)
		if err != nil {
			writeError(w, http.StatusBadGateway, "oauth_error", fmt.Sprintf("failed to fetch user info: %v", err))
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported_provider", "unsupported OAuth provider")
		return
	}
	userID, err := s.findOrCreateOAuthUser(r.Context(), provider, providerUserID, userEmail, userName, userAvatar)
	if err != nil {
		log.Printf("oauth findOrCreateOAuthUser: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not process OAuth login")
		return
	}
	s.auditActor(r, userID, "account.oauth_login", "user", userID, map[string]any{"provider": provider})
	var email string
	if err := s.db.QueryRow(r.Context(), `select email from users where id=$1`, userID).Scan(&email); err == nil {
		s.notifyLogin(r.Context(), email, requestMetadata(r))
	}
	s.createSession(w, r, userID, http.StatusOK)
}

func (s *Service) findOrCreateOAuthUser(ctx context.Context, provider, providerUserID, email, name, avatar string) (string, error) {
	var userID string
	err := s.db.QueryRow(ctx, `select user_id from user_oauth_connections where provider=$1 and provider_user_id=$2`, provider, providerUserID).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if email != "" {
		err = s.db.QueryRow(ctx, `select id from users where email=$1`, email).Scan(&userID)
		if err == nil {
			_, _ = s.db.Exec(ctx, `insert into user_oauth_connections(user_id,provider,provider_user_id,provider_username,provider_avatar_url) values($1,$2,$3,$4,$5) on conflict do nothing`, userID, provider, providerUserID, name, avatar)
			return userID, nil
		}
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = fmt.Sprintf("%s-%s@oauth.local", provider, providerUserID)
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = email
	}
	role, err := registrationRole(ctx, s.db)
	if err != nil {
		return "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(458110)`); err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `insert into users(email,name,role,avatar_url) values($1,$2,$3,$4) returning id`, email, displayName, role, avatar).Scan(&userID)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict do nothing`, userID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `insert into user_oauth_connections(user_id,provider,provider_user_id,provider_username,provider_avatar_url) values($1,$2,$3,$4,$5)`, userID, provider, providerUserID, name, avatar); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Service) githubExchangeToken(ctx context.Context, cfg *oauthProviderConfig, code, redirectURI string) (string, error) {
	body := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s&redirect_uri=%s",
		url.QueryEscape(cfg.ClientID), url.QueryEscape(cfg.ClientSecret), url.QueryEscape(code), url.QueryEscape(redirectURI))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.AccessToken, nil
}

func (s *Service) githubFetchUser(ctx context.Context, accessToken string) (id, email, name, avatar string, err error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	var user struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return "", "", "", "", fmt.Errorf("unmarshal user: %w", err)
	}
	id = fmt.Sprintf("%d", user.ID)
	name = user.Name
	if name == "" {
		name = user.Login
	}
	email = user.Email
	avatar = user.AvatarURL
	if email == "" {
		req2, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
		req2.Header.Set("Authorization", "Bearer "+accessToken)
		req2.Header.Set("Accept", "application/vnd.github.v3+json")
		resp2, err := s.httpClient.Do(req2)
		if err == nil {
			defer resp2.Body.Close()
			raw2, _ := io.ReadAll(io.LimitReader(resp2.Body, 2<<20))
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if json.Unmarshal(raw2, &emails) == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						email = e.Email
						break
					}
				}
				if email == "" && len(emails) > 0 {
					email = emails[0].Email
				}
			}
		}
	}
	return id, email, name, avatar, nil
}

func (s *Service) listOAuthProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,client_id,enabled,created_at,updated_at from oauth_providers order by id`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, clientID string
		var enabled bool
		var created, updated any
		if rows.Scan(&id, &clientID, &enabled, &created, &updated) == nil {
			data = append(data, map[string]any{"id": id, "client_id": clientID, "enabled": enabled, "has_client_secret": true, "created_at": created, "updated_at": updated})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) upsertOAuthProvider(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Enabled      *bool  `json:"enabled"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "client_id is required")
		return
	}
	provider := r.PathValue("provider")
	if provider == "" {
		writeError(w, 400, "invalid_request", "provider is required")
		return
	}
	in.ClientID = strings.TrimSpace(in.ClientID)
	if in.ClientID == "" {
		writeError(w, 400, "invalid_request", "client_id is required")
		return
	}
	secretEnc := ""
	if secret := strings.TrimSpace(in.ClientSecret); secret != "" {
		encrypted, err := crypt(s.cfg.EncryptionKey, secret, false)
		if err != nil {
			writeError(w, 500, "internal_error", "could not encrypt client secret")
			return
		}
		secretEnc = encrypted
	}
	if in.Enabled == nil {
		v := true
		in.Enabled = &v
	}
	_, err := s.db.Exec(r.Context(), `insert into oauth_providers(id,client_id,client_secret_encrypted,enabled) values($1,$2,$3,$4) on conflict (id) do update set client_id=excluded.client_id,client_secret_encrypted=case when $3='' then oauth_providers.client_secret_encrypted else excluded.client_secret_encrypted end,enabled=excluded.enabled,updated_at=now()`,
		provider, in.ClientID, secretEnc, *in.Enabled)
	if err != nil {
		writeError(w, 500, "internal_error", "could not save OAuth provider")
		return
	}
	s.audit(r, "oauth.provider_updated", "oauth_provider", provider, nil)
	writeJSON(w, 200, map[string]any{"status": "ok", "id": provider})
}

func (s *Service) deleteOAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	_, err := s.db.Exec(r.Context(), `delete from oauth_providers where id=$1`, provider)
	if err != nil {
		writeError(w, 500, "internal_error", "could not delete OAuth provider")
		return
	}
	s.audit(r, "oauth.provider_deleted", "oauth_provider", provider, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) listOAuthConnections(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select provider,provider_username,provider_avatar_url,created_at from user_oauth_connections where user_id=$1 order by provider`, account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var provider, username, avatar string
		var created any
		if rows.Scan(&provider, &username, &avatar, &created) == nil {
			data = append(data, map[string]any{"provider": provider, "provider_username": username, "provider_avatar_url": avatar, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) unlinkOAuthConnection(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	provider := r.PathValue("provider")
	_, err := s.db.Exec(r.Context(), `delete from user_oauth_connections where user_id=$1 and provider=$2`, account.userID, provider)
	if err != nil {
		writeError(w, 500, "internal_error", "could not unlink OAuth connection")
		return
	}
	s.audit(r, "oauth.connection_unlinked", "user", account.userID, map[string]any{"provider": provider})
	w.WriteHeader(http.StatusNoContent)
}
