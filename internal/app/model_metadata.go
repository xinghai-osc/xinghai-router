package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	maxModelMetadataDescriptionLength = 2000
	maxModelMetadataNameLength        = 200
	maxModelMetadataOwnerLength       = 200
	maxModelMetadataModalities        = 20
	maxModelMetadataModalityLength    = 32
	maxModelMetadataContextWindow     = int64(1_000_000_000_000)
	maxModelMetadataOutputTokens      = int64(1_000_000_000_000)
	maxModelMetadataReasoningLevels   = 20
	maxModelMetadataReasoningLength   = 32
)

var validModelMetadataModalities = map[string]bool{
	"text": true, "image": true, "audio": true, "video": true, "file": true,
}

type modelReasoningEfforts struct {
	SupportedLevels []string `json:"supported_levels"`
	DefaultLevel    string   `json:"default_level,omitempty"`
}

type modelAnthropicMessagesCapability struct {
	SystemPromptUpdate string `json:"system_prompt_update"`
}

type modelAPICapabilities struct {
	AnthropicMessages *modelAnthropicMessagesCapability `json:"anthropic_messages,omitempty"`
}

type modelMetadata struct {
	ID               string                 `json:"id"`
	Model            string                 `json:"model"`
	Name             string                 `json:"name"`
	OwnedBy          string                 `json:"owned_by"`
	Description      string                 `json:"description"`
	InputModalities  []string               `json:"input_modalities"`
	OutputModalities []string               `json:"output_modalities"`
	ContextWindow    *int64                 `json:"context_window"`
	MaxOutputTokens  *int64                 `json:"max_output_tokens"`
	ReasoningEfforts *modelReasoningEfforts `json:"reasoning_efforts"`
	APICapabilities  *modelAPICapabilities  `json:"api_capabilities"`
}

type modelMetadataInput struct {
	Model            string                 `json:"model"`
	Name             *string                `json:"name"`
	OwnedBy          *string                `json:"owned_by"`
	Description      string                 `json:"description"`
	InputModalities  []string               `json:"input_modalities"`
	OutputModalities []string               `json:"output_modalities"`
	ContextWindow    *int64                 `json:"context_window"`
	MaxOutputTokens  *int64                 `json:"max_output_tokens"`
	ReasoningEfforts *modelReasoningEfforts `json:"reasoning_efforts"`
	APICapabilities  *modelAPICapabilities  `json:"api_capabilities"`

	nameSet, ownedBySet, maxOutputTokensSet, reasoningEffortsSet, apiCapabilitiesSet bool
}

func (in *modelMetadataInput) UnmarshalJSON(data []byte) error {
	type plain modelMetadataInput
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*in = modelMetadataInput(value)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, in.nameSet = fields["name"]
	_, in.ownedBySet = fields["owned_by"]
	_, in.maxOutputTokensSet = fields["max_output_tokens"]
	_, in.reasoningEffortsSet = fields["reasoning_efforts"]
	_, in.apiCapabilitiesSet = fields["api_capabilities"]
	return nil
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

func normalizeModelMetadataReasoning(value *modelReasoningEfforts) bool {
	if value == nil {
		return true
	}
	if len(value.SupportedLevels) == 0 || len(value.SupportedLevels) > maxModelMetadataReasoningLevels {
		return false
	}
	seen := make(map[string]bool, len(value.SupportedLevels))
	for index, level := range value.SupportedLevels {
		level = strings.ToLower(strings.TrimSpace(level))
		if level == "" || level == "none" || level == "off" || len(level) > maxModelMetadataReasoningLength || seen[level] {
			return false
		}
		for _, char := range level {
			if !(unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '-') {
				return false
			}
		}
		seen[level] = true
		value.SupportedLevels[index] = level
	}
	value.DefaultLevel = strings.ToLower(strings.TrimSpace(value.DefaultLevel))
	if value.DefaultLevel == "" {
		return true
	}
	if value.DefaultLevel != "none" && !seen[value.DefaultLevel] {
		return false
	}
	return true
}

func normalizeModelMetadataCapabilities(value *modelAPICapabilities) bool {
	if value == nil || value.AnthropicMessages == nil {
		return true
	}
	value.AnthropicMessages.SystemPromptUpdate = strings.TrimSpace(value.AnthropicMessages.SystemPromptUpdate)
	return value.AnthropicMessages.SystemPromptUpdate == "leading-only" || value.AnthropicMessages.SystemPromptUpdate == "in-history"
}

func validateModelMetadataInput(in *modelMetadataInput) bool {
	in.Model = strings.TrimSpace(in.Model)
	in.Description = strings.TrimSpace(in.Description)
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if len([]rune(*in.Name)) > maxModelMetadataNameLength {
			return false
		}
	}
	if in.OwnedBy != nil {
		*in.OwnedBy = strings.TrimSpace(*in.OwnedBy)
		if len([]rune(*in.OwnedBy)) > maxModelMetadataOwnerLength {
			return false
		}
	}
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
	if in.ContextWindow != nil && (*in.ContextWindow <= 0 || *in.ContextWindow > maxModelMetadataContextWindow) {
		return false
	}
	if in.MaxOutputTokens != nil && (*in.MaxOutputTokens <= 0 || *in.MaxOutputTokens > maxModelMetadataOutputTokens) {
		return false
	}
	if in.ContextWindow != nil && in.MaxOutputTokens != nil && *in.MaxOutputTokens > *in.ContextWindow {
		return false
	}
	return normalizeModelMetadataReasoning(in.ReasoningEfforts) && normalizeModelMetadataCapabilities(in.APICapabilities)
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
		"name":              item.Name,
		"owned_by":          item.OwnedBy,
		"description":       item.Description,
		"input_modalities":  input,
		"output_modalities": output,
		"context_window":    item.ContextWindow,
		"max_output_tokens": item.MaxOutputTokens,
		"reasoning_efforts": item.ReasoningEfforts,
		"api_capabilities":  item.APICapabilities,
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
		var reasoningRaw, capabilitiesRaw []byte
		if err := rows.Scan(&item.ID, &item.Model, &item.Name, &item.OwnedBy, &item.Description, &item.InputModalities, &item.OutputModalities, &item.ContextWindow, &item.MaxOutputTokens, &reasoningRaw, &capabilitiesRaw, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if len(reasoningRaw) > 0 && string(reasoningRaw) != "null" {
			if err := json.Unmarshal(reasoningRaw, &item.ReasoningEfforts); err != nil {
				return nil, err
			}
		}
		if len(capabilitiesRaw) > 0 && string(capabilitiesRaw) != "null" {
			if err := json.Unmarshal(capabilitiesRaw, &item.APICapabilities); err != nil {
				return nil, err
			}
		}
		data = append(data, modelMetadataResponse(item, createdAt, updatedAt))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return data, nil
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

func (s *Service) listModelMetadata(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id::text,model,name,owned_by,description,input_modalities,output_modalities,context_window,max_output_tokens,reasoning_efforts,api_capabilities,created_at,updated_at from model_catalog_metadata order by model`)
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

func modelMetadataJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
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
	name, ownedBy := "", ""
	if in.Name != nil {
		name = *in.Name
	}
	if in.OwnedBy != nil {
		ownedBy = *in.OwnedBy
	}
	reasoning, err := modelMetadataJSON(in.ReasoningEfforts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata")
		return
	}
	capabilities, err := modelMetadataJSON(in.APICapabilities)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata")
		return
	}
	var id string
	err = s.db.QueryRow(r.Context(), `insert into model_catalog_metadata(model,name,owned_by,description,input_modalities,output_modalities,context_window,max_output_tokens,reasoning_efforts,api_capabilities) values($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb) returning id::text`, in.Model, name, ownedBy, in.Description, in.InputModalities, in.OutputModalities, in.ContextWindow, in.MaxOutputTokens, reasoning, capabilities).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "conflict", "model metadata already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create model metadata")
		return
	}
	s.audit(r, "model_metadata.created", "model_catalog_metadata", id, map[string]any{"model": in.Model, "name": name, "owned_by": ownedBy, "description": in.Description, "input_modalities": in.InputModalities, "output_modalities": in.OutputModalities, "context_window": in.ContextWindow, "max_output_tokens": in.MaxOutputTokens, "reasoning_efforts": in.ReasoningEfforts, "api_capabilities": in.APICapabilities})
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
	reasoning, err := modelMetadataJSON(in.ReasoningEfforts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata")
		return
	}
	capabilities, err := modelMetadataJSON(in.APICapabilities)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid model metadata")
		return
	}
	var model string
	err = s.db.QueryRow(r.Context(), `update model_catalog_metadata set name=case when $7 then coalesce($1,'') else name end,owned_by=case when $8 then coalesce($2,'') else owned_by end,description=$3,input_modalities=$4,output_modalities=$5,context_window=$6,max_output_tokens=case when $9 then $10 else max_output_tokens end,reasoning_efforts=case when $11 then $12::jsonb else reasoning_efforts end,api_capabilities=case when $13 then $14::jsonb else api_capabilities end,updated_at=now() where id=$15::uuid and model=$16 returning model`, in.Name, in.OwnedBy, in.Description, in.InputModalities, in.OutputModalities, in.ContextWindow, in.nameSet, in.ownedBySet, in.maxOutputTokensSet, in.MaxOutputTokens, in.reasoningEffortsSet, reasoning, in.apiCapabilitiesSet, capabilities, id, in.Model).Scan(&model)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "model metadata not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update model metadata")
		return
	}
	s.audit(r, "model_metadata.updated", "model_catalog_metadata", id, map[string]any{"model": model, "name": in.Name, "owned_by": in.OwnedBy, "description": in.Description, "input_modalities": in.InputModalities, "output_modalities": in.OutputModalities, "context_window": in.ContextWindow, "max_output_tokens": in.MaxOutputTokens, "reasoning_efforts": in.ReasoningEfforts, "api_capabilities": in.APICapabilities})
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
