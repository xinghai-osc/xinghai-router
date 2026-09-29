package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,48}[a-z0-9])?$`)
var workspaceIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var errWorkspaceAccess = errors.New("workspace access denied")

const workspaceColumns = `w.id::text,w.name,w.slug,w.owner_id::text,w.is_personal,w.created_at,w.updated_at`

type Workspace struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	OwnerID    string    `json:"owner_id"`
	Role       string    `json:"role"`
	IsPersonal bool      `json:"is_personal"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type workspaceMember struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func validWorkspaceID(id string) bool { return workspaceIDPattern.MatchString(id) }
func workspaceSlug(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
		if b.Len() >= 49 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}
func validWorkspaceName(name string) bool {
	return utf8.ValidString(name) && utf8.RuneCountInString(name) >= 1 && utf8.RuneCountInString(name) <= 100 && !strings.ContainsRune(name, 0)
}
func workspaceCanManage(role string) bool { return role == "owner" || role == "admin" }
func workspaceCanRemove(actorRole, targetRole string, self bool) bool {
	return targetRole != "owner" && (actorRole == "owner" || (actorRole == "admin" && targetRole == "member" && !self))
}
func (s *Service) resolveWorkspace(ctx context.Context, userID, requested string) (string, string, error) {
	requested = strings.TrimSpace(requested)
	var id, role string
	var err error
	if requested == "" {
		err = s.db.QueryRow(ctx, `select w.id::text,m.role from workspaces w join workspace_members m on m.workspace_id=w.id where w.owner_id=$1 and w.is_personal and w.archived_at is null and m.user_id=$1`, userID).Scan(&id, &role)
	} else {
		if !validWorkspaceID(requested) {
			return "", "", errWorkspaceAccess
		}
		err = s.db.QueryRow(ctx, `select w.id::text,m.role from workspaces w join workspace_members m on m.workspace_id=w.id where w.id=$1 and m.user_id=$2 and w.archived_at is null`, requested, userID).Scan(&id, &role)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", errWorkspaceAccess
	}
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace: %w", err)
	}
	return id, role, nil
}
func scanWorkspace(row pgx.Row) (Workspace, error) {
	var v Workspace
	err := row.Scan(&v.ID, &v.Name, &v.Slug, &v.OwnerID, &v.IsPersonal, &v.CreatedAt, &v.UpdatedAt, &v.Role)
	return v, err
}
func lockWorkspace(ctx context.Context, tx pgx.Tx, userID, id string) (Workspace, error) {
	var v Workspace
	if !validWorkspaceID(id) {
		return v, errWorkspaceAccess
	}
	err := tx.QueryRow(ctx, `select `+workspaceColumns+` from workspaces w where w.id=$1 and w.archived_at is null for update`, id).Scan(&v.ID, &v.Name, &v.Slug, &v.OwnerID, &v.IsPersonal, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, errWorkspaceAccess
	}
	if err != nil {
		return v, err
	}
	err = tx.QueryRow(ctx, `select role from workspace_members where workspace_id=$1 and user_id=$2`, id, userID).Scan(&v.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, errWorkspaceAccess
	}
	return v, err
}
func writeWorkspaceError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, errWorkspaceAccess) {
		writeError(w, 404, "not_found", "workspace not found")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeError(w, 409, "conflict", "workspace slug or membership already exists")
		return
	}
	writeError(w, 500, "internal_error", message)
}
func (s *Service) beginWorkspaceMutation(w http.ResponseWriter, r *http.Request) (pgx.Tx, Workspace, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !validWorkspaceID(id) {
		writeWorkspaceError(w, errWorkspaceAccess, "")
		return nil, Workspace{}, false
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeWorkspaceError(w, err, "could not update workspace")
		return nil, Workspace{}, false
	}
	v, err := lockWorkspace(r.Context(), tx, accountFromContext(r).userID, id)
	if err != nil {
		_ = tx.Rollback(r.Context())
		writeWorkspaceError(w, err, "could not load workspace")
		return nil, Workspace{}, false
	}
	return tx, v, true
}
func (s *Service) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select `+workspaceColumns+`,m.role from workspaces w join workspace_members m on m.workspace_id=w.id where m.user_id=$1 and w.archived_at is null order by w.is_personal desc,w.created_at,w.id`, account.userID)
	if err != nil {
		writeWorkspaceError(w, err, "could not load workspaces")
		return
	}
	defer rows.Close()
	data := make([]Workspace, 0)
	currentID := ""
	for rows.Next() {
		v, err := scanWorkspace(rows)
		if err != nil {
			writeWorkspaceError(w, err, "could not read workspaces")
			return
		}
		if v.IsPersonal && v.OwnerID == account.userID {
			currentID = v.ID
		}
		data = append(data, v)
	}
	if err := rows.Err(); err != nil {
		writeWorkspaceError(w, err, "could not read workspaces")
		return
	}
	if currentID == "" {
		writeWorkspaceError(w, errWorkspaceAccess, "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": data, "current_id": currentID})
}
func (s *Service) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid workspace")
		return
	}
	name, slug := strings.TrimSpace(in.Name), strings.ToLower(strings.TrimSpace(in.Slug))
	if !validWorkspaceName(name) {
		writeError(w, 400, "invalid_request", "workspace name must be 1-100 characters")
		return
	}
	id, err := randomID()
	if err != nil {
		writeWorkspaceError(w, err, "could not create workspace")
		return
	}
	if slug == "" {
		slug = workspaceSlug(name)
		if slug == "" {
			slug = "workspace-" + id[:8]
		}
	}
	if !workspaceSlugPattern.MatchString(slug) {
		writeError(w, 400, "invalid_request", "workspace slug must use 1-50 lowercase letters, numbers, or hyphens")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeWorkspaceError(w, err, "could not create workspace")
		return
	}
	defer tx.Rollback(r.Context())
	account := accountFromContext(r)
	if _, err := tx.Exec(r.Context(), `insert into workspaces(id,name,slug,owner_id) values($1,$2,$3,$4)`, id, name, slug, account.userID); err != nil {
		writeWorkspaceError(w, err, "could not create workspace")
		return
	}
	if _, err := tx.Exec(r.Context(), `insert into workspace_members(workspace_id,user_id,role) values($1,$2,'owner')`, id, account.userID); err != nil {
		writeWorkspaceError(w, err, "could not create workspace")
		return
	}
	v, err := lockWorkspace(r.Context(), tx, account.userID, id)
	if err != nil {
		writeWorkspaceError(w, err, "could not create workspace")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeWorkspaceError(w, err, "could not create workspace")
		return
	}
	s.audit(r, "workspace.created", "workspace", id, map[string]any{"name": name, "slug": slug})
	writeJSON(w, 201, v)
}
func (s *Service) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name *string `json:"name"`
		Slug *string `json:"slug"`
	}
	if decode(r, &in) != nil || (in.Name == nil && in.Slug == nil) {
		writeError(w, 400, "invalid_request", "workspace name or slug is required")
		return
	}
	tx, v, ok := s.beginWorkspaceMutation(w, r)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	if !workspaceCanManage(v.Role) {
		writeError(w, 403, "forbidden", "workspace administrator access required")
		return
	}
	if in.Name != nil {
		v.Name = strings.TrimSpace(*in.Name)
	}
	if in.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.Slug))
		if slug != v.Slug && v.Role != "owner" {
			writeError(w, 403, "forbidden", "workspace owner access is required to change the slug")
			return
		}
		v.Slug = slug
	}
	if !validWorkspaceName(v.Name) || !workspaceSlugPattern.MatchString(v.Slug) {
		writeError(w, 400, "invalid_request", "workspace name or slug is invalid")
		return
	}
	if err := tx.QueryRow(r.Context(), `update workspaces set name=$1,slug=$2,updated_at=now() where id=$3 returning updated_at`, v.Name, v.Slug, v.ID).Scan(&v.UpdatedAt); err != nil {
		writeWorkspaceError(w, err, "could not update workspace")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeWorkspaceError(w, err, "could not update workspace")
		return
	}
	s.audit(r, "workspace.updated", "workspace", v.ID, map[string]any{"name": v.Name, "slug": v.Slug})
	writeJSON(w, 200, v)
}
func (s *Service) selectWorkspace(w http.ResponseWriter, r *http.Request) {
	requested := strings.TrimSpace(r.PathValue("id"))
	if !validWorkspaceID(requested) {
		writeWorkspaceError(w, errWorkspaceAccess, "")
		return
	}
	id, _, err := s.resolveWorkspace(r.Context(), accountFromContext(r).userID, requested)
	if err != nil {
		writeWorkspaceError(w, err, "could not select workspace")
		return
	}
	writeJSON(w, 200, map[string]string{"workspace_id": id})
}
func (s *Service) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	tx, v, ok := s.beginWorkspaceMutation(w, r)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	if v.Role != "owner" {
		writeError(w, 403, "forbidden", "workspace owner access required")
		return
	}
	if v.IsPersonal {
		writeError(w, 400, "invalid_request", "personal workspace cannot be archived")
		return
	}
	if _, err := tx.Exec(r.Context(), `update workspaces set archived_at=now(),updated_at=now() where id=$1`, v.ID); err != nil {
		writeWorkspaceError(w, err, "could not archive workspace")
		return
	}
	if _, err := tx.Exec(r.Context(), `update api_keys set revoked_at=now() where workspace_id=$1 and revoked_at is null`, v.ID); err != nil {
		writeWorkspaceError(w, err, "could not revoke workspace API keys")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeWorkspaceError(w, err, "could not archive workspace")
		return
	}
	s.audit(r, "workspace.archived", "workspace", v.ID, nil)
	w.WriteHeader(204)
}
func (s *Service) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !validWorkspaceID(id) {
		writeWorkspaceError(w, errWorkspaceAccess, "")
		return
	}
	if _, _, err := s.resolveWorkspace(r.Context(), accountFromContext(r).userID, id); err != nil {
		writeWorkspaceError(w, err, "could not load workspace members")
		return
	}
	rows, err := s.db.Query(r.Context(), `select u.id::text,u.email,u.name,m.role,m.created_at from workspace_members m join users u on u.id=m.user_id join workspaces w on w.id=m.workspace_id where m.workspace_id=$1 and w.archived_at is null and exists(select 1 from workspace_members actor where actor.workspace_id=m.workspace_id and actor.user_id=$2) order by case m.role when 'owner' then 0 when 'admin' then 1 else 2 end,m.created_at,u.id`, id, accountFromContext(r).userID)
	if err != nil {
		writeWorkspaceError(w, err, "could not load workspace members")
		return
	}
	defer rows.Close()
	data := make([]workspaceMember, 0)
	for rows.Next() {
		var m workspaceMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt); err != nil {
			writeWorkspaceError(w, err, "could not read workspace members")
			return
		}
		data = append(data, m)
	}
	if err := rows.Err(); err != nil {
		writeWorkspaceError(w, err, "could not read workspace members")
		return
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
func (s *Service) addWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if decode(r, &in) != nil || !validEmail(strings.TrimSpace(in.Email)) {
		writeError(w, 400, "invalid_request", "a valid member email is required")
		return
	}
	if in.Role == "" {
		in.Role = "member"
	}
	if in.Role != "admin" && in.Role != "member" {
		writeError(w, 400, "invalid_request", "member role must be admin or member")
		return
	}
	tx, v, ok := s.beginWorkspaceMutation(w, r)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	if !workspaceCanManage(v.Role) || (v.Role == "admin" && in.Role != "member") {
		writeError(w, 403, "forbidden", "workspace owner access is required to add administrators")
		return
	}
	if v.IsPersonal {
		writeError(w, 400, "invalid_request", "personal workspace does not support members")
		return
	}
	var m workspaceMember
	err := tx.QueryRow(r.Context(), `select id::text,email,name from users where lower(email)=lower($1) and enabled`, strings.TrimSpace(in.Email)).Scan(&m.UserID, &m.Email, &m.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "user not found")
		return
	}
	if err != nil {
		writeWorkspaceError(w, err, "could not find user")
		return
	}
	m.Role = in.Role
	if err := tx.QueryRow(r.Context(), `insert into workspace_members(workspace_id,user_id,role) values($1,$2,$3) returning created_at`, v.ID, m.UserID, m.Role).Scan(&m.CreatedAt); err != nil {
		writeWorkspaceError(w, err, "could not add workspace member")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeWorkspaceError(w, err, "could not add workspace member")
		return
	}
	s.audit(r, "workspace.member_added", "workspace", v.ID, map[string]any{"user_id": m.UserID, "role": m.Role})
	writeJSON(w, 201, m)
}
func loadWorkspaceMember(ctx context.Context, tx pgx.Tx, workspaceID, userID string) (workspaceMember, error) {
	var m workspaceMember
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || id <= 0 {
		return m, pgx.ErrNoRows
	}
	err = tx.QueryRow(ctx, `select u.id::text,u.email,u.name,m.role,m.created_at from workspace_members m join users u on u.id=m.user_id where m.workspace_id=$1 and m.user_id=$2`, workspaceID, id).Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt)
	return m, err
}
func (s *Service) updateWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if decode(r, &in) != nil || (in.Role != "admin" && in.Role != "member") {
		writeError(w, 400, "invalid_request", "member role must be admin or member")
		return
	}
	tx, v, ok := s.beginWorkspaceMutation(w, r)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	if v.Role != "owner" {
		writeError(w, 403, "forbidden", "workspace owner access required")
		return
	}
	if v.IsPersonal {
		writeError(w, 400, "invalid_request", "personal workspace does not support members")
		return
	}
	m, err := loadWorkspaceMember(r.Context(), tx, v.ID, strings.TrimSpace(r.PathValue("user_id")))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "workspace member not found")
		return
	}
	if err != nil {
		writeWorkspaceError(w, err, "could not load workspace member")
		return
	}
	if m.Role == "owner" {
		writeError(w, 403, "forbidden", "workspace owner cannot be demoted")
		return
	}
	if _, err := tx.Exec(r.Context(), `update workspace_members set role=$1 where workspace_id=$2 and user_id=$3`, in.Role, v.ID, m.UserID); err != nil {
		writeWorkspaceError(w, err, "could not update workspace member")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeWorkspaceError(w, err, "could not update workspace member")
		return
	}
	m.Role = in.Role
	s.audit(r, "workspace.member_updated", "workspace", v.ID, map[string]any{"user_id": m.UserID, "role": m.Role})
	writeJSON(w, 200, m)
}
func (s *Service) removeWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	tx, v, ok := s.beginWorkspaceMutation(w, r)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	if !workspaceCanManage(v.Role) {
		writeError(w, 403, "forbidden", "workspace administrator access required")
		return
	}
	if v.IsPersonal {
		writeError(w, 400, "invalid_request", "personal workspace does not support members")
		return
	}
	m, err := loadWorkspaceMember(r.Context(), tx, v.ID, strings.TrimSpace(r.PathValue("user_id")))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "workspace member not found")
		return
	}
	if err != nil {
		writeWorkspaceError(w, err, "could not load workspace member")
		return
	}
	if !workspaceCanRemove(v.Role, m.Role, m.UserID == accountFromContext(r).userID) {
		writeError(w, 403, "forbidden", "workspace member cannot be removed by this user")
		return
	}
	if _, err := tx.Exec(r.Context(), `update api_keys set revoked_at=now() where workspace_id=$1 and user_id=$2 and revoked_at is null`, v.ID, m.UserID); err != nil {
		writeWorkspaceError(w, err, "could not revoke member API keys")
		return
	}
	if _, err := tx.Exec(r.Context(), `delete from workspace_members where workspace_id=$1 and user_id=$2`, v.ID, m.UserID); err != nil {
		writeWorkspaceError(w, err, "could not remove workspace member")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeWorkspaceError(w, err, "could not remove workspace member")
		return
	}
	s.audit(r, "workspace.member_removed", "workspace", v.ID, map[string]any{"user_id": m.UserID})
	w.WriteHeader(204)
}
