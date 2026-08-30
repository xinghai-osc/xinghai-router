package app

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const performanceWindow = 24 * time.Hour

type modelPerformanceGroup struct {
	GroupID         string   `json:"group_id"`
	GroupName       string   `json:"group_name"`
	Requests        int64    `json:"requests"`
	TPS             float64  `json:"tps"`
	AvgLatencyMs    float64  `json:"avg_latency_ms"`
	AvgFirstTokenMs *float64 `json:"avg_first_token_ms"`
	SuccessRate     float64  `json:"success_rate"`
}

type modelPerformancePayload struct {
	Model       string                  `json:"model"`
	WindowHours int                     `json:"window_hours"`
	Groups      []modelPerformanceGroup `json:"groups"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// modelPerformance serves public per-group performance (requests/second,
// average latency, success rate) for a single model over the last 24 hours.
// It is rate-limited like /rankings and memoised so the aggregate query is not
// re-run on every panel open.
func (s *Service) modelPerformance(w http.ResponseWriter, r *http.Request) {
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	if model == "" {
		writeError(w, 400, "invalid_request", "model is required")
		return
	}
	accessible, err := s.modelPerformanceAccessible(r.Context(), model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not verify model performance")
		return
	}
	if !accessible {
		writeError(w, http.StatusNotFound, "not_found", "model performance not found")
		return
	}
	payload, err := s.performanceCache.get(r.Context(), model, func(ctx context.Context) (modelPerformancePayload, error) {
		return s.computeModelPerformance(ctx, model)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load model performance")
		return
	}
	writeJSON(w, 200, payload)
}

func (s *Service) modelPerformanceAccessible(ctx context.Context, model string) (bool, error) {
	var accessible bool
	err := s.db.QueryRow(ctx, `select exists(
		select 1
		from channels c
		left join channel_groups cg on cg.channel_id=c.id
		left join groups g on g.id=cg.group_id
		where c.enabled and not c.auto_disabled and c.user_id is null
			and (g.id is null or g."public")
			and exists(
				select 1
				from jsonb_array_elements_text(c.models) as item(model)
				where trim(item.model)=$1
			)
		union all
		select 1
		from model_routes m
		join channels c on c.id=m.channel_id
		left join channel_groups cg on cg.channel_id=c.id
		left join groups g on g.id=cg.group_id
		where c.enabled and not c.auto_disabled and c.user_id is null
			and m.enabled and not m.hidden and trim(m.public_model)=$1
			and (g.id is null or g."public")
	)`, model).Scan(&accessible)
	return accessible, err
}

func (s *Service) computeModelPerformance(ctx context.Context, model string) (modelPerformancePayload, error) {
	start := time.Now().UTC().Add(-performanceWindow)
	rows, err := s.db.Query(ctx, `
		with visible_channels as (
			select c.id
			from channels c
			where c.enabled and not c.auto_disabled and c.user_id is null
		), visible_groups as (
			select g.id
			from groups g
			where g."public"
		)
			select coalesce(rl.group_id::text, '__public'),
				coalesce(g.display_name, g.name, '公共'),
				count(*),
			count(*) filter (where rl.status_code >= 200 and rl.status_code < 400),
			coalesce(avg(rl.duration_ms), 0),
			avg(rl.first_token_ms)
		from request_logs rl
		join visible_channels vc on vc.id=rl.channel_id
		left join groups g on g.id = rl.group_id
		where trim(rl.model) = $1 and rl.created_at >= $2
			and (rl.group_id is null or exists(select 1 from visible_groups vg where vg.id=rl.group_id))
			group by rl.group_id, g.id, g.display_name, g.name
			order by coalesce(g.display_name, g.name, '公共')`, model, start)
	if err != nil {
		return modelPerformancePayload{}, err
	}
	defer rows.Close()
	groups := []modelPerformanceGroup{}
	for rows.Next() {
		var groupID, groupName string
		var requests, success int64
		var avgLatency float64
		var avgFirstTokenMs *float64
		if err := rows.Scan(&groupID, &groupName, &requests, &success, &avgLatency, &avgFirstTokenMs); err != nil {
			return modelPerformancePayload{}, err
		}
		successRate := 0.0
		if requests > 0 {
			successRate = float64(success) / float64(requests)
		}
		groups = append(groups, modelPerformanceGroup{
			GroupID:         groupID,
			GroupName:       groupName,
			Requests:        requests,
			TPS:             float64(requests) / performanceWindow.Seconds(),
			AvgLatencyMs:    avgLatency,
			AvgFirstTokenMs: avgFirstTokenMs,
			SuccessRate:     successRate,
		})
	}
	if rows.Err() != nil {
		return modelPerformancePayload{}, rows.Err()
	}
	return modelPerformancePayload{
		Model:       model,
		WindowHours: int(performanceWindow.Hours()),
		Groups:      groups,
		UpdatedAt:   time.Now().UTC(),
	}, nil
}
