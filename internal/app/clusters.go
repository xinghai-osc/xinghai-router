package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type clusterInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Endpoint    string `json:"endpoint"`
	Enabled     *bool  `json:"enabled"`
}
type instanceInput struct {
	Name     string         `json:"name"`
	Address  string         `json:"address"`
	Metadata map[string]any `json:"metadata"`
	Enabled  *bool          `json:"enabled"`
}

func (s *Service) listClusters(w http.ResponseWriter, r *http.Request) {
	rows, e := s.db.Query(r.Context(), `select c.id,c.name,c.description,c.endpoint,c.enabled,c.manager_id,c.created_at,c.updated_at,coalesce(count(i.id),0) from clusters c left join cluster_instances i on i.cluster_id=c.id group by c.id order by c.created_at desc`)
	if e != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, name, desc, endpoint string
		var enabled bool
		var manager any
		var created, updated any
		var count int
		if rows.Scan(&id, &name, &desc, &endpoint, &enabled, &manager, &created, &updated, &count) == nil {
			out = append(out, map[string]any{"id": id, "name": name, "description": desc, "endpoint": endpoint, "enabled": enabled, "manager_id": manager, "instance_count": count, "created_at": created, "updated_at": updated})
		}
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
func (s *Service) createCluster(w http.ResponseWriter, r *http.Request) {
	var v clusterInput
	if decode(r, &v) != nil || strings.TrimSpace(v.Name) == "" {
		writeError(w, 400, "invalid_request", "name required")
		return
	}
	id, _ := randomID()
	en := true
	if v.Enabled != nil {
		en = *v.Enabled
	}
	_, e := s.db.Exec(r.Context(), `insert into clusters(id,name,description,endpoint,enabled) values($1,$2,$3,$4,$5)`, id, v.Name, v.Description, v.Endpoint, en)
	if e != nil {
		writeError(w, 409, "conflict", "could not create cluster")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}
func (s *Service) updateCluster(w http.ResponseWriter, r *http.Request) {
	var v clusterInput
	if decode(r, &v) != nil {
		writeError(w, 400, "invalid_request", "invalid body")
		return
	}
	id := r.PathValue("id")
	en := true
	if v.Enabled != nil {
		en = *v.Enabled
	}
	tag, e := s.db.Exec(r.Context(), `update clusters set name=coalesce(nullif($2,''),name),description=$3,endpoint=$4,enabled=$5,updated_at=now() where id=$1`, id, v.Name, v.Description, v.Endpoint, en)
	if e != nil || tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	writeJSON(w, 200, map[string]any{"id": id})
}
func (s *Service) deleteCluster(w http.ResponseWriter, r *http.Request) {
	tag, e := s.db.Exec(r.Context(), `delete from clusters where id=$1`, r.PathValue("id"))
	if e != nil || tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	w.WriteHeader(204)
}
func (s *Service) assignClusterManager(w http.ResponseWriter, r *http.Request) {
	var v struct {
		ManagerID string `json:"manager_id"`
	}
	if decode(r, &v) != nil {
		writeError(w, 400, "invalid_request", "invalid body")
		return
	}
	v.ManagerID = strings.TrimSpace(v.ManagerID)
	if v.ManagerID == "" {
		writeError(w, 400, "invalid_request", "manager_id required")
		return
	}
	tag, e := s.db.Exec(r.Context(), `update clusters set manager_id=$2,updated_at=now() where id=$1 and exists(select 1 from users where id=$2 and enabled and role in ('admin','operator'))`, r.PathValue("id"), v.ManagerID)
	if e != nil || tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "cluster or manager not found")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Service) listInstances(w http.ResponseWriter, r *http.Request) {
	rows, e := s.db.Query(r.Context(), `select id,name,address,metadata,enabled,last_seen_at,created_at from cluster_instances where cluster_id=$1 order by created_at desc`, r.PathValue("id"))
	if e != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, name, address string
		var meta any
		var enabled bool
		var seen, created any
		if err := rows.Scan(&id, &name, &address, &meta, &enabled, &seen, &created); err != nil {
			writeError(w, 500, "internal_error", "query failed")
			return
		}
		out = append(out, map[string]any{"id": id, "name": name, "address": address, "metadata": meta, "enabled": enabled, "last_seen_at": seen, "created_at": created})
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
func decodeInstanceInput(w http.ResponseWriter, r *http.Request) (instanceInput, bool) {
	var v instanceInput
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(&v)
	if err == nil {
		if err = d.Decode(new(any)); errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = errors.New("multiple JSON values")
		}
	}
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid body")
		return v, false
	}
	v.Name = strings.TrimSpace(v.Name)
	v.Address = strings.TrimSpace(v.Address)
	if v.Name == "" || v.Address == "" {
		writeError(w, 400, "invalid_request", "name and address required")
		return v, false
	}
	return v, true
}

func writeInstanceMutationError(w http.ResponseWriter, err error, message string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeError(w, 409, "conflict", "instance name already exists")
		return
	}
	writeError(w, 500, "internal_error", message)
}

func (s *Service) createInstance(w http.ResponseWriter, r *http.Request) {
	v, ok := decodeInstanceInput(w, r)
	if !ok {
		return
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
	clusterID := r.PathValue("id")
	id, err := randomID()
	if err != nil {
		writeError(w, 500, "internal_error", "could not create instance")
		return
	}
	en := true
	if v.Enabled != nil {
		en = *v.Enabled
	}
	tag, err := s.db.Exec(r.Context(), `insert into cluster_instances(id,cluster_id,name,address,metadata,enabled) select $1,id,$3,$4,$5,$6 from clusters where id=$2`, id, clusterID, v.Name, v.Address, v.Metadata, en)
	if err != nil {
		writeInstanceMutationError(w, err, "could not create instance")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Service) updateInstance(w http.ResponseWriter, r *http.Request) {
	v, ok := decodeInstanceInput(w, r)
	if !ok {
		return
	}
	var metadata any
	if v.Metadata != nil {
		metadata = v.Metadata
	}
	id := r.PathValue("instanceId")
	clusterID := r.PathValue("id")
	tag, err := s.db.Exec(r.Context(), `update cluster_instances set name=$3,address=$4,metadata=coalesce($5,metadata),enabled=coalesce($6,enabled) where id=$1 and cluster_id=$2`, id, clusterID, v.Name, v.Address, metadata, v.Enabled)
	if err != nil {
		writeInstanceMutationError(w, err, "could not update instance")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "instance not found")
		return
	}
	writeJSON(w, 200, map[string]any{"id": id})
}

func (s *Service) listAllInstances(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,cluster_id,name,address,enabled,created_at from cluster_instances order by created_at desc`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, cid, name, address string
		var enabled bool
		var created any
		if rows.Scan(&id, &cid, &name, &address, &enabled, &created) == nil {
			out = append(out, map[string]any{"id": id, "cluster_id": cid, "name": name, "address": address, "endpoint": address, "enabled": enabled, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
func (s *Service) deleteInstance(w http.ResponseWriter, r *http.Request) {
	tag, err := s.db.Exec(r.Context(), `delete from cluster_instances where id=$1 and cluster_id=$2`, r.PathValue("instanceId"), r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "internal_error", "could not delete instance")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "instance not found")
		return
	}
	w.WriteHeader(204)
}
func (s *Service) syncCluster(w http.ResponseWriter, r *http.Request) {
	id, _ := randomID()
	tag, e := s.db.Exec(r.Context(), `insert into cluster_syncs(id,cluster_id,status,message) select $1,id,'pending','sync queued' from clusters where id=$2`, id, r.PathValue("id"))
	if e != nil {
		writeError(w, 500, "internal_error", "could not queue sync")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	go s.processClusterSync(context.Background(), id)
	writeJSON(w, 202, map[string]any{"id": id, "status": "pending"})
}

func (s *Service) listClusterSyncs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,status,message,attempts,started_at,finished_at,created_at,updated_at from cluster_syncs where cluster_id=$1 order by created_at desc limit 50`, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, status, message string
		var attempts int
		var started, finished, created, updated any
		if rows.Scan(&id, &status, &message, &attempts, &started, &finished, &created, &updated) == nil {
			out = append(out, map[string]any{"id": id, "status": status, "message": message, "attempts": attempts, "started_at": started, "finished_at": finished, "created_at": created, "updated_at": updated})
		}
	}
	writeJSON(w, 200, map[string]any{"data": out})
}

func (s *Service) getClusterSync(w http.ResponseWriter, r *http.Request) {
	var status, message string
	var attempts int
	var started, finished, created, updated any
	err := s.db.QueryRow(r.Context(), `select status,message,attempts,started_at,finished_at,created_at,updated_at from cluster_syncs where id=$1 and cluster_id=$2`, r.PathValue("syncId"), r.PathValue("id")).Scan(&status, &message, &attempts, &started, &finished, &created, &updated)
	if err != nil {
		writeError(w, 404, "not_found", "sync not found")
		return
	}
	writeJSON(w, 200, map[string]any{"id": r.PathValue("syncId"), "status": status, "message": message, "attempts": attempts, "started_at": started, "finished_at": finished, "created_at": created, "updated_at": updated})
}

func (s *Service) processClusterSync(ctx context.Context, id string) {
	var endpoint string
	if err := s.db.QueryRow(ctx, `update cluster_syncs set status='running',message='sync started',started_at=now(),updated_at=now(),attempts=attempts+1 where id=$1 and status='pending' returning (select endpoint from clusters where id=cluster_id)`, id).Scan(&endpoint); err != nil {
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/internal/sync", nil)
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		_, err = client.Do(req)
	}
	status, message := "success", "sync completed"
	if err != nil {
		status, message = "failed", "sync failed"
	}
	_, _ = s.db.Exec(ctx, `update cluster_syncs set status=$2,message=$3,finished_at=now(),updated_at=now() where id=$1`, id, status, message)
}
