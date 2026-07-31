package app

import "net/http"

type publicActivityItem struct {
	Model            string `json:"model"`
	StatusCode       int    `json:"status_code"`
	DurationMs       int    `json:"duration_ms"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	CreatedAt        string `json:"created_at"`
}

func (s *Service) publicActivity(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		select rl.model, rl.status_code, rl.duration_ms,
			coalesce(ur.prompt_tokens,0),
			coalesce(ur.completion_tokens,0),
			coalesce(ur.prompt_tokens+ur.completion_tokens,0),
			rl.created_at
		from request_logs rl
		left join usage_records ur on ur.request_id = rl.request_id
		order by rl.created_at desc
		limit 100`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()

	data := []publicActivityItem{}
	for rows.Next() {
		var item publicActivityItem
		if rows.Scan(&item.Model, &item.StatusCode, &item.DurationMs, &item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.CreatedAt) == nil {
			data = append(data, item)
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
