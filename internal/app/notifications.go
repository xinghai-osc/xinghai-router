package app

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type notification struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order"`
	CreatedAt any    `json:"created_at"`
	UpdatedAt any    `json:"updated_at"`
}

const (
	maxNotificationTitleLen   = 200
	maxNotificationContentLen = 5000
)

// publicNotifications returns the enabled notifications, newest first.
func (s *Service) publicNotifications(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,title,content,enabled,sort_order,created_at,updated_at from notifications where enabled order by sort_order asc,created_at desc`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load notifications")
		return
	}
	defer rows.Close()
	writeJSON(w, http.StatusOK, map[string]any{"data": scanNotifications(rows)})
}

func (s *Service) adminListNotifications(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,title,content,enabled,sort_order,created_at,updated_at from notifications order by sort_order asc,created_at desc`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load notifications")
		return
	}
	defer rows.Close()
	writeJSON(w, http.StatusOK, map[string]any{"data": scanNotifications(rows)})
}

func scanNotifications(rows pgx.Rows) []notification {
	data := []notification{}
	for rows.Next() {
		var n notification
		if rows.Scan(&n.ID, &n.Title, &n.Content, &n.Enabled, &n.SortOrder, &n.CreatedAt, &n.UpdatedAt) == nil {
			data = append(data, n)
		}
	}
	return data
}

func (s *Service) createNotification(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		Enabled   *bool  `json:"enabled"`
		SortOrder *int   `json:"sort_order"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid notification")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" || len([]rune(in.Title)) > maxNotificationTitleLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "title must contain 1 to 200 characters")
		return
	}
	if len([]rune(in.Content)) > maxNotificationContentLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "content must be at most 5000 characters")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create notification")
		return
	}
	if _, err := s.db.Exec(r.Context(), `insert into notifications(id,title,content,enabled,sort_order) values($1,$2,$3,coalesce($4,true),coalesce($5,0))`, id, in.Title, in.Content, in.Enabled, in.SortOrder); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create notification")
		return
	}
	s.audit(r, "notifications.created", "notification", id, map[string]any{"title": in.Title, "enabled": in.Enabled})
	writeJSON(w, http.StatusCreated, notification{ID: id, Title: in.Title, Content: in.Content, Enabled: in.Enabled != nil && *in.Enabled, SortOrder: sortOrderValue(in.SortOrder)})
}

func (s *Service) updateNotification(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		Enabled   *bool  `json:"enabled"`
		SortOrder *int   `json:"sort_order"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid notification")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" || len([]rune(in.Title)) > maxNotificationTitleLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "title must contain 1 to 200 characters")
		return
	}
	if len([]rune(in.Content)) > maxNotificationContentLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "content must be at most 5000 characters")
		return
	}
	result, err := s.db.Exec(r.Context(), `update notifications set title=$2,content=$3,enabled=coalesce($4,enabled),sort_order=coalesce($5,sort_order),updated_at=now() where id=$1`, r.PathValue("id"), in.Title, in.Content, in.Enabled, in.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update notification")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "notification not found")
		return
	}
	s.audit(r, "notifications.updated", "notification", r.PathValue("id"), map[string]any{"title": in.Title, "enabled": in.Enabled})
	writeJSON(w, http.StatusOK, notification{ID: r.PathValue("id"), Title: in.Title, Content: in.Content, Enabled: in.Enabled != nil && *in.Enabled, SortOrder: sortOrderValue(in.SortOrder)})
}

func (s *Service) deleteNotification(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `delete from notifications where id=$1`, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete notification")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "notification not found")
		return
	}
	s.audit(r, "notifications.deleted", "notification", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}

func sortOrderValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
