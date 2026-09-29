package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

func (s *Service) workspaceAccount(next http.HandlerFunc) http.Handler {
	return s.account(s.withWorkspace(next))
}

func (s *Service) withWorkspace(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account := accountFromContext(r)
		requested := strings.TrimSpace(r.Header.Get("X-Workspace-ID"))
		if len(r.Header.Values("X-Workspace-ID")) > 1 || (requested != "" && !validWorkspaceID(requested)) {
			writeError(w, http.StatusBadRequest, "invalid_workspace", "invalid workspace id")
			return
		}
		id, role, err := s.resolveWorkspace(r.Context(), account.userID, requested)
		if errors.Is(err, errWorkspaceAccess) {
			writeError(w, http.StatusForbidden, "workspace_access_denied", "workspace is unavailable or you are not a member")
			return
		}
		if err != nil || id == "" {
			writeError(w, http.StatusServiceUnavailable, "workspace_unavailable", "could not load workspace")
			return
		}
		account.workspaceID, account.workspaceRole = id, role
		w.Header().Set("X-Workspace-ID", id)
		next(w, r.WithContext(context.WithValue(r.Context(), accountContextKey{}, account)))
	}
}
