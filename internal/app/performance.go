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
	payload, err := s.performanceCache.get(r.Context(), model, func(ctx context.Context) (modelPerformancePayload, error) {
		return s.computeModelPerformance(ctx, model)
	})
	if err != nil {
		writeError(w, 500, "internal_error", "could not load model performance")
		return
	}
	writeJSON(w, 200, payload)
}

func (s *Service) computeModelPerformance(ctx context.Context, model string) (modelPerformancePayload, error) {
	start := time.Now().UTC().Add(-performanceWindow)
	rows, err := s.db.Query(ctx, `
		select coalesce(rl.group_id::text, '__public'),
			coalesce(g.name, '公共'),
			count(*),
			count(*) filter (where rl.status_code >= 200 and rl.status_code < 400),
			coalesce(avg(rl.duration_ms), 0),
			avg(rl.first_token_ms)
		from request_logs rl
		left join groups g on g.id = rl.group_id
		where rl.model = $1 and rl.created_at >= $2
			and (rl.group_id is null or g."public")
		group by rl.group_id, g.name
		order by g.name`, model, start)
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
		if rows.Scan(&groupID, &groupName, &requests, &success, &avgLatency, &avgFirstTokenMs) != nil {
			continue
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
