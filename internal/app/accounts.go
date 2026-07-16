package app

import (
	"context"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"
)

type accountContext struct {
	userID      string
	role        string
	permissions map[string]bool
}
type accountContextKey struct{}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || !validAccountInput(in.Email, in.Name, in.Password) {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid email, name, and password of at least 8 characters are required")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	passwordHash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not secure password")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `select pg_advisory_xact_lock(458110)`); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	var hasAccountAdmin bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where role='admin' and password_hash is not null)`).Scan(&hasAccountAdmin); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	role := "user"
	if !hasAccountAdmin {
		role = "admin"
	}
	_, err = tx.Exec(r.Context(), `insert into users(id,email,name,role,password_hash) values($1,$2,$3,$4,$5)`, id, email, strings.TrimSpace(in.Name), role, passwordHash)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "email already exists")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict do nothing`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	s.createSession(w, r, id, http.StatusCreated)
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Email) == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "email and password are required")
		return
	}
	var userID, passwordHash string
	err := s.db.QueryRow(r.Context(), `select id,password_hash from users where email=$1 and enabled and password_hash is not null`, strings.ToLower(strings.TrimSpace(in.Email))).Scan(&userID, &passwordHash)
	if err != nil || !passwordMatches(passwordHash, in.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	s.createSession(w, r, userID, http.StatusOK)
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	_, _ = s.db.Exec(r.Context(), `delete from user_sessions where token_hash=$1`, hashSecret(bearer(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) accountMe(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountContextKey{}).(accountContext)
	var email, name, role string
	var balance, reserved any
	err := s.db.QueryRow(r.Context(), `select u.email,u.name,u.role,coalesce(w.balance,0),coalesce(w.reserved,0) from users u left join user_wallets w on w.user_id=u.id where u.id=$1`, account.userID).Scan(&email, &name, &role, &balance, &reserved)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load account")
		return
	}
	permissions := make([]string, 0, len(account.permissions))
	for permission := range account.permissions {
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	writeJSON(w, http.StatusOK, map[string]any{"id": account.userID, "email": email, "name": name, "role": role, "permissions": permissions, "balance": balance, "reserved": reserved})
}

func accountFromContext(r *http.Request) accountContext {
	return r.Context().Value(accountContextKey{}).(accountContext)
}

func (s *Service) account(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "account session required")
			return
		}
		var account accountContext
		err := s.db.QueryRow(r.Context(), `select s.user_id,u.role from user_sessions s join users u on u.id=s.user_id where s.token_hash=$1 and s.expires_at>now() and u.enabled`, hashSecret(token)).Scan(&account.userID, &account.role)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
			return
		}
		account.permissions = map[string]bool{}
		if account.role != "admin" {
			rows, queryErr := s.db.Query(r.Context(), `select permission from user_permissions where user_id=$1`, account.userID)
			if queryErr != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not load permissions")
				return
			}
			defer rows.Close()
			for rows.Next() {
				var permission string
				if rows.Scan(&permission) == nil {
					account.permissions[permission] = true
				}
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), accountContextKey{}, account)))
	})
}

func (s *Service) permission(permission string, next http.HandlerFunc) http.Handler {
	return s.account(func(w http.ResponseWriter, r *http.Request) {
		account := r.Context().Value(accountContextKey{}).(accountContext)
		if account.role != "admin" && !account.permissions[permission] {
			writeError(w, http.StatusForbidden, "forbidden", "missing permission: "+permission)
			return
		}
		next(w, r)
	})
}

func (s *Service) createSession(w http.ResponseWriter, r *http.Request, userID string, status int) {
	token, err := randomSecret("xh_session_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = s.db.Exec(r.Context(), `insert into user_sessions(id,user_id,token_hash,expires_at) values($1,$2,$3,$4)`, id, userID, hashSecret(token), expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	writeJSON(w, status, map[string]any{"token": token, "expires_at": expiresAt})
}

func validAccountInput(email, name, password string) bool {
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	return err == nil && parsed.Address == strings.TrimSpace(email) && len(strings.TrimSpace(name)) > 0 && len(strings.TrimSpace(name)) <= 100 && len(password) >= 8 && len(password) <= 128
}
