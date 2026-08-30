package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	maxModelMetadataDescriptionLength = 2000
	maxModelMetadataModalities        = 20
	maxModelMetadataModalityLength    = 32
	maxModelMetadataContextWindow     = int64(1_000_000_000_000)
)

var validModelMetadataModalities = map[string]bool{
	"text":  true,
	"image": true,
	"audio": true,
	"video": true,
	"file":  true,
}

type modelMetadata struct {
	ID               string   `json:"id"`
	Model            string   `json:"model"`
	Description      string   `json:"description"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	ContextWindow    *int64   `json:"context_window"`
}

type modelMetadataInput struct {
	Model            string   `json:"model"`
	Description      string   `json:"description"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	ContextWindow    *int64   `json:"context_window"`
}

func validMetadataID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func normalizeModelMetadataModalities(values []string) ([]string, bool) {
	if len(values) > maxModelMetadataModalities {
		return nil, false
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !validModelMetadataModalities[value] || len(value) > maxModelMetadataModalityLength || seen[value] {
			return nil, false
		}
		seen[value] = true
		out = append(out, value)
	}
	return out, true
}

func (s *Service) modelExistsInCatalog(ctx context.Context, model string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `select exists(
		select 1 from channels c
		where c.enabled and not c.auto_disabled
			and exists(select 1 from jsonb_array_elements_text(c.models) as item(model) where trim(item.model)=$1)
	) or exists(
		select 1 from model_routes m
		join channels c on c.id=m.channel_id
		where c.enabled and not c.auto_disabled and m.enabled and not m.hidden and trim(m.public_model)=$1
	)`, model).Scan(&exists)
	return exists, err
}

func validateModelMetadataInput(in *modelMetadataInput) bool {
	in.Model = strings.TrimSpace(in.Model)
	in.Description = strings.TrimSpace(in.Description)
	if !validModelName(in.Model) || len([]rune(in.Description)) > maxModelMetadataDescriptionLength {
		return false
	}
	var ok bool
	if in.InputModalities, ok = normalizeModelMetadataModalities(in.InputModalities); !ok {
		return false
	}
	if in.OutputModalities, ok = normalizeModelMetadataModalities(in.OutputModalities); !ok {
		return false
	}
	return in.ContextWindow == nil || (*in.ContextWindow > 0 && *in.ContextWindow <= maxModelMetadataContextWindow)
}

func modelMetadataResponse(item modelMetadata, createdAt, updatedAt any) map[string]any {
	input := item.InputModalities
	if input == nil {
		input = []string{}
	}
	output := item.OutputModalities
	if output == nil {
		output = []string{}
	}
	return map[string]any{
		"id":                item.ID,
		"model":             item.Model,
		"description":       item.Description,
		"input_modalities":  input,
		"output_modalities": output,
		"context_window":    item.ContextWindow,
		"created_at":        createdAt,
		"updated_at":        updatedAt,
	}
}

func scanModelMetadata(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]map[string]any, error) {
	data := []map[string]any{}
	for rows.Next() {
		var item modelMetadata
		var createdAt, updatedAt any
		if err := rows.Scan(&item.ID, &item.Model, &item.Description, &item.InputModalities, &item.OutputModalities, &item.ContextWindow, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		data = append(data, modelMetadataResponse(item, createdAt, updatedAt))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Service) listModelMetadata(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id::text,model,description,input_modalities,output_modalities,context_window,created_at,updated_at from model_catalog_metadata order by model`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load model metadata")
		return
	}
	defer rows.Close()
	data, err := scanModelMetadata(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load model metadata")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Service) createModelMetadata(w http.ResponseWriter, r *http.Request) {
	var in modelMetadataInput
	if decode(r, &in) != nil || !validateModelMetadataInput(&in) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata")
		return
	}
	exists, err := s.modelExistsInCatalog(r.Context(), in.Model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not verify model")
		return
	}
	if !exists {
		writeError(w, http.StatusBadRequest, "invalid_request", "model is not available in the catalog")
		return
	}
	var id string
	err = s.db.QueryRow(r.Context(), `insert into model_catalog_metadata(model,description,input_modalities,output_modalities,context_window) values($1,$2,$3,$4,$5) returning id::text`, in.Model, in.Description, in.InputModalities, in.OutputModalities, in.ContextWindow).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "conflict", "model metadata already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create model metadata")
		return
	}
	s.audit(r, "model_metadata.created", "model_catalog_metadata", id, map[string]any{"model": in.Model, "description": in.Description, "input_modalities": in.InputModalities, "output_modalities": in.OutputModalities, "context_window": in.ContextWindow})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "model": in.Model})
}

func (s *Service) updateModelMetadata(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !validMetadataID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata id")
		return
	}
	var in modelMetadataInput
	if decode(r, &in) != nil || !validateModelMetadataInput(&in) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata")
		return
	}
	var model string
	result, err := s.db.Exec(r.Context(), `update model_catalog_metadata set description=$1,input_modalities=$2,output_modalities=$3,context_window=$4,updated_at=now() where id=$5::uuid and model=$6`, in.Description, in.InputModalities, in.OutputModalities, in.ContextWindow, id, in.Model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update model metadata")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "model metadata not found")
		return
	}
	if err := s.db.QueryRow(r.Context(), `select model from model_catalog_metadata where id=$1::uuid`, id).Scan(&model); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load model metadata")
		return
	}
	s.audit(r, "model_metadata.updated", "model_catalog_metadata", id, map[string]any{"model": model, "description": in.Description, "input_modalities": in.InputModalities, "output_modalities": in.OutputModalities, "context_window": in.ContextWindow})
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "model": model})
}

func (s *Service) deleteModelMetadata(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !validMetadataID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata id")
		return
	}
	result, err := s.db.Exec(r.Context(), `delete from model_catalog_metadata where id=$1::uuid`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete model metadata")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "model metadata not found")
		return
	}
	s.audit(r, "model_metadata.deleted", "model_catalog_metadata", id, nil)
	w.WriteHeader(http.StatusNoContent)
}
