package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

type modelProvider struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Slug     string   `json:"slug"`
	Prefixes []string `json:"prefixes"`
	Priority int      `json:"priority"`
}

func (s *Service) providers(r *http.Request) []modelProvider {
	rows, err := s.db.Query(r.Context(), `select id::text,name,slug,prefixes,priority from model_providers order by priority,name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	data := []modelProvider{}
	for rows.Next() {
		var item modelProvider
		var prefixes []byte
		if rows.Scan(&item.ID, &item.Name, &item.Slug, &prefixes, &item.Priority) == nil {
			_ = json.Unmarshal(prefixes, &item.Prefixes)
			data = append(data, item)
		}
	}
	return data
}

func (s *Service) listProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id::text,name,slug,prefixes,priority from model_providers order by priority,name`)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load providers")
		return
	}
	defer rows.Close()
	data := []modelProvider{}
	for rows.Next() {
		var item modelProvider
		var prefixes []byte
		if rows.Scan(&item.ID, &item.Name, &item.Slug, &prefixes, &item.Priority) == nil {
			_ = json.Unmarshal(prefixes, &item.Prefixes)
			data = append(data, item)
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) saveProvider(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Slug     string   `json:"slug"`
		Prefixes []string `json:"prefixes"`
		Priority int      `json:"priority"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "name, slug, and prefixes are required")
		return
	}
	name := strings.TrimSpace(in.Name)
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	if name == "" || len(name) > 100 || !validProviderSlug(slug) || len(in.Prefixes) == 0 {
		writeError(w, 400, "invalid_request", "name, slug, and prefixes are required")
		return
	}
	for i := range in.Prefixes {
		in.Prefixes[i] = strings.ToLower(strings.TrimSpace(in.Prefixes[i]))
		if in.Prefixes[i] == "" || len(in.Prefixes[i]) > 64 {
			writeError(w, 400, "invalid_request", "prefixes cannot be empty")
			return
		}
	}
	if in.Priority < 0 || in.Priority > 10000 {
		writeError(w, 400, "invalid_request", "priority must be between 0 and 10000")
		return
	}
	in.Name = name
	in.Slug = slug
	prefixes, _ := json.Marshal(in.Prefixes)
	if strings.TrimSpace(in.ID) != "" {
		result, err := s.db.Exec(r.Context(), `update model_providers set name=$1,slug=$2,prefixes=$3,priority=$4,updated_at=now() where id=$5`, in.Name, in.Slug, prefixes, in.Priority, strings.TrimSpace(in.ID))
		if err != nil {
			writeError(w, 409, "conflict", "provider name or slug already exists")
			return
		}
		if result.RowsAffected() != 1 {
			writeError(w, 404, "not_found", "provider not found")
			return
		}
		item := modelProvider{ID: strings.TrimSpace(in.ID), Name: in.Name, Slug: in.Slug, Prefixes: in.Prefixes, Priority: in.Priority}
		s.audit(r, "provider.saved", "model_provider", item.ID, map[string]any{"name": item.Name})
		writeJSON(w, 200, item)
		return
	}
	var item modelProvider
	var raw []byte
	err := s.db.QueryRow(r.Context(), `insert into model_providers(name,slug,prefixes,priority,updated_at) values($1,$2,$3,$4,now()) on conflict (slug) do update set name=excluded.name,prefixes=excluded.prefixes,priority=excluded.priority,updated_at=now() returning id::text,name,slug,prefixes,priority`, in.Name, in.Slug, prefixes, in.Priority).Scan(&item.ID, &item.Name, &item.Slug, &raw, &item.Priority)
	if err != nil {
		writeError(w, 409, "conflict", "provider name or slug already exists")
		return
	}
	_ = json.Unmarshal(raw, &item.Prefixes)
	s.audit(r, "provider.saved", "model_provider", item.ID, map[string]any{"name": item.Name})
	writeJSON(w, 200, item)
}

func (s *Service) deleteProvider(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `delete from model_providers where id=$1`, r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "provider not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func providerForModel(model string, providers []modelProvider) modelProvider {
	name := strings.ToLower(model)
	matches := append([]modelProvider(nil), providers...)
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Priority < matches[j].Priority })
	for _, item := range matches {
		for _, prefix := range item.Prefixes {
			if strings.HasPrefix(name, strings.ToLower(prefix)) {
				return item
			}
		}
	}
	return modelProvider{Name: "其他", Slug: "other"}
}
